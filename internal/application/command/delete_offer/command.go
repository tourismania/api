// Package deleteoffer holds the DeleteOffer command, its handler and
// result.
package deleteoffer

import (
	"github.com/google/uuid"
)

// Command represents the intent to soft-delete an existing offer. The
// caller is identified only by CurrentUserUUID; the handler resolves
// agency_id/role from the DB.
type Command struct {
	UUID uuid.UUID

	CurrentUserUUID uuid.UUID
}
