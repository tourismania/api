package loginhttp

import (
	"errors"
	"net/http"

	"api/internal/domain/service"
	"api/internal/infrastructure/auth"
	"api/internal/infrastructure/persistence/postgres/db"
	"api/internal/presentation/http/httpx"

	"github.com/go-playground/validator/v10"
	"github.com/jackc/pgx/v5"
)

// Handler implements the password grant: look up the user by email, verify
// the bcrypt hash, then mint a JWT. Looking up against the sqlc queries
// layer keeps this handler one-shot readable; if you later add brute-force
// protection it slots in around the call to queries.GetUserByEmail.
type Handler struct {
	queries  *db.Queries
	hasher   service.PasswordHasher
	jwt      *auth.Service
	validate *validator.Validate
}

// NewHandler wires the collaborators.
func NewHandler(
	queries *db.Queries,
	hasher service.PasswordHasher,
	jwt *auth.Service,
	v *validator.Validate,
) *Handler {
	return &Handler{queries: queries, hasher: hasher, jwt: jwt, validate: v}
}

// Handle handles POST /api/login.
//
//	@Summary      Issue a JWT
//	@Description  Exchanges username/password for an RS256-signed JWT.
//	@Tags         Auth
//	@Accept       JSON
//	@Produce      JSON
//	@Param        body      LoginRequest  true  "Credentials"
//	@Success      200   {object}  LoginResponse
//	@Failure      400   {object}  httpx.ErrorBody
//	@Failure      401   {object}  httpx.ErrorBody
//	@Router       /api/login [post]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	var req LoginRequest
	if err := httpx.DecodeJSON(r, &req, h.validate); err != nil {
		if errors.Is(err, httpx.ErrBadJSON) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		httpx.WriteValidationError(w, err)
		return
	}

	user, err := h.queries.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			httpx.WriteError(w, http.StatusUnauthorized, "invalid credentials")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, "lookup failed")
		return
	}

	if err := h.hasher.Verify(user.Password, req.Password); err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}

	token, err := h.jwt.Issue(user.Uuid)
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "could not issue token")
		return
	}
	httpx.WriteJSON(w, http.StatusOK, LoginResponse{Token: token})
}
