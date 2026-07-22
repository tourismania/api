// Package updateoffer holds the UpdateOffer command, its handler and
// result.
package updateoffer

import (
	"api/internal/domain/enum"

	"github.com/google/uuid"
)

// Command represents the intent to partially update an existing offer.
// Only non-nil fields are applied. The caller is identified only by
// CurrentUserUUID; the handler resolves agency_id/role from the DB.
type Command struct {
	UUID        uuid.UUID
	Title       *string
	Description *string
	Status      *enum.OfferStatus

	CurrentUserUUID uuid.UUID
}
