// Package getairline — авиакомпания справочника по id, в т. ч. дубль или
// пропавшая из источника, которых поиск не возвращает (GET /api/v1/airlines/{id}).
package getairline

// Query — id запрошенной авиакомпании.
type Query struct {
	ID int
}
