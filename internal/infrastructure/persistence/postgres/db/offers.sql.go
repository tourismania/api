package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const createOfferSQL = `INSERT INTO offers (
    uuid, title, description, agency_id, created_by, status, created_at, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8
) RETURNING id`

// CreateOfferParams matches the column order of createOfferSQL.
type CreateOfferParams struct {
	Uuid        uuid.UUID
	Title       string
	Description string
	AgencyID    int32
	CreatedBy   int32
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateOffer inserts a row and returns the generated id.
func (q *Queries) CreateOffer(ctx context.Context, arg CreateOfferParams) (int32, error) {
	var id int32
	err := q.db.QueryRow(ctx, createOfferSQL,
		arg.Uuid,
		arg.Title,
		arg.Description,
		arg.AgencyID,
		arg.CreatedBy,
		arg.Status,
		arg.CreatedAt,
		arg.UpdatedAt,
	).Scan(&id)
	return id, err
}

const getOfferByUUIDSQL = `SELECT id, uuid, title, description, agency_id, created_by, status, created_at, updated_at, deleted_at
FROM offers
WHERE uuid = $1 AND deleted_at IS NULL`

// GetOfferByUUID fetches a single non-deleted row by public identifier.
func (q *Queries) GetOfferByUUID(ctx context.Context, uid uuid.UUID) (Offer, error) {
	row := q.db.QueryRow(ctx, getOfferByUUIDSQL, uid)
	var o Offer
	err := row.Scan(
		&o.ID, &o.Uuid, &o.Title, &o.Description, &o.AgencyID, &o.CreatedBy,
		&o.Status, &o.CreatedAt, &o.UpdatedAt, &o.DeletedAt,
	)
	return o, err
}

const updateOfferSQL = `UPDATE offers
SET title = $2, description = $3, status = $4, updated_at = $5
WHERE uuid = $1 AND deleted_at IS NULL`

// UpdateOfferParams matches the column order of updateOfferSQL.
type UpdateOfferParams struct {
	Uuid        uuid.UUID
	Title       string
	Description string
	Status      string
	UpdatedAt   time.Time
}

// UpdateOffer persists title/description/status changes.
func (q *Queries) UpdateOffer(ctx context.Context, arg UpdateOfferParams) error {
	_, err := q.db.Exec(ctx, updateOfferSQL,
		arg.Uuid,
		arg.Title,
		arg.Description,
		arg.Status,
		arg.UpdatedAt,
	)
	return err
}

const softDeleteOfferSQL = `UPDATE offers SET deleted_at = NOW() WHERE uuid = $1 AND deleted_at IS NULL`

// SoftDeleteOffer marks a non-deleted offer row as deleted.
func (q *Queries) SoftDeleteOffer(ctx context.Context, uid uuid.UUID) error {
	_, err := q.db.Exec(ctx, softDeleteOfferSQL, uid)
	return err
}

// listOffersSQL is the hand-maintained positional-param version of
// queries/offers.sql (ListOffers). Parameter order: $1=agency_id
// (nullable), $2=status (nullable), $3=created_by (nullable), $4=limit,
// $5=offset. A NULL filter parameter means "no restriction".
const listOffersSQL = `SELECT id, uuid, title, description, agency_id, created_by, status, created_at, updated_at, deleted_at,
       COUNT(*) OVER() AS total_count
FROM offers
WHERE deleted_at IS NULL
  AND ($1::int     IS NULL OR agency_id  = $1)
  AND ($2::varchar IS NULL OR status     = $2)
  AND ($3::int     IS NULL OR created_by = $3)
ORDER BY created_at DESC, id DESC
LIMIT $4 OFFSET $5`

// ListOffersParams carries the bound parameters for ListOffers.
type ListOffersParams struct {
	AgencyID  *int32
	Status    *string
	CreatedBy *int32
	LimitVal  int32
	OffsetVal int32
}

// ListOffersRow is one row returned by ListOffers.
type ListOffersRow struct {
	ID          int32
	Uuid        uuid.UUID
	Title       string
	Description string
	AgencyID    int32
	CreatedBy   int32
	Status      string
	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
	TotalCount  int64
}

// ListOffers executes the filtered, paginated offer listing query.
func (q *Queries) ListOffers(ctx context.Context, arg ListOffersParams) ([]ListOffersRow, error) {
	rows, err := q.db.Query(ctx, listOffersSQL,
		arg.AgencyID,
		arg.Status,
		arg.CreatedBy,
		arg.LimitVal,
		arg.OffsetVal,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []ListOffersRow
	for rows.Next() {
		var row ListOffersRow
		if err := rows.Scan(
			&row.ID,
			&row.Uuid,
			&row.Title,
			&row.Description,
			&row.AgencyID,
			&row.CreatedBy,
			&row.Status,
			&row.CreatedAt,
			&row.UpdatedAt,
			&row.DeletedAt,
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
