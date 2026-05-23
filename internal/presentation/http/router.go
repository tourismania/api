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
	httpSwagger "github.com/swaggo/http-swagger"
)

// Routes wires every endpoint to its handler. Keep this as the only
// file that knows the URL → handler mapping — easy to audit.
type Routes struct {
	Login      *loginhttp.Handler
	CreateUser *createuserhttp.Handler
	GetMe      *getmehttp.Handler
	JWT        *auth.Service
}

// Build returns a *chi.Mux with all endpoints attached.
func (rt Routes) Build() http.Handler {
	r := chi.NewRouter()

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
	r.Post("/api/login", rt.Login.Handle)

	// Versioned, JWT-guarded API surface.
	r.Route("/api/v1", func(api chi.Router) {
		api.Use(custommw.JWT(rt.JWT))
		api.Post("/users", rt.CreateUser.Handle)
		api.Get("/users/me", rt.GetMe.Handle)
	})

	// Liveness/readiness.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return r
}
