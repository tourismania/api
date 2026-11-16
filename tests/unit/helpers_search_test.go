package unit_test

import (
	"testing"

	"api/internal/application/helpers"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeSearch_TrimsAndCollapsesWhitespace(t *testing.T) {
	for raw, want := range map[string]struct {
		search string
		ok     bool
	}{
		"  Hilton \t  Istanbul ": {search: "Hilton Istanbul", ok: true},
		"ab":                     {search: "ab", ok: true},
		"Ят":                     {search: "Ят", ok: true},
		" h ":                    {search: "h", ok: false},
		"   ":                    {search: "", ok: false},
		"":                       {search: "", ok: false},
	} {
		t.Run(raw, func(t *testing.T) {
			search, ok := helpers.NormalizeSearch(raw, 2)

			assert.Equal(t, want.search, search)
			assert.Equal(t, want.ok, ok, "порог считается в символах, а не байтах")
		})
	}
}
