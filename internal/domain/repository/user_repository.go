// Package repository declares the persistence contracts owned by the
// domain. Concrete implementations live in infrastructure.
package repository

import (
	"context"

	"api/internal/domain/entity"

	"github.com/google/uuid"
)

// UserRepository persists and reads User aggregates. Store's *int return
// matches the original PHP signature: nil means "store did not produce
// an id" which the caller must treat as an error.
type UserRepository interface {
	Store(ctx context.Context, user entity.User, hashPassword string) (*int, error)

	// FindByUuid fetches a user record by its public identifier. Returns
	// (nil, nil) when no row matches — never a sentinel error — so
	// callers (e.g. service.UserFinder) decide what "not found" means
	// for their own use case.
	FindByUuid(ctx context.Context, id uuid.UUID) (*entity.UserRecord, error)
}
