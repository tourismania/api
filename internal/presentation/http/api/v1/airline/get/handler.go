// Package getairlinehttp — HTTP-граница GET /api/v1/airlines/{id}:
// авиакомпания по id, в том числе дубль или пропавшая из источника, которых
// поиск не возвращает.
package getairlinehttp

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"api/internal/application/apperror"
	getairline "api/internal/application/query/get_airline"
	airlinehttp "api/internal/presentation/http/api/v1/airline"
	"api/internal/presentation/http/httpx"

	"github.com/go-chi/chi/v5"
)

// notFoundMessage — ответ на отсутствующую запись и некорректный id.
const notFoundMessage = "airline not found"

// GetResponse — тело успешного ответа.
type GetResponse struct {
	Data airlinehttp.AirlineResponse `json:"data"`
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
//	@Description  Авиакомпания справочника по id, в том числе дубль или пропавшая из источника, которых поиск не возвращает, — чтобы показать авиакомпанию, на которую уже ссылается сохранённый сегмент. Нет записи или id не целое ≥ 1 — 404.
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

	resp := airlinehttp.AirlineResponse{
		ID:          res.ID,
		ICAO:        res.ICAO,
		Name:        res.Name,
		NameRU:      res.NameRU,
		DissolvedOn: airlinehttp.FormatDate(res.DissolvedOn),
	}
	if res.Country != nil {
		resp.Country = &airlinehttp.CountryResponse{ISO2: res.Country.ISO2, Name: res.Country.Name}
	}
	w.Header().Set("Cache-Control", airlinehttp.CacheControl)
	httpx.WriteJSON(w, http.StatusOK, GetResponse{Data: resp})
}
