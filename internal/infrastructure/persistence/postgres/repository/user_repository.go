// Package repository contains pgx-backed adapters implementing the
// domain repository interfaces.
package repository

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"time"

	"api/internal/domain/entity"
	domainrepo "api/internal/domain/repository"
	"api/internal/infrastructure/persistence/postgres/db"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// Compile-time check: UserRepository must satisfy the UserFinder port
// expected by the get_me use-case. Declared here to avoid an import
// cycle (application ← infrastructure is not allowed).
var _ interface {
	FindByUuid(ctx context.Context, uuid uuid.UUID) (*entity.UserRecord, error)
} = (*UserRepository)(nil)

const (
	defaultPhone    = "799999999"
	birthdayYear    = 1994
	defaultRoleUser = "ROLE_USER"
)

// UserRepository persists domain.User aggregates via pgx/sqlc. It is the
// only place that knows about both the domain entity and the row model.
type UserRepository struct {
	queries *db.Queries
}

// NewUserRepository wires the queries layer.
func NewUserRepository(queries *db.Queries) *UserRepository {
	return &UserRepository{queries: queries}
}

// Ensure compile-time interface compliance.
var _ domainrepo.UserRepository = (*UserRepository)(nil)

// Store materialises a domain user into the canonical row shape and
// inserts it. Defaults mirror the original Doctrine code: UUID is
// generated, login defaults to email, phone is the placeholder
// "799999999", birthday gets a deterministic 1994/random-day stamp.
func (r *UserRepository) Store(
	ctx context.Context,
	user entity.User,
	hashPassword string,
) (*int, error) {
	if hashPassword == "" {
		return nil, errors.New("hashPassword is required")
	}

	firstName := nullable(user.FirstName)
	lastName := nullable(user.LastName)
	phone := defaultPhone
	birthday := randomBirthday()

	id, err := r.queries.CreateUser(ctx, db.CreateUserParams{
		Uuid:             uuid.New(),
		FirstName:        firstName,
		LastName:         lastName,
		Email:            user.Email,
		Login:            user.Email,
		Phone:            &phone,
		Password:         hashPassword,
		IsActive:         true,
		Birthday:         &birthday,
		ExtraInformation: []byte("{}"),
		Roles:            []string{defaultRoleUser},
		AgencyID:         intToInt32Ptr(user.AgencyID),
	})
	if err != nil {
		// Database not reachable or no row inserted — bubble up as a
		// regular error; the domain treats a nil-id-without-error as
		// "rejected", but pgx surfaces failures here as concrete errors.
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("insert user: %w", err)
	}
	out := int(id)
	return &out, nil
}

// FindByUuid fetches a user record by primary key. Satisfies the UserFinder
// port defined in the get_me application package.
func (r *UserRepository) FindByUuid(ctx context.Context, uuid uuid.UUID) (*entity.UserRecord, error) {
	u, err := r.queries.GetUserByUuid(ctx, uuid)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, fmt.Errorf("user %s not found", uuid.String())
		}
		return nil, fmt.Errorf("find user by uuid: %w", err)
	}
	return &entity.UserRecord{
		Uuid:      u.Uuid,
		Email:     u.Email,
		Phone:     derefStr(u.Phone),
		FirstName: derefStr(u.FirstName),
		LastName:  derefStr(u.LastName),
		Roles:     u.Roles,
		AgencyID:  int32ToIntPtr(u.AgencyID),
	}, nil
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func derefStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

func intToInt32Ptr(v *int) *int32 {
	if v == nil {
		return nil
	}
	out := int32(*v)
	return &out
}

func int32ToIntPtr(v *int32) *int {
	if v == nil {
		return nil
	}
	out := int(*v)
	return &out
}

// randomBirthday picks a deterministic year (1994) and a random valid
// month/day, matching the PHP fixture behaviour used in the original
// Symfony repository.
func randomBirthday() time.Time {
	// Days-in-month table (non-leap). Pick month first, then day.
	monthDays := [...]int{31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	month := rand.IntN(12)             //nolint:gosec // non-cryptographic
	day := rand.IntN(monthDays[month]) //nolint:gosec
	return time.Date(birthdayYear, time.Month(month+1), day+1, 0, 0, 0, 0, time.UTC)
}
