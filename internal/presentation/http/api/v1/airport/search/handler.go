package searchairporthttp

import (
	"errors"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	searchairports "api/internal/application/query/search_airports"
	"api/internal/presentation/http/httpx"

	"github.com/go-playground/validator/v10"
)

const (
	// DefaultLimit is applied when the caller omits the limit parameter.
	DefaultLimit = 20
	// DefaultOffset is applied when the caller omits the offset parameter.
	DefaultOffset = 0
)

// multiSpace matches two or more consecutive whitespace characters.
var multiSpace = regexp.MustCompile(`\s+`)

// NormalizeSearch trims whitespace, collapses internal spaces,
// and returns an error if the result is shorter than 2 characters.
func NormalizeSearch(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	s = multiSpace.ReplaceAllString(s, " ")
	if len([]rune(s)) < 2 {
		return "", errors.New("search must be at least 2 characters")
	}
	return s, nil
}

// Handler renders the airport search results.
type Handler struct {
	useCase  searchairports.UseCase
	validate *validator.Validate
}

// NewHandler constructs the handler.
func NewHandler(uc searchairports.UseCase, v *validator.Validate) *Handler {
	return &Handler{useCase: uc, validate: v}
}

// Handle is the http.HandlerFunc.
//
//	@Summary      Search airports
//	@Description  Full-text search over airports and cities with ranking and pagination.
//	@Tags         Airports
//	@Produce      json
//	@Param        search  query     string  true   "Search string (min 2 chars)"
//	@Param        limit   query     int     false  "Max results (1–100, default 20)"
//	@Param        offset  query     int     false  "Pagination offset (0–10000)"
//	@Success      200     {object}  SearchResponse
//	@Failure      400     {object}  httpx.StructuredErrorBody
//	@Failure      429     {object}  httpx.StructuredErrorBody
//	@Failure      500     {object}  httpx.StructuredErrorBody
//	@Security     Bearer
//	@Router       /api/v1/airports [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	q := r.URL.Query()

	// Parse and normalise search.
	rawSearch := q.Get("search")
	search, err := NormalizeSearch(rawSearch)
	if err != nil {
		httpx.WriteStructuredError(w, http.StatusBadRequest, "INVALID_SEARCH",
			"Parameter 'search' must be at least 2 characters long", "search")
		return
	}

	// Parse limit/offset with defaults.
	limit := parseIntDefault(q.Get("limit"), DefaultLimit)
	offset := parseIntDefault(q.Get("offset"), DefaultOffset)

	// Validate bounds via go-playground/validator.
	params := SearchParams{
		Search: search,
		Limit:  limit,
		Offset: offset,
		Lang:   q.Get("lang"),
	}
	if err := h.validate.Struct(params); err != nil {
		httpx.WriteValidationError(w, err)
		return
	}

	res, err := h.useCase.Handle(r.Context(), searchairports.Query{
		Search: search,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		slog.ErrorContext(r.Context(), "airport search failed", "err", err)
		httpx.WriteStructuredError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
			"An unexpected error occurred", "")
		return
	}

	slog.InfoContext(r.Context(), "airport search",
		"search", search,
		"limit", limit,
		"offset", offset,
		"count", len(res.Airports),
		"duration_ms", time.Since(start).Milliseconds(),
	)

	w.Header().Set("Cache-Control", "private, max-age=3600")
	httpx.WriteJSON(w, http.StatusOK, buildResponse(res, search, limit, offset))
}

func buildResponse(res searchairports.Result, search string, limit, offset int) SearchResponse {
	data := make([]AirportResponse, 0, len(res.Airports))
	for _, a := range res.Airports {
		data = append(data, AirportResponse{
			ICAO: a.ICAO,
			IATA: a.IATA,
			Name: a.Name,
			Location: LocationResponse{
				Latitude:    a.Location.Latitude,
				Longitude:   a.Location.Longitude,
				ElevationFt: a.Location.ElevationFt,
			},
			City: CityResponse{
				ID:       a.City.ID,
				Name:     a.City.Name,
				State:    a.City.State,
				Timezone: a.City.Timezone,
			},
			Country: CountryResponse{
				ISO2: a.Country.ISO2,
				Name: a.Country.Name,
			},
		})
	}
	return SearchResponse{
		Data: data,
		Meta: MetaResponse{
			Total:  res.TotalCount,
			Limit:  limit,
			Offset: offset,
			Search: search,
		},
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
