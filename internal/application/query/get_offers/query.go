// Package getoffers contains the "list offers" read-side use case
// (pagination + filters + read-side visibility by agency).
package getoffers

import "api/internal/domain/enum"

// Query carries the requested filters plus the caller's agency (if
// any), needed to compute read-side visibility. CurrentAgencyID is nil
// for anonymous/unauthenticated requests — published offers are visible
// to anyone.
type Query struct {
	AgencyID  *int
	Status    *enum.OfferStatus
	CreatedBy *int
	Limit     int
	Offset    int

	CurrentAgencyID *int
}
