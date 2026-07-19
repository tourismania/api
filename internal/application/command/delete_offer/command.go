// Package deleteoffer holds the DeleteOffer command, its handler and
// result.
package deleteoffer

import (
	"api/internal/domain/enum"

	"github.com/google/uuid"
)

// Command represents the intent to soft-delete an existing offer.
type Command struct {
	UUID uuid.UUID

	// Caller identity, resolved by presentation from JWT + DB.
	CurrentUserID   int
	CurrentAgencyID *int
	CurrentRoles    []enum.Role
}
