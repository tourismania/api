package deactivateagency

import (
	"context"

	"api/internal/domain/service"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler executes the DeactivateAgency command by delegating to the
// domain AgencyManager service.
type Handler struct {
	agencyManager *service.AgencyManager
}

// NewHandler constructs the handler.
func NewHandler(agencyManager *service.AgencyManager) *Handler {
	return &Handler{agencyManager: agencyManager}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	if err := h.agencyManager.Deactivate(ctx, cmd.ID); err != nil {
		return Result{}, err
	}
	return Result{}, nil
}
