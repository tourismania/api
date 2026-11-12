package application_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	listhotelroomcategories "api/internal/application/query/list_hotel_room_categories"
	searchhotels "api/internal/application/query/search_hotels"
	"api/internal/domain/agency"
	"api/internal/domain/hotel"
	"api/internal/domain/user"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"
	roomcategorieshttp "api/internal/presentation/http/api/v1/hotel/room_categories"
	searchhotelhttp "api/internal/presentation/http/api/v1/hotel/search"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Сценарии travel-services/hotels на полном стеке (хендлер → use case →
// PostgreSQL). Нужен запущенный Postgres с миграциями (TEST_DATABASE_URL).
// Каждый тест создаёт два свежих агентства A и B и удаляет их данные после
// себя; пользователей подменяет usersByUUID.

// usersByUUID — user.Repository в памяти: запись пользователя по uuid.
type usersByUUID map[uuid.UUID]*user.Record

func (u usersByUUID) FindByUuid(_ context.Context, id uuid.UUID) (*user.Record, error) {
	return u[id], nil
}

func (u usersByUUID) Store(_ context.Context, _ user.User, _ string) (*int, error) {
	return nil, nil
}

type hotelHTTPEnv struct {
	router  http.Handler
	repo    *pgrepo.HotelRepository
	pool    *pgxpool.Pool
	agencyA int
	agencyB int
	// tokenA — агент агентства A; tokenUserA — только ROLE_USER агентства A;
	// tokenB — агент агентства B.
	tokenA, tokenUserA, tokenB string
}

func newHotelHTTPEnv(t *testing.T) hotelHTTPEnv {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping database-backed HTTP test")
	}
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	agencies := pgrepo.NewAgencyRepository(db.New(pool))
	newAgency := func() int {
		id, err := agencies.Store(ctx, agency.Agency{
			UUID: uuid.New(), Name: "Hotel HTTP Agency " + uuid.NewString(),
			Status: agency.StatusActive, CreatedAt: time.Now(),
		})
		require.NoError(t, err)
		return id
	}
	e := hotelHTTPEnv{repo: pgrepo.NewHotelRepository(db.New(pool)), pool: pool, agencyA: newAgency(), agencyB: newAgency()}
	t.Cleanup(func() {
		ctx := context.Background()
		ids := []int{e.agencyA, e.agencyB}
		_, err := pool.Exec(ctx, `DELETE FROM hotel_room_categories WHERE hotel_id IN (SELECT id FROM hotels WHERE agency_id = ANY($1))`, ids)
		assert.NoError(t, err)
		_, err = pool.Exec(ctx, `DELETE FROM hotels WHERE agency_id = ANY($1)`, ids)
		assert.NoError(t, err)
		_, err = pool.Exec(ctx, `DELETE FROM agencies WHERE id = ANY($1)`, ids)
		assert.NoError(t, err)
	})

	jwtSvc := newTestJWTService(t)
	users := usersByUUID{}
	issue := func(agencyID int, roles ...user.Role) string {
		id := uuid.New()
		rs := make([]string, 0, len(roles))
		for _, r := range roles {
			rs = append(rs, string(r))
		}
		users[id] = &user.Record{ID: len(users) + 1, Uuid: id, AgencyID: agencyID, Roles: rs}
		tok, err := jwtSvc.Issue(id)
		require.NoError(t, err)
		return tok
	}
	e.tokenA = issue(e.agencyA, user.RoleUser, user.RoleAgent)
	e.tokenUserA = issue(e.agencyA, user.RoleUser)
	e.tokenB = issue(e.agencyB, user.RoleUser, user.RoleAgent)

	finder := user.NewFinder(users)
	searchH := searchhotelhttp.NewHandler(searchhotels.NewHandler(e.repo, finder), validator.New(validator.WithRequiredStructEnabled()))
	catsH := roomcategorieshttp.NewHandler(listhotelroomcategories.NewHandler(e.repo, finder))
	r := chi.NewRouter()
	r.Route("/api/v1", func(api chi.Router) {
		api.Group(func(priv chi.Router) {
			priv.Use(custommw.JWT(jwtSvc))
			priv.Use(custommw.CurrentUserUUID)
			priv.Get("/hotels", searchH.Handle)
			priv.Get("/hotels/{id}/room-categories", catsH.Handle)
		})
	})
	e.router = r
	return e
}

