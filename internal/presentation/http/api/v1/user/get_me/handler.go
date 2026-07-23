package getmehttp

import (
	"errors"
	"net/http"

	"api/internal/application/apperror"
	getme "api/internal/application/query/get_me"
	"api/internal/presentation/http/httpx"
	custommw "api/internal/presentation/http/middleware"
)

// Handler renders the authenticated user as a JSON document.
type Handler struct {
	useCase getme.UseCase
}

// NewHandler constructs the handler.
func NewHandler(uc getme.UseCase) *Handler {
	return &Handler{useCase: uc}
}

// Handle is the http.HandlerFunc.
//
//	@Summary      Current user profile
//	@Description  Returns the authenticated user's profile and computed rights.
//	@Tags         Profile
//	@Produce      json
//	@Success      200  {object}  GetMeResponse
//	@Failure      401  {object}  httpx.ErrorBody
//	@Failure      500  {object}  httpx.ErrorBody
//	@Security     Bearer
//	@Router       /api/v1/users/me [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	currentUserUUID, ok := custommw.CurrentUserUUIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	res, err := h.useCase.Handle(r.Context(), getme.Query{Uuid: currentUserUUID})
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrUnauthenticated):
			httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	httpx.WriteJSON(w, http.StatusOK, GetMeResponse{
		Uuid:      res.Uuid,
		Email:     res.Email,
		Phone:     res.Phone,
		FirstName: res.FirstName,
		LastName:  res.LastName,
		Rights:    Rights{IsSuperAdmin: res.Rights.IsSuperAdmin},
		Agency: Agency{
			ID:   res.Agency.ID,
			Uuid: res.Agency.UUID,
			Name: res.Agency.Name,
		},
	})
}
