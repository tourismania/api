package wikidata

import (
	"testing"

	syncairports "api/internal/application/command/sync_airports"

	"github.com/stretchr/testify/assert"
)

func TestEscapeSPARQLString_QuotesAndBackslashes_Escaped(t *testing.T) {
	got := escapeSPARQLString(`Coeur d"Alene\Test`)
	assert.Equal(t, `Coeur d\"Alene\\Test`, got)
}

func TestEscapeSPARQLString_PlainString_Unchanged(t *testing.T) {
	assert.Equal(t, "Krasnoyarsk", escapeSPARQLString("Krasnoyarsk"))
}

func TestGroupByCountry_MixedCountries_GroupsByISO2(t *testing.T) {
	got := groupByCountry([]syncairports.CityRef{
		{Name: "Moscow", CountryISO2: "RU"},
		{Name: "Bykovo", CountryISO2: "RU"},
		{Name: "Paris", CountryISO2: "FR"},
	})

	assert.ElementsMatch(t, []string{"Moscow", "Bykovo"}, got["RU"])
	assert.ElementsMatch(t, []string{"Paris"}, got["FR"])
	assert.Len(t, got, 2)
}

func TestChunkStrings_ExactMultiple_SplitsEvenly(t *testing.T) {
	names := []string{"a", "b", "c", "d"}
	got := chunkStrings(names, 2)
	assert.Equal(t, [][]string{{"a", "b"}, {"c", "d"}}, got)
}

func TestChunkStrings_Remainder_LastChunkSmaller(t *testing.T) {
	names := []string{"a", "b", "c"}
	got := chunkStrings(names, 2)
	assert.Equal(t, [][]string{{"a", "b"}, {"c"}}, got)
}

func TestChunkStrings_Empty_ReturnsNoChunks(t *testing.T) {
	got := chunkStrings(nil, 2)
	assert.Empty(t, got)
}

func TestBuildCityQuery_ContainsQIDAndEscapedValues(t *testing.T) {
	q := buildCityQuery("Q159", []string{"Moscow", `Odd"Name`})

	assert.Contains(t, q, "wd:Q159")
	assert.Contains(t, q, `"Moscow"@en`)
	assert.Contains(t, q, `"Odd\"Name"@en`)
	assert.Contains(t, q, "wdt:P625", "must filter to entities with a geographic coordinate")
}
