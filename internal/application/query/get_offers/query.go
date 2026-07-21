// Package getoffers contains the "list offers" read-side use case
// (private endpoint — pagination + filters, always scoped to the
// caller's own agency).
package getoffers

import "api/internal/domain/enum"

// Query carries the requested filters plus the caller's own agency.
// AgencyID is required: the list is always scoped to it, regardless of
// role — ROLE_USER and agency staff see the same set of offers.
type Query struct {
	AgencyID  int
	Status    *enum.OfferStatus
	CreatedBy *int
	Limit     int
	Offset    int
}
