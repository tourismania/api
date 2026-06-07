// Package wikidata queries the Wikidata SPARQL endpoint for Russian-language
// geographic names keyed by ICAO airport code.
package wikidata

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	syncairports "api/internal/application/command/sync_airports"
)

const (
	sparqlEndpoint = "https://query.wikidata.org/sparql"
	pageSize       = 10_000

	// maxRetries is the total number of attempts (1 initial + 3 retries).
	maxRetries = 4
	// baseDelay is the wait before the first retry; subsequent waits double.
	baseDelay = 3 * time.Second
)

// sparqlResult mirrors the Wikidata JSON response for SPARQL queries.
type sparqlResult struct {
	Results struct {
		Bindings []map[string]struct {
			Value string `json:"value"`
		} `json:"bindings"`
	} `json:"results"`
}

// transientErr marks a failure that warrants a retry (5xx, 429, network).
type transientErr struct{ cause error }

func (e *transientErr) Error() string { return e.cause.Error() }
func (e *transientErr) Unwrap() error { return e.cause }

func isTransient(err error) bool {
	var t *transientErr
	return errors.As(err, &t)
}

// Client queries Wikidata for Russian airport and city names.
type Client struct {
	http *http.Client
}

// New returns a Wikidata Client.
func New() *Client {
	return &Client{
		http: &http.Client{Timeout: 120 * time.Second},
	}
}

// FetchAirportNamesRU returns ICAO → Russian airport name.
// Falls back gracefully: airports without a Russian label are omitted.
func (c *Client) FetchAirportNamesRU(ctx context.Context) (map[string]string, error) {
	const tmpl = `
SELECT ?icao ?nameRu WHERE {
  ?airport wdt:P239 ?icao .
  ?airport rdfs:label ?nameRu .
  FILTER(LANG(?nameRu) = "ru")
}
ORDER BY ?icao
LIMIT %d
OFFSET %d`

	return c.fetchPaged(ctx, tmpl, "icao", "nameRu")
}

// FetchCityNamesRU returns ICAO → Russian name of the city the airport is in.
// Uses the Wikidata "located in" (P131) relation.
func (c *Client) FetchCityNamesRU(ctx context.Context) (map[string]string, error) {
	const tmpl = `
SELECT ?icao ?cityRu WHERE {
  ?airport wdt:P239 ?icao .
  ?airport wdt:P131 ?city .
  ?city rdfs:label ?cityRu .
  FILTER(LANG(?cityRu) = "ru")
}
ORDER BY ?icao
LIMIT %d
OFFSET %d`

	return c.fetchPaged(ctx, tmpl, "icao", "cityRu")
}

// fetchPaged pages through SPARQL results collecting keyVar → valVar pairs.
func (c *Client) fetchPaged(ctx context.Context, queryTmpl, keyVar, valVar string) (map[string]string, error) {
	out := make(map[string]string, 15_000)

	for offset := 0; ; offset += pageSize {
		query := fmt.Sprintf(queryTmpl, pageSize, offset)
		bindings, err := c.sparql(ctx, query)
		if err != nil {
			return nil, err
		}

		for _, b := range bindings {
			k, kOK := b[keyVar]
			v, vOK := b[valVar]
			if kOK && vOK && k.Value != "" && v.Value != "" {
				out[k.Value] = v.Value
			}
		}

		if len(bindings) == 0 {
			break
		}
	}

	return out, nil
}

// sparql executes a SPARQL query with exponential-backoff retry on transient
// errors (5xx, 429, network failures). Backoff: 3s → 6s → 12s.
func (c *Client) sparql(ctx context.Context, query string) ([]map[string]struct {
	Value string `json:"value"`
}, error) {
	params := url.Values{"query": {query}, "format": {"json"}}
	reqURL := sparqlEndpoint + "?" + params.Encode()

	var lastErr error
	for attempt := range maxRetries {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<(attempt-1)) // 3s, 6s, 12s
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		bindings, err := c.doSparql(ctx, reqURL)
		if err == nil {
			return bindings, nil
		}
		lastErr = err

		if !isTransient(err) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("sparql: all %d attempts failed: %w", maxRetries, lastErr)
}

// doSparql performs a single HTTP round-trip. Transient failures (5xx, 429,
// network errors) are wrapped in *transientErr so sparql can retry them.
func (c *Client) doSparql(ctx context.Context, reqURL string) ([]map[string]struct {
	Value string `json:"value"`
}, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build sparql request: %w", err)
	}
	req.Header.Set("User-Agent", "Tourismania/1.0 geo-sync (https://github.com/tourismania)")
	req.Header.Set("Accept", "application/sparql-results+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &transientErr{cause: fmt.Errorf("sparql http: %w", err)}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		// success — fall through to decode
	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode >= http.StatusInternalServerError:
		return nil, &transientErr{cause: fmt.Errorf("sparql status %d", resp.StatusCode)}
	default:
		return nil, fmt.Errorf("sparql status %d", resp.StatusCode)
	}

	var result sparqlResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("sparql decode: %w", err)
	}

	return result.Results.Bindings, nil
}

// Compile-time check.
var _ syncairports.TranslationSource = (*Client)(nil)
