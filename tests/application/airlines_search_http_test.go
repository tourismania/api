package application_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	getairline "api/internal/application/query/get_airline"
	searchairlines "api/internal/application/query/search_airlines"
	"api/internal/domain/airline"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"
	getairlinehttp "api/internal/presentation/http/api/v1/airline/get"
	searchairlinehttp "api/internal/presentation/http/api/v1/airline/search"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Сценарии travel-services/airlines на полном стеке (хендлер → use case →
// PostgreSQL). Нужен запущенный Postgres с миграциями (TEST_DATABASE_URL).
// Данные сида изолированы токеном в названиях и тестовой страной
// airlineHTTPCountryISO2; после теста удаляются.

// airlineHTTPCountryISO2 — тестовая страна (не ZZ/ZY других тестов).
const airlineHTTPCountryISO2 = "ZX"

// airlineHTTPFixture — id засеянных записей.
type airlineHTTPFixture struct {
	token       string
	countryName string
	// ru — «Rossiya <t>», русское «Россия <t>», страна ZX.
	ru int
	// foreign — «Foreign <t> Air», страна ZX, два IATA.
	foreign int
	// oldShared / newShared — закрывшаяся и действующая с кодом ZXA.
	oldShared, newShared int
	// dup — дубль newShared; gone — пропавшая.
	dup, gone int
	// alpha — «Alpha <t> Air» для проверки буквального «_».
	alpha int
}

type airlineHTTPEnv struct {
	router http.Handler
	token  string
	f      airlineHTTPFixture
}

func newAirlineHTTPEnv(t *testing.T) airlineHTTPEnv {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database-backed HTTP test")
	}
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	repo := pgrepo.NewAirlineRepository(db.New(pool))

	token := strings.ReplaceAll(strings.ReplaceAll(uuid.NewString()[:8], "-", ""), "0", "o")
	token = strings.Map(func(r rune) rune {
		if r >= '1' && r <= '9' {
			return 'g' + (r - '1')
		}
		return r
	}, token)
	f := airlineHTTPFixture{token: token, countryName: "Zxland" + token}
	require.NoError(t, pgrepo.NewCountryRepository(pool).Upsert(ctx, airlineHTTPCountryISO2, f.countryName))
	t.Cleanup(func() {
		_, err := pool.Exec(context.Background(), `DELETE FROM airlines WHERE source_id LIKE $1`, "http-"+token+"-%")
		assert.NoError(t, err)
	})

	country := &airline.Country{ISO2: airlineHTTPCountryISO2}
	insert := func(key string, a airline.Airline) int {
		id, err := repo.Insert(ctx, "http-"+token+"-"+key, a)
		require.NoError(t, err)
		return id
	}
	code := func(s string) *string { return &s }
	dissolved := time.Date(2015, 10, 26, 0, 0, 0, 0, time.UTC)
	f.ru = insert("ru", airline.Airline{ICAO: code("ZXR"), Name: "Rossiya " + token, NameRU: "Россия " + token, Country: country})
	f.foreign = insert("foreign", airline.Airline{ICAO: code("ZXF"), Name: "Foreign " + token + " Air", Country: country, IATACodes: []string{"X1", "X2"}})
	f.oldShared = insert("old", airline.Airline{ICAO: code("ZXA"), Name: "Shared " + token + " Old", DissolvedOn: &dissolved})
	f.newShared = insert("new", airline.Airline{ICAO: code("ZXA"), Name: "Shared " + token + " New"})
	f.dup = insert("dup", airline.Airline{ICAO: code("ZXA"), Name: "Shared " + token + " Dup"})
	f.gone = insert("gone", airline.Airline{ICAO: code("ZXG"), Name: "Shared " + token + " Gone"})
	f.alpha = insert("alpha", airline.Airline{ICAO: code("ZXL"), Name: "Alpha " + token + " Air"})
	require.NoError(t, repo.MarkMerged(ctx, f.dup, f.newShared))
	require.NoError(t, repo.MarkMissing(ctx, []int{f.gone}, time.Now()))

	jwtSvc := newTestJWTService(t)
	tok, err := jwtSvc.Issue(uuid.New())
	require.NoError(t, err)

	searchH := searchairlinehttp.NewHandler(searchairlines.NewHandler(repo), validator.New(validator.WithRequiredStructEnabled()))
	getH := getairlinehttp.NewHandler(getairline.NewHandler(repo))
	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		api.Group(func(priv chi.Router) {
			priv.Use(custommw.JWT(jwtSvc))
			priv.Get("/airlines", searchH.Handle)
			priv.Get("/airlines/{id}", getH.Handle)
		})
	})
	return airlineHTTPEnv{router: r, token: tok, f: f}
}

