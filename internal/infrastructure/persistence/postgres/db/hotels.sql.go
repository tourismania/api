package db

import (
	"context"

	"github.com/jackc/pgx/v5"
)

// Версии запросов queries/hotels.sql с позиционными параметрами (именованные
// @x → $x); порядок параметров указан у каждого запроса.

// insertHotelSQL: $1=agency_id, $2=name, $3=stars, $4=address,
// $5=description, $6=photo_urls.
const insertHotelSQL = `INSERT INTO hotels (agency_id, name, stars, address, description, photo_urls)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING id`

// InsertHotelParams — параметры InsertHotel в порядке столбцов.
type InsertHotelParams struct {
	AgencyID    int32
	Name        string
	Stars       int16
	Address     string
	Description string
	PhotoUrls   []string
}

// InsertHotel вставляет отель и возвращает его id.
func (q *Queries) InsertHotel(ctx context.Context, arg InsertHotelParams) (int32, error) {
	var id int32
	err := q.db.QueryRow(ctx, insertHotelSQL,
		arg.AgencyID,
		arg.Name,
		arg.Stars,
		arg.Address,
		arg.Description,
		nonNilStrings(arg.PhotoUrls),
	).Scan(&id)
	return id, err
}

// insertHotelRoomCategorySQL: $1=hotel_id, $2=name, $3=description,
// $4=photo_urls.
const insertHotelRoomCategorySQL = `INSERT INTO hotel_room_categories (hotel_id, name, description, photo_urls)
VALUES ($1, $2, $3, $4)
RETURNING id`

// InsertHotelRoomCategoryParams — параметры InsertHotelRoomCategory.
type InsertHotelRoomCategoryParams struct {
	HotelID     int32
	Name        string
	Description string
	PhotoUrls   []string
}

// InsertHotelRoomCategory вставляет категорию номера и возвращает её id.
func (q *Queries) InsertHotelRoomCategory(ctx context.Context, arg InsertHotelRoomCategoryParams) (int32, error) {
	var id int32
	err := q.db.QueryRow(ctx, insertHotelRoomCategorySQL,
		arg.HotelID,
		arg.Name,
		arg.Description,
		nonNilStrings(arg.PhotoUrls),
	).Scan(&id)
	return id, err
}

// searchHotelsSQL: $1=search_prefix, $2=search_like, $3=agency_id,
// $4=limit, $5=offset.
const searchHotelsSQL = `SELECT id, agency_id, name, stars, address, description, photo_urls,
       (CASE WHEN lower(name) LIKE lower($1::text) THEN 0 ELSE 1 END)::int AS rank,
       COUNT(*) OVER () AS total_count
FROM hotels
WHERE agency_id = $3
  AND deleted_at IS NULL
  AND lower(name) LIKE lower($2::text)
ORDER BY rank, name, id
LIMIT $4::int
OFFSET $5::int`

// SearchHotelsParams — параметры SearchHotels.
type SearchHotelsParams struct {
	SearchPrefix string
	SearchLike   string
	AgencyID     int32
	LimitVal     int32
	OffsetVal    int32
}

// SearchHotelsRow — одна строка SearchHotels.
type SearchHotelsRow struct {
	Hotel
	Rank       int32
	TotalCount int64
}

// Hotel — строка таблицы hotels без служебных столбцов.
type Hotel struct {
	ID          int32
	AgencyID    int32
	Name        string
	Stars       int16
	Address     string
	Description string
	PhotoUrls   []string
}

// SearchHotels ищет отели агентства по названию с ранжированием и пагинацией.
func (q *Queries) SearchHotels(ctx context.Context, arg SearchHotelsParams) ([]SearchHotelsRow, error) {
	rows, err := q.db.Query(ctx, searchHotelsSQL,
		arg.SearchPrefix,
		arg.SearchLike,
		arg.AgencyID,
		arg.LimitVal,
		arg.OffsetVal,
	)
	if err != nil {
		return nil, err
	}
	return collect(rows, func(row *SearchHotelsRow) []any {
		return append(hotelDest(&row.Hotel), &row.Rank, &row.TotalCount)
	})
}

