package service

import (
	"context"
	"fmt"

	"api/internal/domain/enum"
	"api/internal/domain/repository"
	"api/internal/domain/valueobject"

	"github.com/google/uuid"
)

// UserFinder resolves the acting principal — a domain valueobject.Actor —
// from the immutable uuid carried in the JWT (Claims.Subject). Building
// an Actor from a user row, including projecting raw role strings into
// enum.Role, is business logic that belongs beside the other domain
// services (e.g. OfferManager), not in the application layer: every
// command/query that needs to know "who is calling" depends on this one
// service, so the projection exists in exactly one place.
type UserFinder struct {
	users repository.UserRepository
}

// NewUserFinder wires the collaborator.
func NewUserFinder(users repository.UserRepository) *UserFinder {
	return &UserFinder{users: users}
}

// Resolve loads the acting principal's row by uuid and projects it into
// a valueobject.Actor. Returns ErrActorNotFound when the uuid no longer
// matches any user row — e.g. the account was deleted after the token
// was issued.
func (f *UserFinder) Resolve(ctx context.Context, id uuid.UUID) (valueobject.Actor, error) {
	record, err := f.users.FindByUuid(ctx, id)
	if err != nil {
		return valueobject.Actor{}, fmt.Errorf("find user: %w", err)
	}
	if record == nil {
		return valueobject.Actor{}, ErrActorNotFound
	}

	roles := make([]enum.Role, 0, len(record.Roles))
	for _, r := range record.Roles {
		roles = append(roles, enum.Role(r))
	}

	return valueobject.Actor{UserID: record.ID, AgencyID: record.AgencyID, Roles: roles}, nil
}
