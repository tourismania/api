// Package identity resolves the acting principal (application layer)
// from the immutable uuid carried in the JWT. Every command/query that
// needs to know "who is calling" depends on UserFinder and calls
// Resolve, so this happens in exactly one place — never in
// presentation-layer middleware, so it always reflects the latest DB
// state (agency_id, roles) rather than a value cached on the request
// context.
package identity

import (
	"context"
	"fmt"

	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/service"
	"api/internal/domain/valueobject"

	"github.com/google/uuid"
)

// UserFinder is the read-port for fetching a user record by its uuid.
// The concrete implementation lives in the infrastructure layer.
type UserFinder interface {
	FindByUuid(ctx context.Context, id uuid.UUID) (*entity.UserRecord, error)
}

// Resolve loads the acting principal's DB row by uuid and projects it
// into a domain valueobject.Actor. Returns service.ErrActorNotFound when
// the uuid no longer matches any user row (e.g. the account was deleted
// after the token was issued).
func Resolve(ctx context.Context, users UserFinder, id uuid.UUID) (valueobject.Actor, error) {
	record, err := users.FindByUuid(ctx, id)
	if err != nil {
		return valueobject.Actor{}, fmt.Errorf("find user: %w", err)
	}
	if record == nil {
		return valueobject.Actor{}, service.ErrActorNotFound
	}

	roles := make([]enum.Role, 0, len(record.Roles))
	for _, r := range record.Roles {
		roles = append(roles, enum.Role(r))
	}

	return valueobject.Actor{UserID: record.ID, AgencyID: record.AgencyID, Roles: roles}, nil
}
