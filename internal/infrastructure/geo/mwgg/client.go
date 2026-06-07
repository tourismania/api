// Package mwgg fetches airport data from the mwgg/Airports GitHub dataset.
package mwgg

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	syncairports "api/internal/application/command/sync_airports"
)

const (
	dataURL = "https://raw.githubusercontent.com/mwgg/Airports/master/airports.json"

	// maxRetries is the total number of attempts (1 initial + 3 retries).
	maxRetries = 4
	// baseDelay is the wait before the first retry; subsequent waits double.
	baseDelay = 3 * time.Second
)

// rawRecord mirrors one entry in the mwgg airports.json.
type rawRecord struct {
	ICAO      string  `json:"icao"`
	IATA      string  `json:"iata"`
	Name      string  `json:"name"`
	City      string  `json:"city"`
	State     string  `json:"state"`
	Country   string  `json:"country"`
	Elevation int     `json:"elevation"`
	Lat       float64 `json:"lat"`
	Lon       float64 `json:"lon"`
	TZ        string  `json:"tz"`
}

// transientErr marks a failure that warrants a retry (5xx, 429, network).
type transientErr struct{ cause error }

func (e *transientErr) Error() string { return e.cause.Error() }
func (e *transientErr) Unwrap() error { return e.cause }

func isTransient(err error) bool {
	var t *transientErr
	return errors.As(err, &t)
}

// Client fetches airport data from the mwgg GitHub dataset.
type Client struct {
	http *http.Client
}

// New returns a Client with a sensible HTTP timeout.
func New() *Client {
	return &Client{
		http: &http.Client{Timeout: 60 * time.Second},
	}
}

// Fetch downloads and parses the full airports.json with exponential-backoff
// retry on transient failures (5xx, 429, network errors). Backoff: 3s → 6s → 12s.
func (c *Client) Fetch(ctx context.Context) ([]syncairports.AirportRecord, error) {
	var lastErr error
	for attempt := range maxRetries {
		if attempt > 0 {
			delay := baseDelay * time.Duration(1<<(attempt-1))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(delay):
			}
		}

		records, err := c.doFetch(ctx)
		if err == nil {
			return records, nil
		}
		lastErr = err

		if !isTransient(err) {
			return nil, err
		}
	}

	return nil, fmt.Errorf("mwgg fetch: all %d attempts failed: %w", maxRetries, lastErr)
}

// doFetch performs a single HTTP round-trip. Transient failures (5xx, 429,
// network errors) are wrapped in *transientErr so Fetch can retry them.
func (c *Client) doFetch(ctx context.Context) ([]syncairports.AirportRecord, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, dataURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("User-Agent", "Tourismania/1.0 airport-sync")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, &transientErr{cause: fmt.Errorf("http get: %w", err)}
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusOK:
		// success — fall through to decode
	case resp.StatusCode == http.StatusTooManyRequests,
		resp.StatusCode >= http.StatusInternalServerError:
		return nil, &transientErr{cause: fmt.Errorf("unexpected status %d", resp.StatusCode)}
	default:
		return nil, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}

	// The JSON is a map of ICAO → record.
	var raw map[string]rawRecord
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode json: %w", err)
	}

	out := make([]syncairports.AirportRecord, 0, len(raw))
	for _, r := range raw {
		out = append(out, syncairports.AirportRecord{
			ICAO:        r.ICAO,
			IATA:        r.IATA,
			Name:        r.Name,
			City:        r.City,
			State:       r.State,
			CountryISO2: r.Country,
			Elevation:   r.Elevation,
			Lat:         r.Lat,
			Lon:         r.Lon,
			TZ:          r.TZ,
		})
	}
	return out, nil
}

// Compile-time check.
var _ syncairports.AirportSource = (*Client)(nil)
