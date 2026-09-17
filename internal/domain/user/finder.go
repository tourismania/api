package user

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ErrActorNotFound is returned when the identity resolved from a valid
// JWT subject no longer has a matching user row — e.g. the account was
// deleted after the token was issued. Application-layer handlers use
// this whenever they resolve the acting principal by uuid before
// delegating to a domain service.
var ErrActorNotFound = errors.New("actor not found")

// Finder resolves the acting principal — a domain Actor — from the
// immutable uuid carried in the JWT (Claims.Subject). Building an Actor
// from a user row, including projecting raw role strings into Role, is
// business logic that belongs beside the other domain services (e.g.
// offer.Manager), not in the application layer: every command/query
// that needs to know "who is calling" depends on this one service, so
// the projection exists in exactly one place.
type Finder struct {
	users Repository
}

// NewFinder wires the collaborator.
func NewFinder(users Repository) *Finder {
	return &Finder{users: users}
}

// Resolve loads the acting principal's row by uuid and projects it into
// an Actor. Returns ErrActorNotFound when the uuid no longer matches
// any user row — e.g. the account was deleted after the token was
// issued.
func (f *Finder) Resolve(ctx context.Context, id uuid.UUID) (Actor, error) {
	record, err := f.users.FindByUuid(ctx, id)
	if err != nil {
		return Actor{}, fmt.Errorf("find user: %w", err)
	}
	if record == nil {
		return Actor{}, ErrActorNotFound
	}

	roles := make([]Role, 0, len(record.Roles))
	for _, r := range record.Roles {
		roles = append(roles, Role(r))
	}

	return Actor{UserID: record.ID, AgencyID: record.AgencyID, Roles: roles}, nil
}
