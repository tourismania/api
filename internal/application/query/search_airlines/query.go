// Package searchairlines — поиск авиакомпаний справочника для выбора в
// сегменте перелёта (GET /api/v1/airlines).
package searchairlines

// Query — уже нормализованные параметры поиска. Search — подстрока
// названия авиакомпании или страны ("" — только вместе с IDs); CountryISO2 —
// код страны заглавными ("" — без отбора); IDs — отбор по id без повторов.
type Query struct {
	Search      string
	CountryISO2 string
	IDs         []int
	Limit       int
	Offset      int
}
