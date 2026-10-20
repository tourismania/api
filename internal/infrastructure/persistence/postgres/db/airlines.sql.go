package db

import (
	"context"
	"time"
)

// searchAirlinesSQL — версия queries/airlines.sql с позиционными параметрами
// (именованные @x → $x). Порядок: $1=search_prefix, $2=search_like,
// $3=country, $4=ids, $5=limit, $6=offset.
const searchAirlinesSQL = `WITH q AS (
    SELECT
        a.id,
        a.icao,
        a.name,
        a.name_ru,
        a.dissolved_on,
        co.iso2 AS country_iso2,
        co.name AS country_name,
        CASE
            WHEN lower(unaccent(a.name))    LIKE lower(unaccent($1::text))
              OR lower(unaccent(a.name_ru)) LIKE lower(unaccent($1::text)) THEN 0
            WHEN lower(unaccent(a.name))    LIKE lower(unaccent($2::text))
              OR lower(unaccent(a.name_ru)) LIKE lower(unaccent($2::text)) THEN 1
            ELSE 2
        END AS rank
    FROM airlines a
    LEFT JOIN countries co ON co.iso2 = a.country_iso2
    WHERE a.merged_into_id IS NULL
      AND a.missing_since IS NULL
      AND (
          lower(unaccent(a.name))    LIKE lower(unaccent($2::text))
          OR lower(unaccent(a.name_ru)) LIKE lower(unaccent($2::text))
          OR lower(unaccent(co.name))   LIKE lower(unaccent($2::text))
      )
      AND ($3::text = '' OR a.country_iso2 = $3::text)
      AND (cardinality($4::int[]) = 0 OR a.id = ANY($4::int[]))
)
SELECT id, icao, name, name_ru, dissolved_on, country_iso2, country_name, rank,
       COUNT(*) OVER () AS total_count
FROM q
ORDER BY (dissolved_on IS NOT NULL), rank, name, id
LIMIT $5::int
OFFSET $6::int`

// SearchAirlinesParams — параметры SearchAirlines.
type SearchAirlinesParams struct {
	SearchPrefix string
	SearchLike   string
	Country      string
	IDs          []int32
	LimitVal     int32
	OffsetVal    int32
}

// SearchAirlinesRow — одна строка SearchAirlines.
type SearchAirlinesRow struct {
	ID          int32
	Icao        *string
	Name        string
	NameRu      string
	DissolvedOn *time.Time
	CountryIso2 *string
	CountryName *string
	Rank        int32
	TotalCount  int64
}

