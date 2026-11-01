package unit_test

import (
	"encoding/json"
	"testing"
	"time"

	offerhttp "api/internal/presentation/http/api/v1/offer"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLocalDateTime_Valid_ParsedAsWallClockInUTC(t *testing.T) {
	var got offerhttp.LocalDateTime

	require.NoError(t, json.Unmarshal([]byte(`"2026-05-14T10:00:00"`), &got))

	assert.Equal(t, time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC), got.Time())
}

func TestLocalDateTime_Marshal_NoOffset(t *testing.T) {
	// Часы пишутся как есть, в какой бы зоне ни лежало значение.
	cases := map[string]time.Time{
		"UTC":      time.Date(2026, 5, 14, 10, 0, 0, 0, time.UTC),
		"не UTC":   time.Date(2026, 5, 14, 10, 0, 0, 0, time.FixedZone("", 5*60*60)),
		"с долями": time.Date(2026, 5, 14, 10, 0, 0, 999, time.UTC),
	}
	for name, wall := range cases {
		t.Run(name, func(t *testing.T) {
			got, err := json.Marshal(offerhttp.NewLocalDateTime(wall))

			require.NoError(t, err)
			assert.Equal(t, `"2026-05-14T10:00:00"`, string(got))
		})
	}
}

func TestLocalDateTime_Invalid_ReturnsFormatError(t *testing.T) {
	cases := map[string]string{
		"Z":          `"2026-05-14T10:00:00Z"`,
		"смещение":   `"2026-05-14T10:00:00+05:00"`,
		"без секунд": `"2026-05-14T10:00"`,
		"дробные":    `"2026-05-14T10:00:00.5"`,
		"пробел":     `"2026-05-14 10:00:00"`,
		"нет даты":   `"2026-02-30T10:00:00"`,
		"мусор":      `"завтра"`,
		"пусто":      `""`,
		"число":      `1747216800`,
		"объект":     `{}`,
	}
	for name, input := range cases {
		t.Run(name, func(t *testing.T) {
			var got offerhttp.LocalDateTime

			err := json.Unmarshal([]byte(input), &got)

			require.Error(t, err)
			assert.Contains(t, err.Error(), "invalid date/time value: ")
			assert.Contains(t, err.Error(), "(expected local YYYY-MM-DDTHH:MM:SS without timezone)")
		})
	}
}

func TestLocalDateTime_Null_LeavesZeroValue(t *testing.T) {
	var got offerhttp.LocalDateTime

	require.NoError(t, json.Unmarshal([]byte(`null`), &got))

	assert.True(t, got.Time().IsZero())
}
