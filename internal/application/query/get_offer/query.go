// Package getoffer contains the "single offer" read-side use case.
package getoffer

import (
	"api/internal/domain/enum"

	"github.com/google/uuid"
)

// Query carries the requested offer identifier plus the caller's
// identity, needed to compute read-side visibility.
type Query struct {
	UUID uuid.UUID

	CurrentUserID   int
	CurrentAgencyID *int
	CurrentRoles    []enum.Role
}
