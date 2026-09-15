package unit_test

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"api/internal/presentation/http/httpx"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeTarget mirrors the shape of a real request DTO closely enough to
// exercise every branch of httpx.WriteDecodeError: a string field (for the
// type-mismatch case) and a time.Time field (for the layout-mismatch case,
// the one that motivated this whole file — a bare "invalid JSON body" gave
// no clue that departure_at/arrival_at need RFC3339).
type decodeTarget struct {
	Status      string    `json:"status"`
	DepartureAt time.Time `json:"departure_at"`
}

func decodeBody(t *testing.T, body string) error {
	t.Helper()
	req := httptest.NewRequest("POST", "/whatever", strings.NewReader(body))
	var v decodeTarget
	return httpx.DecodeJSON(req, &v, nil)
}

func TestWriteDecodeError_SyntaxError_ReportsByteOffset(t *testing.T) {
	err := decodeBody(t, `{"status":"draft",}`)
	require.Error(t, err)

	rr := httptest.NewRecorder()
	httpx.WriteDecodeError(rr, err)

	assert.Equal(t, 400, rr.Code)
	var body httpx.ErrorBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Contains(t, body.Error, "byte offset")
	assert.NotEqual(t, "invalid JSON body", body.Error, "syntax errors must not collapse to the generic message")
}

func TestWriteDecodeError_TypeMismatch_NamesTheField(t *testing.T) {
	err := decodeBody(t, `{"status":123}`)
	require.Error(t, err)

	rr := httptest.NewRecorder()
	httpx.WriteDecodeError(rr, err)

	assert.Equal(t, 400, rr.Code)
	var body httpx.ErrorBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Contains(t, body.Error, "status")
	assert.Equal(t, "status", body.Meta["field"])
}

func TestWriteDecodeError_BadTimeLayout_HintsRFC3339(t *testing.T) {
	// Ровно сценарий из бага: "2026-08-30 12:00:00" вместо RFC3339.
	err := decodeBody(t, `{"status":"draft","departure_at":"2026-08-30 12:00:00"}`)
	require.Error(t, err)

	rr := httptest.NewRecorder()
	httpx.WriteDecodeError(rr, err)

	assert.Equal(t, 400, rr.Code)
	var body httpx.ErrorBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Contains(t, body.Error, "RFC3339")
	assert.Contains(t, body.Error, "2026-08-30 12:00:00", "исходное невалидное значение должно остаться в ответе")
}

func TestWriteDecodeError_UnknownField_SurfacesRawDecoderMessage(t *testing.T) {
	err := decodeBody(t, `{"status":"draft","totally_unknown_field":1}`)
	require.Error(t, err)

	rr := httptest.NewRecorder()
	httpx.WriteDecodeError(rr, err)

	assert.Equal(t, 400, rr.Code)
	var body httpx.ErrorBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Contains(t, body.Error, "totally_unknown_field")
}

func TestWriteDecodeError_EmptyBody_ReturnsGenericMessage(t *testing.T) {
	err := decodeBody(t, ``)
	require.Error(t, err)

	rr := httptest.NewRecorder()
	httpx.WriteDecodeError(rr, err)

	assert.Equal(t, 400, rr.Code)
	var body httpx.ErrorBody
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "invalid JSON body", body.Error)
}
