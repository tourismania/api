package createagency

import "github.com/google/uuid"

// Result returns the persisted identifiers of the new agency.
type Result struct {
	ID   int
	UUID uuid.UUID
}
