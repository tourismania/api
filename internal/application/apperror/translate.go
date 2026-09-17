package apperror

import (
	"api/internal/domain/agency"
	"api/internal/domain/offer"
	"api/internal/domain/offer/flight"
	"api/internal/domain/user"
	"errors"
	"fmt"
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
	case errors.Is(err, user.ErrActorNotFound):
		return fmt.Errorf("%w: %s", ErrUnauthenticated, err.Error())
	case errors.Is(err, offer.ErrInsufficientRole):
		return fmt.Errorf("%w: %s", ErrForbidden, err.Error())
	case errors.Is(err, offer.ErrNotFound):
		return fmt.Errorf("%w: %s", ErrNotFound, err.Error())
	case errors.Is(err, offer.ErrTitleInvalid),
		errors.Is(err, offer.ErrStatusInvalid),
		errors.Is(err, agency.ErrNotFound),
		errors.Is(err, agency.ErrInactive),
		errors.Is(err, flight.ErrAirportNotFound),
		errors.Is(err, flight.ErrSegmentsEmpty),
		errors.Is(err, flight.ErrSegmentChronologyInvalid),
		errors.Is(err, flight.ErrSegmentDiscontinuous),
		errors.Is(err, flight.ErrLayoverNonPositive):
		return fmt.Errorf("%w: %s", ErrValidation, err.Error())
	default:
		return err
	}
}
