// Package roomcategorieshttp — HTTP-граница категорий номеров отеля
// справочника GET /api/v1/hotels/{id}/room-categories.
package roomcategorieshttp

// RoomCategoryResponse — категория номера отеля.
type RoomCategoryResponse struct {
	ID   int    `json:"id" example:"7"`
	Name string `json:"name" example:"Стандарт с видом на море"`
	// Description — описание; "" — не указано.
	Description string `json:"description" example:"Две отдельные кровати"`
	// PhotoURLs — ссылки на фото в сохранённом порядке; всегда массив.
	PhotoURLs []string `json:"photo_urls" example:"https://cdn.example.com/r.jpg"`
}

// ListResponse — тело успешного ответа: все категории в порядке создания.
type ListResponse struct {
	Data []RoomCategoryResponse `json:"data"`
}