func (e airlineHTTPEnv) get(t *testing.T, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+e.token)
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

// airlineSearchBody — разобранный успешный ответ поиска.
type airlineSearchBody struct {
	Data []map[string]any `json:"data"`
	Meta map[string]any   `json:"meta"`
}

func (e airlineHTTPEnv) search(t *testing.T, query string) airlineSearchBody {
	t.Helper()
	rr := e.get(t, "/api/v1/airlines?"+query)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var body airlineSearchBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	return body
}

func ids(body airlineSearchBody) []int {
	out := make([]int, 0, len(body.Data))
	for _, a := range body.Data {
		out = append(out, int(a["id"].(float64)))
	}
	return out
}

func q(s string) string { return url.QueryEscape(s) }

// ---------------------------------------------------------------------------
// Служебная запись
// ---------------------------------------------------------------------------

func TestAirlinesHTTP_UnknownRecord_FoundByBothNames(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	for _, s := range []string{"неизв", "unknown"} {
		body := e.search(t, "search="+q(s))
		var found map[string]any
		for _, a := range body.Data {
			if a["id"] == float64(airline.UnknownID) {
				found = a
			}
		}
		require.NotNil(t, found, s)
		assert.Equal(t, map[string]any{
			"id": float64(1), "icao": nil, "name": "Unknown", "name_ru": "Неизвестная", "country": nil, "dissolved_on": nil,
		}, found)
	}
}

// ---------------------------------------------------------------------------
// Поиск по названиям, стране, кодам
// ---------------------------------------------------------------------------

func TestAirlinesHTTP_SearchByNamesAndCountry(t *testing.T) {
	e := newAirlineHTTPEnv(t)
	tok := e.f.token

	assert.Equal(t, []int{e.f.ru}, ids(e.search(t, "search="+q("россия "+tok))), "русское название")
	assert.Equal(t, []int{e.f.ru}, ids(e.search(t, "search="+q("ROSSIYA "+tok))), "английское название")
	assert.Equal(t, []int{e.f.foreign, e.f.ru}, ids(e.search(t, "search="+q(strings.ToLower(e.f.countryName)))), "название страны")
	assert.Equal(t, []int{e.f.foreign}, ids(e.search(t, "search="+q("air")+"&country=zx")), "отбор по стране без учёта регистра")
}

func TestAirlinesHTTP_CodesNotSearched(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	assert.Empty(t, e.search(t, "search=zxr&country="+airlineHTTPCountryISO2).Data, "по ICAO не ищется")
}

func TestAirlinesHTTP_SharedCode_BothActiveFirst_HiddenAbsent(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	body := e.search(t, "search="+q("shared "+e.f.token))

	assert.Equal(t, []int{e.f.newShared, e.f.oldShared}, ids(body), "дубль и пропавшая не выводятся, действующая выше")
	assert.Equal(t, "2015-10-26", body.Data[1]["dissolved_on"])
	assert.Equal(t, "ZXA", body.Data[0]["icao"])
}

