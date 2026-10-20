package wikidata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	syncairlines "api/internal/application/command/sync_airlines"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestClient — клиент поверх тестового сервера с короткими паузами
// между повторами.
func newTestClient(srv *httptest.Server) *Client {
	return &Client{http: srv.Client(), endpoint: srv.URL, baseDelay: time.Millisecond}
}

// sparqlJSON — тело ответа SPARQL из строк «переменная → значение».
func sparqlJSON(t *testing.T, rows ...map[string]string) []byte {
	t.Helper()
	bindings := make([]map[string]map[string]string, 0, len(rows))
	for _, r := range rows {
		b := map[string]map[string]string{}
		for k, v := range r {
			b[k] = map[string]string{"value": v}
		}
		bindings = append(bindings, b)
	}
	body, err := json.Marshal(map[string]any{"results": map[string]any{"bindings": bindings}})
	require.NoError(t, err)
	return body
}

func TestFetchAirlines_ParsesRecordsAndDatePrecision(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Contains(t, r.URL.Query().Get("query"), "wdt:P230")
		_, _ = w.Write(sparqlJSON(t,
			map[string]string{
				"item": wikidataEntityPrefix + "Q1", "icao": "AFL", "iata": "SU",
				"nameEn": "Aeroflot", "nameRu": "Аэрофлот", "iso2": "ru",
			},
			map[string]string{
				"item": wikidataEntityPrefix + "Q2", "icao": "TSO", "nameEn": "Transaero",
				"dissolved": "2015-10-26T00:00:00Z", "precision": "11",
			},
			map[string]string{
				"item": wikidataEntityPrefix + "Q3", "icao": "OLD", "nameEn": "Old Air",
				"dissolved": "1998-06-15T00:00:00Z", "precision": "9",
			},
			map[string]string{
				"item": wikidataEntityPrefix + "Q4", "icao": "MON", "nameEn": "Month Air",
				"dissolved": "2001-07-20T00:00:00Z", "precision": "10",
			},
		))
	}))
	defer srv.Close()

	got, err := newTestClient(srv).FetchAirlines(context.Background())

	require.NoError(t, err)
	require.Len(t, got, 4)
	assert.Equal(t, syncairlines.AirlineRecord{
		SourceID: "Q1", ICAO: "AFL", IATA: "SU", NameEN: "Aeroflot", NameRU: "Аэрофлот", CountryISO2: "RU",
	}, got[0])
	require.NotNil(t, got[1].DissolvedOn)
	assert.Equal(t, "2015-10-26", got[1].DissolvedOn.Format("2006-01-02"), "точность до дня")
	assert.Equal(t, "1998-01-01", got[2].DissolvedOn.Format("2006-01-02"), "точность до года — 1 января")
	assert.Equal(t, "2001-07-01", got[3].DissolvedOn.Format("2006-01-02"), "точность до месяца — 1-е число")
}

func TestFetchAirlines_FullPage_RequestsNextPage(t *testing.T) {
	var offsets []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		offsets = append(offsets, strings.TrimSpace(q[strings.LastIndex(q, "OFFSET"):]))
		if len(offsets) == 1 {
			rows := make([]map[string]string, pageSize)
			for i := range rows {
				rows[i] = map[string]string{"item": wikidataEntityPrefix + "Q1", "icao": "AAA", "nameEn": "A"}
			}
			_, _ = w.Write(sparqlJSON(t, rows...))
			return
		}
		_, _ = w.Write(sparqlJSON(t))
	}))
	defer srv.Close()

	got, err := newTestClient(srv).FetchAirlines(context.Background())

	require.NoError(t, err)
	assert.Len(t, got, pageSize)
	assert.Equal(t, []string{"OFFSET 0", fmt.Sprintf("OFFSET %d", pageSize)}, offsets)
}

func TestFetchAirlines_503ThenOK_Retries(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(sparqlJSON(t, map[string]string{"item": wikidataEntityPrefix + "Q1", "icao": "AFL", "nameEn": "Aeroflot"}))
	}))
	defer srv.Close()

	got, err := newTestClient(srv).FetchAirlines(context.Background())

	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Equal(t, 2, calls)
}

func TestFetchAirlines_AlwaysFailing_ErrorAfterAllAttempts(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).FetchAirlines(context.Background())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "all 4 attempts failed")
	assert.Equal(t, maxRetries, calls)
}

func TestFetchAirlines_ClientError_NotRetried(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).FetchAirlines(context.Background())

	require.Error(t, err)
	assert.Equal(t, 1, calls)
}

func TestFetchRedirects_ParsesAndBatches(t *testing.T) {
	var batches []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		batches = append(batches, q)
		var rows []map[string]string
		if strings.Contains(q, "wd:Q1 ") {
			rows = append(rows, map[string]string{"old": wikidataEntityPrefix + "Q1", "new": wikidataEntityPrefix + "Q500"})
		}
		_, _ = w.Write(sparqlJSON(t, rows...))
	}))
	defer srv.Close()

	qids := []string{"Q1", "bad id", `Q2"}`}
	for i := range redirectBatchSize {
		qids = append(qids, fmt.Sprintf("Q%d", 1000+i))
	}

	got, err := newTestClient(srv).FetchRedirects(context.Background(), qids)

	require.NoError(t, err)
	assert.Equal(t, map[string]string{"Q1": "Q500"}, got)
	require.Len(t, batches, 2, "201 корректный QID — две пачки по 200")
	assert.Contains(t, batches[0], "owl:sameAs")
	for _, b := range batches {
		assert.NotContains(t, b, "bad id", "некорректный QID в запрос не попадает")
		assert.NotContains(t, b, `Q2"`)
	}
}

func TestFetchRedirects_Empty_NoRequest(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		t.Error("запроса быть не должно")
	}))
	defer srv.Close()

	got, err := newTestClient(srv).FetchRedirects(context.Background(), nil)

	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestParseWikidataDate_Invalid_Nil(t *testing.T) {
	for _, v := range []string{"", "-0500-01-01T00:00:00Z", "abcd-ef-ghT00:00:00Z"} {
		assert.Nil(t, parseWikidataDate(v, "11"), v)
	}
}
