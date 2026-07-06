package db

import (
	"context"
	"time"

	"github.com/google/uuid"
)

const createAgencySQL = `INSERT INTO agencies (
    uuid, name, status, created_at
) VALUES (
    $1, $2, $3, $4
) RETURNING id`

// CreateAgencyParams matches the column order of createAgencySQL.
type CreateAgencyParams struct {
	Uuid      uuid.UUID
	Name      string
	Status    string
	CreatedAt time.Time
}

// CreateAgency inserts a row and returns the generated id.
func (q *Queries) CreateAgency(ctx context.Context, arg CreateAgencyParams) (int32, error) {
	var id int32
	err := q.db.QueryRow(ctx, createAgencySQL,
		arg.Uuid,
		arg.Name,
		arg.Status,
		arg.CreatedAt,
	).Scan(&id)
	return id, err
}

const getAgencyByIDSQL = `SELECT id, uuid, name, status, created_at, deleted_at
FROM agencies
WHERE id = $1 AND deleted_at IS NULL`

// GetAgencyByID fetches a single non-deleted row by primary key.
func (q *Queries) GetAgencyByID(ctx context.Context, id int32) (Agency, error) {
	row := q.db.QueryRow(ctx, getAgencyByIDSQL, id)
	var a Agency
	err := row.Scan(&a.ID, &a.Uuid, &a.Name, &a.Status, &a.CreatedAt, &a.DeletedAt)
	return a, err
}

const setAgencyStatusSQL = `UPDATE agencies SET status = $2 WHERE id = $1`

// SetAgencyStatus updates the lifecycle status column.
func (q *Queries) SetAgencyStatus(ctx context.Context, id int32, status string) error {
	_, err := q.db.Exec(ctx, setAgencyStatusSQL, id, status)
	return err
}

const agencyExistsSQL = `SELECT EXISTS (
    SELECT 1 FROM agencies WHERE id = $1 AND deleted_at IS NULL
)`

// AgencyExists reports whether a non-deleted agency row exists for id.
func (q *Queries) AgencyExists(ctx context.Context, id int32) (bool, error) {
	var exists bool
	err := q.db.QueryRow(ctx, agencyExistsSQL, id).Scan(&exists)
	return exists, err
}
