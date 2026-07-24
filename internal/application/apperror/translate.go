package apperror

import (
	"errors"
	"fmt"

	"api/internal/domain/service"
)

// FromDomainError translates a domain sentinel error into one of the
// apperror outcomes above, preserving the original message text so
// clients still see a specific reason. Unrecognized errors are returned
// unchanged — the caller's default case treats them as unexpected
// (500), which is the correct behaviour for a bug rather than a
// business rule violation.
func FromDomainError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, service.ErrActorNotFound):
		return fmt.Errorf("%w: %s", ErrUnauthenticated, err.Error())
	case errors.Is(err, service.ErrInsufficientRole):
		return fmt.Errorf("%w: %s", ErrForbidden, err.Error())
	case errors.Is(err, service.ErrOfferNotFound):
		return fmt.Errorf("%w: %s", ErrNotFound, err.Error())
	case errors.Is(err, service.ErrOfferTitleInvalid),
		errors.Is(err, service.ErrOfferStatusInvalid),
		errors.Is(err, service.ErrAgencyNotFound),
		errors.Is(err, service.ErrAgencyInactive):
		return fmt.Errorf("%w: %s", ErrValidation, err.Error())
	default:
		return err
	}
}
