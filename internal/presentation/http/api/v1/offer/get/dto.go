// Package getofferhttp is the HTTP boundary for the GetOffer query.
package getofferhttp

import (
	"time"

	"github.com/google/uuid"
)

// OfferResponse is the public projection of a single offer.
type OfferResponse struct {
	ID          int       `json:"id"`
	UUID        uuid.UUID `json:"uuid"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	AgencyID    int       `json:"agency_id"`
	CreatedBy   int       `json:"created_by"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
