// Package hotelhttp — общее для эндпоинтов справочника отелей агентства.
// Хендлеры — в подпакетах search (GET /api/v1/hotels) и room_categories
// (GET /api/v1/hotels/{id}/room-categories).
package hotelhttp

// CacheControl — справочник пополняется при сохранении оффера, поэтому
// ответы не кэшируются.
const CacheControl = "no-store"
