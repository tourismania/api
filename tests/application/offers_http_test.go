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

// stubOfferFlightRepo is a minimal repository.OfferFlightRepository test
// double. Most of these HTTP tests never send a "flights" key, so it is
// usually just wired to satisfy the handler constructors; the flights
// tests below assert on the recorded calls.
type stubOfferFlightRepo struct {
	replaceCalled   bool
	replacedOfferID int
	replacedFlights []entity.Flight
}

func (s *stubOfferFlightRepo) FindByOfferID(_ context.Context, _ int) ([]entity.Flight, error) {
	return nil, nil
}

func (s *stubOfferFlightRepo) ReplaceForOffer(_ context.Context, offerID int, flights []entity.Flight) error {
	s.replaceCalled = true
	s.replacedOfferID = offerID
	s.replacedFlights = flights
	return nil
}

// stubAirportRepo is a minimal repository.AirportRepository test double.
type stubAirportRepo struct{}

func (stubAirportRepo) Search(_ context.Context, _ repository.AirportFilter) (repository.AirportSearchResult, error) {
	return repository.AirportSearchResult{}, nil
}

func (stubAirportRepo) Upsert(_ context.Context, _ string, _ *string, _ string, _, _ float64, _ *int, _ int) error {
	return nil
}

// FindByICAOs reports every requested icao as existing — these HTTP
// tests care about the request/response contract, not airport
// reference-data validation (covered at the domain/integration level).
func (stubAirportRepo) FindByICAOs(_ context.Context, icaos []string) ([]entity.Airport, error) {
	airports := make([]entity.Airport, 0, len(icaos))
	for _, icao := range icaos {
		airports = append(airports, entity.Airport{ICAO: icao})
	}
	return airports, nil
}

// noopTxManager is a hand-written test double for txmanager.TxManager
// that runs fn directly against the given context.
type noopTxManager struct{}

