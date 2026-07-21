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
	getpublishedoffer "api/internal/application/query/get_published_offer"
	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/service"
	"api/internal/infrastructure/auth"
	createofferhttp "api/internal/presentation/http/api/v1/offer/create"
	listoffershttp "api/internal/presentation/http/api/v1/offer/get_list"
	getpublicofferhttp "api/internal/presentation/http/api/v1/offer/get_public"
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

// stubGetPublishedOfferUseCase implements getpublishedoffer.UseCase.
type stubGetPublishedOfferUseCase struct {
	result getpublishedoffer.Result
	err    error
}

func (s *stubGetPublishedOfferUseCase) Handle(_ context.Context, _ getpublishedoffer.Query) (getpublishedoffer.Result, error) {
	return s.result, s.err
}

// newOffersTestRouter mirrors the production router split: a fully
// anonymous public endpoint for published offers, and a private group
// (JWT + CurrentUser) for offer reads/writes, with writes additionally
// gated by RequireRole.
func newOffersTestRouter(jwtSvc *auth.Service, users custommw.UserFinder, createUC createoffer.UseCase, listUC getoffers.UseCase, publicUC getpublishedoffer.UseCase) http.Handler {
	validate := validator.New(validator.WithRequiredStructEnabled())
	createH := createofferhttp.NewHandler(createUC, validate)
	listH := listoffershttp.NewHandler(listUC, validate)
	publicH := getpublicofferhttp.NewHandler(publicUC)

	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/public/offers/{uuid}", publicH.Handle)

		api.Group(func(priv chi.Router) {
			priv.Use(custommw.JWT(jwtSvc))
			priv.Use(custommw.CurrentUserMiddleware(users))

			priv.Get("/offers", listH.Handle)

			agentOrAdmin := custommw.RequireRole(enum.RoleAgent, enum.RoleSuperAdmin)
			priv.With(agentOrAdmin).Post("/offers", createH.Handle)
		})
	})
	return r
}

func TestOffersHTTP_CreateOffer_NoToken_Returns401(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	uc := &stubCreateOfferUseCase{}
	r := newOffersTestRouter(jwtSvc, stubUserFinder{}, uc, &stubGetOffersUseCase{}, &stubGetPublishedOfferUseCase{})

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
	r := newOffersTestRouter(jwtSvc, users, uc, &stubGetOffersUseCase{}, &stubGetPublishedOfferUseCase{})

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
	r := newOffersTestRouter(jwtSvc, users, uc, &stubGetOffersUseCase{}, &stubGetPublishedOfferUseCase{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/offers",
		strings.NewReader(`{"title":"Test","description":"d","status":"draft"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
	assert.True(t, uc.called)
}

func TestOffersHTTP_ListOffers_NoToken_Returns401(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	listUC := &stubGetOffersUseCase{}
	r := newOffersTestRouter(jwtSvc, stubUserFinder{}, &stubCreateOfferUseCase{}, listUC, &stubGetPublishedOfferUseCase{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/offers", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code, "the offer list is a private endpoint — no JWT means no access")
}

func TestOffersHTTP_ListOffers_RoleUser_ScopesToOwnAgency(t *testing.T) {
	// ROLE_USER is read-only but not visibility-restricted: it sees the
	// same set of offers (any status) as agency staff, within its own
	// agency.
	jwtSvc := newTestJWTService(t)
	userUUID := uuid.New()
	token, err := jwtSvc.Issue(userUUID)
	require.NoError(t, err)

	users := stubUserFinder{record: &entity.UserRecord{
		ID: 1, Uuid: userUUID, Roles: []string{string(enum.RoleUser)}, AgencyID: 2,
	}}
	listUC := &stubGetOffersUseCase{}
	r := newOffersTestRouter(jwtSvc, users, &stubCreateOfferUseCase{}, listUC, &stubGetPublishedOfferUseCase{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/offers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 2, listUC.gotQuery.AgencyID)
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
	r := newOffersTestRouter(jwtSvc, users, &stubCreateOfferUseCase{}, listUC, &stubGetPublishedOfferUseCase{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/offers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, 7, listUC.gotQuery.AgencyID)
}

func TestOffersHTTP_GetPublicOffer_Published_Returns200NoAuth(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	id := uuid.New()
	publicUC := &stubGetPublishedOfferUseCase{result: getpublishedoffer.Result{ID: 1, UUID: id, AgencyID: 5}}
	r := newOffersTestRouter(jwtSvc, stubUserFinder{}, &stubCreateOfferUseCase{}, &stubGetOffersUseCase{}, publicUC)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/offers/"+id.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "a published offer must be readable without any Authorization header")
}

func TestOffersHTTP_GetPublicOffer_NotPublished_Returns404(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	id := uuid.New()
	publicUC := &stubGetPublishedOfferUseCase{err: service.ErrOfferNotFound}
	r := newOffersTestRouter(jwtSvc, stubUserFinder{}, &stubCreateOfferUseCase{}, &stubGetOffersUseCase{}, publicUC)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/offers/"+id.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}
