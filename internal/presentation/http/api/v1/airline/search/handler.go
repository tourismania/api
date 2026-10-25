package searchairlinehttp

import (
	"errors"
	"log/slog"
	"net/http"
	"time"

	"api/internal/application/apperror"
	searchairlines "api/internal/application/query/search_airlines"
	airlinehttp "api/internal/presentation/http/api/v1/airline"
	"api/internal/presentation/http/httpx"

	"github.com/go-playground/validator/v10"
)

const (
	// DefaultLimit — размер страницы, если limit не передан или некорректен.
	DefaultLimit = 20
	// DefaultOffset — сдвиг, если offset не передан или некорректен.
	DefaultOffset = 0
)

// Handler отдаёт результаты поиска авиакомпаний.
type Handler struct {
	useCase  searchairlines.UseCase
	validate *validator.Validate
}

// NewHandler собирает хендлер.
func NewHandler(uc searchairlines.UseCase, v *validator.Validate) *Handler {
	return &Handler{useCase: uc, validate: v}
}

// Handle — http.HandlerFunc. Здесь только разбор query-строки (форма
// запроса); нормализацию строки поиска, правило «без ids строка обязательна»
// и проверку страны делает use case searchairlines.
//
//	@Summary      Search airlines
//	@Description  Поиск авиакомпаний справочника по подстроке английского или русского названия либо названия страны (без учёта регистра и диакритики, символы сравниваются буквально), с отбором по стране и по списку id. По кодам ICAO/IATA не ищет. Возвращает только выбираемые для сегмента записи: дубли и пропавшие из источника не возвращаются (их отдаёт GET /api/v1/airlines/{id}). Действующие выше прекративших работу; внутри — начало названия, подстрока названия, только страна. id из выдачи — значение airline_id сегмента перелёта; «Неизвестная» — id 1.
//	@Tags         Airlines
//	@Produce      json
//	@Param        search   query     string  false  "Строка поиска, от 3 символов после нормализации; обязательна без ids"
//	@Param        country  query     string  false  "Код страны ISO 3166-1 alpha-2, без учёта регистра"
//	@Param        ids      query     string  false  "Id через запятую: от 1 до 100 целых ≥ 1, повторы не учитываются"
//	@Param        limit    query     int     false  "Размер страницы (1–100, по умолчанию 20)"
//	@Param        offset   query     int     false  "Сдвиг (0–10000, по умолчанию 0)"
//	@Success      200      {object}  SearchResponse
//	@Failure      400      {object}  httpx.StructuredErrorBody  "INVALID_SEARCH"
//	@Failure      401      {object}  httpx.ErrorBody
//	@Failure      429      {object}  httpx.StructuredErrorBody
//	@Failure      500      {object}  httpx.StructuredErrorBody
//	@Security     Bearer
//	@Router       /api/v1/airlines [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q := r.URL.Query()

	var ids []int
	if q.Has("ids") {
		parsed, err := httpx.ParseIDList(q.Get("ids"))
		if err != nil {
			httpx.WriteFieldValidationError(w, "IDs", "min=1")
			return
		}
		ids = parsed
	}
	params := SearchParams{
		Limit:  httpx.QueryInt(q.Get("limit"), DefaultLimit),
		Offset: httpx.QueryInt(q.Get("offset"), DefaultOffset),
		IDs:    ids,
	}
	if err := h.validate.Struct(params); err != nil {
		httpx.WriteValidationErrorWithParams(w, err)
		return
	}

	query := searchairlines.Query{Country: q.Get("country"), IDs: ids, Limit: params.Limit, Offset: params.Offset}
	if q.Has("search") {
		s := q.Get("search")
		query.Search = &s
	}
	res, err := h.useCase.Handle(r.Context(), query)
	if err != nil {
		writeError(w, r, err)
		return
	}

	slog.InfoContext(r.Context(), "airline search",
		"search", valueOf(res.Search),
		"country", valueOf(res.Country),
		"ids", len(ids),
		"limit", params.Limit,
		"offset", params.Offset,
		"count", len(res.Airlines),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	w.Header().Set("Cache-Control", airlinehttp.CacheControl)
	httpx.WriteJSON(w, http.StatusOK, SearchResponse{
		Data: toAirlineResponses(res.Airlines),
		Meta: MetaResponse{
			Total:   res.TotalCount,
			Limit:   params.Limit,
			Offset:  params.Offset,
			Search:  res.Search,
			Country: res.Country,
			IDs:     ids,
		},
	})
}

// writeError переводит ошибку use case в ответ: короткая строка поиска —
// INVALID_SEARCH, ошибка поля (страна) — 400 validation failed, прочее — 500.
func writeError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(err, searchairlines.ErrSearchTooShort) {
		httpx.WriteStructuredError(w, http.StatusBadRequest, "INVALID_SEARCH",
			"Parameter 'search' must be at least 3 characters long", "search")
		return
	}
	if fe, ok := apperror.AsFieldError(err); ok {
		httpx.WriteFieldValidationError(w, fe.Field, fe.Rule)
		return
	}
	slog.ErrorContext(r.Context(), "airline search failed", "err", err)
	httpx.WriteStructuredError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
		"An unexpected error occurred", "")
}

func toAirlineResponses(airlines []searchairlines.AirlineResult) []airlinehttp.AirlineResponse {
	out := make([]airlinehttp.AirlineResponse, 0, len(airlines))
	for _, a := range airlines {
		resp := airlinehttp.AirlineResponse{
			ID:          a.ID,
			ICAO:        a.ICAO,
			Name:        a.Name,
			NameRU:      a.NameRU,
			DissolvedOn: airlinehttp.FormatDate(a.DissolvedOn),
		}
		if a.Country != nil {
			resp.Country = &airlinehttp.CountryResponse{ISO2: a.Country.ISO2, Name: a.Country.Name}
		}
		out = append(out, resp)
	}
	return out
}

// valueOf — значение необязательной строки для журнала ("" — не передана).
func valueOf(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
