package getmehttp

import (
	"api/internal/presentation/http/middleware"
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrNoAuthClaims means the request didn't traverse the CurrentUser
// resolver middleware (no principal on context).
var ErrNoAuthClaims = errors.New("no auth claims on context")

// ErrUserMissingID covers the case where a resolved principal somehow
// carries a zero UUID — a server-side problem since the resolver
// middleware must enforce it upstream.
var ErrUserMissingID = errors.New("token has no user id")

// Resolver reads the authenticated principal off the request context,
// as placed by the shared middleware.CurrentUserMiddleware resolver
// (reused across get_me and the offer endpoints).
type Resolver struct{}

// NewResolver constructs the resolver.
func NewResolver() *Resolver { return &Resolver{} }

// Resolve returns only the immutable identity of the current principal.
// All mutable profile data (phone, name) is fetched from the DB by the
// use-case.
func (r *Resolver) Resolve(ctx context.Context) (*GetMeDto, error) {
	cu, ok := middleware.CurrentUserFromContext(ctx)
	if !ok {
		return nil, ErrNoAuthClaims
	}
	if cu.UUID == uuid.Nil {
		return nil, ErrUserMissingID
	}

	return &GetMeDto{Uuid: cu.UUID}, nil
}