func (noopTxManager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	return fn(ctx)
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
// (JWT + CurrentUserUUID) for offer reads/writes. There is no middleware
// resolving the principal's mutable profile or gating by role — both
// the ownership check and the write-role gate live in the domain
// OfferManager, reached through the real application command/query
// handlers wired here.
func newOffersTestRouter(jwtSvc *auth.Service, offers *stubOfferRepo, users stubUserFinder, publicUC getpublishedoffer.UseCase, flights *stubOfferFlightRepo) http.Handler {
	validate := validator.New(validator.WithRequiredStructEnabled())

	offerManager := service.NewOfferManager(offers, stubAgencyRepo{})
	offerFlightManager := service.NewOfferFlightManager(flights, stubAirportRepo{})
	userFinder := service.NewUserFinder(users)

	createApp := createoffer.NewHandler(offerManager, offerFlightManager, userFinder, noopTxManager{})
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
	r := newOffersTestRouter(jwtSvc, offers, stubUserFinder{}, &stubGetPublishedOfferUseCase{}, &stubOfferFlightRepo{})

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
	r := newOffersTestRouter(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{}, &stubOfferFlightRepo{})

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
	r := newOffersTestRouter(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{}, &stubOfferFlightRepo{})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/offers",
		strings.NewReader(`{"title":"Test","description":"d","status":"draft"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusCreated, rr.Code)
}

func TestOffersHTTP_CreateOffer_WithFlights_Returns201_AndReplacesFlights(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	userUUID := uuid.New()
	token, err := jwtSvc.Issue(userUUID)
	require.NoError(t, err)

	users := stubUserFinder{record: &entity.UserRecord{
		ID: 1, Uuid: userUUID, Roles: []string{string(enum.RoleAgent)}, AgencyID: 3,
	}}
	offers := &stubOfferRepo{storeID: 9}
	flights := &stubOfferFlightRepo{}
	r := newOffersTestRouter(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{}, flights)

	body := `{"title":"Test","description":"d","status":"draft","flights":[{"segments":[
		{"departure_airport_icao":"UUEE","arrival_airport_icao":"LFPG","departure_at":"2026-08-01T10:00:00Z","arrival_at":"2026-08-01T14:00:00Z"}
	]}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/offers", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusCreated, rr.Code, rr.Body.String())
	assert.True(t, flights.replaceCalled)
	assert.Equal(t, 9, flights.replacedOfferID)
	require.Len(t, flights.replacedFlights, 1)
	assert.Equal(t, "UUEE", flights.replacedFlights[0].DepartureAirportICAO())
	assert.Equal(t, "LFPG", flights.replacedFlights[0].ArrivalAirportICAO())
}

func TestOffersHTTP_CreateOffer_InvalidFlightSegments_Returns400_NeverStores(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	userUUID := uuid.New()
	token, err := jwtSvc.Issue(userUUID)
	require.NoError(t, err)

	users := stubUserFinder{record: &entity.UserRecord{
		ID: 1, Uuid: userUUID, Roles: []string{string(enum.RoleAgent)}, AgencyID: 3,
	}}
	offers := &stubOfferRepo{storeID: 9}
	flights := &stubOfferFlightRepo{}
	r := newOffersTestRouter(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{}, flights)

	// arrival_at before departure_at violates the domain chronology
	// invariant, surfaced as a 400 by apperror.ErrValidation.
	body := `{"title":"Test","description":"d","status":"draft","flights":[{"segments":[
		{"departure_airport_icao":"UUEE","arrival_airport_icao":"LFPG","departure_at":"2026-08-01T14:00:00Z","arrival_at":"2026-08-01T10:00:00Z"}
	]}]}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/offers", strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	assert.False(t, flights.replaceCalled)
}

func TestOffersHTTP_ListOffers_NoToken_Returns401(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	offers := &stubOfferRepo{}
	r := newOffersTestRouter(jwtSvc, offers, stubUserFinder{}, &stubGetPublishedOfferUseCase{}, &stubOfferFlightRepo{})

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
	r := newOffersTestRouter(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{}, &stubOfferFlightRepo{})

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
	r := newOffersTestRouter(jwtSvc, offers, users, &stubGetPublishedOfferUseCase{}, &stubOfferFlightRepo{})

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
	r := newOffersTestRouter(jwtSvc, &stubOfferRepo{}, stubUserFinder{}, publicUC, &stubOfferFlightRepo{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/offers/"+id.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code, "a published offer must be readable without any Authorization header")
}

func TestOffersHTTP_GetPublicOffer_NotPublished_Returns404(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	id := uuid.New()
	publicUC := &stubGetPublishedOfferUseCase{err: service.ErrOfferNotFound}
	r := newOffersTestRouter(jwtSvc, &stubOfferRepo{}, stubUserFinder{}, publicUC, &stubOfferFlightRepo{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/offers/"+id.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestOffersHTTP_GetPublicOffer_WithFlights_IncludesComputedDurations(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	id := uuid.New()
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	publicUC := &stubGetPublishedOfferUseCase{result: getpublishedoffer.Result{
		ID: 1, UUID: id, AgencyID: 5,
		Flights: []entity.Flight{{
			ID: 1,
			Segments: []entity.FlightSegment{
				{DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
				{DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(3 * time.Hour), ArrivalAt: base.Add(6 * time.Hour)},
			},
		}},
	}}
	r := newOffersTestRouter(jwtSvc, &stubOfferRepo{}, stubUserFinder{}, publicUC, &stubOfferFlightRepo{})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/public/offers/"+id.String(), nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	body := rr.Body.String()
	assert.Contains(t, body, `"departure_airport_icao":"UUEE"`)
	assert.Contains(t, body, `"arrival_airport_icao":"LFPG"`)
	assert.Contains(t, body, `"total_duration_seconds":21600`)
	assert.Contains(t, body, `"airport_icao":"UUDD"`)
	assert.Contains(t, body, `"duration_seconds":3600`)
}
