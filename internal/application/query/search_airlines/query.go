// Package searchairlines — поиск авиакомпаний справочника для выбора в
// сегменте перелёта (GET /api/v1/airlines).
package searchairlines

// Query — параметры поиска как пришли от клиента; нормализует и проверяет их
// Handler. Search nil — строка поиска не передана (допустимо только с IDs).
// Country — код страны ISO 3166-1 alpha-2 заглавными, формат уже проверен
// presentation ("" — без отбора); IDs — отбор по id без повторов (пусто — без
// отбора).
type Query struct {
	Search  *string
	Country string
	IDs     []int
	Limit   int
	Offset  int
}
