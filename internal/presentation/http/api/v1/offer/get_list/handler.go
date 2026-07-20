package listoffershttp

import (
	"net/http"
	"strconv"

	getoffers "api/internal/application/query/get_offers"
	"api/internal/domain/enum"
	"api/internal/presentation/http/httpx"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-playground/validator/v10"
)

const (
	// DefaultLimit is applied when the caller omits the limit parameter.
	DefaultLimit = 20
	// DefaultOffset is applied when the caller omits the offset parameter.
	DefaultOffset = 0
)

// Handler renders a paginated, filtered offer listing.
type Handler struct {
	useCase  getoffers.UseCase
	validate *validator.Validate
}

// NewHandler constructs the handler.
func NewHandler(uc getoffers.UseCase, v *validator.Validate) *Handler {
	return &Handler{useCase: uc, validate: v}
}

// Handle is the http.HandlerFunc.
//
//	@Summary      List offers
//	@Description  Paginated offer listing with filters. Published offers are visible to anyone, including anonymous callers. An authenticated agent/super admin additionally sees any-status offers of their own agency (the agency_id filter is then forced to their own agency).
//	@Tags         Offers
//	@Produce      json
//	@Param        agency_id  query     int     false  "Filter by agency id"
//	@Param        status     query     string  false  "Filter by status (draft|published)"
//	@Param        limit      query     int     false  "Max results (1–100, default 20)"
//	@Param        offset     query     int     false  "Pagination offset (0–10000)"
//	@Success      200        {object}  ListOffersResponse
//	@Failure      400        {object}  httpx.ErrorBody
//	@Router       /api/v1/offers [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	limit := parseIntDefault(q.Get("limit"), DefaultLimit)
	offset := parseIntDefault(q.Get("offset"), DefaultOffset)

	params := ListOffersParams{
		Status: q.Get("status"),
		Limit:  limit,
		Offset: offset,
	}
	if raw := q.Get("agency_id"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid agency_id")
			return
		}
		params.AgencyID = v
	}
	if err := h.validate.Struct(params); err != nil {
		httpx.WriteValidationError(w, err)
		return
	}

	var agencyID *int
	if params.AgencyID > 0 {
		agencyID = &params.AgencyID
	}
	var status *enum.OfferStatus
	if params.Status != "" {
		s := enum.OfferStatus(params.Status)
		status = &s
	}

	res, err := h.useCase.Handle(r.Context(), getoffers.Query{
		AgencyID:        agencyID,
		Status:          status,
		Limit:           limit,
		Offset:          offset,
		CurrentAgencyID: staffAgencyID(r),
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, buildResponse(res, limit, offset))
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

func buildResponse(res getoffers.Result, limit, offset int) ListOffersResponse {
	data := make([]OfferResponse, 0, len(res.Offers))
	for _, o := range res.Offers {
		data = append(data, OfferResponse{
			ID:          o.ID,
			UUID:        o.UUID,
			Title:       o.Title,
			Description: o.Description,
			AgencyID:    o.AgencyID,
			CreatedBy:   o.CreatedBy,
			Status:      o.Status.String(),
			CreatedAt:   o.CreatedAt,
			UpdatedAt:   o.UpdatedAt,
		})
	}
	return ListOffersResponse{
		Data: data,
		Meta: MetaResponse{Total: res.TotalCount, Limit: limit, Offset: offset},
	}
}

func parseIntDefault(raw string, def int) int {
	if raw == "" {
		return def
	}
	v, err := strconv.Atoi(raw)
	if err != nil || v < 0 {
		return def
	}
	return v
}
