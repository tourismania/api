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
	"strings"
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

	// wikidataEntityPrefix is stripped from result URIs to get a short QID.
	wikidataEntityPrefix = "http://www.wikidata.org/entity/"

	// cityBatchSize caps how many city names go into a single VALUES clause.
	// Chosen empirically against the public WDQS endpoint: batches around
	// this size resolve in under ~1s, while a reverse ISO2→country lookup or
	// a settlement-type filter in the same query causes timeouts at far
	// smaller batch sizes. Binding the country by its Wikidata QID directly
	// (instead of a reverse P297 lookup) is what keeps this fast.
	cityBatchSize = 100
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

// FetchCityNamesRU returns the Russian name for each given city, matched by
// its English source name within its country. Cities whose country has no
// known Wikidata QID, or that have no matching Russian label, are omitted —
// callers should fall back to the English name in that case.
//
// Matching is done by exact English label rather than by tracing any
// specific airport's "located in" (P131) relation: many airports sit
// physically outside city limits, so P131 from an airport can resolve to a
// district, region, or unrelated place instead of the city itself. Matches
// are additionally required to have a geographic coordinate (P625), which
// cheaply filters out same-named non-places (e.g. a ship or submarine
// sharing a city's name) without the cost of a settlement-type check.
func (c *Client) FetchCityNamesRU(ctx context.Context, cities []syncairports.CityRef) (map[syncairports.CityRef]string, error) {
	out := make(map[syncairports.CityRef]string, len(cities))
	if len(cities) == 0 {
		return out, nil
	}

	countryQIDs, err := c.fetchCountryQIDs(ctx)
	if err != nil {
		return nil, fmt.Errorf("fetch country qids: %w", err)
	}

	for iso2, names := range groupByCountry(cities) {
		qid, ok := countryQIDs[iso2]
		if !ok {
			continue // unknown country — cities fall back to their English name.
		}

		for _, batch := range chunkStrings(names, cityBatchSize) {
			bindings, err := c.sparql(ctx, buildCityQuery(qid, batch))
			if err != nil {
				return nil, fmt.Errorf("fetch city names ru (country %s): %w", iso2, err)
			}
			for _, b := range bindings {
				nameEn, nameOK := b["nameEn"]
				cityRu, ruOK := b["cityRu"]
				if !nameOK || !ruOK || nameEn.Value == "" || cityRu.Value == "" {
					continue
				}
				out[syncairports.CityRef{Name: nameEn.Value, CountryISO2: iso2}] = cityRu.Value
			}
		}
	}

	return out, nil
}

// fetchCountryQIDs returns ISO 3166-1 alpha-2 → short Wikidata QID (e.g.
// "Q159") for every country Wikidata knows an ISO-2 code for. This is a
// single fast bulk query, reused across all FetchCityNamesRU calls in a run.
func (c *Client) fetchCountryQIDs(ctx context.Context) (map[string]string, error) {
	const query = `SELECT ?country ?iso2 WHERE { ?country wdt:P297 ?iso2 . }`

	bindings, err := c.sparql(ctx, query)
	if err != nil {
		return nil, err
	}

	out := make(map[string]string, len(bindings))
	for _, b := range bindings {
		country, countryOK := b["country"]
		iso2, iso2OK := b["iso2"]
		if !countryOK || !iso2OK || iso2.Value == "" {
			continue
		}
		out[strings.ToUpper(iso2.Value)] = strings.TrimPrefix(country.Value, wikidataEntityPrefix)
	}
	return out, nil
}

// buildCityQuery builds a SPARQL query matching the given English city names
// against Wikidata entities located in the country identified by qid (a
// short QID such as "Q159"). Binding the country by QID directly — instead
// of a reverse ISO2 lookup — is what keeps this fast at batch scale.
func buildCityQuery(qid string, names []string) string {
	values := make([]string, len(names))
	for i, n := range names {
		values[i] = `"` + escapeSPARQLString(n) + `"@en`
	}

	return fmt.Sprintf(`
SELECT DISTINCT ?nameEn ?cityRu WHERE {
  VALUES ?nameEn { %s }
  ?city rdfs:label ?nameEn .
  ?city wdt:P17 wd:%s .
  ?city wdt:P625 ?coord .
  ?city rdfs:label ?cityRu .
  FILTER(LANG(?cityRu) = "ru")
}`, strings.Join(values, " "), qid)
}

// escapeSPARQLString escapes a string for safe embedding in a SPARQL string
// literal (backslashes and double quotes).
func escapeSPARQLString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}

// groupByCountry buckets city names by ISO-2 country code.
func groupByCountry(cities []syncairports.CityRef) map[string][]string {
	out := make(map[string][]string)
	for _, c := range cities {
		out[c.CountryISO2] = append(out[c.CountryISO2], c.Name)
	}
	return out
}

// chunkStrings splits names into batches of at most size elements.
func chunkStrings(names []string, size int) [][]string {
	var chunks [][]string
	for i := 0; i < len(names); i += size {
		end := i + size
		if end > len(names) {
			end = len(names)
		}
		chunks = append(chunks, names[i:end])
	}
	return chunks
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