func TestAirlinesHTTP_WildcardsLiteral(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	assert.Empty(t, e.search(t, "search="+q("al_ha "+e.f.token)).Data)
	assert.Equal(t, []int{e.f.alpha}, ids(e.search(t, "search="+q("alpha "+e.f.token))))
}

func TestAirlinesHTTP_ResponseFormat(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	rr := e.get(t, "/api/v1/airlines?search="+q("foreign "+e.f.token))

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "private, max-age=3600", rr.Header().Get("Cache-Control"))
	var body airlineSearchBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	assert.Equal(t, map[string]any{
		"id": float64(e.f.foreign), "icao": "ZXF", "name": "Foreign " + e.f.token + " Air", "name_ru": "",
		"country": map[string]any{"iso2": "ZX", "name": e.f.countryName}, "dissolved_on": nil,
	}, body.Data[0], "у иностранной name_ru пустой, кодов IATA нет")
	assert.NotContains(t, rr.Body.String(), "X1")
	assert.Equal(t, map[string]any{
		"total": float64(1), "limit": float64(20), "offset": float64(0),
		"search": "foreign " + e.f.token, "country": nil, "ids": nil,
	}, body.Meta)
}

func TestAirlinesHTTP_SearchNormalized(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	body := e.search(t, "search="+q("  Foreign   "+e.f.token+"  "))

	assert.Equal(t, "Foreign "+e.f.token, body.Meta["search"])
	assert.Equal(t, []int{e.f.foreign}, ids(body))
}

func TestAirlinesHTTP_NothingFound_EmptyData(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	rr := e.get(t, "/api/v1/airlines?search="+q("nothing "+e.f.token))

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), `"data":[]`)
	assert.Contains(t, rr.Body.String(), `"total":0`)
}

func TestAirlinesHTTP_Pagination(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	body := e.search(t, "search="+q("shared "+e.f.token)+"&limit=1&offset=1")
	assert.Equal(t, []int{e.f.oldShared}, ids(body))
	assert.Equal(t, float64(2), body.Meta["total"])

	for _, p := range []string{"limit=abc&offset=-5", ""} {
		body = e.search(t, "search="+q("shared "+e.f.token)+"&"+p)
		assert.Equal(t, float64(20), body.Meta["limit"], p)
		assert.Equal(t, float64(0), body.Meta["offset"], p)
	}
}

// ---------------------------------------------------------------------------
// Поиск по ids
// ---------------------------------------------------------------------------

func TestAirlinesHTTP_ByIDs(t *testing.T) {
	e := newAirlineHTTPEnv(t)
	f := e.f

	body := e.search(t, fmt.Sprintf("ids=%d,%d,%d", f.foreign, f.ru, f.foreign))
	assert.Equal(t, []int{f.foreign, f.ru}, ids(body), "повторы не дублируют выдачу")
	assert.Equal(t, []any{float64(f.foreign), float64(f.ru)}, body.Meta["ids"])
	assert.Nil(t, body.Meta["search"])
	assert.Equal(t, float64(2), body.Meta["total"])

	assert.Equal(t, []int{f.newShared}, ids(e.search(t, fmt.Sprintf("ids=%d,%d,%d", f.newShared, f.dup, f.gone))),
		"скрытые по ids не находятся")
	assert.Empty(t, e.search(t, "ids=999999999").Data)
}

func TestAirlinesHTTP_InvalidIDs_Returns400(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	for _, raw := range []string{"0", "-1", "abc", "42,,57"} {
		rr := e.get(t, "/api/v1/airlines?ids="+q(raw))
		require.Equal(t, http.StatusBadRequest, rr.Code, raw)
		assert.Equal(t, map[string]any{"IDs": "min=1"}, decodeError(t, rr).Meta, raw)
	}

	many := make([]string, 101)
	for i := range many {
		many[i] = fmt.Sprint(i + 1)
	}
	rr := e.get(t, "/api/v1/airlines?ids="+strings.Join(many, ","))
	require.Equal(t, http.StatusBadRequest, rr.Code)
	assert.Equal(t, map[string]any{"IDs": "max=100"}, decodeError(t, rr).Meta)
}

