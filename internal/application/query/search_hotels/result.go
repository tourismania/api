package searchhotels

// HotelResult — один отель выдачи, без категорий номеров. Address и
// Description — "" — не указаны; PhotoURLs — всегда не nil.
type HotelResult struct {
	ID          int
	Name        string
	Stars       int
	Address     string
	Description string
	PhotoURLs   []string
}

// Result — страница выдачи, общее число найденных и нормализованная строка
// поиска.
type Result struct {
	Hotels     []HotelResult
	TotalCount int64
	Search     string
}
