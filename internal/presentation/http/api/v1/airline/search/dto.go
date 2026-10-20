// Package searchairlinehttp — HTTP-граница поиска авиакомпаний
// GET /api/v1/airlines: разбор параметров, формат ответа и ошибок.
package searchairlinehttp

// SearchParams — границы пагинации и списка ids, которые проверяет
// go-playground/validator; ошибка отдаётся с границей: {"Limit": "max=100"}.
type SearchParams struct {
	Limit  int   `validate:"min=1,max=100"`
	Offset int   `validate:"min=0,max=10000"`
	IDs    []int `validate:"max=100"`
}

// CountryResponse — страна авиакомпании.
type CountryResponse struct {
	ISO2 string `json:"iso2" example:"RU"`
	Name string `json:"name" example:"Россия"`
}

// AirlineResponse — одна авиакомпания выдачи. id — им сегмент перелёта
// ссылается на авиакомпанию (поле airline_id).
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

// MetaResponse — параметры выдачи. Search, Country и IDs — null, если не
// переданы; Search — уже нормализованная строка, Country — заглавными.
type MetaResponse struct {
	Total   int64   `json:"total"`
	Limit   int     `json:"limit"`
	Offset  int     `json:"offset"`
	Search  *string `json:"search"`
	Country *string `json:"country"`
	IDs     []int   `json:"ids"`
}

// SearchResponse — тело успешного ответа.
type SearchResponse struct {
	Data []AirlineResponse `json:"data"`
	Meta MetaResponse      `json:"meta"`
}
