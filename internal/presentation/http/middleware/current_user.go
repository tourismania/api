package middleware

import (
	"context"
	"net/http"

	"api/internal/presentation/http/httpx"

	"github.com/google/uuid"
)

// currentUserUUIDKey is the unexported context key under which the
// acting principal's uuid is stashed by CurrentUserUUID.
type currentUserUUIDKey struct{}

// CurrentUserUUID is genuine middleware — not a bare helper — mounted
// right after JWT for every private route. It extracts the acting
// principal's immutable identity (the uuid carried in Claims.Subject): a
// pure token read, no DB access. Resolving mutable profile data
// (agency_id, roles) from that uuid is the domain layer's job (see
// domain/service.UserFinder), never this middleware's.
//
// Centralizing the extraction — and its own 401 on failure — here means
// handlers no longer each repeat "parse the uuid, write 401 on error":
// they read it once via CurrentUserUUIDFromContext, which cannot fail as
// long as this middleware ran.
func CurrentUserUUID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, ok := ClaimsFromContext(r.Context())
		if !ok || claims == nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		id, err := uuid.Parse(claims.Subject)
		if err != nil {
			httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
			return
		}
		ctx := context.WithValue(r.Context(), currentUserUUIDKey{}, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CurrentUserUUIDFromContext reads the uuid stored by the CurrentUserUUID
// middleware. ok is false only if that middleware was not mounted for
// this route — a routing bug, not a runtime condition handlers branch
// on.
func CurrentUserUUIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(currentUserUUIDKey{}).(uuid.UUID)
	return id, ok
}
