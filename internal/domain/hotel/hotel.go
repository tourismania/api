// Package hotel — справочник отелей агентства (travel-services/hotels):
// отель с категориями номеров, правила полей, фабрики и контракт
// репозитория. Отель принадлежит агентству, в котором его создали, и виден
// только ему. Запись справочника создаётся только при сохранении оффера:
// фабрики не проверяют роль — права проверяет команда оффера, агентство
// берётся из оффера. Пакет зависит только от domain/content.
package hotel

import (
	"errors"
	"strings"
	"unicode/utf8"

	"api/internal/domain/content"
)

// Границы полей записей справочника, в символах.
const (
	// NameMaxLength — название отеля и категории номера.
	NameMaxLength = 200
	// AddressMaxLength — адрес отеля.
	AddressMaxLength = 500
	// DescriptionMaxLength — описание отеля.
	DescriptionMaxLength = 5000
	// RoomCategoryDescriptionMaxLength — описание категории номера.
	RoomCategoryDescriptionMaxLength = 2000
	// PhotoURLsMaxCount — число ссылок на фото у записи.
	PhotoURLsMaxCount = 20
	// PhotoURLMaxLength — длина одной ссылки на фото.
	PhotoURLMaxLength = 2000
)

var (
	// ErrNotFound — отеля нет, он удалён или принадлежит другому агентству:
	// чужой отель неотличим от несуществующего.
	ErrNotFound = errors.New("hotel not found")
	// ErrNameInvalid — название пустое после обрезки пробелов, длиннее
	// NameMaxLength или содержит запрещённый управляющий символ.
	ErrNameInvalid = errors.New("hotel name is required, at most 200 characters, without control characters")
	// ErrAddressInvalid — адрес длиннее AddressMaxLength или содержит
	// запрещённый управляющий символ.
	ErrAddressInvalid = errors.New("hotel address must be at most 500 characters, without control characters")
	// ErrDescriptionInvalid — описание отеля длиннее DescriptionMaxLength или
	// содержит запрещённый управляющий символ.
	ErrDescriptionInvalid = errors.New("hotel description must be at most 5000 characters, without control characters")
	// ErrPhotoURLsInvalid — ссылок больше PhotoURLsMaxCount, ссылка длиннее
	// PhotoURLMaxLength или не абсолютный http(s) URL с хостом.
	ErrPhotoURLsInvalid = errors.New("photo urls: at most 20 absolute http(s) urls of at most 2000 characters")
	// ErrRoomCategoryNameInvalid — название категории пустое после обрезки,
	// длиннее NameMaxLength или содержит запрещённый управляющий символ.
	ErrRoomCategoryNameInvalid = errors.New("room category name is required, at most 200 characters, without control characters")
	// ErrRoomCategoryDescriptionInvalid — описание категории длиннее
	// RoomCategoryDescriptionMaxLength или содержит запрещённый управляющий
	// символ.
	ErrRoomCategoryDescriptionInvalid = errors.New("room category description must be at most 2000 characters, without control characters")
)

// Hotel — отель справочника агентства AgencyID. Name — без пробелов по
// краям; Stars — 0–5, 0 — без звёзд; Address и Description — "" — не
// указаны; PhotoURLs — ссылки на фото в сохранённом порядке, всегда не nil.
type Hotel struct {
	ID          int
	AgencyID    int
	Name        string
	Stars       int
	Address     string
	Description string
	PhotoURLs   []string
}

// RoomCategory — категория номера отеля HotelID. Name — без пробелов по
// краям; Description — "" — не указано; PhotoURLs — как у Hotel.
type RoomCategory struct {
	ID          int
	HotelID     int
	Name        string
	Description string
	PhotoURLs   []string
}

// New собирает отель агентства agencyID по правилам полей: название
// обрезается по краям и обязательно (ErrNameInvalid), звёзды — 0–5
// (content.ErrStarsInvalid), адрес и описание хранятся как есть
// (ErrAddressInvalid, ErrDescriptionInvalid), фото — по правилам
// ErrPhotoURLsInvalid. Во всех текстах запрещены управляющие символы, кроме
// \t, \n, \r. Длина считается в символах. ID не назначается — его даёт
// репозиторий.
func New(agencyID int, name string, stars int, address, description string, photoURLs []string) (Hotel, error) {
	name, ok := normalizeName(name)
	if !ok {
		return Hotel{}, ErrNameInvalid
	}
	if err := content.CheckStars(stars); err != nil {
		return Hotel{}, err
	}
	if !textValid(address, AddressMaxLength) {
		return Hotel{}, ErrAddressInvalid
	}
	if !textValid(description, DescriptionMaxLength) {
		return Hotel{}, ErrDescriptionInvalid
	}
	photos, err := photoURLsCopy(photoURLs)
	if err != nil {
		return Hotel{}, err
	}
	return Hotel{
		AgencyID:    agencyID,
		Name:        name,
		Stars:       stars,
		Address:     address,
		Description: description,
		PhotoURLs:   photos,
	}, nil
}

// NewRoomCategory собирает категорию номера отеля hotelID: название — как у
// отеля (ErrRoomCategoryNameInvalid), описание — до
// RoomCategoryDescriptionMaxLength (ErrRoomCategoryDescriptionInvalid), фото —
// по правилам ErrPhotoURLsInvalid. Существование отеля и его агентство
// NewRoomCategory не проверяет — это делает вызывающая команда.
func NewRoomCategory(hotelID int, name, description string, photoURLs []string) (RoomCategory, error) {
	name, ok := normalizeName(name)
	if !ok {
		return RoomCategory{}, ErrRoomCategoryNameInvalid
	}
	if !textValid(description, RoomCategoryDescriptionMaxLength) {
		return RoomCategory{}, ErrRoomCategoryDescriptionInvalid
	}
	photos, err := photoURLsCopy(photoURLs)
	if err != nil {
		return RoomCategory{}, err
	}
	return RoomCategory{HotelID: hotelID, Name: name, Description: description, PhotoURLs: photos}, nil
}

// normalizeName обрезает пробелы по краям и проверяет обязательное название.
func normalizeName(raw string) (string, bool) {
	name := strings.TrimSpace(raw)
	if name == "" || !textValid(name, NameMaxLength) {
		return "", false
	}
	return name, true
}

// textValid — текст не длиннее maxLen символов и без запрещённых
// управляющих символов.
func textValid(s string, maxLen int) bool {
	return utf8.RuneCountInString(s) <= maxLen && content.CheckText(s) == nil
}

// photoURLsCopy проверяет ссылки на фото и возвращает их копию (nil — пустой
// список), чтобы запись не делила срез с вызывающим.
func photoURLsCopy(urls []string) ([]string, error) {
	if len(urls) > PhotoURLsMaxCount {
		return nil, ErrPhotoURLsInvalid
	}
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		if utf8.RuneCountInString(u) > PhotoURLMaxLength || content.CheckPhotoURL(u) != nil {
			return nil, ErrPhotoURLsInvalid
		}
		out = append(out, u)
	}
	return out, nil
}
