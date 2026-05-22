package getmehttp

import (
	"errors"
	"net/http"

	getme "api/internal/application/query/get_me"
	"api/internal/presentation/http/httpx"
)

// Handler renders the authenticated user as a JSON document.
type Handler struct {
	useCase  getme.UseCase
	resolver *Resolver
}

// NewHandler constructs the handler.
func NewHandler(uc getme.UseCase, resolver *Resolver) *Handler {
	return &Handler{useCase: uc, resolver: resolver}
}

// Handle is the http.HandlerFunc.
//
//	@Summary      Current user profile
//	@Description  Returns the authenticated user's profile and computed rights.
//	@Tags         Profile
//	@Produce      json
//	@Success      200  {object}  GetMeResponse
//	@Failure      401  {object}  httpx.ErrorBody
//	@Failure      404  {object}  httpx.ErrorBody
//	@Failure      500  {object}  httpx.ErrorBody
//	@Security     BearerAuth
//	@Router       /api/v1/me [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	dto, err := h.resolver.Resolve(r.Context())
	if err != nil {
		switch {
		case errors.Is(err, ErrNoAuthClaims):
			httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		case errors.Is(err, ErrUserMissingID):
			httpx.WriteError(w, http.StatusInternalServerError, "token missing user id")
		default:
			httpx.WriteError(w, http.StatusNotFound, "user not found")
		}
		return
	}

	res, err := h.useCase.Handle(r.Context(), getme.Query{Uuid: dto.Uuid})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, GetMeResponse{
		Uuid:      res.Uuid,
		Email:     res.Email,
		Phone:     res.Phone,
		FirstName: res.FirstName,
		LastName:  res.LastName,
		Rights:    Rights{IsSuperAdmin: res.Rights.IsSuperAdmin},
	})
}
