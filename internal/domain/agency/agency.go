// Package agency is the travel-agency aggregate: the Agency entity, its
// lifecycle status, repository contract and management service.
package agency

import (
	"time"

	"github.com/google/uuid"
)

// Agency is the travel agency that owns offers. Agents (User with
// user.RoleAgent) belong to exactly one agency (1 user = 1 agency).
type Agency struct {
	ID        int
	UUID      uuid.UUID
	Name      string
	Status    Status
	CreatedAt time.Time
	// DeletedAt marks a soft-deleted agency; nil means active row.
	// Deactivation (Status = inactive) is distinct from soft delete.
	DeletedAt *time.Time
}

// IsActive reports whether the agency may currently own new offers or
// receive newly registered agents.
func (a Agency) IsActive() bool {
	return a.Status == StatusActive && a.DeletedAt == nil
}
