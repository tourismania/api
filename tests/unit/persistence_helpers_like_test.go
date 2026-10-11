package unit_test

import (
	"testing"

	"api/internal/infrastructure/persistence/helpers"

	"github.com/stretchr/testify/assert"
)

func TestEscapeLike_EscapesSpecialChars(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{name: "без спецсимволов", in: "Москва Шереметьево", want: "Москва Шереметьево"},
		{name: "процент", in: "%%", want: `\%\%`},
		{name: "подчёркивание", in: "Al_ha", want: `Al\_ha`},
		{name: "обратная черта", in: `Al\pha`, want: `Al\\pha`},
		// Вставленные `\` не экранируются повторно.
		{name: "уже экранированный процент", in: `\%`, want: `\\\%`},
		{name: "смесь", in: `Test_Field 50%`, want: `Test\_Field 50\%`},
		{name: "пустая строка", in: "", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, helpers.EscapeLike(tc.in))
		})
	}
}
