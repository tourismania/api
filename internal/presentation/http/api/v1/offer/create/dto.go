// Package createofferhttp is the HTTP boundary for the CreateOffer
// command — request decoding, validation and response shaping.
package createofferhttp

import "github.com/google/uuid"

// CreateOfferRequest is the public request body schema. agency_id is
// deliberately absent: it is always derived server-side from the
// caller's own agency.
type CreateOfferRequest struct {
	Title       string `json:"title"       validate:"required,max=200"`
	Description string `json:"description" validate:"max=5000"`
	Status      string `json:"status"      validate:"required,oneof=draft ready published"`
}

// CreateOfferResponse is the response envelope.
type CreateOfferResponse struct {
	ID   int       `json:"id"`
	UUID uuid.UUID `json:"uuid"`
}
