package getofferhttp

import (
	"errors"
	"net/http"

	getoffer "api/internal/application/query/get_offer"
	"api/internal/domain/service"
	"api/internal/presentation/http/httpx"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler renders a single offer as JSON. Private endpoint: the caller
// must be authenticated and only ever sees offers of their own agency,
// any status.
type Handler struct {
	useCase getoffer.UseCase
}

// NewHandler constructs the handler.
func NewHandler(uc getoffer.UseCase) *Handler {
	return &Handler{useCase: uc}
}

// Handle is the http.HandlerFunc.
//
//	@Summary      Get an offer
//	@Description  Returns a single offer by uuid, scoped to the caller's own agency (any status). An offer belonging to another agency is reported as not found.
//	@Tags         Offers
//	@Produce      json
//	@Param        uuid  path      string  true  "Offer UUID"
//	@Success      200   {object}  OfferResponse
//	@Failure      400   {object}  httpx.ErrorBody
//	@Failure      401   {object}  httpx.ErrorBody
//	@Failure      404   {object}  httpx.ErrorBody
//	@Security     Bearer
//	@Router       /api/v1/offers/{uuid} [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid offer uuid")
		return
	}

	currentUserUUID, err := custommw.CurrentUserUUID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	res, err := h.useCase.Handle(r.Context(), getoffer.Query{
		UUID:            id,
		CurrentUserUUID: currentUserUUID,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrActorNotFound):
			httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		case errors.Is(err, service.ErrOfferNotFound):
			httpx.WriteError(w, http.StatusNotFound, "offer not found")
		default:
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	httpx.WriteJSON(w, http.StatusOK, toResponse(res))
}

func toResponse(res getoffer.Result) OfferResponse {
	return OfferResponse{
		ID:          res.ID,
		UUID:        res.UUID,
		Title:       res.Title,
		Description: res.Description,
		AgencyID:    res.AgencyID,
		CreatedBy:   res.CreatedBy,
		Status:      res.Status.String(),
		CreatedAt:   res.CreatedAt,
		UpdatedAt:   res.UpdatedAt,
	}
}
