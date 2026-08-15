package updateofferhttp

import (
	"errors"
	"net/http"

	"api/internal/application/apperror"
	updateoffer "api/internal/application/command/update_offer"
	"api/internal/domain/enum"
	"api/internal/presentation/http/httpx"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// Handler turns HTTP requests into UpdateOffer commands.
type Handler struct {
	useCase  updateoffer.UseCase
	validate *validator.Validate
}

// NewHandler constructs the handler.
func NewHandler(uc updateoffer.UseCase, v *validator.Validate) *Handler {
	return &Handler{useCase: uc, validate: v}
}

// Handle is the http.HandlerFunc.
//
//	@Summary      Update an offer
//	@Description  Partially updates an offer. Only an agent/super admin belonging to the offer's own agency may update it — 1 user = 1 agency, no cross-agency access. An offer belonging to another agency is reported as not found. "flights" follows the same optional-field convention: absent leaves flights untouched, present (including []) fully replaces them.
//	@Tags         Offers
//	@Accept       json
//	@Produce      json
//	@Param        uuid  path      string               true  "Offer UUID"
//	@Param        body  body      UpdateOfferRequest   true  "Fields to update"
//	@Success      200   {object}  UpdateOfferResponse
//	@Failure      400   {object}  httpx.ErrorBody
//	@Failure      401   {object}  httpx.ErrorBody
//	@Failure      403   {object}  httpx.ErrorBody
//	@Failure      404   {object}  httpx.ErrorBody
//	@Security     Bearer
//	@Router       /api/v1/offers/{uuid} [patch]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid offer uuid")
		return
	}

	var req UpdateOfferRequest
	if err := httpx.DecodeJSON(r, &req, h.validate); err != nil {
		if errors.Is(err, httpx.ErrBadJSON) {
			httpx.WriteDecodeError(w, err)
			return
		}
		httpx.WriteValidationError(w, err)
		return
	}

	currentUserUUID, ok := custommw.CurrentUserUUIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var status *enum.OfferStatus
	if req.Status != nil {
		s := enum.OfferStatus(*req.Status)
		status = &s
	}

	var flights *[][]updateoffer.FlightSegmentInput
	if req.Flights != nil {
		groups := toFlightSegmentGroups(*req.Flights)
		flights = &groups
	}

	res, err := h.useCase.Handle(r.Context(), updateoffer.Command{
		UUID:            id,
		Title:           req.Title,
		Description:     req.Description,
		Status:          status,
		Flights:         flights,
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
		case errors.Is(err, apperror.ErrValidation):
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	httpx.WriteJSON(w, http.StatusOK, UpdateOfferResponse{ID: res.ID, UUID: res.UUID})
}

// toFlightSegmentGroups конвертирует presentation-DTO запроса в
// Application-DTO (updateoffer.FlightSegmentInput), по одной группе на
// перелёт. Presentation-слой ничего не знает про domain/entity — сборку
// доменных entity.FlightSegment/entity.Flight и их валидацию делает уже
// updateoffer.Handler.
func toFlightSegmentGroups(in []FlightInput) [][]updateoffer.FlightSegmentInput {
	groups := make([][]updateoffer.FlightSegmentInput, 0, len(in))
	for _, f := range in {
		segs := make([]updateoffer.FlightSegmentInput, 0, len(f.Segments))
		for _, s := range f.Segments {
			segs = append(segs, updateoffer.FlightSegmentInput{
				DepartureAirportICAO: s.DepartureAirportICAO,
				ArrivalAirportICAO:   s.ArrivalAirportICAO,
				DepartureAt:          s.DepartureAt,
				ArrivalAt:            s.ArrivalAt,
			})
		}
		groups = append(groups, segs)
	}
	return groups
}