// findHotelsByIDsAndAgencySQL: $1=ids, $2=agency_id.
const findHotelsByIDsAndAgencySQL = `SELECT id, agency_id, name, stars, address, description, photo_urls
FROM hotels
WHERE id = ANY($1::int[])
  AND agency_id = $2
  AND deleted_at IS NULL
ORDER BY id`

// FindHotelsByIDsAndAgency возвращает действующие отели агентства с
// перечисленными id; чужих и отсутствующих в результате нет.
func (q *Queries) FindHotelsByIDsAndAgency(ctx context.Context, ids []int32, agencyID int32) ([]Hotel, error) {
	rows, err := q.db.Query(ctx, findHotelsByIDsAndAgencySQL, ids, agencyID)
	if err != nil {
		return nil, err
	}
	return collect(rows, hotelDest)
}

// HotelRoomCategory — строка таблицы hotel_room_categories без служебных
// столбцов.
type HotelRoomCategory struct {
	ID          int32
	HotelID     int32
	Name        string
	Description string
	PhotoUrls   []string
}

// listHotelRoomCategoriesSQL: $1=hotel_id.
const listHotelRoomCategoriesSQL = `SELECT c.id, c.hotel_id, c.name, c.description, c.photo_urls
FROM hotel_room_categories c
JOIN hotels h ON h.id = c.hotel_id
WHERE c.hotel_id = $1
  AND c.deleted_at IS NULL
  AND h.deleted_at IS NULL
ORDER BY c.id`

// ListHotelRoomCategories возвращает действующие категории действующего
// отеля в порядке создания.
func (q *Queries) ListHotelRoomCategories(ctx context.Context, hotelID int32) ([]HotelRoomCategory, error) {
	rows, err := q.db.Query(ctx, listHotelRoomCategoriesSQL, hotelID)
	if err != nil {
		return nil, err
	}
	return collect(rows, roomCategoryDest)
}

// findRoomCategoriesByIDsSQL: $1=ids.
const findRoomCategoriesByIDsSQL = `SELECT c.id, c.hotel_id, c.name, c.description, c.photo_urls
FROM hotel_room_categories c
JOIN hotels h ON h.id = c.hotel_id
WHERE c.id = ANY($1::int[])
  AND c.deleted_at IS NULL
  AND h.deleted_at IS NULL
ORDER BY c.id`

// FindRoomCategoriesByIDs возвращает действующие категории действующих
// отелей с перечисленными id.
func (q *Queries) FindRoomCategoriesByIDs(ctx context.Context, ids []int32) ([]HotelRoomCategory, error) {
	rows, err := q.db.Query(ctx, findRoomCategoriesByIDsSQL, ids)
	if err != nil {
		return nil, err
	}
	return collect(rows, roomCategoryDest)
}

// hotelDest — адреса полей Hotel в порядке столбцов SELECT.
func hotelDest(h *Hotel) []any {
	return []any{&h.ID, &h.AgencyID, &h.Name, &h.Stars, &h.Address, &h.Description, &h.PhotoUrls}
}

// roomCategoryDest — адреса полей HotelRoomCategory в порядке столбцов SELECT.
func roomCategoryDest(c *HotelRoomCategory) []any {
	return []any{&c.ID, &c.HotelID, &c.Name, &c.Description, &c.PhotoUrls}
}

// collect читает все строки rows в срез T; dest отдаёт адреса полей строки
// в порядке столбцов. Пустой результат — пустой срез, не nil.
func collect[T any](rows pgx.Rows, dest func(*T) []any) ([]T, error) {
	defer rows.Close()
	result := []T{}
	for rows.Next() {
		var row T
		if err := rows.Scan(dest(&row)...); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

// nonNilStrings — пустой массив вместо NULL для столбцов text[] NOT NULL.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
