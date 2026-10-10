package unit_test

import (
	"api/internal/application/apperror"
	"api/internal/domain/money"
	"api/internal/domain/offer/flight"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFromDomainError_MoneyErrors_AreValidation(t *testing.T) {
	for _, domainErr := range []error{money.ErrCurrencyUnsupported, money.ErrAmountInvalid} {
		err := apperror.FromDomainError(domainErr)
		assert.ErrorIs(t, err, apperror.ErrValidation, domainErr.Error())
		assert.Contains(t, err.Error(), domainErr.Error(), "причина должна сохраниться в тексте")
	}
}

func TestFromDomainError_FlightErrors_AreValidation(t *testing.T) {
	err := apperror.FromDomainError(flight.ErrSegmentDiscontinuous)
	assert.ErrorIs(t, err, apperror.ErrValidation)
}

func TestFromDomainError_UnknownError_PassesThrough(t *testing.T) {
	unexpected := errors.New("boom")
	assert.Equal(t, unexpected, apperror.FromDomainError(unexpected))
}
