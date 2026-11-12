package unit_test

import (
	"context"
	"errors"
	"testing"

	"api/internal/application/apperror"
	listhotelroomcategories "api/internal/application/query/list_hotel_room_categories"
	searchhotels "api/internal/application/query/search_hotels"
	"api/internal/domain/hotel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubHotelRepo — справочник отелей в памяти: запоминает фильтр поиска и
// отдаёт заданные отели и категории.
type stubHotelRepo struct {
	gotFilter  hotel.Filter
	searched   bool
	result     hotel.SearchResult
	hotels     []hotel.Hotel
	gotAgency  int
	categories []hotel.RoomCategory
	listed     bool
	err        error
}

func (s *stubHotelRepo) Search(_ context.Context, f hotel.Filter) (hotel.SearchResult, error) {
	s.gotFilter, s.searched = f, true
	return s.result, s.err
}

func (s *stubHotelRepo) FindByIDsAndAgency(_ context.Context, ids []int, agencyID int) ([]hotel.Hotel, error) {
	s.gotAgency = agencyID
	var out []hotel.Hotel
	for _, h := range s.hotels {
		for _, id := range ids {
			if h.ID == id && h.AgencyID == agencyID {
				out = append(out, h)
			}
		}
	}
	return out, s.err
}

func (s *stubHotelRepo) ListRoomCategories(_ context.Context, _ int) ([]hotel.RoomCategory, error) {
	s.listed = true
	return s.categories, nil
}

func hotelSearchQuery(search *string) searchhotels.Query {
	return searchhotels.Query{Search: search, Limit: 20}
}

func TestSearchHotels_SearchNormalized_ScopedToCallerAgency(t *testing.T) {
	repo := &stubHotelRepo{}

	res, err := searchhotels.NewHandler(repo, userRecordWithAgency(7)).Handle(context.Background(),
		searchhotels.Query{Search: strp("  Hilton \t  Istanbul "), Limit: 50, Offset: 10})

	require.NoError(t, err)
	assert.Equal(t, hotel.Filter{AgencyID: 7, Search: "Hilton Istanbul", Limit: 50, Offset: 10}, repo.gotFilter)
	assert.Equal(t, "Hilton Istanbul", res.Search, "в meta — нормализованная строка")
}

func TestSearchHotels_ShortOrMissingSearch_ErrSearchTooShort(t *testing.T) {
	for name, q := range map[string]searchhotels.Query{
		"один символ":    hotelSearchQuery(strp("h")),
		"только пробелы": hotelSearchQuery(strp("  ")),
		"пустая":         hotelSearchQuery(strp("")),
		"без search":     hotelSearchQuery(nil),
		"пробел и буква": hotelSearchQuery(strp(" я ")),
	} {
		t.Run(name, func(t *testing.T) {
			repo := &stubHotelRepo{}

			_, err := searchhotels.NewHandler(repo, userRecordWithAgency(1)).Handle(context.Background(), q)

			assert.ErrorIs(t, err, searchhotels.ErrSearchTooShort)
			assert.False(t, repo.searched, "в репозиторий не ходим")
		})
	}
}

func TestSearchHotels_TwoCharacters_Searched(t *testing.T) {
	repo := &stubHotelRepo{}

	_, err := searchhotels.NewHandler(repo, userRecordWithAgency(1)).Handle(context.Background(), hotelSearchQuery(strp("Ят")))

	require.NoError(t, err)
	assert.Equal(t, "Ят", repo.gotFilter.Search, "порог в символах, а не байтах")
}

func TestSearchHotels_MapsHotels(t *testing.T) {
	repo := &stubHotelRepo{result: hotel.SearchResult{TotalCount: 3, Hotels: []hotel.Hotel{
		{ID: 5, AgencyID: 1, Name: "Rixos", Stars: 5, Address: "Анталья", Description: "У моря", PhotoURLs: []string{"https://cdn.example.com/a.jpg"}},
	}}}

	res, err := searchhotels.NewHandler(repo, userRecordWithAgency(1)).Handle(context.Background(), hotelSearchQuery(strp("rix")))

	require.NoError(t, err)
	assert.EqualValues(t, 3, res.TotalCount)
	assert.Equal(t, []searchhotels.HotelResult{
		{ID: 5, Name: "Rixos", Stars: 5, Address: "Анталья", Description: "У моря", PhotoURLs: []string{"https://cdn.example.com/a.jpg"}},
	}, res.Hotels)
}

func TestSearchHotels_ActorNotFound_ReturnsUnauthenticated(t *testing.T) {
	_, err := searchhotels.NewHandler(&stubHotelRepo{}, noUserFound()).Handle(context.Background(), hotelSearchQuery(strp("rix")))

	assert.ErrorIs(t, err, apperror.ErrUnauthenticated)
}

func TestSearchHotels_RepositoryError_Wrapped(t *testing.T) {
	boom := errors.New("boom")

	_, err := searchhotels.NewHandler(&stubHotelRepo{err: boom}, userRecordWithAgency(1)).Handle(context.Background(), hotelSearchQuery(strp("rix")))

	assert.ErrorIs(t, err, boom)
}

func TestListHotelRoomCategories_OwnHotel_ReturnsCategories(t *testing.T) {
	repo := &stubHotelRepo{
		hotels: []hotel.Hotel{{ID: 3, AgencyID: 7}},
		categories: []hotel.RoomCategory{
			{ID: 10, HotelID: 3, Name: "Standard", PhotoURLs: []string{}},
			{ID: 11, HotelID: 3, Name: "Deluxe", Description: "Вид на море", PhotoURLs: []string{"https://cdn.example.com/d.jpg"}},
		},
	}

	res, err := listhotelroomcategories.NewHandler(repo, userRecordWithAgency(7)).Handle(context.Background(),
		listhotelroomcategories.Query{HotelID: 3})

	require.NoError(t, err)
	assert.Equal(t, 7, repo.gotAgency)
	assert.Equal(t, []listhotelroomcategories.RoomCategoryResult{
		{ID: 10, Name: "Standard", PhotoURLs: []string{}},
		{ID: 11, Name: "Deluxe", Description: "Вид на море", PhotoURLs: []string{"https://cdn.example.com/d.jpg"}},
	}, res.RoomCategories)
}

func TestListHotelRoomCategories_NoCategories_EmptyList(t *testing.T) {
	repo := &stubHotelRepo{hotels: []hotel.Hotel{{ID: 3, AgencyID: 7}}}

	res, err := listhotelroomcategories.NewHandler(repo, userRecordWithAgency(7)).Handle(context.Background(),
		listhotelroomcategories.Query{HotelID: 3})

	require.NoError(t, err)
	assert.NotNil(t, res.RoomCategories)
	assert.Empty(t, res.RoomCategories)
}

func TestListHotelRoomCategories_ForeignOrMissingHotel_NotFound(t *testing.T) {
	for name, id := range map[string]int{"чужой": 3, "несуществующий": 99} {
		t.Run(name, func(t *testing.T) {
			repo := &stubHotelRepo{hotels: []hotel.Hotel{{ID: 3, AgencyID: 8}}}

			_, err := listhotelroomcategories.NewHandler(repo, userRecordWithAgency(7)).Handle(context.Background(),
				listhotelroomcategories.Query{HotelID: id})

			assert.ErrorIs(t, err, apperror.ErrNotFound)
			assert.Contains(t, err.Error(), "hotel not found")
			assert.False(t, repo.listed, "категории чужого отеля не читаются")
		})
	}
}

func TestListHotelRoomCategories_ActorNotFound_ReturnsUnauthenticated(t *testing.T) {
	_, err := listhotelroomcategories.NewHandler(&stubHotelRepo{}, noUserFound()).Handle(context.Background(),
		listhotelroomcategories.Query{HotelID: 3})

	assert.ErrorIs(t, err, apperror.ErrUnauthenticated)
}
