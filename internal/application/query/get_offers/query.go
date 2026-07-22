// Package getoffers contains the "list offers" read-side use case
// (private endpoint — pagination + filters, always scoped to the
// caller's own agency).
package getoffers

import (
	"api/internal/domain/enum"

	"github.com/google/uuid"
)

// Query carries the requested filters plus the caller's immutable uuid.
// The handler resolves the caller's own agency from that uuid itself —
// the list is always scoped to it, regardless of role — ROLE_USER and
// agency staff see the same set of offers.
type Query struct {
	CurrentUserUUID uuid.UUID
	Status          *enum.OfferStatus
	CreatedBy       *int
	Limit           int
	Offset          int
}
