package listhotelroomcategories

import (
	"context"
	"fmt"

	"api/internal/application/apperror"
	"api/internal/domain/hotel"
	"api/internal/domain/user"
)

// UseCase — порт, от которого зависит presentation-слой.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// HotelReader — порт чтения справочника; реализует infrastructure.
type HotelReader interface {
	FindByIDsAndAgency(ctx context.Context, ids []int, agencyID int) ([]hotel.Hotel, error)
	ListRoomCategories(ctx context.Context, hotelID int) ([]hotel.RoomCategory, error)
}

// Handler отдаёт категории номеров отеля агентства пользователя. Читать
// справочник может любой пользователь агентства — роль не проверяется.
type Handler struct {
	hotels     HotelReader
	userFinder *user.Finder
}

// NewHandler собирает обработчик.
func NewHandler(hotels HotelReader, userFinder *user.Finder) *Handler {
	return &Handler{hotels: hotels, userFinder: userFinder}
}

// Handle проверяет, что отель принадлежит агентству пользователя и не
// удалён, и возвращает его категории в порядке создания. Чужой, удалённый
// или несуществующий отель — apperror.ErrNotFound (hotel not found);
// пользователь не найден — apperror.ErrUnauthenticated.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	actor, err := h.userFinder.Resolve(ctx, q.CurrentUserUUID)
	if err != nil {
		return Result{}, apperror.FromDomainError(err)
	}

	found, err := h.hotels.FindByIDsAndAgency(ctx, []int{q.HotelID}, actor.AgencyID)
	if err != nil {
		return Result{}, fmt.Errorf("find hotel: %w", err)
	}
	if len(found) == 0 {
		return Result{}, apperror.FromDomainError(hotel.ErrNotFound)
	}

	cats, err := h.hotels.ListRoomCategories(ctx, q.HotelID)
	if err != nil {
		return Result{}, fmt.Errorf("list room categories: %w", err)
	}
	out := make([]RoomCategoryResult, 0, len(cats))
	for _, c := range cats {
		out = append(out, RoomCategoryResult{
			ID:          c.ID,
			Name:        c.Name,
			Description: c.Description,
			PhotoURLs:   c.PhotoURLs,
		})
	}
	return Result{RoomCategories: out}, nil
}

// Compile-time check.
var _ UseCase = (*Handler)(nil)
