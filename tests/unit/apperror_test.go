package unit_test

import (
	"api/internal/application/apperror"
	"api/internal/domain/money"
	"api/internal/domain/offer/flight"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestFromDomainError_ManagerCommentInvalid_IsValidation(t *testing.T) {
	err := apperror.FromDomainError(flight.ErrManagerCommentInvalid)

	assert.ErrorIs(t, err, apperror.ErrValidation)
	assert.Contains(t, err.Error(), flight.ErrManagerCommentInvalid.Error())
}

func TestFromDomainError_CabinClassInvalid_IsValidation(t *testing.T) {
	err := apperror.FromDomainError(flight.ErrCabinClassInvalid)

	assert.ErrorIs(t, err, apperror.ErrValidation)
	assert.Contains(t, err.Error(), flight.ErrCabinClassInvalid.Error())
}

func TestFieldError_IsValidationAndKeepsCause(t *testing.T) {
	err := apperror.NewFieldError("Price", "money", money.ErrAmountInvalid)

	assert.ErrorIs(t, err, apperror.ErrValidation)
	assert.ErrorIs(t, err, money.ErrAmountInvalid)
	assert.Equal(t, "validation failed: "+money.ErrAmountInvalid.Error(), err.Error())
}

func TestFromDomainError_FieldError_PassesThroughWithField(t *testing.T) {
	in := apperror.NewFieldError("Currency", "currency", money.ErrCurrencyUnsupported)

	out := apperror.FromDomainError(in)

	fe, ok := apperror.AsFieldError(out)
	require.True(t, ok, "поле не должно теряться при переводе ошибок")
	assert.Equal(t, "Currency", fe.Field)
	assert.Equal(t, "currency", fe.Rule)
}

func TestFromDomainError_UnknownError_PassesThrough(t *testing.T) {
	unexpected := errors.New("boom")
	assert.Equal(t, unexpected, apperror.FromDomainError(unexpected))
}
