// Package apperror defines the small, fixed set of HTTP-shaped outcomes
// that application-layer handlers can return. Domain sentinel errors
// (see internal/domain/service) never cross into the presentation
// layer directly: every command/query handler translates them via
// FromDomainError before returning, so presentation only ever imports
// this package — never internal/domain/service — to decide the
// response code. This keeps the domain's vocabulary of errors private
// to the domain and application layers, matching the dependency
// direction Presentation → Application → Domain.
package apperror

import "errors"

var (
	// ErrUnauthenticated means the acting principal could not be resolved.
	ErrUnauthenticated = errors.New("unauthenticated")
	// ErrForbidden means the principal is known but lacks a required permission.
	ErrForbidden = errors.New("forbidden")
	// ErrNotFound means the requested resource does not exist for this principal.
	ErrNotFound = errors.New("not found")
	// ErrValidation means the request failed a business validation rule.
	ErrValidation = errors.New("validation failed")
)
