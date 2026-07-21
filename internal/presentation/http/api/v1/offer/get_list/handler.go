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

// Handler renders a paginated, filtered offer listing. Private
// endpoint: the caller must be authenticated; the list is always scoped
// to their own agency.
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
//	@Description  Paginated offer listing (any status) scoped to the caller's own agency. Role has no effect on visibility, only on write access.
//	@Tags         Offers
//	@Produce      json
//	@Param        status      query     string  false  "Filter by status (draft|ready|published)"
//	@Param        created_by  query     int     false  "Filter by author user id"
//	@Param        limit       query     int     false  "Max results (1–100, default 20)"
//	@Param        offset      query     int     false  "Pagination offset (0–10000)"
//	@Success      200         {object}  ListOffersResponse
//	@Failure      400         {object}  httpx.ErrorBody
//	@Failure      401         {object}  httpx.ErrorBody
//	@Security     Bearer
//	@Router       /api/v1/offers [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	cu, ok := custommw.CurrentUserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	q := r.URL.Query()

	limit := parseIntDefault(q.Get("limit"), DefaultLimit)
	offset := parseIntDefault(q.Get("offset"), DefaultOffset)

	params := ListOffersParams{
		Status: q.Get("status"),
		Limit:  limit,
		Offset: offset,
	}
	if raw := q.Get("created_by"); raw != "" {
		v, err := strconv.Atoi(raw)
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid created_by")
			return
		}
		params.CreatedBy = v
	}
	if err := h.validate.Struct(params); err != nil {
		httpx.WriteValidationError(w, err)
		return
	}

	var createdBy *int
	if params.CreatedBy > 0 {
		createdBy = &params.CreatedBy
	}
	var status *enum.OfferStatus
	if params.Status != "" {
		s := enum.OfferStatus(params.Status)
		status = &s
	}

	res, err := h.useCase.Handle(r.Context(), getoffers.Query{
		AgencyID:  cu.AgencyID,
		Status:    status,
		CreatedBy: createdBy,
		Limit:     limit,
		Offset:    offset,
	})
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}

	httpx.WriteJSON(w, http.StatusOK, buildResponse(res, limit, offset))
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
