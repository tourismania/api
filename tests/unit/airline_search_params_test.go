package unit_test

import (
	"testing"

	searchairlinehttp "api/internal/presentation/http/api/v1/airline/search"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAirlineNormalizeSearch_TrimsAndCollapsesSpaces(t *testing.T) {
	got, err := searchairlinehttp.NormalizeSearch("  Turkish \t  Airlines ")
	require.NoError(t, err)
	assert.Equal(t, "Turkish Airlines", got)
}

func TestAirlineNormalizeSearch_ShorterThanThree_Error(t *testing.T) {
	for _, s := range []string{"", "  ", "SU", " S  "} {
		_, err := searchairlinehttp.NormalizeSearch(s)
		assert.ErrorIs(t, err, searchairlinehttp.ErrSearchTooShort, "%q", s)
	}
}

func TestAirlineNormalizeSearch_ThreeRunes_OK(t *testing.T) {
	got, err := searchairlinehttp.NormalizeSearch("тур")
	require.NoError(t, err)
	assert.Equal(t, "тур", got)
}

func TestAirlineParseIDs_ValidDedupedInOrder(t *testing.T) {
	got, err := searchairlinehttp.ParseIDs("42,57,42, 7")
	require.NoError(t, err)
	assert.Equal(t, []int{42, 57, 7}, got)
}

func TestAirlineParseIDs_Invalid_Error(t *testing.T) {
	for _, raw := range []string{"0", "-1", "abc", "42,,57", "", "1.5"} {
		_, err := searchairlinehttp.ParseIDs(raw)
		assert.ErrorIs(t, err, searchairlinehttp.ErrIDsInvalid, "%q", raw)
	}
}

func TestAirlineParseCountry_UppercasedOrError(t *testing.T) {
	got, err := searchairlinehttp.ParseCountry("tr")
	require.NoError(t, err)
	assert.Equal(t, "TR", got)

	for _, raw := range []string{"TUR", "1", "T", "т1"} {
		_, err := searchairlinehttp.ParseCountry(raw)
		assert.ErrorIs(t, err, searchairlinehttp.ErrCountryInvalid, "%q", raw)
	}
}

func TestAirlineValidationMeta_IncludesBound(t *testing.T) {
	v := validator.New(validator.WithRequiredStructEnabled())
	for name, tc := range map[string]struct {
		params searchairlinehttp.SearchParams
		want   map[string]any
	}{
		"limit":  {searchairlinehttp.SearchParams{Limit: 101}, map[string]any{"Limit": "max=100"}},
		"offset": {searchairlinehttp.SearchParams{Limit: 20, Offset: 10_001}, map[string]any{"Offset": "max=10000"}},
		"ids":    {searchairlinehttp.SearchParams{Limit: 20, IDs: make([]int, 101)}, map[string]any{"IDs": "max=100"}},
	} {
		t.Run(name, func(t *testing.T) {
			err := v.Struct(tc.params)
			require.Error(t, err)
			assert.Equal(t, tc.want, searchairlinehttp.ValidationMeta(err))
		})
	}
}

func TestAirlineSearchDefaults(t *testing.T) {
	assert.Equal(t, 20, searchairlinehttp.DefaultLimit)
	assert.Equal(t, 0, searchairlinehttp.DefaultOffset)
	assert.Equal(t, 3, searchairlinehttp.MinSearchLen)
}
