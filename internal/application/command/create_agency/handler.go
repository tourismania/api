package createagency

import (
	"api/internal/domain/agency"
	"context"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler executes the CreateAgency command by delegating to the domain
// agency.Manager service. Keeping the handler thin preserves DDD: business
// invariants stay in the domain layer.
type Handler struct {
	agencyManager *agency.Manager
}

// NewHandler constructs the handler.
func NewHandler(agencyManager *agency.Manager) *Handler {
	return &Handler{agencyManager: agencyManager}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	agency, err := h.agencyManager.Create(ctx, cmd.Name)
	if err != nil {
		return Result{}, err
	}
	return Result{ID: agency.ID, UUID: agency.UUID}, nil
}
