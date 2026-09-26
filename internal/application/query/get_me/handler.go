package getme

import (
	"context"
	"fmt"

	"api/internal/application/apperror"
	"api/internal/domain/agency"
	"api/internal/domain/user"

	"github.com/google/uuid"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// UserFinder is the read-port for fetching a user record by primary key.
// The concrete implementation lives in the infrastructure layer.
type UserFinder interface {
	FindByUuid(ctx context.Context, uuid uuid.UUID) (*user.Record, error)
}

// AgencyFinder is the read-port for fetching the agency a user belongs to.
type AgencyFinder interface {
	FindByID(ctx context.Context, id int) (*agency.Agency, error)
}

// Handler fetches the full user profile from the DB and derives rights.
type Handler struct {
	users           UserFinder
	agencies        AgencyFinder
	rightsDescriber *user.RightsDescriber
}

// NewHandler constructs the handler.
func NewHandler(users UserFinder, agencies AgencyFinder, rightsDescriber *user.RightsDescriber) *Handler {
	return &Handler{users: users, agencies: agencies, rightsDescriber: rightsDescriber}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	record, err := h.users.FindByUuid(ctx, q.Uuid)
	if err != nil {
		return Result{}, fmt.Errorf("get user: %w", err)
	}
	if record == nil {
		// The uuid comes straight from a valid JWT's Claims.Subject, but
		// the account may have been deleted after the token was issued.
		return Result{}, apperror.FromDomainError(user.ErrActorNotFound)
	}

	ag, err := h.agencies.FindByID(ctx, record.AgencyID)
	if err != nil {
		return Result{}, fmt.Errorf("get agency: %w", err)
	}
	if ag == nil {
		return Result{}, fmt.Errorf("get agency: agency %d not found", record.AgencyID)
	}

	rights := h.rightsDescriber.ByRoles(record.Roles)
	return Result{
		Uuid:      record.Uuid,
		Email:     record.Email,
		Phone:     record.Phone,
		FirstName: record.FirstName,
		LastName:  record.LastName,
		Rights:    rights,
		Agency: Agency{
			ID:   ag.ID,
			UUID: ag.UUID,
			Name: ag.Name,
		},
	}, nil
}