func (e hotelHTTPEnv) insertHotel(t *testing.T, agencyID int, name string, stars int, address string, photos ...string) int {
	t.Helper()
	h, err := hotel.New(agencyID, name, stars, address, "", photos)
	require.NoError(t, err)
	id, err := e.repo.Insert(context.Background(), h)
	require.NoError(t, err)
	return id
}

func (e hotelHTTPEnv) insertCategory(t *testing.T, hotelID int, name string) int {
	t.Helper()
	c, err := hotel.NewRoomCategory(hotelID, name, "desc "+name, []string{"https://cdn.example.com/" + name + ".jpg"})
	require.NoError(t, err)
	id, err := e.repo.InsertRoomCategory(context.Background(), c)
	require.NoError(t, err)
	return id
}

func (e hotelHTTPEnv) markDeleted(t *testing.T, table string, id int) {
	t.Helper()
	_, err := e.pool.Exec(context.Background(), `UPDATE `+table+` SET deleted_at = now() WHERE id = $1`, id)
	require.NoError(t, err)
}

func (e hotelHTTPEnv) get(t *testing.T, token, path string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rr := httptest.NewRecorder()
	e.router.ServeHTTP(rr, req)
	return rr
}

// hotelListBody — разобранное тело ответа со списком в data.
type hotelListBody struct {
	Data []map[string]any `json:"data"`
	Meta map[string]any   `json:"meta"`
}

func decodeHotelBody(t *testing.T, rr *httptest.ResponseRecorder) hotelListBody {
	t.Helper()
	var body hotelListBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body), rr.Body.String())
	return body
}

func (e hotelHTTPEnv) searchOK(t *testing.T, token, query string) hotelListBody {
	t.Helper()
	rr := e.get(t, token, "/api/v1/hotels?"+query)
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, "no-store", rr.Header().Get("Cache-Control"))
	return decodeHotelBody(t, rr)
}

func names(body hotelListBody) []string {
	out := make([]string, 0, len(body.Data))
	for _, d := range body.Data {
		out = append(out, d["name"].(string))
	}
	return out
}

func categoriesPath(id any) string { return fmt.Sprintf("/api/v1/hotels/%v/room-categories", id) }

// [hotels-4.1]
func TestHotelsHTTP_SearchSubstring_PrefixFirst_NoCategories(t *testing.T) {
	e := newHotelHTTPEnv(t)
	e.insertHotel(t, e.agencyA, "DoubleTree by Hilton", 4, "")
	h := e.insertHotel(t, e.agencyA, "Hilton Istanbul Bomonti", 5, "Şişli", "https://cdn.example.com/a.jpg")
	e.insertCategory(t, h, "Standard")

	body := e.searchOK(t, e.tokenA, "search=hilt")

	assert.Equal(t, []string{"Hilton Istanbul Bomonti", "DoubleTree by Hilton"}, names(body))
	first := body.Data[0]
	assert.Equal(t, map[string]any{
		"id": float64(h), "name": "Hilton Istanbul Bomonti", "stars": float64(5), "address": "Şişli",
		"description": "", "photo_urls": []any{"https://cdn.example.com/a.jpg"},
	}, first, "ровно поля контракта, категорий нет")
	assert.Equal(t, map[string]any{"total": float64(2), "limit": float64(20), "offset": float64(0), "search": "hilt"}, body.Meta)
}

// [hotels-1.2]
func TestHotelsHTTP_SameNames_BothReturned(t *testing.T) {
	e := newHotelHTTPEnv(t)
	a := e.insertHotel(t, e.agencyA, "Hilton", 5, "Москва")
	b := e.insertHotel(t, e.agencyA, "Hilton", 5, "Казань")

	body := e.searchOK(t, e.tokenA, "search=Hilton")

	require.Len(t, body.Data, 2)
	assert.Equal(t, []any{float64(a), float64(b)}, []any{body.Data[0]["id"], body.Data[1]["id"]})
}

