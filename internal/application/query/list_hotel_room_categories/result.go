package listhotelroomcategories

// RoomCategoryResult — категория номера. Description — "" — не указано;
// PhotoURLs — всегда не nil.
type RoomCategoryResult struct {
	ID          int
	Name        string
	Description string
	PhotoURLs   []string
}

// Result — все категории отеля в порядке создания.
type Result struct {
	RoomCategories []RoomCategoryResult
}
