package mapper

import (
	"api/internal/domain/hotel"
	"api/internal/infrastructure/persistence/postgres/db"
)

// ToHotelDomainFromSearchRow собирает отель справочника из строки поиска.
func ToHotelDomainFromSearchRow(row db.SearchHotelsRow) hotel.Hotel {
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

// ToHotelDomainFromIDsRow собирает отель справочника из строки
// FindHotelsByIDsAndAgency.
func ToHotelDomainFromIDsRow(row db.FindHotelsByIDsAndAgencyRow) hotel.Hotel {
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

// ToRoomCategoryDomainFromListRow собирает категорию номера из строки
// ListHotelRoomCategories.
func ToRoomCategoryDomainFromListRow(row db.ListHotelRoomCategoriesRow) hotel.RoomCategory {
	return hotel.RoomCategory{
		ID:          int(row.ID),
		HotelID:     int(row.HotelID),
		Name:        row.Name,
		Description: row.Description,
		PhotoURLs:   nonNilStrings(row.PhotoUrls),
	}
}

// ToRoomCategoryDomainFromIDsRow собирает категорию номера из строки
// FindRoomCategoriesByIDs.
func ToRoomCategoryDomainFromIDsRow(row db.FindRoomCategoriesByIDsRow) hotel.RoomCategory {
	return hotel.RoomCategory{
		ID:          int(row.ID),
		HotelID:     int(row.HotelID),
		Name:        row.Name,
		Description: row.Description,
		PhotoURLs:   nonNilStrings(row.PhotoUrls),
	}
}

// NonNilPhotoURLs — пустой список вместо nil: столбцы photo_urls — text[]
// NOT NULL, а у записи фото всегда список.
func NonNilPhotoURLs(s []string) []string {
	return nonNilStrings(s)
}

// nonNilStrings — пустой список вместо nil.
func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