// [hotels-2.1], [hotels-2.2]
func TestHotelsHTTP_ZeroStarsNoAddress_PhotoOrder(t *testing.T) {
	e := newHotelHTTPEnv(t)
	photos := []string{"https://a.example.com/3.jpg", "https://b.example.com/1.jpg", "https://c.example.com/2.jpg"}
	e.insertHotel(t, e.agencyA, "Guest House Sea", 0, "", photos...)
	e.insertHotel(t, e.agencyA, "Guest House Hill", 0, "")

	body := e.searchOK(t, e.tokenA, "search=guest")

	require.Len(t, body.Data, 2)
	byName := map[string]map[string]any{}
	for _, d := range body.Data {
		byName[d["name"].(string)] = d
	}
	sea := byName["Guest House Sea"]
	assert.Equal(t, float64(0), sea["stars"])
	assert.Equal(t, "", sea["address"])
	assert.Equal(t, []any{photos[0], photos[1], photos[2]}, sea["photo_urls"])
	assert.Equal(t, []any{}, byName["Guest House Hill"]["photo_urls"], "без фото — пустой массив, не null")
}

// [hotels-4.2]
func TestHotelsHTTP_ShortSearch_InvalidSearch(t *testing.T) {
	e := newHotelHTTPEnv(t)
	for _, query := range []string{"search=h", "search=%20%20", ""} {
		t.Run(query, func(t *testing.T) {
			rr := e.get(t, e.tokenA, "/api/v1/hotels?"+query)

			require.Equal(t, http.StatusBadRequest, rr.Code)
			assert.JSONEq(t, `{"error":{"code":"INVALID_SEARCH","message":"Parameter 'search' must be at least 2 characters long","field":"search"}}`, rr.Body.String())
		})
	}
}

// [hotels-4.3]
func TestHotelsHTTP_PatternCharsLiteral(t *testing.T) {
	e := newHotelHTTPEnv(t)
	e.insertHotel(t, e.agencyA, "Hotel 100% Sea", 3, "")

	body := e.searchOK(t, e.tokenA, "search=%25%25")

	assert.Empty(t, body.Data)
	assert.Equal(t, float64(0), body.Meta["total"])
}

