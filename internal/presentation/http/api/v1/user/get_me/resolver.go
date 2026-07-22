package getmehttp

import (
	"context"
	"errors"

	"api/internal/presentation/http/middleware"
)

// ErrNoAuthClaims means the request didn't traverse the JWT middleware
// (no claims on context).
var ErrNoAuthClaims = errors.New("no auth claims on context")

// ErrUserMissingID covers the case where the JWT subject isn't a
// parseable uuid — a server-side problem since Issue always emits one.
var ErrUserMissingID = errors.New("token has no user id")

// Resolver reads the authenticated principal's uuid off the JWT claims
// on the request context — no DB access happens here; that's the same
// flow used by the offer endpoints (application/identity resolves
// mutable profile data from the uuid).
type Resolver struct{}

// NewResolver constructs the resolver.
func NewResolver() *Resolver { return &Resolver{} }

// Resolve returns only the immutable identity of the current principal.
// All mutable profile data (phone, name) is fetched from the DB by the
// use-case.
func (r *Resolver) Resolve(ctx context.Context) (*GetMeDto, error) {
	id, err := middleware.CurrentUserUUID(ctx)
	switch {
	case errors.Is(err, middleware.ErrNoAuthClaims):
		return nil, ErrNoAuthClaims
	case errors.Is(err, middleware.ErrInvalidSubject):
		return nil, ErrUserMissingID
	case err != nil:
		return nil, err
	}

	return &GetMeDto{Uuid: id}, nil
}
