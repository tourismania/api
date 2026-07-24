// Package getpublicofferhttp is the HTTP boundary for the
// GetPublishedOffer query (GET /api/v1/public/offers/{uuid} — the fully
// anonymous "share link" endpoint).
package getpublicofferhttp

import (
	"time"

	"github.com/google/uuid"
)

// OfferResponse is the public projection of a published offer. It is
// deliberately narrower than the private OfferResponse: no created_by
// (audit-only, internal) and no status (this endpoint only ever returns
// published offers).
type OfferResponse struct {
	ID          int       `json:"id"`
	UUID        uuid.UUID `json:"uuid"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	AgencyID    int       `json:"agency_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
