package getmehttp

import (
	"api/internal/presentation/http/middleware"
	"context"
	"errors"

	"github.com/google/uuid"
)

// ErrNoAuthClaims means the request didn't traverse JWT middleware.
var ErrNoAuthClaims = errors.New("no auth claims on context")

// ErrUserMissingID covers the case where a token without an identifier
// reaches us — a server-side problem because the issuer must enforce it.
var ErrUserMissingID = errors.New("token has no user id")

var ErrIncorrectUuid = errors.New("token has incorrect user uuid")

// Resolver reads the authenticated principal off the request context.
type Resolver struct{}

// NewResolver constructs the resolver.
func NewResolver() *Resolver { return &Resolver{} }

// Resolve returns only the immutable identity from the JWT claims.
// All mutable data (phone, name, roles) is fetched from the DB by the use-case.
func (r *Resolver) Resolve(ctx context.Context) (*GetMeDto, error) {
	claims, ok := middleware.ClaimsFromContext(ctx)
	if !ok || claims == nil {
		return nil, ErrNoAuthClaims
	}
	if len(claims.Subject) == 0 {
		return nil, ErrUserMissingID
	}

	// Парсинг строки в UUID
	uuidParsed, err := uuid.Parse(claims.Subject)
	if err != nil {
		return nil, ErrIncorrectUuid
	}

	return &GetMeDto{Uuid: uuidParsed}, nil
}
