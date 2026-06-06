package unit_test

import (
	"testing"

	searchairporthttp "api/internal/presentation/http/api/v1/airport/search"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNormalizeSearch_Trim(t *testing.T) {
	got, err := searchairporthttp.NormalizeSearch("  SVO  ")
	require.NoError(t, err)
	assert.Equal(t, "SVO", got)
}

func TestNormalizeSearch_CollapseSpaces(t *testing.T) {
	got, err := searchairporthttp.NormalizeSearch("Moscow   Airport")
	require.NoError(t, err)
	assert.Equal(t, "Moscow Airport", got)
}

func TestNormalizeSearch_TooShort_ReturnsError(t *testing.T) {
	_, err := searchairporthttp.NormalizeSearch("a")
	assert.Error(t, err)
}

func TestNormalizeSearch_Empty_ReturnsError(t *testing.T) {
	_, err := searchairporthttp.NormalizeSearch("")
	assert.Error(t, err)
}

func TestNormalizeSearch_ExactlyTwo_OK(t *testing.T) {
	got, err := searchairporthttp.NormalizeSearch("SV")
	require.NoError(t, err)
	assert.Equal(t, "SV", got)
}

func TestDefaultLimit(t *testing.T) {
	assert.Equal(t, 20, searchairporthttp.DefaultLimit)
}

func TestDefaultOffset(t *testing.T) {
	assert.Equal(t, 0, searchairporthttp.DefaultOffset)
}
