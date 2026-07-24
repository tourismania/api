// Package updateofferhttp is the HTTP boundary for the UpdateOffer
// command — request decoding, validation and response shaping.
package updateofferhttp

import "github.com/google/uuid"

// UpdateOfferRequest is the public request body schema. Every field is
// optional: only non-nil fields are applied (partial update).
type UpdateOfferRequest struct {
	Title       *string `json:"title"       validate:"omitempty,max=200"`
	Description *string `json:"description" validate:"omitempty,max=5000"`
	Status      *string `json:"status"      validate:"omitempty,oneof=draft ready published"`
}

// UpdateOfferResponse is the response envelope.
type UpdateOfferResponse struct {
	ID   int       `json:"id"`
	UUID uuid.UUID `json:"uuid"`
}
