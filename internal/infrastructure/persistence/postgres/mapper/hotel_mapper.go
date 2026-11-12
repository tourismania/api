package mapper

import (
	"api/internal/domain/hotel"
	"api/internal/infrastructure/persistence/postgres/db"
)

// ToHotelDomain собирает отель справочника из строки hotels.
func ToHotelDomain(row db.Hotel) hotel.Hotel {
	return hotel.Hotel{
		ID:          int(row.ID),
		AgencyID:    int(row.AgencyID),
		Name:        row.Name,
		Stars:       int(row.Stars),
		Address:     row.Address,
		Description: row.Description,
		PhotoURLs:   nonNilStrings(row.PhotoUrls),
	}
}

// ToRoomCategoryDomain собирает категорию номера из строки
// hotel_room_categories.
func ToRoomCategoryDomain(row db.HotelRoomCategory) hotel.RoomCategory {
	return hotel.RoomCategory{
		ID:          int(row.ID),
		HotelID:     int(row.HotelID),
		Name:        row.Name,
		Description: row.Description,
		PhotoURLs:   nonNilStrings(row.PhotoUrls),
	}
}

// nonNilStrings — пустой список вместо nil: у записи фото всегда список.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
