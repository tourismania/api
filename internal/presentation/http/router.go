// Package http wires the HTTP layer: middlewares, routes, swagger.
package http

import (
	"api/internal/presentation/http/api/v1/user/create"
	"api/internal/presentation/http/api/v1/user/get_me"
	"net/http"

	"api/internal/infrastructure/auth"
	loginhttp "api/internal/presentation/http/api/login"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	httpSwagger "github.com/swaggo/http-swagger"
)

// Server holds every HTTP-layer dependency and builds the final handler.
// It is the single place that knows the URL → handler mapping — easy to audit.
type Server struct {
	Login      *loginhttp.Handler
	CreateUser *createuserhttp.Handler
	GetMe      *getmehttp.Handler
	JWT        *auth.Service

	// CORSAllowedOrigins is forwarded to the CORS middleware.
	// Empty slice disables CORS headers entirely.
	CORSAllowedOrigins []string
}

// Build returns a *chi.Mux with all endpoints attached.
func (s Server) Build() http.Handler {
	r := chi.NewRouter()

	// CORS must be first so preflight OPTIONS never hits auth middleware.
	if len(s.CORSAllowedOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   s.CORSAllowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Authorization", "Content-Type", "X-Request-Id"},
			AllowCredentials: true,
			MaxAge:           86400,
		}))
	}

	// Recommended chi middlewares.
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	// Swagger UI is public — keep it out of /api/v1.
	r.Get("/api/docs/*", httpSwagger.Handler(
		httpSwagger.URL("/api/docs/doc.json"),
	))

	// Public auth endpoint.
	r.Post("/api/login", s.Login.Handle)

	// Versioned, JWT-guarded API surface.
	r.Route("/api/v1", func(api chi.Router) {
		api.Use(custommw.JWT(s.JWT))
		api.Post("/users", s.CreateUser.Handle)
		api.Get("/users/me", s.GetMe.Handle)
	})

	// Liveness/readiness.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return r
}
