package user

import (
	"context"

	"github.com/google/uuid"
)

// Repository persists and reads User aggregates. Store's *int return
// matches the original PHP signature: nil means "store did not produce
// an id" which the caller must treat as an error. Concrete
// implementations live in infrastructure.
type Repository interface {
	Store(ctx context.Context, user User, hashPassword string) (*int, error)

	// FindByUuid fetches a user record by its public identifier. Returns
	// (nil, nil) when no row matches — never a sentinel error — so
	// callers (e.g. user.Finder) decide what "not found" means for their
	// own use case.
	FindByUuid(ctx context.Context, id uuid.UUID) (*Record, error)
}
