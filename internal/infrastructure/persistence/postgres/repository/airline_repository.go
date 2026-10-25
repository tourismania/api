package repository

import (
	"context"
	"fmt"
	"time"

	"api/internal/domain/airline"
	"api/internal/infrastructure/persistence/helpers"
	"api/internal/infrastructure/persistence/postgres/db"
	"api/internal/infrastructure/persistence/postgres/mapper"
)

// Compile-time interface check.
var _ airline.Repository = (*AirlineRepository)(nil)

// AirlineRepository хранит справочник авиакомпаний в Postgres через слой db.
type AirlineRepository struct {
	queries *db.Queries
}

// NewAirlineRepository создаёт репозиторий поверх слоя запросов.
func NewAirlineRepository(queries *db.Queries) *AirlineRepository {
	return &AirlineRepository{queries: queries}
}

// Search ищет выбираемые авиакомпании; строка поиска сравнивается буквально
// (EscapeLike), пустая строка — без отбора по названию.
func (r *AirlineRepository) Search(ctx context.Context, f airline.Filter) (airline.SearchResult, error) {
	escaped := helpers.EscapeLike(f.Search)
	rows, err := queriesFor(ctx, r.queries).SearchAirlines(ctx, db.SearchAirlinesParams{
		SearchPrefix: escaped + "%",
		SearchLike:   "%" + escaped + "%",
		Country:      f.CountryISO2,
		IDs:          toInt32s(f.IDs),
		LimitVal:     int32(f.Limit),
		OffsetVal:    int32(f.Offset),
	})
	if err != nil {
		return airline.SearchResult{}, fmt.Errorf("search airlines: %w", err)
	}

	airlines := make([]airline.Airline, 0, len(rows))
	var total int64
	for _, row := range rows {
		total = row.TotalCount
		airlines = append(airlines, mapper.ToAirlineDomainFromSearchRow(row))
	}
	return airline.SearchResult{Airlines: airlines, TotalCount: total}, nil
}

// FindByIDs возвращает записи с перечисленными id, включая скрытые.
func (r *AirlineRepository) FindByIDs(ctx context.Context, ids []int) ([]airline.Airline, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := queriesFor(ctx, r.queries).ListAirlinesByIDs(ctx, toInt32s(ids))
	if err != nil {
		return nil, fmt.Errorf("find airlines by id: %w", err)
	}
	airlines := make([]airline.Airline, 0, len(rows))
	for _, row := range rows {
		airlines = append(airlines, mapper.ToAirlineDomainFromIDRow(row))
	}
	return airlines, nil
}

// Insert добавляет авиакомпанию источника и возвращает id новой записи.
func (r *AirlineRepository) Insert(ctx context.Context, sourceID string, a airline.Airline) (int, error) {
	id, err := queriesFor(ctx, r.queries).InsertAirline(ctx, db.InsertAirlineParams{
		SourceID:    sourceID,
		Icao:        a.ICAO,
		Name:        a.Name,
		NameRu:      a.NameRU,
		CountryIso2: countryISO2(a.Country),
		DissolvedOn: a.DissolvedOn,
		IataCodes:   a.IATACodes,
	})
	if err != nil {
		return 0, fmt.Errorf("insert airline: %w", err)
	}
	return int(id), nil
}

// Update перезаписывает данные записи id, сохраняя её id и состояние в
// источнике.
func (r *AirlineRepository) Update(ctx context.Context, id int, a airline.Airline) error {
	if err := queriesFor(ctx, r.queries).UpdateAirline(ctx, db.UpdateAirlineParams{
		ID:          int32(id),
		Icao:        a.ICAO,
		Name:        a.Name,
		NameRu:      a.NameRU,
		CountryIso2: countryISO2(a.Country),
		DissolvedOn: a.DissolvedOn,
		IataCodes:   a.IATACodes,
	}); err != nil {
		return fmt.Errorf("update airline %d: %w", id, err)
	}
	return nil
}

// ListSourceIDs возвращает записи источника по их идентификатору в источнике.
func (r *AirlineRepository) ListSourceIDs(ctx context.Context) (map[string][]airline.SourceRecord, error) {
	rows, err := queriesFor(ctx, r.queries).ListAirlineSourceIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("list airline source ids: %w", err)
	}
	out := make(map[string][]airline.SourceRecord, len(rows))
	for _, row := range rows {
		out[row.SourceID] = append(out[row.SourceID], airline.SourceRecord{
			ID:      int(row.ID),
			Merged:  row.Merged,
			Missing: row.Missing,
		})
	}
	return out, nil
}

// MarkMerged помечает fromID дублем toID.
func (r *AirlineRepository) MarkMerged(ctx context.Context, fromID, toID int) error {
	if err := queriesFor(ctx, r.queries).MergeAirline(ctx, int32(fromID), int32(toID)); err != nil {
		return fmt.Errorf("merge airline %d into %d: %w", fromID, toID, err)
	}
	return nil
}

// MarkMissing помечает записи пропавшими из источника с даты since.
func (r *AirlineRepository) MarkMissing(ctx context.Context, ids []int, since time.Time) error {
	if len(ids) == 0 {
		return nil
	}
	if err := queriesFor(ctx, r.queries).MarkAirlinesMissing(ctx, toInt32s(ids), since); err != nil {
		return fmt.Errorf("mark airlines missing: %w", err)
	}
	return nil
}

// ClearMissing снимает пометку «пропала из источника».
func (r *AirlineRepository) ClearMissing(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}
	if err := queriesFor(ctx, r.queries).ClearAirlinesMissing(ctx, toInt32s(ids)); err != nil {
		return fmt.Errorf("clear airlines missing: %w", err)
	}
	return nil
}

// countryISO2 — код страны авиакомпании или nil.
func countryISO2(c *airline.Country) *string {
	if c == nil {
		return nil
	}
	iso2 := c.ISO2
	return &iso2
}

// toInt32s конвертирует id в тип столбцов int.
func toInt32s(ids []int) []int32 {
	out := make([]int32, len(ids))
	for i, id := range ids {
		out[i] = int32(id)
	}
	return out
}