// [hotels-4.4]
func TestHotelsHTTP_PaginationBounds(t *testing.T) {
	e := newHotelHTTPEnv(t)
	for query, meta := range map[string]string{
		"search=hilton&limit=101":    `{"Limit":"max=100"}`,
		"search=hilton&offset=10001": `{"Offset":"max=10000"}`,
	} {
		t.Run(query, func(t *testing.T) {
			rr := e.get(t, e.tokenA, "/api/v1/hotels?"+query)

			require.Equal(t, http.StatusBadRequest, rr.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			assert.Equal(t, "validation failed", body["error"])
			raw, err := json.Marshal(body["meta"])
			require.NoError(t, err)
			assert.JSONEq(t, meta, string(raw))
		})
	}
}

func TestHotelsHTTP_PaginationDefaultsAndOffset(t *testing.T) {
	e := newHotelHTTPEnv(t)
	for _, n := range []string{"Sea A", "Sea B", "Sea C"} {
		e.insertHotel(t, e.agencyA, n, 3, "")
	}

	body := e.searchOK(t, e.tokenA, "search=sea&limit=abc&offset=-5")
	assert.Equal(t, float64(20), body.Meta["limit"], "некорректное значение — по умолчанию")
	assert.Equal(t, float64(0), body.Meta["offset"])

	page := e.searchOK(t, e.tokenA, "search=sea&limit=1&offset=1")
	assert.Equal(t, []string{"Sea B"}, names(page))
	assert.Equal(t, float64(3), page.Meta["total"])
}

// [hotels-3.1]
func TestHotelsHTTP_SearchDoesNotSeeOtherAgency(t *testing.T) {
	e := newHotelHTTPEnv(t)
	e.insertHotel(t, e.agencyB, "Rixos", 5, "")

	assert.Empty(t, e.searchOK(t, e.tokenA, "search=rixos").Data)
	assert.Equal(t, []string{"Rixos"}, names(e.searchOK(t, e.tokenB, "search=rixos")))
}

// [hotels-3.3]
func TestHotelsHTTP_RoleUser_CanRead(t *testing.T) {
	e := newHotelHTTPEnv(t)
	id := e.insertHotel(t, e.agencyA, "Rixos", 5, "")

	assert.Equal(t, []string{"Rixos"}, names(e.searchOK(t, e.tokenUserA, "search=rixos")))
	rr := e.get(t, e.tokenUserA, categoriesPath(id))
	assert.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
}

// [hotels-3.4]
func TestHotelsHTTP_NoToken_Unauthorized(t *testing.T) {
	e := newHotelHTTPEnv(t)

	assert.Equal(t, http.StatusUnauthorized, e.get(t, "", "/api/v1/hotels?search=rixos").Code)
	assert.Equal(t, http.StatusUnauthorized, e.get(t, "", categoriesPath(1)).Code)
}

// [hotels-5.1]
func TestHotelsHTTP_RoomCategories_InCreationOrder(t *testing.T) {
	e := newHotelHTTPEnv(t)
	id := e.insertHotel(t, e.agencyA, "Rixos", 5, "")
	std := e.insertCategory(t, id, "Standard")
	dlx := e.insertCategory(t, id, "Deluxe")

	rr := e.get(t, e.tokenA, categoriesPath(id))

	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Equal(t, "no-store", rr.Header().Get("Cache-Control"))
	assert.JSONEq(t, fmt.Sprintf(`{"data":[
		{"id":%d,"name":"Standard","description":"desc Standard","photo_urls":["https://cdn.example.com/Standard.jpg"]},
		{"id":%d,"name":"Deluxe","description":"desc Deluxe","photo_urls":["https://cdn.example.com/Deluxe.jpg"]}
	]}`, std, dlx), rr.Body.String())
}

// [hotels-1.1]
func TestHotelsHTTP_RoomCategories_EmptyList(t *testing.T) {
	e := newHotelHTTPEnv(t)
	id := e.insertHotel(t, e.agencyA, "Rixos", 5, "")

	rr := e.get(t, e.tokenA, categoriesPath(id))

	require.Equal(t, http.StatusOK, rr.Code)
	assert.JSONEq(t, `{"data":[]}`, rr.Body.String())
}

// [hotels-5.2]
func TestHotelsHTTP_RoomCategories_InvalidID(t *testing.T) {
	e := newHotelHTTPEnv(t)
	for _, id := range []string{"abc", "0", "-1", "1.5"} {
		t.Run(id, func(t *testing.T) {
			rr := e.get(t, e.tokenA, categoriesPath(id))

			require.Equal(t, http.StatusBadRequest, rr.Code)
			assert.JSONEq(t, `{"error":"invalid hotel id","code":400}`, rr.Body.String())
		})
	}
}

// [hotels-3.2], [hotels-5.3]
func TestHotelsHTTP_RoomCategories_ForeignOrMissingHotel_NotFound(t *testing.T) {
	e := newHotelHTTPEnv(t)
	foreign := e.insertHotel(t, e.agencyB, "Rixos", 5, "")
	e.insertCategory(t, foreign, "Standard")
	missing := e.insertHotel(t, e.agencyA, "Gone", 5, "")
	_, err := e.pool.Exec(context.Background(), `DELETE FROM hotels WHERE id = $1`, missing)
	require.NoError(t, err)

	for name, id := range map[string]int{"чужой": foreign, "несуществующий": missing} {
		t.Run(name, func(t *testing.T) {
			rr := e.get(t, e.tokenA, categoriesPath(id))

			require.Equal(t, http.StatusNotFound, rr.Code)
			assert.JSONEq(t, `{"error":"hotel not found","code":404}`, rr.Body.String())
		})
	}
}

// [hotels-6.1]
func TestHotelsHTTP_DeletedHotel_Hidden(t *testing.T) {
	e := newHotelHTTPEnv(t)
	id := e.insertHotel(t, e.agencyA, "Rixos", 5, "")
	e.insertCategory(t, id, "Standard")
	e.markDeleted(t, "hotels", id)

	assert.Empty(t, e.searchOK(t, e.tokenA, "search=rixos").Data)
	rr := e.get(t, e.tokenA, categoriesPath(id))
	require.Equal(t, http.StatusNotFound, rr.Code)
	assert.JSONEq(t, `{"error":"hotel not found","code":404}`, rr.Body.String())
}

// [hotels-6.2]
func TestHotelsHTTP_DeletedCategory_Hidden(t *testing.T) {
	e := newHotelHTTPEnv(t)
	id := e.insertHotel(t, e.agencyA, "Rixos", 5, "")
	std := e.insertCategory(t, id, "Standard")
	e.insertCategory(t, id, "Deluxe")
	e.markDeleted(t, "hotel_room_categories", std)

	rr := e.get(t, e.tokenA, categoriesPath(id))

	require.Equal(t, http.StatusOK, rr.Code)
	body := decodeHotelBody(t, rr)
	assert.Equal(t, []string{"Deluxe"}, names(body))
}
