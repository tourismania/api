package unit_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"api/internal/presentation/http/httpx"

	"github.com/go-playground/validator/v10"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseIDList_ValidDedupedInOrder(t *testing.T) {
	got, err := httpx.ParseIDList("42,57,42, 7")
	require.NoError(t, err)
	assert.Equal(t, []int{42, 57, 7}, got)
}

func TestParseIDList_Invalid_Error(t *testing.T) {
	for _, raw := range []string{"0", "-1", "abc", "42,,57", "", "1.5"} {
		_, err := httpx.ParseIDList(raw)
		assert.ErrorIs(t, err, httpx.ErrIDListInvalid, "%q", raw)
	}
}

func TestQueryInt_InvalidOrEmpty_Default(t *testing.T) {
	for raw, want := range map[string]int{"": 20, "abc": 20, "-5": 20, "0": 0, "35": 35} {
		assert.Equal(t, want, httpx.QueryInt(raw, 20), "%q", raw)
	}
}

func TestWriteValidationErrorWithParams_IncludesBound(t *testing.T) {
	type params struct {
		Limit  int   `validate:"min=1,max=100"`
		Offset int   `validate:"min=0,max=10000"`
		IDs    []int `validate:"max=100"`
	}
	v := validator.New(validator.WithRequiredStructEnabled())
	for name, tc := range map[string]struct {
		p    params
		want map[string]any
	}{
		"limit":  {params{Limit: 101}, map[string]any{"Limit": "max=100"}},
		"offset": {params{Limit: 20, Offset: 10_001}, map[string]any{"Offset": "max=10000"}},
		"ids":    {params{Limit: 20, IDs: make([]int, 101)}, map[string]any{"IDs": "max=100"}},
	} {
		t.Run(name, func(t *testing.T) {
			rr := httptest.NewRecorder()

			httpx.WriteValidationErrorWithParams(rr, v.Struct(tc.p))

			require.Equal(t, http.StatusBadRequest, rr.Code)
			var body httpx.ErrorBody
			require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
			assert.Equal(t, "validation failed", body.Error)
			assert.Equal(t, tc.want, body.Meta)
		})
	}
}
