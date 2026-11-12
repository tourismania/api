package integration_test

import (
	"context"
	"testing"
	"time"

	"api/internal/domain/agency"
	"api/internal/domain/hotel"
	"api/internal/infrastructure/persistence/postgres"
	"api/internal/infrastructure/persistence/postgres/db"
	pgrepo "api/internal/infrastructure/persistence/postgres/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// hotelRepoEnv — репозиторий отелей, пул для пометки удаления и два свежих
// агентства: данные каждого теста изолированы его агентствами.
type hotelRepoEnv struct {
	repo   *pgrepo.HotelRepository
	pool   *pgxpool.Pool
	agency int
	other  int
}

func newHotelRepoEnv(t *testing.T) hotelRepoEnv {
	t.Helper()
	ctx := context.Background()
	pool, err := postgres.NewPool(ctx, testDB(t))
	require.NoError(t, err)
	t.Cleanup(pool.Close)

	agencies := pgrepo.NewAgencyRepository(db.New(pool))
	newAgency := func() int {
		id, err := agencies.Store(ctx, agency.Agency{
			UUID: uuid.New(), Name: "Hotel Agency " + uuid.NewString(),
			Status: agency.StatusActive, CreatedAt: time.Now(),
		})
		require.NoError(t, err)
		return id
	}
	env := hotelRepoEnv{repo: pgrepo.NewHotelRepository(db.New(pool)), pool: pool, agency: newAgency(), other: newAgency()}
	t.Cleanup(func() {
		ctx := context.Background()
		ids := []int{env.agency, env.other}
		_, err := pool.Exec(ctx, `DELETE FROM hotel_room_categories WHERE hotel_id IN (SELECT id FROM hotels WHERE agency_id = ANY($1))`, ids)
		assert.NoError(t, err)
		_, err = pool.Exec(ctx, `DELETE FROM hotels WHERE agency_id = ANY($1)`, ids)
		assert.NoError(t, err)
		_, err = pool.Exec(ctx, `DELETE FROM agencies WHERE id = ANY($1)`, ids)
		assert.NoError(t, err)
	})
	return env
}

func (e hotelRepoEnv) insertHotel(t *testing.T, agencyID int, name string, photos ...string) int {
	t.Helper()
	h, err := hotel.New(agencyID, name, 4, "addr "+name, "desc "+name, photos)
	require.NoError(t, err)
	id, err := e.repo.Insert(context.Background(), h)
	require.NoError(t, err)
	return id
}

func (e hotelRepoEnv) insertCategory(t *testing.T, hotelID int, name string, photos ...string) int {
	t.Helper()
	c, err := hotel.NewRoomCategory(hotelID, name, "desc "+name, photos)
	require.NoError(t, err)
	id, err := e.repo.InsertRoomCategory(context.Background(), c)
	require.NoError(t, err)
	return id
}

func (e hotelRepoEnv) search(t *testing.T, agencyID int, s string, limit, offset int) hotel.SearchResult {
	t.Helper()
	res, err := e.repo.Search(context.Background(), hotel.Filter{AgencyID: agencyID, Search: s, Limit: limit, Offset: offset})
	require.NoError(t, err)
	return res
}

func (e hotelRepoEnv) markDeleted(t *testing.T, table string, id int) {
	t.Helper()
	_, err := e.pool.Exec(context.Background(), `UPDATE `+table+` SET deleted_at = now() WHERE id = $1`, id)
	require.NoError(t, err)
}

func hotelNames(hs []hotel.Hotel) []string {
	out := make([]string, 0, len(hs))
	for _, h := range hs {
		out = append(out, h.Name)
	}
	return out
}

func categoryNames(cs []hotel.RoomCategory) []string {
	out := make([]string, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.Name)
	}
	return out
}

func TestHotelRepository_InsertThenRead_FieldsAndPhotoOrder(t *testing.T) {
	e := newHotelRepoEnv(t)
	ctx := context.Background()
	photos := []string{"https://a.example.com/3.jpg", "https://b.example.com/1.jpg", "https://c.example.com/2.jpg"}
	h, err := hotel.New(e.agency, "Rixos Premium", 0, "", "", photos)
	require.NoError(t, err)

	id, err := e.repo.Insert(ctx, h)
	require.NoError(t, err)

	found, err := e.repo.FindByIDsAndAgency(ctx, []int{id}, e.agency)
	require.NoError(t, err)
	require.Len(t, found, 1)
	assert.Equal(t, hotel.Hotel{ID: id, AgencyID: e.agency, Name: "Rixos Premium", Stars: 0, Address: "", Description: "", PhotoURLs: photos}, found[0])
}

func TestHotelRepository_RoomCategories_InCreationOrder(t *testing.T) {
	e := newHotelRepoEnv(t)
	ctx := context.Background()
	hotelID := e.insertHotel(t, e.agency, "Hilton")
	empty := e.insertHotel(t, e.agency, "Empty")
	std := e.insertCategory(t, hotelID, "Standard", "https://cdn.example.com/s.jpg")
	dlx := e.insertCategory(t, hotelID, "Deluxe")

	cats, err := e.repo.ListRoomCategories(ctx, hotelID)
	require.NoError(t, err)
	assert.Equal(t, []string{"Standard", "Deluxe"}, categoryNames(cats))
	assert.Equal(t, []string{"https://cdn.example.com/s.jpg"}, cats[0].PhotoURLs)
	assert.NotNil(t, cats[1].PhotoURLs)
	assert.Empty(t, cats[1].PhotoURLs)

	none, err := e.repo.ListRoomCategories(ctx, empty)
	require.NoError(t, err)
	assert.NotNil(t, none)
	assert.Empty(t, none)

	byID, err := e.repo.FindRoomCategoriesByIDs(ctx, []int{dlx, std})
	require.NoError(t, err)
	assert.Equal(t, []string{"Standard", "Deluxe"}, categoryNames(byID))
	assert.Equal(t, hotelID, byID[0].HotelID)
}

