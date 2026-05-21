// Package loginhttp is the HTTP boundary for the password-grant login.
package loginhttp

// LoginRequest check_path body: {"email","password"}.
type LoginRequest struct {
	Email    string `json:"email" validate:"required"`
	Password string `json:"password" validate:"required"`
}

// LoginResponse is the issued token envelope.
type LoginResponse struct {
	Token string `json:"token"`
}
