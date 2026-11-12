package searchhotelhttp

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"api/internal/application/apperror"
	searchhotels "api/internal/application/query/search_hotels"
	hotelhttp "api/internal/presentation/http/api/v1/hotel"
	"api/internal/presentation/http/httpx"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-playground/validator/v10"
)

const (
	// DefaultLimit — размер страницы, если limit не передан или некорректен.
	DefaultLimit = 20
	// DefaultOffset — сдвиг, если offset не передан или некорректен.
	DefaultOffset = 0
)

// Handler отдаёт результаты поиска отелей агентства пользователя.
type Handler struct {
	useCase  searchhotels.UseCase
	validate *validator.Validate
}

// NewHandler собирает хендлер.
func NewHandler(uc searchhotels.UseCase, v *validator.Validate) *Handler {
	return &Handler{useCase: uc, validate: v}
}

// Handle — http.HandlerFunc. Здесь только разбор query-строки; нормализацию
// и длину строки поиска проверяет use case searchhotels.
//
//	@Summary      Search hotels
//	@Description  Поиск отелей справочника своего агентства по подстроке названия без учёта регистра (символы, включая % _ \, сравниваются буквально). Сначала названия, начинающиеся со строки поиска, затем остальные; внутри — по названию, затем по id. Категорий номеров в выдаче нет — их отдаёт GET /api/v1/hotels/{id}/room-categories. Отели других агентств не видны; читать может любой пользователь агентства.
//	@Tags         Hotels
//	@Produce      json
//	@Param        search  query     string  true   "Строка поиска, от 2 символов после обрезки пробелов по краям и схлопывания подряд идущих"
//	@Param        limit   query     int     false  "Размер страницы (1–100, по умолчанию 20)"
//	@Param        offset  query     int     false  "Сдвиг (0–10000, по умолчанию 0)"
//	@Success      200     {object}  SearchResponse
//	@Failure      400     {object}  httpx.StructuredErrorBody  "INVALID_SEARCH или validation failed"
//	@Failure      401     {object}  httpx.ErrorBody
//	@Failure      429     {object}  httpx.StructuredErrorBody
//	@Failure      500     {object}  httpx.StructuredErrorBody
//	@Security     Bearer
//	@Router       /api/v1/hotels [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q := r.URL.Query()

	params := SearchParams{
		Limit:  httpx.QueryInt(q.Get("limit"), DefaultLimit),
		Offset: httpx.QueryInt(q.Get("offset"), DefaultOffset),
	}
	if err := h.validate.Struct(params); err != nil {
		httpx.WriteValidationErrorWithParams(w, err)
		return
	}

	currentUserUUID, ok := custommw.CurrentUserUUIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	query := searchhotels.Query{CurrentUserUUID: currentUserUUID, Limit: params.Limit, Offset: params.Offset}
	if q.Has("search") {
		s := q.Get("search")
		query.Search = &s
	}
	res, err := h.useCase.Handle(r.Context(), query)
	if err != nil {
		writeError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "hotel search",
		"search", res.Search,
		"limit", params.Limit,
		"offset", params.Offset,
		"count", len(res.Hotels),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	w.Header().Set("Cache-Control", hotelhttp.CacheControl)
	httpx.WriteJSON(w, http.StatusOK, SearchResponse{
		Data: toHotelResponses(res.Hotels),
		Meta: MetaResponse{Total: res.TotalCount, Limit: params.Limit, Offset: params.Offset, Search: res.Search},
	})
}

// writeError переводит ошибку use case в ответ: короткая строка поиска —
// INVALID_SEARCH, пользователь не найден — 401, прочее — 500.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, searchhotels.ErrSearchTooShort):
		httpx.WriteStructuredError(w, http.StatusBadRequest, "INVALID_SEARCH",
			"Parameter 'search' must be at least 2 characters long", "search")
	case errors.Is(err, apperror.ErrUnauthenticated):
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
	default:
		slog.ErrorContext(r.Context(), "hotel search failed", "err", err)
		httpx.WriteStructuredError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"An unexpected error occurred", "")
	}
}

func toHotelResponses(hotels []searchhotels.HotelResult) []HotelResponse {
	out := make([]HotelResponse, 0, len(hotels))
	for _, ht := range hotels {
		out = append(out, HotelResponse{
			ID:          ht.ID,
			Name:        ht.Name,
			Stars:       ht.Stars,
			Address:     ht.Address,
			Description: ht.Description,
			PhotoURLs:   nonNil(ht.PhotoURLs),
		})
	}
	return out
}

// nonNil — пустой массив вместо null в ответе.
func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
