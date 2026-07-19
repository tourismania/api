// Package updateoffer holds the UpdateOffer command, its handler and
// result.
package updateoffer

import (
	"api/internal/domain/enum"

	"github.com/google/uuid"
)

// Command represents the intent to partially update an existing offer.
// Only non-nil fields are applied.
type Command struct {
	UUID        uuid.UUID
	Title       *string
	Description *string
	Status      *enum.OfferStatus

	// Caller identity, resolved by presentation from JWT + DB.
	CurrentUserID   int
	CurrentAgencyID *int
	CurrentRoles    []enum.Role
}
