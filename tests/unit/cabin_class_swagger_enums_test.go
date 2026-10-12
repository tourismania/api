package unit_test

import (
	"reflect"
	"strings"
	"testing"

	"api/internal/domain/offer/flight"
	createofferhttp "api/internal/presentation/http/api/v1/offer/create"
	getofferhttp "api/internal/presentation/http/api/v1/offer/get"
	getpublicofferhttp "api/internal/presentation/http/api/v1/offer/get_public"
	updateofferhttp "api/internal/presentation/http/api/v1/offer/update"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Тег enums у cabin_class в DTO — только документация swagger, источник
// правды — flight.CabinClasses(). Тест ловит расхождение при добавлении,
// переименовании или удалении класса (см. docs/cabin-classes.md).
func TestCabinClassSwaggerEnums_MatchDomainCabinClasses(t *testing.T) {
	classes := []string{string(flight.CabinClassUnknown)}
	for _, c := range flight.CabinClasses() {
		classes = append(classes, string(c))
	}
	want := strings.Join(classes, ",")

	for name, dto := range map[string]any{
		"create.FlightSegmentInput":        createofferhttp.FlightSegmentInput{},
		"update.FlightSegmentInput":        updateofferhttp.FlightSegmentInput{},
		"get.FlightSegmentResponse":        getofferhttp.FlightSegmentResponse{},
		"get_public.FlightSegmentResponse": getpublicofferhttp.FlightSegmentResponse{},
	} {
		field, ok := reflect.TypeOf(dto).FieldByName("CabinClass")
		require.True(t, ok, name)
		assert.Equal(t, want, field.Tag.Get("enums"), name)
		assert.Empty(t, field.Tag.Get("validate"), "%s: допустимые классы проверяет домен, не валидатор", name)
	}
}
