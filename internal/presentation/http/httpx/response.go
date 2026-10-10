// Package httpx is a tiny helper layer for JSON encoding and uniform
// error responses. It exists so handlers stay one-shot readable.
package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-playground/validator/v10"
)

// ErrorBody is the canonical error envelope.
type ErrorBody struct {
	Error string         `json:"error"`
	Code  int            `json:"code"`
	Meta  map[string]any `json:"meta,omitempty"`
}

// WriteJSON writes status + body as JSON. Errors during encoding are
// logged; we deliberately do not try to recover (the client already
// has a status code by then).
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("encode response", "err", err)
	}
}

// WriteError sends a structured error.
func WriteError(w http.ResponseWriter, status int, msg string) {
	WriteJSON(w, status, ErrorBody{Error: msg, Code: status})
}

// WriteFieldValidationError отдаёт 400 «validation failed» с meta
// {field: rule} — в том же виде, что WriteValidationError, но для ошибки
// поля, найденной application-слоем (apperror.FieldError).
func WriteFieldValidationError(w http.ResponseWriter, field, rule string) {
	WriteJSON(w, http.StatusBadRequest, ErrorBody{
		Error: "validation failed",
		Code:  http.StatusBadRequest,
		Meta:  map[string]any{field: rule},
	})
}

// WriteValidationError flattens validator errors into a 400 response.
func WriteValidationError(w http.ResponseWriter, err error) {
	meta := map[string]any{}
	var verrs validator.ValidationErrors
	if errors.As(err, &verrs) {
		for _, fe := range verrs {
			meta[fe.Field()] = fe.Tag()
		}
	}
	WriteJSON(w, http.StatusBadRequest, ErrorBody{
		Error: "validation failed",
		Code:  http.StatusBadRequest,
		Meta:  meta,
	})
}
