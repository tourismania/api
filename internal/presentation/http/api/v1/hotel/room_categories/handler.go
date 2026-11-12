package roomcategorieshttp

import (
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"api/internal/application/apperror"
	listhotelroomcategories "api/internal/application/query/list_hotel_room_categories"
	hotelhttp "api/internal/presentation/http/api/v1/hotel"
	"api/internal/presentation/http/httpx"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-chi/chi/v5"
)

// Сообщения ответов об ошибках.
const (
	invalidIDMessage = "invalid hotel id"
	notFoundMessage  = "hotel not found"
)

// Handler отдаёт категории номеров отеля агентства пользователя.
type Handler struct {
	useCase listhotelroomcategories.UseCase
}

// NewHandler собирает хендлер.
func NewHandler(uc listhotelroomcategories.UseCase) *Handler {
	return &Handler{useCase: uc}
}

// Handle — http.HandlerFunc.
//
//	@Summary      List hotel room categories
//	@Description  Все категории номеров отеля справочника своего агентства в порядке создания, без пагинации. Отель другого агентства, удалённый или несуществующий — 404 hotel not found; id не целое ≥ 1 — 400 invalid hotel id. Читать может любой пользователь агентства.
//	@Tags         Hotels
//	@Produce      json
//	@Param        id   path      int  true  "Id отеля справочника"
//	@Success      200  {object}  ListResponse
//	@Failure      400  {object}  httpx.ErrorBody
//	@Failure      401  {object}  httpx.ErrorBody
//	@Failure      404  {object}  httpx.ErrorBody
//	@Failure      429  {object}  httpx.StructuredErrorBody
//	@Failure      500  {object}  httpx.StructuredErrorBody
//	@Security     Bearer
//	@Router       /api/v1/hotels/{id}/room-categories [get]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil || id < 1 {
		httpx.WriteError(w, http.StatusBadRequest, invalidIDMessage)
		return
	}

	currentUserUUID, ok := custommw.CurrentUserUUIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	res, err := h.useCase.Handle(r.Context(), listhotelroomcategories.Query{
		CurrentUserUUID: currentUserUUID,
		HotelID:         id,
	})
	if err != nil {
		switch {
		case errors.Is(err, apperror.ErrUnauthenticated):
			httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		case errors.Is(err, apperror.ErrNotFound):
			httpx.WriteError(w, http.StatusNotFound, notFoundMessage)
		default:
			slog.ErrorContext(r.Context(), "list hotel room categories failed", "hotel_id", id, "err", err)
			httpx.WriteStructuredError(w, http.StatusInternalServerError, "INTERNAL_ERROR",
				"An unexpected error occurred", "")
		}
		return
	}

	out := make([]RoomCategoryResponse, 0, len(res.RoomCategories))
	for _, c := range res.RoomCategories {
		photos := c.PhotoURLs
		if photos == nil {
			photos = []string{}
		}
		out = append(out, RoomCategoryResponse{ID: c.ID, Name: c.Name, Description: c.Description, PhotoURLs: photos})
	}
	w.Header().Set("Cache-Control", hotelhttp.CacheControl)
	httpx.WriteJSON(w, http.StatusOK, ListResponse{Data: out})
}
