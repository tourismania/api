package application_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	createoffer "api/internal/application/command/create_offer"
	getoffers "api/internal/application/query/get_offers"
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/infrastructure/auth"
	createofferhttp "api/internal/presentation/http/api/v1/offer/create"
	listoffershttp "api/internal/presentation/http/api/v1/offer/get_list"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestJWTService generates a throwaway RSA keypair and returns a fully
// working auth.Service, so these tests exercise the real Issue/Verify
// path without touching config/jwt/*.pem (which are gitignored, real
// deployment keys).
func newTestJWTService(t *testing.T) *auth.Service {
	t.Helper()
	dir := t.TempDir()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	privPath := filepath.Join(dir, "private.pem")
	privPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	require.NoError(t, os.WriteFile(privPath, privPEM, 0o600))

	pubBytes, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	require.NoError(t, err)
	pubPath := filepath.Join(dir, "public.pem")
	pubPEM := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	require.NoError(t, os.WriteFile(pubPath, pubPEM, 0o600))

	svc, err := auth.NewService(privPath, pubPath, "", time.Hour)
	require.NoError(t, err)
	return svc
}

// stubUserFinder implements custommw.UserFinder for tests.
type stubUserFinder struct {
	record *entity.UserRecord
}

func (s stubUserFinder) FindByUuid(_ context.Context, _ uuid.UUID) (*entity.UserRecord, error) {
	return s.record, nil
}

// stubCreateOfferUseCase implements createoffer.UseCase.
type stubCreateOfferUseCase struct {
	called bool
}

func (s *stubCreateOfferUseCase) Handle(_ context.Context, _ createoffer.Command) (createoffer.Result, error) {
	s.called = true
	return createoffer.Result{ID: 1, UUID: uuid.New()}, nil
}

// stubGetOffersUseCase implements getoffers.UseCase.
type stubGetOffersUseCase struct {
	gotQuery getoffers.Query
}

func (s *stubGetOffersUseCase) Handle(_ context.Context, q getoffers.Query) (getoffers.Result, error) {
	s.gotQuery = q
	return getoffers.Result{}, nil
}

// newOffersTestRouter mirrors the production router split: offer reads
// are public (optional auth), offer writes require JWT + a role guard.
func newOffersTestRouter(jwtSvc *auth.Service, users custommw.UserFinder, createUC createoffer.UseCase, listUC getoffers.UseCase) http.Handler {
	validate := validator.New(validator.WithRequiredStructEnabled())
	createH := createofferhttp.NewHandler(createUC, validate)
	listH := listoffershttp.NewHandler(listUC, validate)

	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		api.Group(func(pub chi.Router) {
			pub.Use(custommw.OptionalJWT(jwtSvc))
			pub.Use(custommw.OptionalCurrentUser(users))
			pub.Get("/offers", listH.Handle)
		})

		api.Group(func(priv chi.Router) {
			priv.Use(custommw.JWT(jwtSvc))
			priv.Use(custommw.CurrentUserMiddleware(users))

			agentOrAdmin := custommw.RequireRole(enum.RoleAgent, enum.RoleSuperAdmin)
			priv.With(agentOrAdmin).Post("/offers", createH.Handle)
		})
	})
	return r
}

func TestOffersHTTP_CreateOffer_NoToken_Returns401(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	uc := &stubCreateOfferUseCase{}
	r := newOffersTestRouter(jwtSvc, stubUserFinder{}, uc, &stubGetOffersUseCase{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/offers", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	assert.False(t, uc.called)
}

func TestOffersHTTP_CreateOffer_RoleUser_Returns403(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	userUUID := uuid.New()
	token, err := jwtSvc.Issue(userUUID)
	require.NoError(t, err)

	users := stubUserFinder{record: &entity.UserRecord{
		ID: 1, Uuid: userUUID, Roles: []string{string(enum.RoleUser)}, AgencyID: 1,
	}}
	uc := &stubCreateOfferUseCase{}
	r := newOffersTestRouter(jwtSvc, users, uc, &stubGetOffersUseCase{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/offers",
		strings.NewReader(`{"title":"Test","description":"d","status":"draft"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code)
	assert.False(t, uc.called, "the use-case must never run when the role guard rejects the request")
}

func TestOffersHTTP_CreateOffer_RoleAgent_Returns201(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	userUUID := uuid.New()
	token, err := jwtSvc.Issue(userUUID)
	require.NoError(t, err)

	users := stubUserFinder{record: &entity.UserRecord{
		ID: 1, Uuid: userUUID, Roles: []string{string(enum.RoleAgent)}, AgencyID: 3,
	}}
	uc := &stubCreateOfferUseCase{}
	r := newOffersTestRouter(jwtSvc, users, uc, &stubGetOffersUseCase{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/offers",
		strings.NewReader(`{"title":"Test","description":"d","status":"draft"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.True(t, uc.called)
}

func TestOffersHTTP_ListOffers_Anonymous_Returns200AndForcesPublishedFilter(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	listUC := &stubGetOffersUseCase{}
	r := newOffersTestRouter(jwtSvc, stubUserFinder{}, &stubCreateOfferUseCase{}, listUC)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/offers", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "published offers must be listable without any Authorization header")
	assert.Nil(t, listUC.gotQuery.CurrentAgencyID)
}

func TestOffersHTTP_ListOffers_RoleUser_ForcesPublishedFilter(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	userUUID := uuid.New()
	token, err := jwtSvc.Issue(userUUID)
	require.NoError(t, err)

	users := stubUserFinder{record: &entity.UserRecord{
		ID: 1, Uuid: userUUID, Roles: []string{string(enum.RoleUser)}, AgencyID: 2,
	}}
	listUC := &stubGetOffersUseCase{}
	r := newOffersTestRouter(jwtSvc, users, &stubCreateOfferUseCase{}, listUC)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/offers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Nil(t, listUC.gotQuery.CurrentAgencyID, "a plain ROLE_USER client is not agency staff — must not get elevated visibility just because they have an agency_id")
}

func TestOffersHTTP_ListOffers_RoleAgent_ScopesToOwnAgency(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	userUUID := uuid.New()
	token, err := jwtSvc.Issue(userUUID)
	require.NoError(t, err)

	users := stubUserFinder{record: &entity.UserRecord{
		ID: 1, Uuid: userUUID, Roles: []string{string(enum.RoleAgent)}, AgencyID: 7,
	}}
	listUC := &stubGetOffersUseCase{}
	r := newOffersTestRouter(jwtSvc, users, &stubCreateOfferUseCase{}, listUC)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/offers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, listUC.gotQuery.CurrentAgencyID)
	assert.Equal(t, 7, *listUC.gotQuery.CurrentAgencyID)
}
