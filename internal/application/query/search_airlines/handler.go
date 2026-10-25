package searchairlines

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"api/internal/application/apperror"
	"api/internal/domain/airline"
)

// MinSearchLen — минимальная длина строки поиска после нормализации.
const MinSearchLen = 3

// Имя поля и правило, под которыми ошибка страны уходит в meta ответа
// «validation failed».
const (
	countryField = "Country"
	countryRule  = "iso2"
)

var (
	// ErrSearchTooShort — строка поиска после нормализации короче
	// MinSearchLen или не передана без IDs.
	ErrSearchTooShort = errors.New("search must be at least 3 characters")
	// ErrCountryInvalid — country не две латинские буквы.
	ErrCountryInvalid = errors.New("country must be an ISO 3166-1 alpha-2 code")

	// multiSpace — подряд идущие пробельные символы.
	multiSpace = regexp.MustCompile(`\s+`)
	// countryPattern — код страны ISO 3166-1 alpha-2 в любом регистре.
	countryPattern = regexp.MustCompile(`^[A-Za-z]{2}$`)
)

// UseCase — порт, от которого зависит presentation-слой.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// AirlineSearcher — порт поиска; реализует infrastructure.
type AirlineSearcher interface {
	Search(ctx context.Context, f airline.Filter) (airline.SearchResult, error)
}

// Handler выполняет поиск авиакомпаний.
type Handler struct {
	airlines AirlineSearcher
}

// NewHandler собирает обработчик.
func NewHandler(airlines AirlineSearcher) *Handler {
	return &Handler{airlines: airlines}
}

// Handle нормализует строку поиска и код страны и возвращает выбираемые
// авиакомпании (без дублей и пропавших). Строка поиска короче MinSearchLen
// (или не передана без IDs) — ErrSearchTooShort; некорректная страна —
// *apperror.FieldError (Country/iso2).
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	search, err := normalizeSearch(q.Search, len(q.IDs) > 0)
	if err != nil {
		return Result{}, err
	}
	country, err := normalizeCountry(q.Country)
	if err != nil {
		return Result{}, err
	}

	res, err := h.airlines.Search(ctx, airline.Filter{
		Search:      deref(search),
		CountryISO2: deref(country),
		IDs:         q.IDs,
		Limit:       q.Limit,
		Offset:      q.Offset,
	})
	if err != nil {
		return Result{}, fmt.Errorf("search airlines: %w", err)
	}

	out := make([]AirlineResult, 0, len(res.Airlines))
	for _, a := range res.Airlines {
		r := AirlineResult{
			ID:          a.ID,
			ICAO:        a.ICAO,
			Name:        a.Name,
			NameRU:      a.NameRU,
			DissolvedOn: a.DissolvedOn,
		}
		if a.Country != nil {
			r.Country = &CountryResult{ISO2: a.Country.ISO2, Name: a.Country.Name}
		}
		out = append(out, r)
	}
	return Result{Airlines: out, TotalCount: res.TotalCount, Search: search, Country: country}, nil
}

// normalizeSearch обрезает пробелы по краям и схлопывает подряд идущие
// пробельные символы. Переданная строка проверяется всегда, в т. ч. вместе с
// IDs; не переданная допустима только с IDs (nil — без отбора по названию).
func normalizeSearch(raw *string, withIDs bool) (*string, error) {
	if raw == nil {
		if withIDs {
			return nil, nil
		}
		return nil, ErrSearchTooShort
	}
	s := multiSpace.ReplaceAllString(strings.TrimSpace(*raw), " ")
	if len([]rune(s)) < MinSearchLen {
		return nil, ErrSearchTooShort
	}
	return &s, nil
}

// normalizeCountry проверяет код страны и приводит его к верхнему регистру;
// "" — без отбора (nil).
func normalizeCountry(raw string) (*string, error) {
	if raw == "" {
		return nil, nil
	}
	if !countryPattern.MatchString(raw) {
		return nil, apperror.NewFieldError(countryField, countryRule, ErrCountryInvalid)
	}
	c := strings.ToUpper(raw)
	return &c, nil
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// Compile-time check.
var _ UseCase = (*Handler)(nil)
