package application_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	searchairports "api/internal/application/query/search_airports"
	searchairporthttp "api/internal/presentation/http/api/v1/airport/search"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeUseCase satisfies searchairports.UseCase using pre-set data.
type fakeUseCase struct {
	result searchairports.Result
	err    error
}

func (f *fakeUseCase) Handle(_ context.Context, _ searchairports.Query) (searchairports.Result, error) {
	return f.result, f.err
}

// newTestRouter wires a real Handler against a stub use-case.
func newTestRouter(uc searchairports.UseCase) http.Handler {
	v := validator.New(validator.WithRequiredStructEnabled())
	h := searchairporthttp.NewHandler(uc, v)
	r := chi.NewRouter()
	r.Get("/api/v1/airports", h.Handle)
	return r
}

// moscowResults returns a realistic stub result for Moscow queries.
func moscowResults() searchairports.Result {
	iata := func(s string) *string { return &s }
	return searchairports.Result{
		TotalCount: 4,
		Airports: []searchairports.AirportResult{
			{ICAO: "UUEE", IATA: iata("SVO"), Name: "Sheremetyevo International Airport"},
			{ICAO: "UUDD", IATA: iata("DME"), Name: "Domodedovo International Airport"},
			{ICAO: "UUWW", IATA: iata("VKO"), Name: "Vnukovo International Airport"},
			{ICAO: "UUBW", IATA: iata("ZIA"), Name: "Zhukovsky International Airport"},
		},
	}
}

func TestSearchAirports_Moscow(t *testing.T) {
	uc := &fakeUseCase{result: moscowResults()}
	r := newTestRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports?search=Moscow", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var resp searchairporthttp.SearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.GreaterOrEqual(t, len(resp.Data), 1)
}

func TestSearchAirports_IATA_SVO(t *testing.T) {
	iata := "SVO"
	uc := &fakeUseCase{result: searchairports.Result{
		TotalCount: 1,
		Airports:   []searchairports.AirportResult{{ICAO: "UUEE", IATA: &iata, Name: "Sheremetyevo"}},
	}}
	r := newTestRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports?search=SVO", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp searchairporthttp.SearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.NotEmpty(t, resp.Data)
	require.NotNil(t, resp.Data[0].IATA)
	assert.Equal(t, "SVO", *resp.Data[0].IATA)
}

func TestSearchAirports_ICAO_UUEE(t *testing.T) {
	iata := "SVO"
	uc := &fakeUseCase{result: searchairports.Result{
		TotalCount: 1,
		Airports:   []searchairports.AirportResult{{ICAO: "UUEE", IATA: &iata, Name: "Sheremetyevo"}},
	}}
	r := newTestRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports?search=UUEE", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp searchairporthttp.SearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	require.NotEmpty(t, resp.Data)
	assert.Equal(t, "UUEE", resp.Data[0].ICAO)
}

func TestSearchAirports_TooShort(t *testing.T) {
	uc := &fakeUseCase{}
	r := newTestRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports?search=a", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)

	var body map[string]map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "INVALID_SEARCH", body["error"]["code"])
}

func TestSearchAirports_NoSearch(t *testing.T) {
	uc := &fakeUseCase{}
	r := newTestRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusBadRequest, rr.Code)
	var body map[string]map[string]string
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&body))
	assert.Equal(t, "INVALID_SEARCH", body["error"]["code"])
}

func TestSearchAirports_Pagination(t *testing.T) {
	uc := &fakeUseCase{result: searchairports.Result{
		TotalCount: 4,
		Airports: []searchairports.AirportResult{
			{ICAO: "UUDD", Name: "Domodedovo"},
			{ICAO: "UUWW", Name: "Vnukovo"},
		},
	}}
	r := newTestRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports?search=Moscow&limit=2&offset=2", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var resp searchairporthttp.SearchResponse
	require.NoError(t, json.NewDecoder(rr.Body).Decode(&resp))
	assert.LessOrEqual(t, len(resp.Data), 2)
	assert.Equal(t, int64(4), resp.Meta.Total)
}

func TestSearchAirports_CacheControlHeader(t *testing.T) {
	uc := &fakeUseCase{result: moscowResults()}
	r := newTestRouter(uc)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/airports?search=Moscow", nil)
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, req)

	assert.Equal(t, "private, max-age=3600", rr.Header().Get("Cache-Control"))
}
