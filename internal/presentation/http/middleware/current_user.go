package middleware

import (
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrNoAuthClaims means the request didn't traverse the JWT middleware.
var ErrNoAuthClaims = errors.New("no auth claims on context")

// ErrInvalidSubject means the token subject isn't a parseable uuid.
var ErrInvalidSubject = errors.New("token subject is not a valid uuid")

// CurrentUserUUID extracts the acting principal's immutable identity
// from the JWT claims placed on context by the JWT middleware. This is
// a pure token read — no DB access happens here. Resolving mutable
// profile data (agency_id, roles) from that uuid is the application
// layer's job (see application/identity, get_me, offer use-cases), so
// it always reflects the latest DB state and is never duplicated in a
// middleware.
func CurrentUserUUID(ctx context.Context) (uuid.UUID, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok || claims == nil {
		return uuid.Nil, ErrNoAuthClaims
	}
	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidSubject
	}
	return id, nil
}
