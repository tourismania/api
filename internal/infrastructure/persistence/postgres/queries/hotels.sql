-- name: InsertHotel :one
INSERT INTO hotels (agency_id, name, stars, address, description, photo_urls)
VALUES (@agency_id, @name, @stars, @address, @description, @photo_urls)
RETURNING id;

-- name: InsertHotelRoomCategory :one
INSERT INTO hotel_room_categories (hotel_id, name, description, photo_urls)
VALUES (@hotel_id, @name, @description, @photo_urls)
RETURNING id;

-- name: SearchHotels :many
-- Отели агентства, не помеченные удалёнными, по подстроке названия без учёта
-- регистра. search_like / search_prefix — уже экранированные
-- (persistence/helpers.EscapeLike) шаблоны '%s%' и 's%'. rank: 0 — название
-- начинается со строки, 1 — подстрока.
SELECT id, agency_id, name, stars, address, description, photo_urls,
       (CASE WHEN lower(name) LIKE lower(@search_prefix::text) THEN 0 ELSE 1 END)::int AS rank,
       COUNT(*) OVER () AS total_count
FROM hotels
WHERE agency_id = @agency_id
  AND deleted_at IS NULL
  AND lower(name) LIKE lower(@search_like::text)
ORDER BY rank, name, id
LIMIT @limit_val::int
OFFSET @offset_val::int;

-- name: FindHotelsByIDsAndAgency :many
SELECT id, agency_id, name, stars, address, description, photo_urls
FROM hotels
WHERE id = ANY(@ids::int[])
  AND agency_id = @agency_id
  AND deleted_at IS NULL
ORDER BY id;

-- name: ListHotelRoomCategories :many
-- Категории в порядке создания; категории удалённого отеля скрыты вместе с ним.
SELECT c.id, c.hotel_id, c.name, c.description, c.photo_urls
FROM hotel_room_categories c
JOIN hotels h ON h.id = c.hotel_id
WHERE c.hotel_id = @hotel_id
  AND c.deleted_at IS NULL
  AND h.deleted_at IS NULL
ORDER BY c.id;

-- name: FindRoomCategoriesByIDs :many
SELECT c.id, c.hotel_id, c.name, c.description, c.photo_urls
FROM hotel_room_categories c
JOIN hotels h ON h.id = c.hotel_id
WHERE c.id = ANY(@ids::int[])
  AND c.deleted_at IS NULL
  AND h.deleted_at IS NULL
ORDER BY c.id;
