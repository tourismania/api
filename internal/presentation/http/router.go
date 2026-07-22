// Package http wires the HTTP layer: middlewares, routes, swagger.
package http

import (
	"api/internal/presentation/http/api/v1/user/create"
	"api/internal/presentation/http/api/v1/user/get_me"
	"net/http"
	"time"

	"api/internal/infrastructure/auth"
	loginhttp "api/internal/presentation/http/api/login"
	searchairporthttp "api/internal/presentation/http/api/v1/airport/search"
	createofferhttp "api/internal/presentation/http/api/v1/offer/create"
	deleteofferhttp "api/internal/presentation/http/api/v1/offer/delete"
	getofferhttp "api/internal/presentation/http/api/v1/offer/get"
	listoffershttp "api/internal/presentation/http/api/v1/offer/get_list"
	getpublicofferhttp "api/internal/presentation/http/api/v1/offer/get_public"
	updateofferhttp "api/internal/presentation/http/api/v1/offer/update"
	"api/internal/presentation/http/httpx"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	httpSwagger "github.com/swaggo/http-swagger"
)

// Server holds every HTTP-layer dependency and builds the final handler.
// It is the single place that knows the URL → handler mapping — easy to audit.
type Server struct {
	Login             *loginhttp.Handler
	CreateUser        *createuserhttp.Handler
	GetMe             *getmehttp.Handler
	Airports          *searchairporthttp.Handler
	CreateOffer       *createofferhttp.Handler
	GetOffer          *getofferhttp.Handler
	GetOffers         *listoffershttp.Handler
	GetPublishedOffer *getpublicofferhttp.Handler
	UpdateOffer       *updateofferhttp.Handler
	DeleteOffer       *deleteofferhttp.Handler
	JWT               *auth.Service

	// RateLimit is the per-IP cap in requests per minute for the airports endpoint.
	RateLimit int

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

	limiter := httprate.NewRateLimiter(
		s.RateLimit,
		time.Minute,
		httprate.WithLimitHandler(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "60")
			httpx.WriteStructuredError(
				w,
				http.StatusTooManyRequests,
				"RATE_LIMITED",
				"Too many requests",
				"",
			)
		}),
	)

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

	// Versioned API surface.
	r.With(limiter.Handler).Route("/api/v1", func(api chi.Router) {
		// Fully anonymous "share link": only ever returns a published
		// offer, regardless of agency. No auth middleware at all.
		api.Get("/public/offers/{uuid}", s.GetPublishedOffer.Handle)

		// Everything else requires a valid JWT. Resolving the principal's
		// mutable profile (agency_id, roles) from its uuid is always done
		// application-side (see application/identity) — there is no
		// middleware here that touches the DB.
		api.Group(func(priv chi.Router) {
			priv.Use(custommw.JWT(s.JWT))

			// Users
			priv.Post("/users", s.CreateUser.Handle)
			priv.Get("/users/me", s.GetMe.Handle)

			// Airports
			priv.Get("/airports", s.Airports.Handle)

			// Offer reads: any authenticated principal, scoped to their
			// own agency, any status. Role has no effect on visibility.
			priv.Get("/offers", s.GetOffers.Handle)
			priv.Get("/offers/{uuid}", s.GetOffer.Handle)

			// Offer writes: agent/super admin, own agency only. Both the
			// role and the ownership check are enforced by the domain
			// OfferManager, not by router-level middleware.
			priv.Post("/offers", s.CreateOffer.Handle)
			priv.Patch("/offers/{uuid}", s.UpdateOffer.Handle)
			priv.Delete("/offers/{uuid}", s.DeleteOffer.Handle)
		})
	})

	// Liveness/readiness.
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	return r
}
