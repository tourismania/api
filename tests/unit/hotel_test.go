package unit_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"api/internal/application/apperror"
	"api/internal/domain/content"
	"api/internal/domain/hotel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// photoURLs — n корректных ссылок на фото.
func photoURLs(n int) []string {
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, fmt.Sprintf("https://cdn.example.com/%d.jpg", i))
	}
	return out
}

func TestHotelNew_NameTrimmed_FieldsStoredAsIs(t *testing.T) {
	h, err := hotel.New(7, "  Hilton Istanbul Bomonti \t", 5, " ул. Морская, 1 ", "Описание\nв две строки", nil)

	require.NoError(t, err)
	assert.Equal(t, 7, h.AgencyID)
	assert.Equal(t, "Hilton Istanbul Bomonti", h.Name)
	assert.Equal(t, 5, h.Stars)
	assert.Equal(t, " ул. Морская, 1 ", h.Address, "адрес хранится как есть")
	assert.Equal(t, "Описание\nв две строки", h.Description)
	assert.NotNil(t, h.PhotoURLs, "без фото — пустой список, не nil")
	assert.Empty(t, h.PhotoURLs)
}

// [hotels-2.1]
func TestHotelNew_ZeroStarsWithoutAddress(t *testing.T) {
	h, err := hotel.New(1, "Guest House", 0, "", "", nil)

	require.NoError(t, err)
	assert.Equal(t, 0, h.Stars)
	assert.Equal(t, "", h.Address)
}

// [hotels-2.2]
func TestHotelNew_PhotoOrderKept_SliceCopied(t *testing.T) {
	in := []string{"https://a.example.com/3.jpg", "http://b.example.com/1.jpg", "https://c.example.com/2.jpg"}

	h, err := hotel.New(1, "Rixos", 5, "", "", in)

	require.NoError(t, err)
	assert.Equal(t, in, h.PhotoURLs)
	in[0] = "https://changed.example.com/x.jpg"
	assert.Equal(t, "https://a.example.com/3.jpg", h.PhotoURLs[0], "отель не делит срез с вызывающим")
}

func TestHotelNew_InvalidFields(t *testing.T) {
	long := func(n int) string { return strings.Repeat("я", n) }
	for name, tc := range map[string]struct {
		name, address, description string
		stars                      int
		photos                     []string
		want                       error
	}{
		"пустое название":            {name: "", want: hotel.ErrNameInvalid},
		"название из пробелов":       {name: "  \t ", want: hotel.ErrNameInvalid},
		"название 201 символ":        {name: long(201), want: hotel.ErrNameInvalid},
		"управляющий в названии":     {name: "Hil\u001bton", want: hotel.ErrNameInvalid},
		"звёзд -1":                   {name: "H", stars: -1, want: content.ErrStarsInvalid},
		"звёзд 6":                    {name: "H", stars: 6, want: content.ErrStarsInvalid},
		"адрес 501 символ":           {name: "H", address: long(501), want: hotel.ErrAddressInvalid},
		"управляющий в адресе":       {name: "H", address: "a\u0000", want: hotel.ErrAddressInvalid},
		"описание 5001 символ":       {name: "H", description: long(5001), want: hotel.ErrDescriptionInvalid},
		"управляющий в описании":     {name: "H", description: "a\u007f", want: hotel.ErrDescriptionInvalid},
		"21 фото":                    {name: "H", photos: photoURLs(21), want: hotel.ErrPhotoURLsInvalid},
		"относительная ссылка":       {name: "H", photos: []string{"cdn/a.jpg"}, want: hotel.ErrPhotoURLsInvalid},
		"ссылка длиннее 2000":        {name: "H", photos: []string{"https://cdn.example.com/" + strings.Repeat("a", 1977)}, want: hotel.ErrPhotoURLsInvalid},
		"пустая ссылка среди верных": {name: "H", photos: []string{"https://cdn.example.com/a.jpg", ""}, want: hotel.ErrPhotoURLsInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := hotel.New(1, tc.name, tc.stars, tc.address, tc.description, tc.photos)

			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestHotelNew_BoundaryLengthsAccepted(t *testing.T) {
	url2000 := "https://cdn.example.com/" + strings.Repeat("a", 1976)
	require.Len(t, url2000, hotel.PhotoURLMaxLength)

	_, err := hotel.New(1, strings.Repeat("я", 200), 3, strings.Repeat("я", 500), strings.Repeat("я", 5000),
		append(photoURLs(19), url2000))

	assert.NoError(t, err, "длина считается в символах, границы включительно")
}

func TestHotelNewRoomCategory_Valid(t *testing.T) {
	c, err := hotel.NewRoomCategory(3, " Стандарт с видом на море ", "Две кровати", []string{"https://cdn.example.com/r.jpg"})

	require.NoError(t, err)
	assert.Equal(t, 3, c.HotelID)
	assert.Equal(t, "Стандарт с видом на море", c.Name)
	assert.Equal(t, "Две кровати", c.Description)
	assert.Equal(t, []string{"https://cdn.example.com/r.jpg"}, c.PhotoURLs)
}

func TestHotelNewRoomCategory_InvalidFields(t *testing.T) {
	for name, tc := range map[string]struct {
		name, description string
		photos            []string
		want              error
	}{
		"пустое название":          {name: " ", want: hotel.ErrRoomCategoryNameInvalid},
		"название 201 символ":      {name: strings.Repeat("я", 201), want: hotel.ErrRoomCategoryNameInvalid},
		"описание 2001 символ":     {name: "S", description: strings.Repeat("я", 2001), want: hotel.ErrRoomCategoryDescriptionInvalid},
		"управляющий в описании":   {name: "S", description: "\u0001", want: hotel.ErrRoomCategoryDescriptionInvalid},
		"ftp-ссылка":               {name: "S", photos: []string{"ftp://x/a.jpg"}, want: hotel.ErrPhotoURLsInvalid},
		"21 фото":                  {name: "S", photos: photoURLs(21), want: hotel.ErrPhotoURLsInvalid},
		"javascript вместо ссылки": {name: "S", photos: []string{"javascript:alert(1)"}, want: hotel.ErrPhotoURLsInvalid},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := hotel.NewRoomCategory(1, tc.name, tc.description, tc.photos)

			assert.ErrorIs(t, err, tc.want)
		})
	}
}

func TestFromDomainError_HotelNotFound_IsNotFound(t *testing.T) {
	err := apperror.FromDomainError(hotel.ErrNotFound)

	assert.True(t, errors.Is(err, apperror.ErrNotFound))
	assert.Contains(t, err.Error(), "hotel not found")
}
