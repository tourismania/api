package createofferhttp

import (
	"errors"
	"net/http"

	createoffer "api/internal/application/command/create_offer"
	"api/internal/domain/enum"
	"api/internal/domain/service"
	"api/internal/presentation/http/httpx"
	custommw "api/internal/presentation/http/middleware"

	"github.com/go-playground/validator/v10"
)

// Handler turns HTTP requests into CreateOffer commands.
type Handler struct {
	useCase  createoffer.UseCase
	validate *validator.Validate
}

// NewHandler constructs the handler.
func NewHandler(uc createoffer.UseCase, v *validator.Validate) *Handler {
	return &Handler{useCase: uc, validate: v}
}

// Handle is the http.HandlerFunc.
//
//	@Summary      Create an offer
//	@Description  Publishes a new offer under the caller's own agency. Requires ROLE_AGENT or ROLE_SUPER_ADMIN.
//	@Tags         Offers
//	@Accept       json
//	@Produce      json
//	@Param        body  body      CreateOfferRequest  true  "Offer payload"
//	@Success      201   {object}  CreateOfferResponse
//	@Failure      400   {object}  httpx.ErrorBody
//	@Failure      401   {object}  httpx.ErrorBody
//	@Failure      403   {object}  httpx.ErrorBody
//	@Failure      500   {object}  httpx.ErrorBody
//	@Security     Bearer
//	@Router       /api/v1/offers [post]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	var req CreateOfferRequest
	if err := httpx.DecodeJSON(r, &req, h.validate); err != nil {
		if errors.Is(err, httpx.ErrBadJSON) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		httpx.WriteValidationError(w, err)
		return
	}

	cu, ok := custommw.CurrentUserFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "unauthenticated")
		return
	}

	res, err := h.useCase.Handle(r.Context(), createoffer.Command{
		Title:           req.Title,
		Description:     req.Description,
		Status:          enum.OfferStatus(req.Status),
		CurrentUserID:   cu.ID,
		CurrentAgencyID: cu.AgencyID,
		CurrentRoles:    cu.Roles,
	})
	if err != nil {
		switch {
		case errors.Is(err, service.ErrOfferForbidden):
			httpx.WriteError(w, http.StatusForbidden, err.Error())
		case errors.Is(err, service.ErrOfferTitleInvalid),
			errors.Is(err, service.ErrOfferStatusInvalid),
			errors.Is(err, service.ErrActorHasNoAgency),
			errors.Is(err, service.ErrAgencyNotFound),
			errors.Is(err, service.ErrAgencyInactive):
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
		default:
			httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, CreateOfferResponse{ID: res.ID, UUID: res.UUID})
}
