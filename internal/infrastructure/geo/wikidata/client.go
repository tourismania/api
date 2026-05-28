// Package wikidata queries the Wikidata SPARQL endpoint for Russian-language
// geographic names keyed by ICAO airport code.
package wikidata

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	syncairports "api/internal/application/command/sync_airports"
)

const (
	sparqlEndpoint = "https://query.wikidata.org/sparql"
	pageSize       = 10_000
)

// sparqlResult mirrors the Wikidata JSON response for SPARQL queries.
type sparqlResult struct {
	Results struct {
		Bindings []map[string]struct {
			Value string `json:"value"`
		} `json:"bindings"`
	} `json:"results"`
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

		if len(bindings) < pageSize {
			break
		}
	}

	return out, nil
}

// sparql executes a single SPARQL query and returns the raw bindings.
func (c *Client) sparql(ctx context.Context, query string) ([]map[string]struct{ Value string `json:"value"` }, error) {
	params := url.Values{"query": {query}, "format": {"json"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sparqlEndpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("build sparql request: %w", err)
	}
	req.Header.Set("User-Agent", "Tourismania/1.0 geo-sync (https://github.com/tourismania)")
	req.Header.Set("Accept", "application/sparql-results+json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sparql http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
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
