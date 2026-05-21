package getme

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

// UserFinder is the read-port for fetching a user record by primary key.
// The concrete implementation lives in the infrastructure layer.
type UserFinder interface {
	FindByUuid(ctx context.Context, uuid uuid.UUID) (*entity.UserRecord, error)
}

// Handler fetches the full user profile from the DB and derives rights.
type Handler struct {
	users           UserFinder
	rightsDescriber *service.RightsDescriber
}

// NewHandler constructs the handler.
func NewHandler(users UserFinder, rightsDescriber *service.RightsDescriber) *Handler {
	return &Handler{users: users, rightsDescriber: rightsDescriber}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	user, err := h.users.FindByUuid(ctx, q.Uuid)
	if err != nil {
		return Result{}, fmt.Errorf("get user: %w", err)
	}
	rights := h.rightsDescriber.ByRoles(user.Roles)
	return Result{
		Uuid:      user.Uuid,
		Email:     user.Email,
		Phone:     user.Phone,
		FirstName: user.FirstName,
		LastName:  user.LastName,
		Rights:    rights,
	}, nil
}
