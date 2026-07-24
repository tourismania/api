package getpublishedoffer

import (
	"time"

	"github.com/google/uuid"
)

// Result is the application-layer view of a published offer returned to
// the presentation layer.
type Result struct {
	ID          int
	UUID        uuid.UUID
	Title       string
	Description string
	AgencyID    int
	CreatedAt   time.Time
	UpdatedAt   time.Time
}
