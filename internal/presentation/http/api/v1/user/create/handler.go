package createuserhttp

import (
	"errors"
	"net/http"

	createuser "api/internal/application/command/create_user"
	"api/internal/domain/service"
	"api/internal/presentation/http/httpx"

	"github.com/go-playground/validator/v10"
)

// Handler turns HTTP requests into CreateUser commands.
// Validation, decoding and response shaping live here so the use-case
// stays transport-agnostic.
type Handler struct {
	useCase  createuser.UseCase
	validate *validator.Validate
}

// NewHandler constructs the handler.
func NewHandler(uc createuser.UseCase, v *validator.Validate) *Handler {
	return &Handler{useCase: uc, validate: v}
}

// Handle is the http.HandlerFunc.
//
//	@Summary      Create a user
//	@Description  Registers a new user account.
//	@Tags         Users
//	@Accept       json
//	@Produce      json
//	@Param        body  body      CreateUserRequest  true  "User payload"
//	@Success      201   {object}  CreateUserResponse
//	@Failure      400   {object}  httpx.ErrorBody
//	@Failure      401   {object}  httpx.ErrorBody
//	@Failure      500   {object}  httpx.ErrorBody
//	@Security     Bearer
//	@Router       /api/v1/users [post]
func (h *Handler) Handle(w http.ResponseWriter, r *http.Request) {
	var req CreateUserRequest
	if err := httpx.DecodeJSON(r, &req, h.validate); err != nil {
		if errors.Is(err, httpx.ErrBadJSON) {
			httpx.WriteError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		httpx.WriteValidationError(w, err)
		return
	}

	res, err := h.useCase.Handle(r.Context(), createuser.Command{
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Email:     req.Email,
		Password:  req.Password,
		AgencyID:  req.AgencyID,
	})
	if err != nil {
		if errors.Is(err, service.ErrAgencyNotFound) || errors.Is(err, service.ErrAgencyInactive) {
			httpx.WriteError(w, http.StatusBadRequest, err.Error())
			return
		}
		httpx.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, CreateUserResponse{ID: res.ID})
}
