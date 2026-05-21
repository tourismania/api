package db

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const createUserSQL = `INSERT INTO "users" (
    uuid, first_name, last_name, email, login,
    created_at, updated_at, phone, password, is_active,
    birthday, extra_information, roles
) VALUES (
    $1, $2, $3, $4, $5, NOW(), NOW(), $6, $7, $8, $9, $10, $11
) RETURNING id`

// CreateUserParams matches the column order of createUserSQL.
type CreateUserParams struct {
	Uuid             uuid.UUID
	FirstName        *string
	LastName         *string
	Email            string
	Login            string
	Phone            *string
	Password         string
	IsActive         bool
	Birthday         *time.Time
	ExtraInformation []byte
	Roles            []string
}

// CreateUser inserts a row and returns the generated id.
func (q *Queries) CreateUser(ctx context.Context, arg CreateUserParams) (int32, error) {
	var id int32
	err := q.db.QueryRow(ctx, createUserSQL,
		arg.Uuid,
		arg.FirstName,
		arg.LastName,
		arg.Email,
		arg.Login,
		arg.Phone,
		arg.Password,
		arg.IsActive,
		arg.Birthday,
		arg.ExtraInformation,
		arg.Roles,
	).Scan(&id)
	return id, err
}

const getUserByEmailSQL = `SELECT id, uuid, first_name, last_name, email, login,
       created_at, updated_at, phone, password, is_active, birthday,
       extra_information, roles
FROM "users"
WHERE email = $1`

// scanRow scans a full user row into u. The uuid column is received as a
// plain string so we are independent of pgx type-map quirks; uuid.Parse
// then produces the canonical [16]byte value.
func scanRow(row interface {
	Scan(...any) error
}, u *User) error {
	var uuidStr string
	err := row.Scan(
		&u.ID, &uuidStr, &u.FirstName, &u.LastName, &u.Email, &u.Login,
		&u.CreatedAt, &u.UpdatedAt, &u.Phone, &u.Password, &u.IsActive,
		&u.Birthday, &u.ExtraInformation, &u.Roles,
	)
	if err != nil {
		return err
	}
	parsed, err := uuid.Parse(uuidStr)
	if err != nil {
		return fmt.Errorf("parse uuid %q: %w", uuidStr, err)
	}
	u.Uuid = parsed
	return nil
}

// GetUserByEmail fetches a single row by unique email.
func (q *Queries) GetUserByEmail(ctx context.Context, email string) (User, error) {
	row := q.db.QueryRow(ctx, getUserByEmailSQL, email)
	var u User
	return u, scanRow(row, &u)
}

const getUserByUuidSQL = `SELECT id, uuid, first_name, last_name, email, login,
       created_at, updated_at, phone, password, is_active, birthday,
       extra_information, roles
FROM "users"
WHERE uuid = $1`

// GetUserByUuid fetches a single row by uuid.
func (q *Queries) GetUserByUuid(ctx context.Context, id uuid.UUID) (User, error) {
	row := q.db.QueryRow(ctx, getUserByUuidSQL, id)
	var u User
	return u, scanRow(row, &u)
}
