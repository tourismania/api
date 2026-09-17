package activateagency

import (
	"api/internal/domain/agency"
	"context"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler executes the ActivateAgency command by delegating to the
// domain agency.Manager service.
type Handler struct {
	agencyManager *agency.Manager
}

// NewHandler constructs the handler.
func NewHandler(agencyManager *agency.Manager) *Handler {
	return &Handler{agencyManager: agencyManager}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	if err := h.agencyManager.Activate(ctx, cmd.ID); err != nil {
		return Result{}, err
	}
	return Result{}, nil
}
