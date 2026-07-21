package getpublishedoffer

import (
	"context"
	"fmt"

	"api/internal/domain/entity"
	"api/internal/domain/service"

	"github.com/google/uuid"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// OfferFinder is the read-port consumed by this use-case. The concrete
// implementation lives in the infrastructure layer.
type OfferFinder interface {
	FindByUUID(ctx context.Context, id uuid.UUID) (*entity.Offer, error)
}

// Handler fetches a single offer and only ever returns it if published —
// draft/ready offers are reported as not found, regardless of agency.
type Handler struct {
	offers OfferFinder
}

// NewHandler constructs the handler.
func NewHandler(offers OfferFinder) *Handler {
	return &Handler{offers: offers}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	offer, err := h.offers.FindByUUID(ctx, q.UUID)
	if err != nil {
		return Result{}, fmt.Errorf("find offer: %w", err)
	}
	if offer == nil || !offer.IsPublished() {
		return Result{}, service.ErrOfferNotFound
	}

	return Result{
		ID:          offer.ID,
		UUID:        offer.UUID,
		Title:       offer.Title,
		Description: offer.Description,
		AgencyID:    offer.AgencyID,
		CreatedAt:   offer.CreatedAt,
		UpdatedAt:   offer.UpdatedAt,
	}, nil
}
