package hotel

import "context"

// Filter — нормализованные параметры поиска отелей: агентство, строка
// поиска (подстрока названия, сравнивается буквально) и страница.
type Filter struct {
	AgencyID int
	Search   string
	Limit    int
	Offset   int
}

// SearchResult — страница найденных отелей и их общее число без учёта
// пагинации.
type SearchResult struct {
	Hotels     []Hotel
	TotalCount int64
}

// Repository — порт справочника отелей; реализация — в
// infrastructure/persistence. Все методы чтения не видят записей с отметкой
// удаления, а категории удалённого отеля скрыты вместе с ним.
type Repository interface {
	// Search ищет отели агентства по подстроке названия без учёта регистра:
	// сначала названия, начинающиеся со строки, затем остальные; внутри — по
	// названию, затем по id.
	Search(ctx context.Context, f Filter) (SearchResult, error)
	// FindByIDsAndAgency возвращает отели агентства agencyID с перечисленными
	// id. Чужих и отсутствующих в результате просто нет — вызывающий
	// сравнивает результат со списком сам.
	FindByIDsAndAgency(ctx context.Context, ids []int, agencyID int) ([]Hotel, error)
	// ListRoomCategories возвращает категории отеля hotelID в порядке
	// создания. Агентство отеля не проверяется — это делает вызывающий.
	ListRoomCategories(ctx context.Context, hotelID int) ([]RoomCategory, error)
	// FindRoomCategoriesByIDs возвращает категории с перечисленными id;
	// принадлежность отелю вызывающий проверяет по HotelID.
	FindRoomCategoriesByIDs(ctx context.Context, ids []int) ([]RoomCategory, error)
	// Insert добавляет отель и возвращает его id; h.ID не учитывается.
	Insert(ctx context.Context, h Hotel) (int, error)
	// InsertRoomCategory добавляет категорию и возвращает её id; c.ID не
	// учитывается.
	InsertRoomCategory(ctx context.Context, c RoomCategory) (int, error)
}
