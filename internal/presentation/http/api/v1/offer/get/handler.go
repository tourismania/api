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

// Handler renders a single offer as JSON, honouring read-side visibility.
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
//	@Description  Returns a single offer by uuid. Visibility depends on the caller's role: super admins see any offer, agents see any offer of their own agency, everyone else only sees published offers.
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

	cu, ok := custommw.CurrentUserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	res, err := h.useCase.Handle(r.Context(), getoffer.Query{
		UUID:            id,
		CurrentUserID:   cu.ID,
		CurrentAgencyID: cu.AgencyID,
		CurrentRoles:    cu.Roles,
	})
	if err != nil {
		if errors.Is(err, service.ErrOfferNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "offer not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
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
