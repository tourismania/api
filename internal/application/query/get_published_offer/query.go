// Package getpublishedoffer contains the "single published offer"
// read-side use case backing the fully anonymous public endpoint — the
// link an agent shares with a client. No identity is involved.
package getpublishedoffer

import "github.com/google/uuid"

// Query carries the requested offer identifier. There is no identity:
// this use-case only ever returns published offers.
type Query struct {
	UUID uuid.UUID
}