// ---------------------------------------------------------------------------
// Ошибки параметров и аутентификации
// ---------------------------------------------------------------------------

func TestAirlinesHTTP_InvalidSearch_Returns400InvalidSearch(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	for _, query := range []string{"search=SU", "search=%20%20", "", fmt.Sprintf("ids=%d&search=SU", e.f.ru)} {
		rr := e.get(t, "/api/v1/airlines?"+query)
		require.Equal(t, http.StatusBadRequest, rr.Code, query)
		var body struct {
			Error struct{ Code, Message, Field string } `json:"error"`
		}
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
		assert.Equal(t, "INVALID_SEARCH", body.Error.Code, query)
		assert.Equal(t, "search", body.Error.Field)
		assert.Equal(t, "Parameter 'search' must be at least 3 characters long", body.Error.Message)
	}
}

func TestAirlinesHTTP_InvalidCountryOrBounds_Returns400WithMeta(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	for query, want := range map[string]map[string]any{
		"search=air&country=TUR":  {"Country": "iso2"},
		"search=air&country=1":    {"Country": "iso2"},
		"search=air&limit=101":    {"Limit": "max=100"},
		"search=air&offset=10001": {"Offset": "max=10000"},
	} {
		rr := e.get(t, "/api/v1/airlines?"+query)
		require.Equal(t, http.StatusBadRequest, rr.Code, query)
		body := decodeError(t, rr)
		assert.Equal(t, "validation failed", body.Error, query)
		assert.Equal(t, want, body.Meta, query)
	}
}

func TestAirlinesHTTP_NoToken_Returns401(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	for _, path := range []string{"/api/v1/airlines?search=air", "/api/v1/airlines/1"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()
		e.router.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusUnauthorized, rr.Code, path)
	}
}

// ---------------------------------------------------------------------------
// GET /api/v1/airlines/{id}
// ---------------------------------------------------------------------------

func (e airlineHTTPEnv) getOne(t *testing.T, id string) (int, map[string]any) {
	t.Helper()
	rr := e.get(t, "/api/v1/airlines/"+id)
	var body struct {
		Data map[string]any `json:"data"`
	}
	if rr.Code == http.StatusOK {
		assert.Equal(t, "private, max-age=3600", rr.Header().Get("Cache-Control"))
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	}
	return rr.Code, body.Data
}

func TestAirlinesHTTP_GetByID_IncludingHidden(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	code, data := e.getOne(t, fmt.Sprint(e.f.ru))
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, false, data["hidden"])
	assert.Equal(t, "Россия "+e.f.token, data["name_ru"])

	code, data = e.getOne(t, fmt.Sprint(e.f.dup))
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, data["hidden"], "дубль отдаётся с hidden")
	assert.Equal(t, "Shared "+e.f.token+" Dup", data["name"])
	assert.Equal(t, "ZXA", data["icao"])

	code, data = e.getOne(t, fmt.Sprint(e.f.gone))
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, data["hidden"], "пропавшая отдаётся с hidden")

	code, data = e.getOne(t, "1")
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, map[string]any{
		"id": float64(1), "icao": nil, "name": "Unknown", "name_ru": "Неизвестная", "country": nil, "dissolved_on": nil, "hidden": false,
	}, data)
}

func TestAirlinesHTTP_GetByID_NotFound_Returns404(t *testing.T) {
	e := newAirlineHTTPEnv(t)

	for _, id := range []string{"999999999", "abc", "0", "-1"} {
		rr := e.get(t, "/api/v1/airlines/"+id)
		require.Equal(t, http.StatusNotFound, rr.Code, id)
		assert.JSONEq(t, `{"error":"airline not found","code":404}`, rr.Body.String(), id)
	}
}
