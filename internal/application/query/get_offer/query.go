// Package getoffer contains the "single offer" read-side use case.
package getoffer

import "github.com/google/uuid"

// Query carries the requested offer identifier plus the caller's agency
// (if any), needed to compute read-side visibility. CurrentAgencyID is
// nil for anonymous/unauthenticated requests — published offers are
// visible to anyone.
type Query struct {
	UUID uuid.UUID

	CurrentAgencyID *int
}
