// Package deleteoffer holds the DeleteOffer command, its handler and
// result.
package deleteoffer

import (
	"github.com/google/uuid"
)

// Command represents the intent to soft-delete an existing offer.
type Command struct {
	UUID uuid.UUID

	// Caller identity, resolved by presentation from JWT + DB. AgencyID
	// is required — every user belongs to exactly one agency.
	CurrentUserID int
	AgencyID      int
}
