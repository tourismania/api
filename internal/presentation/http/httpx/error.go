package httpx

import "net/http"

// StructuredErrorBody is the error envelope used by public endpoints
// that return machine-readable error codes (e.g. airport search).
type StructuredErrorBody struct {
	Error StructuredError `json:"error"`
}

// StructuredError carries a machine-readable code alongside the human message.
type StructuredError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

// WriteStructuredError sends a structured error response.
func WriteStructuredError(w http.ResponseWriter, status int, code, message, field string) {
	WriteJSON(w, status, StructuredErrorBody{
		Error: StructuredError{Code: code, Message: message, Field: field},
	})
}
