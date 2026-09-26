package offer

import (
	"context"

	"github.com/google/uuid"
)

// Filter narrows Repository.List. Nil pointer fields mean
// "no restriction on this column".
type Filter struct {
	AgencyID  *int
	Status    *Status
	CreatedBy *int
	Limit     int
	Offset    int
}

// ListResult is the paginated read-side projection.
type ListResult struct {
	Offers     []Offer
	TotalCount int64
}

// Repository persists Offer aggregates. 1 entity = 1 repository.
// Every read filters deleted_at IS NULL.
type Repository interface {
	// Store inserts a new offer and returns its id.
	Store(ctx context.Context, o Offer) (int, error)
	// FindByUUID fetches a non-deleted offer by its public identifier.
	// Returns nil if not found.
	FindByUUID(ctx context.Context, id uuid.UUID) (*Offer, error)
	// List returns a filtered, paginated page of non-deleted offers.
	List(ctx context.Context, f Filter) (ListResult, error)
	// Update persists changes to an existing offer (title/description/status).
	Update(ctx context.Context, o Offer) error
	// SoftDelete marks the offer as deleted (sets deleted_at).
	SoftDelete(ctx context.Context, id uuid.UUID) error
}
