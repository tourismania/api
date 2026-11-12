package searchhotels

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"api/internal/application/apperror"
	"api/internal/domain/hotel"
	"api/internal/domain/user"
)

// MinSearchLen — минимальная длина строки поиска после нормализации.
const MinSearchLen = 2

var (
	// ErrSearchTooShort — строка поиска после нормализации короче
	// MinSearchLen или не передана.
	ErrSearchTooShort = errors.New("search must be at least 2 characters")

	// multiSpace — подряд идущие пробельные символы.
	multiSpace = regexp.MustCompile(`\s+`)
)

// UseCase — порт, от которого зависит presentation-слой.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// HotelSearcher — порт поиска; реализует infrastructure.
type HotelSearcher interface {
	Search(ctx context.Context, f hotel.Filter) (hotel.SearchResult, error)
}

// Handler выполняет поиск отелей агентства пользователя. Читать справочник
// может любой пользователь агентства — роль не проверяется.
type Handler struct {
	hotels     HotelSearcher
	userFinder *user.Finder
}

// NewHandler собирает обработчик.
func NewHandler(hotels HotelSearcher, userFinder *user.Finder) *Handler {
	return &Handler{hotels: hotels, userFinder: userFinder}
}

// Handle нормализует строку поиска и ищет отели агентства пользователя.
// Строка короче MinSearchLen (или не передана) — ErrSearchTooShort;
// пользователь не найден — apperror.ErrUnauthenticated.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	search, err := normalizeSearch(q.Search)
	if err != nil {
		return Result{}, err
	}
	actor, err := h.userFinder.Resolve(ctx, q.CurrentUserUUID)
	if err != nil {
		return Result{}, apperror.FromDomainError(err)
	}

	res, err := h.hotels.Search(ctx, hotel.Filter{
		AgencyID: actor.AgencyID,
		Search:   search,
		Limit:    q.Limit,
		Offset:   q.Offset,
	})
	if err != nil {
		return Result{}, fmt.Errorf("search hotels: %w", err)
	}

	out := make([]HotelResult, 0, len(res.Hotels))
	for _, ht := range res.Hotels {
		out = append(out, HotelResult{
			ID:          ht.ID,
			Name:        ht.Name,
			Stars:       ht.Stars,
			Address:     ht.Address,
			Description: ht.Description,
			PhotoURLs:   ht.PhotoURLs,
		})
	}
	return Result{Hotels: out, TotalCount: res.TotalCount, Search: search}, nil
}

// normalizeSearch обрезает пробелы по краям и схлопывает подряд идущие
// пробельные символы; результат короче MinSearchLen символов или
// отсутствие строки — ErrSearchTooShort.
func normalizeSearch(raw *string) (string, error) {
	if raw == nil {
		return "", ErrSearchTooShort
	}
	s := multiSpace.ReplaceAllString(strings.TrimSpace(*raw), " ")
	if len([]rune(s)) < MinSearchLen {
		return "", ErrSearchTooShort
	}
	return s, nil
}

// Compile-time check.
var _ UseCase = (*Handler)(nil)
