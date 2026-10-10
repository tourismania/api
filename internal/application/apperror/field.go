package apperror

import (
	"errors"
	"fmt"
)

// FieldError — нарушение бизнес-правила в конкретном поле входных данных
// (например, валюта не из справочника или лишние знаки после точки в
// цене). Это ErrValidation (errors.Is(err, ErrValidation) == true), но с
// именем поля и правила: presentation отдаёт их в meta ответа 400 в том
// же виде, что и ошибки структурной валидации запроса.
type FieldError struct {
	// Field — имя поля, как в meta ошибок валидатора (например, "Price").
	Field string
	// Rule — нарушенное правило (например, "money", "currency").
	Rule string
	// Err — исходная доменная ошибка с причиной.
	Err error
}

// NewFieldError создаёт ошибку поля field, нарушившего правило rule.
func NewFieldError(field, rule string, err error) *FieldError {
	return &FieldError{Field: field, Rule: rule, Err: err}
}

// Error возвращает «validation failed: <причина>».
func (e *FieldError) Error() string {
	return fmt.Sprintf("%s: %s", ErrValidation.Error(), e.Err.Error())
}

// Unwrap делает FieldError одновременно ErrValidation и исходной доменной
// ошибкой для errors.Is / errors.As.
func (e *FieldError) Unwrap() []error {
	return []error{ErrValidation, e.Err}
}

// AsFieldError достаёт *FieldError из цепочки ошибок.
func AsFieldError(err error) (*FieldError, bool) {
	var fe *FieldError
	ok := errors.As(err, &fe)
	return fe, ok
}
