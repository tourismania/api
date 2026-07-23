// Package createoffer holds the CreateOffer command, its handler and
// result.
package createoffer

import (
	"api/internal/domain/enum"

	"github.com/google/uuid"
)

// Command represents the intent to publish a new offer under the
// caller's own agency. The request body never carries agency_id: the
// caller is identified only by CurrentUserUUID (the immutable uuid
// carried in the JWT); the handler resolves agency_id and role from the
// DB itself, so they always reflect the latest state rather than a
// value trusted from presentation.
type Command struct {
	Title       string
	Description string
	Status      enum.OfferStatus

	CurrentUserUUID uuid.UUID
}
