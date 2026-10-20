// Package getairlinehttp — HTTP-граница GET /api/v1/airlines/{id}:
// авиакомпания по id, в том числе скрытая из поиска.
package getairlinehttp

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"api/internal/application/apperror"
	getairline "api/internal/application/query/get_airline"
	"api/internal/presentation/http/httpx"

	"github.com/go-chi/chi/v5"
)

const (
	// dateLayout — формат dissolved_on в ответе.
	dateLayout = "2006-01-02"
	// cacheControl — как у поиска авиакомпаний.
	cacheControl = "private, max-age=3600"
	// notFoundMessage — ответ на отсутствующую запись и некорректный id.
	notFoundMessage = "airline not found"
)

// CountryResponse — страна авиакомпании.
type CountryResponse struct {
	ISO2 string `json:"iso2" example:"RU"`
	Name string `json:"name" example:"Россия"`
}

// AirlineResponse — авиакомпания: поля элемента поиска и hidden.
type AirlineResponse struct {
	ID int `json:"id" example:"42"`
	// ICAO — код ICAO; null только у служебной записи «Неизвестная» (id 1).
	ICAO *string `json:"icao" example:"AFL"`
	// Name — английское название.
	Name string `json:"name" example:"Aeroflot"`
	// NameRU — русское название, только у авиакомпаний России; "" — нет.
	NameRU string `json:"name_ru" example:"Аэрофлот"`
	// Country — страна или null, если неизвестна.
	Country *CountryResponse `json:"country"`
	// DissolvedOn — дата прекращения деятельности YYYY-MM-DD; null — действует.
	DissolvedOn *string `json:"dissolved_on" example:"2015-10-26"`
	// Hidden — запись скрыта из поиска (дубль или пропавшая из источника) и для
	// нового сегмента не выбирается; ссылки сохранённых сегментов на неё верны.
	Hidden bool `json:"hidden" example:"false"`
}

// GetResponse — тело успешного ответа.
type GetResponse struct {
	Data AirlineResponse `json:"data"`
}

// Handler отдаёт авиакомпанию по id.
type Handler struct {
	useCase getairline.UseCase
}

// NewHandler собирает хендлер.
func NewHandler(uc getairline.UseCase) *Handler {
	return &Handler{useCase: uc}
}

// Handle — http.HandlerFunc.
//
//	@Summary      Get an airline
//	@Description  Авиакомпания справочника по id, в том числе скрытая из поиска (дубль или пропавшая из источника) — чтобы показать авиакомпанию, на которую уже ссылается сохранённый сегмент. hidden=true — для нового сегмента не выбирается. Нет записи или id не целое ≥ 1 — 404.
//	@Tags         Airlines
//	@Produce      json
//	@Param        id   path      int  true  "Id авиакомпании"
//	@Success      200  {object}  GetResponse
//	@Failure      401  {object}  httpx.ErrorBody
//	@Failure      404  {object}  httpx.ErrorBody
//	@Failure      429  {object}  httpx.StructuredErrorBody
//	@Failure      500  {object}  httpx.StructuredErrorBody
//	@Security     Bearer
//	@Router       /api/v1/airlines/{id} [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		httpx.WriteError(w, http.StatusNotFound, notFoundMessage)
		return
	}

	res, err := h.useCase.Handle(r.Context(), getairline.Query{ID: id})
	if err != nil {
		if errors.Is(err, apperror.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, notFoundMessage)
			return
		}
		slog.ErrorContext(r.Context(), "get airline failed", "id", id, "err", err)
		httpx.WriteStructuredError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"An unexpected error occurred", "")
		return
	}

	resp := AirlineResponse{
		ID:     res.ID,
		ICAO:   res.ICAO,
		Name:   res.Name,
		NameRU: res.NameRU,
		Hidden: res.Hidden,
	}
	if res.Country != nil {
		resp.Country = &CountryResponse{ISO2: res.Country.ISO2, Name: res.Country.Name}
	}
	if res.DissolvedOn != nil {
		d := res.DissolvedOn.Format(dateLayout)
		resp.DissolvedOn = &d
	}
	w.Header().Set("Cache-Control", cacheControl)
	httpx.WriteJSON(w, http.StatusOK, GetResponse{Data: resp})
}
