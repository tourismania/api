// Package searchhotelhttp — HTTP-граница поиска отелей справочника агентства
// GET /api/v1/hotels: разбор параметров, формат ответа и ошибок.
package searchhotelhttp

// SearchParams — параметры страницы, разобранные по форме: числа в границах
// (go-playground/validator; ошибка — с границей: {"Limit": "max=100"}).
// Длину строки поиска проверяет use case searchhotels.
type SearchParams struct {
	Limit  int `validate:"min=1,max=100"`
	Offset int `validate:"min=0,max=10000"`
}

// HotelResponse — отель справочника, без категорий номеров. id — им позиция
// оффера ссылается на отель справочника.
type HotelResponse struct {
	ID   int    `json:"id" example:"42"`
	Name string `json:"name" example:"Hilton Istanbul Bomonti"`
	// Stars — звёзды 0–5; 0 — без звёзд.
	Stars int `json:"stars" example:"5"`
	// Address — адрес текстом; "" — не указан.
	Address string `json:"address" example:"Şişli, İstanbul"`
	// Description — описание; "" — не указано.
	Description string `json:"description" example:"Отель в деловом районе"`
	// PhotoURLs — ссылки на фото в сохранённом порядке; всегда массив.
	PhotoURLs []string `json:"photo_urls" example:"https://cdn.example.com/a.jpg"`
}

// MetaResponse — параметры выдачи; search — уже нормализованная строка.
type MetaResponse struct {
	Total  int64  `json:"total" example:"1"`
	Limit  int    `json:"limit" example:"20"`
	Offset int    `json:"offset" example:"0"`
	Search string `json:"search" example:"hilton"`
}

// SearchResponse — тело успешного ответа.
type SearchResponse struct {
	Data []HotelResponse `json:"data"`
	Meta MetaResponse    `json:"meta"`
}
