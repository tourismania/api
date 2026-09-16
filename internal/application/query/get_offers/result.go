package getoffers

import (
	"api/internal/domain/offer"
	"time"

	"github.com/google/uuid"
)

// OfferResult is a single offer projection in the use-case response.
type OfferResult struct {
	ID          int
	UUID        uuid.UUID
	Title       string
	Description string
	AgencyID    int
	CreatedBy   int
	Status      offer.Status
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Result is what the Handler returns to the presentation layer.
type Result struct {
	Offers     []OfferResult
	TotalCount int64
}
