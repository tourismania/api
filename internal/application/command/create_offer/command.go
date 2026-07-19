// Package createoffer holds the CreateOffer command, its handler and
// result.
package createoffer

import "api/internal/domain/enum"

// Command represents the intent to publish a new offer under the
// caller's own agency. AgencyID is deliberately absent: it is always
// derived server-side from the caller's identity.
type Command struct {
	Title       string
	Description string
	Status      enum.OfferStatus

	// Caller identity, resolved by presentation from JWT + DB. The
	// domain layer never touches HTTP/JWT directly.
	CurrentUserID   int
	CurrentAgencyID *int
	CurrentRoles    []enum.Role
}
