package createuser

import (
	"api/internal/domain/user"
	"context"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler executes the CreateUser command by delegating to the domain
// user.Creator service. Keeping the handler thin preserves DDD: business
// invariants stay in the domain layer.
type Handler struct {
	userCreator *user.Creator
}

// NewHandler constructs the handler.
func NewHandler(userCreator *user.Creator) *Handler {
	return &Handler{userCreator: userCreator}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	id, err := h.userCreator.Create(ctx, user.User{
		FirstName: cmd.FirstName,
		LastName:  cmd.LastName,
		Email:     cmd.Email,
		Password:  cmd.Password,
		AgencyID:  cmd.AgencyID,
	})
	if err != nil {
		return Result{}, err
	}
	return Result{ID: id}, nil
}
