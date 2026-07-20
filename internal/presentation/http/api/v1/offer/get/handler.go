package getofferhttp

import (
	"errors"
	"net/http"

	getoffer "api/internal/application/query/get_offer"
	"api/internal/domain/enum"
	"api/internal/domain/service"
	"api/internal/presentation/http/httpx"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Handler renders a single offer as JSON, honouring read-side visibility.
// This endpoint is public: no Authorization header is required.
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
//	@Description  Returns a single offer by uuid. Published offers are visible to anyone, including anonymous callers. An authenticated agent/super admin additionally sees any-status offers of their own agency.
//	@Tags         Offers
//	@Produce      json
//	@Param        uuid  path      string  true  "Offer UUID"
//	@Success      200   {object}  OfferResponse
//	@Failure      400   {object}  httpx.ErrorBody
//	@Failure      404   {object}  httpx.ErrorBody
//	@Router       /api/v1/offers/{uuid} [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid offer uuid")
		return
	}

	res, err := h.useCase.Handle(r.Context(), getoffer.Query{
		UUID:            id,
		CurrentAgencyID: staffAgencyID(r),
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

// staffAgencyID returns the caller's own agency id when they are staff
// (ROLE_AGENT or ROLE_SUPER_ADMIN) of that agency, or nil otherwise
// (anonymous callers and plain ROLE_USER clients) — nil means the
// use-case restricts visibility to published offers only.
func staffAgencyID(r *http.Request) *int {
	cu, ok := custommw.CurrentUserFromContext(r.Context())
	if !ok || (!cu.HasRole(enum.RoleAgent) && !cu.HasRole(enum.RoleSuperAdmin)) {
		return nil
	}
	agencyID := cu.AgencyID
	return &agencyID
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
