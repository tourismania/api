package getoffer

import (
	"context"
	"fmt"

	"api/internal/application/identity"
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

// Handler fetches a single offer for its own agency's staff/users: the
// caller sees it regardless of status as long as it belongs to their
// agency; any other agency's offer is reported as not found. Published
// offers of other agencies are served separately by get_published_offer.
// The caller's own agency is resolved from its uuid via
// application/identity, not presentation-layer middleware.
type Handler struct {
	offers OfferFinder
	users  identity.UserFinder
}

// NewHandler constructs the handler.
func NewHandler(offers OfferFinder, users identity.UserFinder) *Handler {
	return &Handler{offers: offers, users: users}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	actor, err := identity.Resolve(ctx, h.users, q.CurrentUserUUID)
	if err != nil {
		return Result{}, err
	}

	offer, err := h.offers.FindByUUID(ctx, q.UUID)
	if err != nil {
		return Result{}, fmt.Errorf("find offer: %w", err)
	}
	if offer == nil || offer.AgencyID != actor.AgencyID {
		// A different agency's offer is reported the same as not-found —
		// 1 user = 1 agency, no bypass, existence is never leaked.
		return Result{}, service.ErrOfferNotFound
	}

	return Result{
		ID:          offer.ID,
		UUID:        offer.UUID,
		Title:       offer.Title,
		Description: offer.Description,
		AgencyID:    offer.AgencyID,
		CreatedBy:   offer.CreatedBy,
		Status:      offer.Status,
		CreatedAt:   offer.CreatedAt,
		UpdatedAt:   offer.UpdatedAt,
	}, nil
}
