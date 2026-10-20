package searchairlinehttp

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	searchairlines "api/internal/application/query/search_airlines"
	"api/internal/presentation/http/httpx"

	"github.com/go-playground/validator/v10"
)

const (
	// DefaultLimit — размер страницы, если limit не передан или некорректен.
	DefaultLimit = 20
	// DefaultOffset — сдвиг, если offset не передан или некорректен.
	DefaultOffset = 0
	// MinSearchLen — минимальная длина строки поиска после нормализации.
	MinSearchLen = 3
	// DateLayout — формат dissolved_on в ответе.
	DateLayout = "2006-01-02"
	// cacheControl — ответ справочника можно кэшировать в браузере час.
	cacheControl = "private, max-age=3600"
)

var (
	// ErrSearchTooShort — строка поиска короче MinSearchLen.
	ErrSearchTooShort = errors.New("search must be at least 3 characters")
	// ErrIDsInvalid — в ids есть значение, не являющееся целым ≥ 1.
	ErrIDsInvalid = errors.New("ids must be positive integers")
	// ErrCountryInvalid — country не две латинские буквы.
	ErrCountryInvalid = errors.New("country must be an ISO 3166-1 alpha-2 code")

	// multiSpace — подряд идущие пробельные символы.
	multiSpace = regexp.MustCompile(`\s+`)
	// countryPattern — код страны ISO 3166-1 alpha-2 в любом регистре.
	countryPattern = regexp.MustCompile(`^[A-Za-z]{2}$`)
)

// NormalizeSearch обрезает пробелы по краям и схлопывает подряд идущие
// пробельные символы; короче MinSearchLen — ErrSearchTooShort.
func NormalizeSearch(raw string) (string, error) {
	s := multiSpace.ReplaceAllString(strings.TrimSpace(raw), " ")
	if len([]rune(s)) < MinSearchLen {
		return "", ErrSearchTooShort
	}
	return s, nil
}

// ParseIDs разбирает список id через запятую: каждый — целое ≥ 1 (иначе
// ErrIDsInvalid), повторы убираются с сохранением порядка. Число id
// проверяет валидатор (SearchParams.IDs).
func ParseIDs(raw string) ([]int, error) {
	parts := strings.Split(raw, ",")
	ids := make([]int, 0, len(parts))
	seen := make(map[int]struct{}, len(parts))
	for _, p := range parts {
		id, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || id < 1 {
			return nil, ErrIDsInvalid
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	return ids, nil
}

// ParseCountry проверяет код страны и приводит его к верхнему регистру.
func ParseCountry(raw string) (string, error) {
	if !countryPattern.MatchString(raw) {
		return "", ErrCountryInvalid
	}
	return strings.ToUpper(raw), nil
}

// ValidationMeta собирает meta ошибки валидации с границей правила:
// {"Limit": "max=100"}. Общий перевод в httpx пишет только тег — граница
// там пока не нужна остальным эндпоинтам (строка бэклога).
func ValidationMeta(err error) map[string]any {
	meta := map[string]any{}
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		for _, fe := range verrs {
			rule := fe.Tag()
			if fe.Param() != "" {
				rule += "=" + fe.Param()
			}
			meta[fe.Field()] = rule
		}
	}
	return meta
}

// Handler отдаёт результаты поиска авиакомпаний.
type Handler struct {
	useCase  searchairlines.UseCase
	validate *validator.Validate
}

// NewHandler собирает хендлер.
func NewHandler(uc searchairlines.UseCase, v *validator.Validate) *Handler {
	return &Handler{useCase: uc, validate: v}
}

// Handle — http.HandlerFunc.
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
		parsed, err := ParseIDs(q.Get("ids"))
		if err != nil {
			httpx.WriteFieldValidationError(w, "IDs", "min=1")
			return
		}
		ids = parsed
	}

	// Без ids строка поиска обязательна; переданная строка проверяется всегда.
	var search *string
	if q.Has("search") || ids == nil {
		s, err := NormalizeSearch(q.Get("search"))
		if err != nil {
			httpx.WriteStructuredError(w, http.StatusBadRequest, "INVALID_SEARCH",
				"Parameter 'search' must be at least 3 characters long", "search")
			return
		}
		search = &s
	}

	var country *string
	if raw := q.Get("country"); raw != "" {
		c, err := ParseCountry(raw)
		if err != nil {
			httpx.WriteFieldValidationError(w, "Country", "iso2")
			return
		}
		country = &c
	}

	params := SearchParams{
		Limit:  parseIntDefault(q.Get("limit"), DefaultLimit),
		Offset: parseIntDefault(q.Get("offset"), DefaultOffset),
		IDs:    ids,
	}
	if err := h.validate.Struct(params); err != nil {
		httpx.WriteJSON(w, http.StatusBadRequest, httpx.ErrorBody{
			Error: "validation failed",
			Code:  http.StatusBadRequest,
			Meta:  ValidationMeta(err),
		})
		return
	}

	res, err := h.useCase.Handle(r.Context(), searchairlines.Query{
		Search:      deref(search),
		CountryISO2: deref(country),
		IDs:         ids,
		Limit:       params.Limit,
		Offset:      params.Offset,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "airline search failed", "err", err)
		httpx.WriteStructuredError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"An unexpected error occurred", "")
		return
	}

	slog.InfoContext(r.Context(), "airline search",
		"search", deref(search),
		"country", deref(country),
		"ids", len(ids),
		"limit", params.Limit,
		"offset", params.Offset,
		"count", len(res.Airlines),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	w.Header().Set("Cache-Control", cacheControl)
	httpx.WriteJSON(w, http.StatusOK, SearchResponse{
		Data: toAirlineResponses(res.Airlines),
		Meta: MetaResponse{
			Total:   res.TotalCount,
			Limit:   params.Limit,
			Offset:  params.Offset,
			Search:  search,
			Country: country,
			IDs:     ids,
		},
	})
}

func toAirlineResponses(airlines []searchairlines.AirlineResult) []AirlineResponse {
	out := make([]AirlineResponse, 0, len(airlines))
	for _, a := range airlines {
		resp := AirlineResponse{
			ID:     a.ID,
			ICAO:   a.ICAO,
			Name:   a.Name,
			NameRU: a.NameRU,
		}
		if a.Country != nil {
			resp.Country = &CountryResponse{ISO2: a.Country.ISO2, Name: a.Country.Name}
		}
		if a.DissolvedOn != nil {
			d := a.DissolvedOn.Format(DateLayout)
			resp.DissolvedOn = &d
		}
		out = append(out, resp)
	}
	return out
}

// parseIntDefault — нечисловое или отрицательное значение заменяется def.
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

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
