package getpublicofferhttp

import (
	"errors"
	"net/http"

	getpublishedoffer "api/internal/application/query/get_published_offer"
	"api/internal/domain/service"
	"api/internal/presentation/http/httpx"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler renders a single published offer as JSON. Fully public: no
// Authorization header is used or required. This is the link an agent
// shares with a client.
type Handler struct {
	useCase getpublishedoffer.UseCase
}

// NewHandler constructs the handler.
func NewHandler(uc getpublishedoffer.UseCase) *Handler {
	return &Handler{useCase: uc}
}

// Handle is the http.HandlerFunc.
//
//	@Summary      Get a published offer (public)
//	@Description  Returns a single offer by uuid, no authentication required. Only published offers are visible — draft/ready offers of any agency are reported as not found.
//	@Tags         Offers
//	@Produce      json
//	@Param        uuid  path      string  true  "Offer UUID"
//	@Success      200   {object}  OfferResponse
//	@Failure      400   {object}  httpx.ErrorBody
//	@Failure      404   {object}  httpx.ErrorBody
//	@Router       /api/v1/public/offers/{uuid} [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid offer uuid")
		return
	}

	res, err := h.useCase.Handle(r.Context(), getpublishedoffer.Query{UUID: id})
	if err != nil {
		if errors.Is(err, service.ErrOfferNotFound) {
			httpx.WriteError(w, http.StatusNotFound, "offer not found")
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, OfferResponse{
		ID:          res.ID,
		UUID:        res.UUID,
		Title:       res.Title,
		Description: res.Description,
		AgencyID:    res.AgencyID,
		CreatedAt:   res.CreatedAt,
		UpdatedAt:   res.UpdatedAt,
	})
}
