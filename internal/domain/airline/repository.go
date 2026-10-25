package airline

import (
	"context"
	"time"
)

// Repository — порт справочника авиакомпаний; реализация — в
// infrastructure/persistence.
type Repository interface {
	// Search ищет выбираемые авиакомпании (без дублей и пропавших) по
	// фильтру с ранжированием: действующие выше прекративших работу, внутри —
	// начало названия, подстрока названия, только страна.
	Search(ctx context.Context, f Filter) (SearchResult, error)
	// FindByIDs возвращает авиакомпании с перечисленными id, включая
	// невыбираемые (дубли и пропавшие — с MergedIntoID / MissingSince).
	// Отсутствующих id в результате просто нет — вызывающий сравнивает
	// результат со списком сам.
	FindByIDs(ctx context.Context, ids []int) ([]Airline, error)
	// Insert добавляет авиакомпанию источника с идентификатором sourceID и
	// возвращает id новой записи. a.ID, a.MergedIntoID и a.MissingSince не
	// учитываются.
	Insert(ctx context.Context, sourceID string, a Airline) (int, error)
	// Update перезаписывает коды, названия, страну и дату прекращения
	// деятельности записи id; id и состояние в источнике не меняются.
	Update(ctx context.Context, id int, a Airline) error
	// ListSourceIDs возвращает все записи, загруженные из источника, по
	// идентификатору в источнике. Обычно у source_id одна запись; несколько —
	// нарушение, которое синхронизация разбирает до записи.
	ListSourceIDs(ctx context.Context) (map[string][]SourceRecord, error)
	// MarkMerged помечает запись fromID дублем записи toID; записи, уже
	// слитые в fromID, тоже переводятся на toID (цепочка не растёт). Сегменты
	// перелётов, ссылающиеся на fromID, не меняются.
	MarkMerged(ctx context.Context, fromID, toID int) error
	// MarkMissing помечает записи ids пропавшими из источника с даты since;
	// у уже пропавших дата не меняется.
	MarkMissing(ctx context.Context, ids []int, since time.Time) error
	// ClearMissing снимает пометку «пропала из источника» с записей ids.
	ClearMissing(ctx context.Context, ids []int) error
}
