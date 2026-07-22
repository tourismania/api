package updateofferhttp

import (
	"errors"
	"net/http"

	updateoffer "api/internal/application/command/update_offer"
	"api/internal/domain/enum"
	"api/internal/domain/service"
	"api/internal/presentation/http/httpx"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
)

// Handler turns HTTP requests into UpdateOffer commands.
type Handler struct {
	useCase  updateoffer.UseCase
	validate *validator.Validate
}

// NewHandler constructs the handler.
func NewHandler(uc updateoffer.UseCase, v *validator.Validate) *Handler {
	return &Handler{useCase: uc, validate: v}
}

// Handle is the http.HandlerFunc.
//
//	@Summary      Update an offer
//	@Description  Partially updates an offer. Only an agent/super admin belonging to the offer's own agency may update it — 1 user = 1 agency, no cross-agency access. An offer belonging to another agency is reported as not found.
//	@Tags         Offers
//	@Accept       json
//	@Produce      json
//	@Param        uuid  path      string               true  "Offer UUID"
//	@Param        body  body      UpdateOfferRequest   true  "Fields to update"
//	@Success      200   {object}  UpdateOfferResponse
//	@Failure      400   {object}  httpx.ErrorBody
//	@Failure      401   {object}  httpx.ErrorBody
//	@Failure      403   {object}  httpx.ErrorBody
//	@Failure      404   {object}  httpx.ErrorBody
//	@Security     Bearer
//	@Router       /api/v1/offers/{uuid} [patch]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "uuid"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "invalid offer uuid")
		return
	}

	var req UpdateOfferRequest
	if err := httpx.DecodeJSON(r, &req, h.validate); err != nil {
		if errors.Is(err, httpx.ErrBadJSON) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		httpx.WriteValidationError(w, err)
		return
	}

	currentUserUUID, err := custommw.CurrentUserUUID(r.Context())
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	var status *enum.OfferStatus
	if req.Status != nil {
		s := enum.OfferStatus(*req.Status)
		status = &s
	}

	res, err := h.useCase.Handle(r.Context(), updateoffer.Command{
		UUID:            id,
		Title:           req.Title,
		Description:     req.Description,
		Status:          status,
		CurrentUserUUID: currentUserUUID,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrActorNotFound):
			httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		case errors.Is(err, service.ErrInsufficientRole):
			httpx.WriteError(w, http.StatusForbidden, "insufficient role")
		case errors.Is(err, service.ErrOfferNotFound):
			httpx.WriteError(w, http.StatusNotFound, "offer not found")
		case errors.Is(err, service.ErrOfferTitleInvalid),
			errors.Is(err, service.ErrOfferStatusInvalid):
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	httpx.WriteJSON(w, http.StatusOK, UpdateOfferResponse{ID: res.ID, UUID: res.UUID})
}
