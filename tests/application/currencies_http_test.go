package application_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	listcurrencies "api/internal/application/query/list_currencies"
	"api/internal/infrastructure/auth"
	listcurrencieshttp "api/internal/presentation/http/api/v1/currency/list"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newCurrenciesTestRouter повторяет продовую раскладку: /currencies — в
// группе с JWT и CurrentUserUUID.
func newCurrenciesTestRouter(jwtSvc *auth.Service) http.Handler {
	h := listcurrencieshttp.NewHandler(listcurrencies.NewHandler())

	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		api.Group(func(priv chi.Router) {
			priv.Use(custommw.JWT(jwtSvc))
			priv.Use(custommw.CurrentUserUUID)
			priv.Get("/currencies", h.Handle)
		})
	})
	return r
}

func TestCurrenciesHTTP_Authenticated_ReturnsDirectory(t *testing.T) {
	jwtSvc := newTestJWTService(t)
	token, err := jwtSvc.Issue(uuid.New())
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/currencies", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	newCurrenciesTestRouter(jwtSvc).ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.JSONEq(t, `{"data":[
		{"code":"RUB","exponent":2},
		{"code":"USD","exponent":2},
		{"code":"EUR","exponent":2},
		{"code":"TRY","exponent":2}
	]}`, rr.Body.String())
	assert.Equal(t, "private, max-age=3600", rr.Header().Get("Cache-Control"))
}

func TestCurrenciesHTTP_NoToken_Returns401(t *testing.T) {
	jwtSvc := newTestJWTService(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/currencies", nil)
	rr := httptest.NewRecorder()
	newCurrenciesTestRouter(jwtSvc).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}
