// Package listoffershttp is the HTTP boundary for the GetOffers query
// (GET /api/v1/offers — pagination + filters).
package listoffershttp

import (
	"time"

	"github.com/google/uuid"
)

// ListOffersParams contains the query parameters for the offers list
// endpoint. Validation uses go-playground/validator; business rules
// (read-side visibility) are applied by the use-case.
type ListOffersParams struct {
	Status    string `validate:"omitempty,oneof=draft ready published"`
	CreatedBy int    `validate:"omitempty,gt=0"`
	Limit     int    `validate:"omitempty,min=1,max=100"`
	Offset    int    `validate:"omitempty,min=0,max=10000"`
}

// OfferResponse is the public projection of a single offer within the list.
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

// MetaResponse carries pagination metadata.
type MetaResponse struct {
	Total  int64 `json:"total"`
	Limit  int   `json:"limit"`
	Offset int   `json:"offset"`
}

// ListOffersResponse is the top-level JSON envelope for a successful response.
type ListOffersResponse struct {
	Data []OfferResponse `json:"data"`
	Meta MetaResponse    `json:"meta"`
}
