// Package getoffers contains the "list offers" read-side use case
// (pagination + filters + read-side visibility by role).
package getoffers

import "api/internal/domain/enum"

// Query carries the requested filters plus the caller's identity, needed
// to compute read-side visibility.
type Query struct {
	AgencyID  *int
	Status    *enum.OfferStatus
	CreatedBy *int
	Limit     int
	Offset    int

	CurrentUserID   int
	CurrentAgencyID *int
	CurrentRoles    []enum.Role
}
