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
//	@Description  Returns a single offer by uuid, no authentication required, including its flights with computed total/layover durations. Only published offers are visible — draft/ready offers of any agency are reported as not found.
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
		Flights:     toFlightResponses(res.Flights),
	})
}

// toFlightResponses переносит уже готовую проекцию
// getpublishedoffer.FlightResult в wire-DTO: только копирование полей,
// без обращения к domain-типам и без вызова доменного поведения —
// вычисление длительностей и пересадок сделано в Application-слое
// (getpublishedoffer.Handler.toFlightResults).
func toFlightResponses(flights []getpublishedoffer.FlightResult) []FlightResponse {
	out := make([]FlightResponse, 0, len(flights))
	for _, f := range flights {
		segments := make([]FlightSegmentResponse, 0, len(f.Segments))
		for _, s := range f.Segments {
			segments = append(segments, FlightSegmentResponse{
				DepartureAirportICAO: s.DepartureAirportICAO,
				ArrivalAirportICAO:   s.ArrivalAirportICAO,
				DepartureAt:          s.DepartureAt,
				ArrivalAt:            s.ArrivalAt,
				DurationSeconds:      s.DurationSeconds,
			})
		}

		layovers := make([]LayoverResponse, 0, len(f.Layovers))
		for _, l := range f.Layovers {
			layovers = append(layovers, LayoverResponse{
				AirportICAO:     l.AirportICAO,
				DurationSeconds: l.DurationSeconds,
			})
		}

		out = append(out, FlightResponse{
			ID:                   f.ID,
			DepartureAirportICAO: f.DepartureAirportICAO,
			ArrivalAirportICAO:   f.ArrivalAirportICAO,
			TotalDurationSeconds: f.TotalDurationSeconds,
			Segments:             segments,
			Layovers:             layovers,
		})
	}
	return out
}