func TestHotelRepository_Search_OnlyOwnAgency(t *testing.T) {
	e := newHotelRepoEnv(t)
	e.insertHotel(t, e.agency, "Rixos Own")
	foreign := e.insertHotel(t, e.other, "Rixos Foreign")

	res := e.search(t, e.agency, "rixos", 20, 0)
	assert.Equal(t, []string{"Rixos Own"}, hotelNames(res.Hotels))
	assert.EqualValues(t, 1, res.TotalCount)

	found, err := e.repo.FindByIDsAndAgency(context.Background(), []int{foreign}, e.agency)
	require.NoError(t, err)
	assert.Empty(t, found, "чужой отель не находится по id")
}

func TestHotelRepository_Search_PrefixFirstThenNameThenID(t *testing.T) {
	e := newHotelRepoEnv(t)
	e.insertHotel(t, e.agency, "DoubleTree by Hilton")
	e.insertHotel(t, e.agency, "Hilton Istanbul Bomonti")
	e.insertHotel(t, e.agency, "Garden Inn Hilton")
	dup1 := e.insertHotel(t, e.agency, "Hilton")
	dup2 := e.insertHotel(t, e.agency, "Hilton")
	e.insertHotel(t, e.agency, "Ibis")

	res := e.search(t, e.agency, "HILT", 20, 0)

	assert.Equal(t, []string{"Hilton", "Hilton", "Hilton Istanbul Bomonti", "DoubleTree by Hilton", "Garden Inn Hilton"}, hotelNames(res.Hotels))
	assert.Equal(t, []int{dup1, dup2}, []int{res.Hotels[0].ID, res.Hotels[1].ID}, "одинаковые названия — по id")
	assert.EqualValues(t, 5, res.TotalCount)
}

func TestHotelRepository_Search_PatternCharsLiteral(t *testing.T) {
	e := newHotelRepoEnv(t)
	e.insertHotel(t, e.agency, "Hotel 100% Sea")
	e.insertHotel(t, e.agency, "Hotel_One")
	e.insertHotel(t, e.agency, `Back\slash Inn`)
	e.insertHotel(t, e.agency, "HotelXOne")

	assert.Empty(t, e.search(t, e.agency, "%%", 20, 0).Hotels)
	assert.Equal(t, []string{"Hotel 100% Sea"}, hotelNames(e.search(t, e.agency, "0%", 20, 0).Hotels))
	assert.Equal(t, []string{"Hotel_One"}, hotelNames(e.search(t, e.agency, "l_o", 20, 0).Hotels))
	assert.Equal(t, []string{`Back\slash Inn`}, hotelNames(e.search(t, e.agency, `k\s`, 20, 0).Hotels))
}

func TestHotelRepository_Search_TotalWithOffset(t *testing.T) {
	e := newHotelRepoEnv(t)
	for _, n := range []string{"Sea A", "Sea B", "Sea C"} {
		e.insertHotel(t, e.agency, n)
	}

	res := e.search(t, e.agency, "sea", 1, 1)

	assert.Equal(t, []string{"Sea B"}, hotelNames(res.Hotels))
	assert.EqualValues(t, 3, res.TotalCount)
}

func TestHotelRepository_DeletedHotel_Hidden(t *testing.T) {
	e := newHotelRepoEnv(t)
	ctx := context.Background()
	id := e.insertHotel(t, e.agency, "Rixos Gone")
	cat := e.insertCategory(t, id, "Standard")
	e.markDeleted(t, "hotels", id)

	assert.Empty(t, e.search(t, e.agency, "rixos", 20, 0).Hotels)
	found, err := e.repo.FindByIDsAndAgency(ctx, []int{id}, e.agency)
	require.NoError(t, err)
	assert.Empty(t, found)
	cats, err := e.repo.ListRoomCategories(ctx, id)
	require.NoError(t, err)
	assert.Empty(t, cats, "категории удалённого отеля скрыты вместе с ним")
	byID, err := e.repo.FindRoomCategoriesByIDs(ctx, []int{cat})
	require.NoError(t, err)
	assert.Empty(t, byID)
}

func TestHotelRepository_DeletedCategory_Hidden(t *testing.T) {
	e := newHotelRepoEnv(t)
	ctx := context.Background()
	id := e.insertHotel(t, e.agency, "Rixos")
	std := e.insertCategory(t, id, "Standard")
	e.insertCategory(t, id, "Deluxe")
	e.markDeleted(t, "hotel_room_categories", std)

	cats, err := e.repo.ListRoomCategories(ctx, id)
	require.NoError(t, err)
	assert.Equal(t, []string{"Deluxe"}, categoryNames(cats))
	byID, err := e.repo.FindRoomCategoriesByIDs(ctx, []int{std})
	require.NoError(t, err)
	assert.Empty(t, byID)
}
