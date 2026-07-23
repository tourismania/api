package repository

import (
	"context"

	"api/internal/domain/entity"
	"api/internal/domain/enum"

	"github.com/google/uuid"
)

// OfferFilter narrows OfferRepository.List. Nil pointer fields mean
// "no restriction on this column".
type OfferFilter struct {
	AgencyID  *int
	Status    *enum.OfferStatus
	CreatedBy *int
	Limit     int
	Offset    int
}

// OfferListResult is the paginated read-side projection.
type OfferListResult struct {
	Offers     []entity.Offer
	TotalCount int64
}

// OfferRepository persists Offer aggregates. 1 entity = 1 repository.
// Every read filters deleted_at IS NULL.
type OfferRepository interface {
	// Store inserts a new offer and returns its id.
	Store(ctx context.Context, o entity.Offer) (int, error)
	// FindByUUID fetches a non-deleted offer by its public identifier.
	// Returns nil if not found.
	FindByUUID(ctx context.Context, id uuid.UUID) (*entity.Offer, error)
	// List returns a filtered, paginated page of non-deleted offers.
	List(ctx context.Context, f OfferFilter) (OfferListResult, error)
	// Update persists changes to an existing offer (title/description/status).
	Update(ctx context.Context, o entity.Offer) error
	// SoftDelete marks the offer as deleted (sets deleted_at).
	SoftDelete(ctx context.Context, id uuid.UUID) error
}
