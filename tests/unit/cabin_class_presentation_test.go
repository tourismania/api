package unit_test

import (
	"reflect"
	"testing"

	"api/internal/domain/offer/flight"
	createofferhttp "api/internal/presentation/http/api/v1/offer/create"
	getofferhttp "api/internal/presentation/http/api/v1/offer/get"
	getpublicofferhttp "api/internal/presentation/http/api/v1/offer/get_public"
	updateofferhttp "api/internal/presentation/http/api/v1/offer/update"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Список классов обслуживания живёт только в домене (flight.CabinClasses).
// Presentation его не дублирует — ни проверкой (validate:"oneof=…"), ни
// документацией swagger (enums:"…"): поле имеет доменный тип
// flight.CabinClass, и swag строит enum из его констант сам. Иначе при
// добавлении класса список пришлось бы править в нескольких местах (см.
// docs/cabin-classes.md).
func TestCabinClassDTO_DoesNotDuplicateDomainCabinClasses(t *testing.T) {
	for name, dto := range map[string]any{
		"create.FlightSegmentInput":        createofferhttp.FlightSegmentInput{},
		"update.FlightSegmentInput":        updateofferhttp.FlightSegmentInput{},
		"get.FlightSegmentResponse":        getofferhttp.FlightSegmentResponse{},
		"get_public.FlightSegmentResponse": getpublicofferhttp.FlightSegmentResponse{},
	} {
		field, ok := reflect.TypeOf(dto).FieldByName("CabinClass")
		require.True(t, ok, name)
		assert.Equal(t, reflect.TypeOf(flight.CabinClass("")), field.Type, "%s: тип поля — доменный enum, swag берёт значения из его констант", name)
		assert.Empty(t, field.Tag.Get("validate"), "%s: допустимые классы проверяет домен, не валидатор", name)
		assert.Empty(t, field.Tag.Get("enums"), "%s: список классов не дублируется в swagger", name)
	}
}
