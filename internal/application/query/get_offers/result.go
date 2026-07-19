package getoffers

import (
	"time"

	"api/internal/domain/enum"

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
	Status      enum.OfferStatus
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Result is what the Handler returns to the presentation layer.
type Result struct {
	Offers     []OfferResult
	TotalCount int64
}
