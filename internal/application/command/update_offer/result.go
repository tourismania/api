package updateoffer

import "github.com/google/uuid"

// Result returns the identifiers of the updated offer.
type Result struct {
	ID   int
	UUID uuid.UUID
}
