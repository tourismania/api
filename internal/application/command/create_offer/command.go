// Package createoffer holds the CreateOffer command, its handler and
// result.
package createoffer

import "api/internal/domain/enum"

// Command represents the intent to publish a new offer under the
// caller's own agency. The request body never carries agency_id: it is
// always derived server-side from the authenticated caller.
type Command struct {
	Title       string
	Description string
	Status      enum.OfferStatus

	// Caller identity, resolved by presentation from JWT + DB. AgencyID
	// is required — every user belongs to exactly one agency. The
	// domain layer never touches HTTP/JWT directly.
	CurrentUserID int
	AgencyID      int
}
