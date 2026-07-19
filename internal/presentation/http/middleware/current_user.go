package middleware

import (
	"context"
	"errors"
	"net/http"

	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/presentation/http/httpx"

	"github.com/google/uuid"
)

// currentUserKey is the unexported context key under which the resolved
// principal is stashed; consumers go through CurrentUserFromContext.
type currentUserKey struct{}

// ErrNoAuthClaims means the request didn't traverse the JWT middleware.
var ErrNoAuthClaims = errors.New("no auth claims on context")

// ErrInvalidSubject means the token subject isn't a parseable UUID.
var ErrInvalidSubject = errors.New("token subject is not a valid uuid")

// CurrentUser is the identity of the authenticated principal, resolved
// once per request from JWT claims + the user's DB row. It is the single
// place presentation-layer handlers pull CurrentUserID / CurrentAgencyID /
// CurrentRoles from before handing them to application use-cases.
type CurrentUser struct {
	ID       int
	UUID     uuid.UUID
	AgencyID *int
	Roles    []enum.Role
}

// HasRole reports whether the principal carries the given role.
func (u CurrentUser) HasRole(role enum.Role) bool {
	for _, r := range u.Roles {
		if r == role {
			return true
		}
	}
	return false
}

// UserFinder is the read-port used to resolve the authenticated
// principal from its UUID. The concrete implementation (UserRepository)
// lives in the infrastructure layer.
type UserFinder interface {
	FindByUuid(ctx context.Context, id uuid.UUID) (*entity.UserRecord, error)
}

// CurrentUserMiddleware resolves the authenticated principal: it reads
// Claims.Subject (placed on the context by JWT), loads the matching
// user row, and stores a CurrentUser on the context for downstream
// handlers. Reused by every /api/v1 route that needs identity beyond the
// bare JWT subject (e.g. get_me, offers).
func CurrentUserMiddleware(users UserFinder) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cu, err := resolveCurrentUser(r.Context(), users)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
				return
			}
			ctx := context.WithValue(r.Context(), currentUserKey{}, cu)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func resolveCurrentUser(ctx context.Context, users UserFinder) (CurrentUser, error) {
	claims, ok := ClaimsFromContext(ctx)
	if !ok || claims == nil {
		return CurrentUser{}, ErrNoAuthClaims
	}

	subject, err := uuid.Parse(claims.Subject)
	if err != nil {
		return CurrentUser{}, ErrInvalidSubject
	}

	record, err := users.FindByUuid(ctx, subject)
	if err != nil || record == nil {
		return CurrentUser{}, errors.New("user not found")
	}

	agencyID := record.AgencyID
	roles := make([]enum.Role, 0, len(record.Roles))
	for _, r := range record.Roles {
		roles = append(roles, enum.Role(r))
	}

	return CurrentUser{
		ID:       record.ID,
		UUID:     record.Uuid,
		AgencyID: &agencyID,
		Roles:    roles,
	}, nil
}

// CurrentUserFromContext extracts the principal placed by
// CurrentUserMiddleware. The boolean is false when the request did not
// pass through that middleware.
func CurrentUserFromContext(ctx context.Context) (CurrentUser, bool) {
	cu, ok := ctx.Value(currentUserKey{}).(CurrentUser)
	return cu, ok
}

// RequireRole returns a guard that rejects the request with 403 unless
// the resolved CurrentUser carries at least one of the given roles. Must
// be mounted after CurrentUserMiddleware.
func RequireRole(roles ...enum.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cu, ok := CurrentUserFromContext(r.Context())
			if !ok {
				httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
				return
			}
			for _, want := range roles {
				if cu.HasRole(want) {
					next.ServeHTTP(w, r)
					return
				}
			}
			httpx.WriteError(w, http.StatusForbidden, "insufficient role")
		})
	}
}
