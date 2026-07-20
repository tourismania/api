// Package middleware contains HTTP middlewares — keep them tiny and
// framework-agnostic where possible.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"api/internal/infrastructure/auth"
	"api/internal/presentation/http/httpx"
)

// claimsKey is the unexported context key under which JWT claims are
// stashed; consumers go through ClaimsFromContext.
type claimsKey struct{}

// JWT returns a middleware that requires a valid Bearer token. On
// success the parsed *auth.Claims is placed on the request context.
func JWT(svc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearerToken(r.Header.Get("Authorization"))
			if raw == "" {
				httpx.WriteError(w, http.StatusUnauthorized, "missing or malformed Authorization header")
				return
			}
			claims, err := svc.Verify(raw)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// OptionalJWT parses the Bearer token if one is present, so downstream
// handlers can serve both anonymous and authenticated callers (e.g.
// public offer reads, where a published offer is visible to anyone but
// an authenticated agent additionally sees their own agency's drafts).
// A missing Authorization header is not an error — the request simply
// continues without claims. A malformed/expired token is still
// rejected: silently downgrading a bad token to "anonymous" would mask
// real client bugs.
func OptionalJWT(svc *auth.Service) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw := bearerToken(r.Header.Get("Authorization"))
			if raw == "" {
				next.ServeHTTP(w, r)
				return
			}
			claims, err := svc.Verify(raw)
			if err != nil {
				httpx.WriteError(w, http.StatusUnauthorized, "invalid token")
				return
			}
			ctx := context.WithValue(r.Context(), claimsKey{}, claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ClaimsFromContext extracts claims placed by the JWT middleware. The
// boolean is false when the request did not pass through that
// middleware.
func ClaimsFromContext(ctx context.Context) (*auth.Claims, bool) {
	c, ok := ctx.Value(claimsKey{}).(*auth.Claims)
	return c, ok
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(header[len(prefix):])
}
