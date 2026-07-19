package createoffer

import "github.com/google/uuid"

// Result returns the persisted identifiers of the new offer.
type Result struct {
	ID   int
	UUID uuid.UUID
}
