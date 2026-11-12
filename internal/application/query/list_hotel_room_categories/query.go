// Package listhotelroomcategories — категории номеров отеля справочника
// своего агентства (GET /api/v1/hotels/{id}/room-categories).
package listhotelroomcategories

import "github.com/google/uuid"

// Query — отель HotelID и пользователь CurrentUserUUID, из текущей записи
// которого берётся агентство.
type Query struct {
	CurrentUserUUID uuid.UUID
	HotelID         int
}
