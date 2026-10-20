// Package getairline — авиакомпания справочника по id, включая скрытые из
// поиска (GET /api/v1/airlines/{id}).
package getairline

// Query — id запрошенной авиакомпании.
type Query struct {
	ID int
}
