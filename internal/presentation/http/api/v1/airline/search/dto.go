// Package searchairlinehttp — HTTP-граница поиска авиакомпаний
// GET /api/v1/airlines: разбор параметров, формат ответа и ошибок.
package searchairlinehttp

import airlinehttp "api/internal/presentation/http/api/v1/airline"

// SearchParams — параметры запроса, разобранные по форме: ids — список целых,
// limit/offset — числа в границах (go-playground/validator; ошибка — с
// границей: {"Limit": "max=100"}), country — строка как пришла ("" — не
// передан). Правила значений (формат и регистр кода страны, длина строки
// поиска) проверяет use case searchairlines.
type SearchParams struct {
	Limit   int   `validate:"min=1,max=100"`
	Offset  int   `validate:"min=0,max=10000"`
	IDs     []int `validate:"max=100"`
	Country string
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
	Data []airlinehttp.AirlineResponse `json:"data"`
	Meta MetaResponse                  `json:"meta"`
}
