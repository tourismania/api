// Package deleteofferhttp is the HTTP boundary for the DeleteOffer
// command.
package deleteofferhttp

import (
	"errors"
	"net/http"

	"api/internal/application/apperror"
	deleteoffer "api/internal/application/command/delete_offer"
	"api/internal/presentation/http/httpx"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler turns HTTP requests into DeleteOffer commands.
type Handler struct {
	useCase deleteoffer.UseCase
}

// NewHandler constructs the handler.
func NewHandler(uc deleteoffer.UseCase) *Handler {
	return &Handler{useCase: uc}
}

// Handle is the http.HandlerFunc.
//
//	@Summary      Delete an offer
//	@Description  Soft-deletes an offer. Only an agent/super admin belonging to the offer's own agency may delete it — 1 user = 1 agency, no cross-agency access. An offer belonging to another agency is reported as not found.
//	@Tags         Offers
//	@Param        uuid  path  string  true  "Offer UUID"
//	@Success      204
//	@Failure      400   {object}  httpx.ErrorBody
//	@Failure      401   {object}  httpx.ErrorBody
//	@Failure      403   {object}  httpx.ErrorBody
//	@Failure      404   {object}  httpx.ErrorBody
//	@Security     Bearer
//	@Router       /api/v1/offers/{uuid} [delete]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid offer uuid")
		return
	}

	currentUserUUID, ok := custommw.CurrentUserUUIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	_, err = h.useCase.Handle(r.Context(), deleteoffer.Command{
		UUID:            id,
		CurrentUserUUID: currentUserUUID,
	})
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrUnauthenticated):
			httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		case errors.Is(err, apperror.ErrForbidden):
			httpx.WriteError(w, http.StatusForbidden, "insufficient role")
		case errors.Is(err, apperror.ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, "offer not found")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
