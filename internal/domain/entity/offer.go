package entity

import (
	"time"

	"api/internal/domain/enum"

	"github.com/google/uuid"
)

// OfferTitleMaxLength is the maximum allowed length of Offer.Title.
const OfferTitleMaxLength = 200

// Offer is a travel-package showcase entry published by an agency.
// The final price is deliberately absent: it will be derived from child
// entities (flights/hotels/trips) added in a later iteration.
type Offer struct {
	ID          int
	UUID        uuid.UUID
	Title       string
	Description string
	// AgencyID is the owning agency (FK -> agencies.id). It determines
	// write access: an agent may only manage offers of their own agency.
	AgencyID int
	// CreatedBy is the author (FK -> users.id); audit only, does not
	// affect access control.
	CreatedBy int
	Status    enum.OfferStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	// DeletedAt marks a soft-deleted offer; nil means an active row.
	DeletedAt *time.Time
}

// IsPublished reports whether the offer is visible to plain clients.
func (o Offer) IsPublished() bool {
	return o.Status == enum.OfferStatusPublished
}
