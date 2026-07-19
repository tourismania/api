package createoffer

import (
	"context"

	"api/internal/domain/service"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler executes the CreateOffer command by delegating to the domain
// OfferManager service. Keeping the handler thin preserves DDD: business
// invariants and ownership rules stay in the domain layer.
type Handler struct {
	offerManager *service.OfferManager
}

// NewHandler constructs the handler.
func NewHandler(offerManager *service.OfferManager) *Handler {
	return &Handler{offerManager: offerManager}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	offer, err := h.offerManager.Insert(ctx, cmd.Title, cmd.Description, cmd.Status, service.Actor{
		UserID:   cmd.CurrentUserID,
		AgencyID: cmd.CurrentAgencyID,
		Roles:    cmd.CurrentRoles,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{ID: offer.ID, UUID: offer.UUID}, nil
}
