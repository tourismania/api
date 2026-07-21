// Package getoffer contains the "single offer" read-side use case
// (private endpoint — the caller is always authenticated).
package getoffer

import "github.com/google/uuid"

// Query carries the requested offer identifier plus the caller's own
// agency. AgencyID is required: this use-case backs the private
// endpoint only, reachable exclusively by authenticated principals.
type Query struct {
	UUID     uuid.UUID
	AgencyID int
}
