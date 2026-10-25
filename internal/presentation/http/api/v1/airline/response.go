// Package airlinehttp — общие для эндпоинтов справочника авиакомпаний формы
// ответа: элемент авиакомпании и её страна. Хендлеры — в подпакетах search
// (GET /api/v1/airlines) и get (GET /api/v1/airlines/{id}).
package airlinehttp

import "time"

// dateLayout — формат dissolved_on в ответе.
const dateLayout = "2006-01-02"

// CacheControl — ответы справочника можно кэшировать в браузере час.
const CacheControl = "private, max-age=3600"

// CountryResponse — страна авиакомпании.
type CountryResponse struct {
	ISO2 string `json:"iso2" example:"RU"`
	Name string `json:"name" example:"Россия"`
}

// AirlineResponse — авиакомпания. id — им сегмент перелёта ссылается на
// авиакомпанию (поле airline_id). Коды IATA не отдаются.
type AirlineResponse struct {
	ID int `json:"id" example:"42"`
	// ICAO — код ICAO; null только у служебной записи «Неизвестная» (id 1).
	ICAO *string `json:"icao" example:"AFL"`
	// Name — английское название.
	Name string `json:"name" example:"Aeroflot"`
	// NameRU — русское название, только у авиакомпаний России; поле
	// присутствует всегда, "" — русского названия нет.
	NameRU string `json:"name_ru" example:"Аэрофлот"`
	// Country — страна или null, если неизвестна.
	Country *CountryResponse `json:"country"`
	// DissolvedOn — дата прекращения деятельности YYYY-MM-DD; null — действует.
	DissolvedOn *string `json:"dissolved_on" example:"2015-10-26"`
}

// FormatDate — дата в формате ответа YYYY-MM-DD; nil — даты нет.
func FormatDate(d *time.Time) *string {
	if d == nil {
		return nil
	}
	s := d.Format(dateLayout)
	return &s
}
