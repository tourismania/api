package getoffer

import (
	"time"

	"api/internal/domain/enum"

	"github.com/google/uuid"
)

// Result is the application-layer view of a single offer returned to
// the presentation layer.
type Result struct {
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