// SearchAirlines ищет выбираемые авиакомпании с ранжированием и пагинацией.
func (q *Queries) SearchAirlines(ctx context.Context, arg SearchAirlinesParams) ([]SearchAirlinesRow, error) {
	ids := arg.IDs
	if ids == nil {
		// NULL в cardinality() дал бы NULL, а не 0 — без отбора нужен пустой массив.
		ids = []int32{}
	}
	rows, err := q.db.Query(ctx, searchAirlinesSQL,
		arg.SearchPrefix,
		arg.SearchLike,
		arg.Country,
		ids,
		arg.LimitVal,
		arg.OffsetVal,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []SearchAirlinesRow
	for rows.Next() {
		var row SearchAirlinesRow
		if err := rows.Scan(
			&row.ID,
			&row.Icao,
			&row.Name,
			&row.NameRu,
			&row.DissolvedOn,
			&row.CountryIso2,
			&row.CountryName,
			&row.Rank,
			&row.TotalCount,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

const listAirlinesByIDsSQL = `SELECT a.id, a.icao, a.name, a.name_ru, a.dissolved_on, a.iata_codes,
       co.iso2 AS country_iso2, co.name AS country_name,
       (a.merged_into_id IS NOT NULL OR a.missing_since IS NOT NULL) AS hidden
FROM airlines a
LEFT JOIN countries co ON co.iso2 = a.country_iso2
WHERE a.id = ANY($1::int[])`

// ListAirlinesByIDsRow — одна строка ListAirlinesByIDs.
type ListAirlinesByIDsRow struct {
	ID          int32
	Icao        *string
	Name        string
	NameRu      string
	DissolvedOn *time.Time
	IataCodes   []string
	CountryIso2 *string
	CountryName *string
	Hidden      bool
}

// ListAirlinesByIDs возвращает записи с перечисленными id, включая скрытые.
// Отсутствующих id в результате нет.
func (q *Queries) ListAirlinesByIDs(ctx context.Context, ids []int32) ([]ListAirlinesByIDsRow, error) {
	rows, err := q.db.Query(ctx, listAirlinesByIDsSQL, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ListAirlinesByIDsRow
	for rows.Next() {
		var row ListAirlinesByIDsRow
		if err := rows.Scan(
			&row.ID,
			&row.Icao,
			&row.Name,
			&row.NameRu,
			&row.DissolvedOn,
			&row.IataCodes,
			&row.CountryIso2,
			&row.CountryName,
			&row.Hidden,
		); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

const insertAirlineSQL = `INSERT INTO airlines (source_id, icao, name, name_ru, country_iso2, dissolved_on, iata_codes)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING id`

// InsertAirlineParams — параметры InsertAirline в порядке столбцов.
type InsertAirlineParams struct {
	SourceID    string
	Icao        *string
	Name        string
	NameRu      string
	CountryIso2 *string
	DissolvedOn *time.Time
	IataCodes   []string
}

// InsertAirline вставляет авиакомпанию источника и возвращает её id.
func (q *Queries) InsertAirline(ctx context.Context, arg InsertAirlineParams) (int32, error) {
	var id int32
	err := q.db.QueryRow(ctx, insertAirlineSQL,
		arg.SourceID,
		arg.Icao,
		arg.Name,
		arg.NameRu,
		arg.CountryIso2,
		arg.DissolvedOn,
		nonNilCodes(arg.IataCodes),
	).Scan(&id)
	return id, err
}

const updateAirlineSQL = `UPDATE airlines
SET icao = $2, name = $3, name_ru = $4, country_iso2 = $5,
    dissolved_on = $6, iata_codes = $7
WHERE id = $1 AND source_id IS NOT NULL`

// UpdateAirlineParams — параметры UpdateAirline.
type UpdateAirlineParams struct {
	ID          int32
	Icao        *string
	Name        string
	NameRu      string
	CountryIso2 *string
	DissolvedOn *time.Time
	IataCodes   []string
}

// UpdateAirline перезаписывает данные записи источника по id.
func (q *Queries) UpdateAirline(ctx context.Context, arg UpdateAirlineParams) error {
	_, err := q.db.Exec(ctx, updateAirlineSQL,
		arg.ID,
		arg.Icao,
		arg.Name,
		arg.NameRu,
		arg.CountryIso2,
		arg.DissolvedOn,
		nonNilCodes(arg.IataCodes),
	)
	return err
}

const listAirlineSourceIDsSQL = `SELECT id, source_id, merged_into_id IS NOT NULL AS merged, missing_since IS NOT NULL AS missing
FROM airlines
WHERE source_id IS NOT NULL
ORDER BY id`

// ListAirlineSourceIDsRow — одна строка ListAirlineSourceIDs.
type ListAirlineSourceIDsRow struct {
	ID       int32
	SourceID string
	Merged   bool
	Missing  bool
}

// ListAirlineSourceIDs возвращает все записи, загруженные из источника.
func (q *Queries) ListAirlineSourceIDs(ctx context.Context) ([]ListAirlineSourceIDsRow, error) {
	rows, err := q.db.Query(ctx, listAirlineSourceIDsSQL)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ListAirlineSourceIDsRow
	for rows.Next() {
		var row ListAirlineSourceIDsRow
		if err := rows.Scan(&row.ID, &row.SourceID, &row.Merged, &row.Missing); err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

const mergeAirlineSQL = `UPDATE airlines
SET merged_into_id = $2
WHERE (id = $1 OR merged_into_id = $1)
  AND id <> $2
  AND source_id IS NOT NULL`

// MergeAirline помечает fromID дублем toID и переводит на toID записи, уже
// слитые в fromID.
func (q *Queries) MergeAirline(ctx context.Context, fromID, toID int32) error {
	_, err := q.db.Exec(ctx, mergeAirlineSQL, fromID, toID)
	return err
}

const markAirlinesMissingSQL = `UPDATE airlines
SET missing_since = $2
WHERE id = ANY($1::int[]) AND missing_since IS NULL AND source_id IS NOT NULL`

// MarkAirlinesMissing помечает записи пропавшими из источника с даты since.
func (q *Queries) MarkAirlinesMissing(ctx context.Context, ids []int32, since time.Time) error {
	_, err := q.db.Exec(ctx, markAirlinesMissingSQL, ids, since)
	return err
}

const clearAirlinesMissingSQL = `UPDATE airlines
SET missing_since = NULL
WHERE id = ANY($1::int[]) AND missing_since IS NOT NULL`

// ClearAirlinesMissing снимает пометку «пропала из источника».
func (q *Queries) ClearAirlinesMissing(ctx context.Context, ids []int32) error {
	_, err := q.db.Exec(ctx, clearAirlinesMissingSQL, ids)
	return err
}

// nonNilCodes заменяет nil на пустой срез: pgx передаёт nil как NULL, а
// iata_codes — NOT NULL.
func nonNilCodes(codes []string) []string {
	if codes == nil {
		return []string{}
	}
	return codes
}
