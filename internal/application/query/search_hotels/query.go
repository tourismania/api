// Package searchhotels — поиск отелей справочника своего агентства по
// названию для выбора в диалоге отеля оффера (GET /api/v1/hotels).
package searchhotels

import "github.com/google/uuid"

// Query — параметры поиска как пришли от клиента; нормализует и проверяет их
// Handler. Search nil — строка поиска не передана. Агентство берётся из
// текущей записи пользователя CurrentUserUUID.
type Query struct {
	CurrentUserUUID uuid.UUID
	Search          *string
	Limit           int
	Offset          int
}
