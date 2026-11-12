package repository

import (
	"context"
	"fmt"

	"api/internal/domain/hotel"
	"api/internal/infrastructure/persistence/helpers"
	"api/internal/infrastructure/persistence/postgres/db"
	"api/internal/infrastructure/persistence/postgres/mapper"
)

// Compile-time interface check.
var _ hotel.Repository = (*HotelRepository)(nil)

// HotelRepository хранит справочник отелей агентств и их категорий номеров
// в Postgres через слой db. Записи с отметкой удаления методы чтения не
// видят.
type HotelRepository struct {
	queries *db.Queries
}

// NewHotelRepository создаёт репозиторий поверх слоя запросов.
func NewHotelRepository(queries *db.Queries) *HotelRepository {
	return &HotelRepository{queries: queries}
}

// Search ищет отели агентства; строка поиска сравнивается буквально
// (EscapeLike).
func (r *HotelRepository) Search(ctx context.Context, f hotel.Filter) (hotel.SearchResult, error) {
	escaped := helpers.EscapeLike(f.Search)
	rows, err := queriesFor(ctx, r.queries).SearchHotels(ctx, db.SearchHotelsParams{
		SearchPrefix: escaped + "%",
		SearchLike:   "%" + escaped + "%",
		AgencyID:     int32(f.AgencyID),
		LimitVal:     int32(f.Limit),
		OffsetVal:    int32(f.Offset),
	})
	if err != nil {
		return hotel.SearchResult{}, fmt.Errorf("search hotels: %w", err)
	}

	hotels := make([]hotel.Hotel, 0, len(rows))
	var total int64
	for _, row := range rows {
		total = row.TotalCount
		hotels = append(hotels, mapper.ToHotelDomain(row.Hotel))
	}
	return hotel.SearchResult{Hotels: hotels, TotalCount: total}, nil
}

// FindByIDsAndAgency возвращает действующие отели агентства agencyID с
// перечисленными id.
func (r *HotelRepository) FindByIDsAndAgency(ctx context.Context, ids []int, agencyID int) ([]hotel.Hotel, error) {
	if len(ids) == 0 {
		return []hotel.Hotel{}, nil
	}
	rows, err := queriesFor(ctx, r.queries).FindHotelsByIDsAndAgency(ctx, toInt32s(ids), int32(agencyID))
	if err != nil {
		return nil, fmt.Errorf("find hotels by id: %w", err)
	}
	out := make([]hotel.Hotel, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapper.ToHotelDomain(row))
	}
	return out, nil
}

// ListRoomCategories возвращает категории отеля hotelID в порядке создания.
func (r *HotelRepository) ListRoomCategories(ctx context.Context, hotelID int) ([]hotel.RoomCategory, error) {
	rows, err := queriesFor(ctx, r.queries).ListHotelRoomCategories(ctx, int32(hotelID))
	if err != nil {
		return nil, fmt.Errorf("list room categories of hotel %d: %w", hotelID, err)
	}
	return toRoomCategories(rows), nil
}

// FindRoomCategoriesByIDs возвращает действующие категории с перечисленными
// id.
func (r *HotelRepository) FindRoomCategoriesByIDs(ctx context.Context, ids []int) ([]hotel.RoomCategory, error) {
	if len(ids) == 0 {
		return []hotel.RoomCategory{}, nil
	}
	rows, err := queriesFor(ctx, r.queries).FindRoomCategoriesByIDs(ctx, toInt32s(ids))
	if err != nil {
		return nil, fmt.Errorf("find room categories by id: %w", err)
	}
	return toRoomCategories(rows), nil
}

// Insert добавляет отель и возвращает его id.
func (r *HotelRepository) Insert(ctx context.Context, h hotel.Hotel) (int, error) {
	id, err := queriesFor(ctx, r.queries).InsertHotel(ctx, db.InsertHotelParams{
		AgencyID:    int32(h.AgencyID),
		Name:        h.Name,
		Stars:       int16(h.Stars),
		Address:     h.Address,
		Description: h.Description,
		PhotoUrls:   h.PhotoURLs,
	})
	if err != nil {
		return 0, fmt.Errorf("insert hotel: %w", err)
	}
	return int(id), nil
}

// InsertRoomCategory добавляет категорию номера и возвращает её id.
func (r *HotelRepository) InsertRoomCategory(ctx context.Context, c hotel.RoomCategory) (int, error) {
	id, err := queriesFor(ctx, r.queries).InsertHotelRoomCategory(ctx, db.InsertHotelRoomCategoryParams{
		HotelID:     int32(c.HotelID),
		Name:        c.Name,
		Description: c.Description,
		PhotoUrls:   c.PhotoURLs,
	})
	if err != nil {
		return 0, fmt.Errorf("insert room category of hotel %d: %w", c.HotelID, err)
	}
	return int(id), nil
}

func toRoomCategories(rows []db.HotelRoomCategory) []hotel.RoomCategory {
	out := make([]hotel.RoomCategory, 0, len(rows))
	for _, row := range rows {
		out = append(out, mapper.ToRoomCategoryDomain(row))
	}
	return out
}
