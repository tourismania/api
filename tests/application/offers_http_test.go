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
	"api/internal/domain/repository"
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

// stubUserFinder implements the domain repository.UserRepository port for
// tests — role and agency_id are resolved by domain/service.UserFinder,
// not by any presentation-layer middleware, so these HTTP tests wire the
// real application handlers and drive the whole resolve → authorize
// flow. Store is a no-op: these tests only exercise the read path.
type stubUserFinder struct {
	record *entity.UserRecord
}

func (s stubUserFinder) FindByUuid(_ context.Context, _ uuid.UUID) (*entity.UserRecord, error) {
	return s.record, nil
}

func (s stubUserFinder) Store(_ context.Context, _ entity.User, _ string) (*int, error) {
	return nil, nil
}

// stubOfferRepo is a minimal repository.OfferRepository test double. Its
// List method also satisfies getoffers.OfferLister (identical
// signature), so the same instance backs both the domain OfferManager
// (write side) and the list query in these full-stack tests.
type stubOfferRepo struct {
	storeID   int
	gotFilter repository.OfferFilter
}

func (s *stubOfferRepo) Store(_ context.Context, _ entity.Offer) (int, error) {
	return s.storeID, nil
}

func (s *stubOfferRepo) FindByUUID(_ context.Context, _ uuid.UUID) (*entity.Offer, error) {
	return nil, nil
}

func (s *stubOfferRepo) List(_ context.Context, f repository.OfferFilter) (repository.OfferListResult, error) {
	s.gotFilter = f
	return repository.OfferListResult{}, nil
}

func (s *stubOfferRepo) Update(_ context.Context, _ entity.Offer) error { return nil }

func (s *stubOfferRepo) SoftDelete(_ context.Context, _ uuid.UUID) error { return nil }

// stubAgencyRepo is a minimal repository.AgencyRepository test double —
// every agency looked up is reported active.
type stubAgencyRepo struct{}

func (s stubAgencyRepo) Store(_ context.Context, _ entity.Agency) (int, error) { return 0, nil }

func (s stubAgencyRepo) FindByID(_ context.Context, id int) (*entity.Agency, error) {
	return &entity.Agency{ID: id, Status: enum.AgencyStatusActive}, nil
}

func (s stubAgencyRepo) SetStatus(_ context.Context, _ int, _ enum.AgencyStatus) error { return nil }

func (s stubAgencyRepo) Exists(_ context.Context, _ int) (bool, error) { return true, nil }

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
// (JWT + CurrentUserUUID) for offer reads/writes. There is no middleware
// resolving the principal's mutable profile or gating by role — both
// the ownership check and the write-role gate live in the domain
// OfferManager, reached through the real application command/query
// handlers wired here.
func newOffersTestRouter(jwtSvc *auth.Service, offers *stubOfferRepo, users stubUserFinder, publicUC getpublishedoffer.UseCase) http.Handler {
	validate := validator.New(validator.WithRequiredStructEnabled())

	offerManager := service.NewOfferManager(offers, stubAgencyRepo{})
	userFinder := service.NewUserFinder(users)

	createApp := createoffer.NewHandler(offerManager, userFinder)
	createH := createofferhttp.NewHandler(createApp, validate)

	listApp := getoffers.NewHandler(offers, userFinder)
	listH := listoffershttp.NewHandler(listApp, validate)

	publicH := getpublicofferhttp.NewHandler(publicUC)

	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		api.Get("/public/offers/{uuid}", publicH.Handle)

		api.Group(func(priv chi.Router) {
			priv.Use(custommw.JWT(jwtSvc))
			priv.Use(custommw.CurrentUserUUID)

			priv.Get("/offers", listH.Handle)
			priv.Post("/offers", createH.Handle)
		})
	})
	return r
}

func TestOffersHTTP_CreateOffer_NoToken_Returns401(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	offers := &stubOfferRepo{}
	r := newOffersTestRouter(jwtSvc, offers, stubUserFinder{}, &stubGetPublishedOfferUseCase{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/offers", strings.NewReader(`{}`))
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestOffersHTTP_CreateOffer_RoleUser_Returns403(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	userUUID := uuid.New()
	token, err := jwtSvc.Issue(userUUID)
	require.NoError(t, err)

	users := stubUserFinder{record: &entity.UserRecord{
		ID: 1, Uuid: userUUID, Roles: []string{string(enum.RoleUser)}, AgencyID: 1,
	}}
	offers := &stubOfferRepo{}
	r := newOffersTestRouter(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/offers",
		strings.NewReader(`{"title":"Test","description":"d","status":"draft"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusForbidden, rr.Code, "the domain OfferManager must reject the write before persisting anything")
}

func TestOffersHTTP_CreateOffer_RoleAgent_Returns201(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	userUUID := uuid.New()
	token, err := jwtSvc.Issue(userUUID)
	require.NoError(t, err)

	users := stubUserFinder{record: &entity.UserRecord{
		ID: 1, Uuid: userUUID, Roles: []string{string(enum.RoleAgent)}, AgencyID: 3,
	}}
	offers := &stubOfferRepo{storeID: 1}
	r := newOffersTestRouter(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/offers",
		strings.NewReader(`{"title":"Test","description":"d","status":"draft"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestOffersHTTP_ListOffers_NoToken_Returns401(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	offers := &stubOfferRepo{}
	r := newOffersTestRouter(jwtSvc, offers, stubUserFinder{}, &stubGetPublishedOfferUseCase{})

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
	offers := &stubOfferRepo{}
	r := newOffersTestRouter(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/offers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, offers.gotFilter.AgencyID)
	assert.Equal(t, 2, *offers.gotFilter.AgencyID)
}

func TestOffersHTTP_ListOffers_RoleAgent_ScopesToOwnAgency(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	userUUID := uuid.New()
	token, err := jwtSvc.Issue(userUUID)
	require.NoError(t, err)

	users := stubUserFinder{record: &entity.UserRecord{
		ID: 1, Uuid: userUUID, Roles: []string{string(enum.RoleAgent)}, AgencyID: 7,
	}}
	offers := &stubOfferRepo{}
	r := newOffersTestRouter(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/offers", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, offers.gotFilter.AgencyID)
	assert.Equal(t, 7, *offers.gotFilter.AgencyID)
}

func TestOffersHTTP_GetPublicOffer_Published_Returns200NoAuth(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	id := uuid.New()
	publicUC := &stubGetPublishedOfferUseCase{result: getpublishedoffer.Result{ID: 1, UUID: id, AgencyID: 5}}
	r := newOffersTestRouter(jwtSvc, &stubOfferRepo{}, stubUserFinder{}, publicUC)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/offers/"+id.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "a published offer must be readable without any Authorization header")
}

func TestOffersHTTP_GetPublicOffer_NotPublished_Returns404(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	id := uuid.New()
	publicUC := &stubGetPublishedOfferUseCase{err: service.ErrOfferNotFound}
	r := newOffersTestRouter(jwtSvc, &stubOfferRepo{}, stubUserFinder{}, publicUC)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/offers/"+id.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}
