package syncairports

import (
	"context"
	"fmt"
	"io"
	"strings"

	domainrepo "api/internal/domain/repository"
)

// AirportRecord is the raw record fetched from the external source.
type AirportRecord struct {
	ICAO      string
	IATA      string
	Name      string
	City      string
	State     string
	CountryISO2 string
	Elevation int
	Lat       float64
	Lon       float64
	TZ        string
}

// AirportSource fetches the master list of airports from an external provider.
type AirportSource interface {
	Fetch(ctx context.Context) ([]AirportRecord, error)
}

// TranslationSource provides Russian-language names keyed by ICAO code.
// Missing keys mean the English name should be used as a fallback.
type TranslationSource interface {
	// FetchAirportNamesRU returns a map of ICAO → Russian airport name.
	FetchAirportNamesRU(ctx context.Context) (map[string]string, error)
	// FetchCityNamesRU returns a map of ICAO → Russian city name for that airport.
	FetchCityNamesRU(ctx context.Context) (map[string]string, error)
}

// CountryNameSource provides the full Russian country name for an ISO-2 code.
type CountryNameSource interface {
	NameRU(iso2 string) string
}

// UseCase is the port the presentation layer calls.
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler orchestrates the sync-airports use-case.
type Handler struct {
	repo        domainrepo.GeoSyncRepository
	source      AirportSource
	translations TranslationSource
	countries   CountryNameSource
}

// NewHandler wires the sync handler.
func NewHandler(
	repo domainrepo.GeoSyncRepository,
	source AirportSource,
	translations TranslationSource,
	countries CountryNameSource,
) *Handler {
	return &Handler{
		repo:         repo,
		source:       source,
		translations: translations,
		countries:    countries,
	}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	log := func(format string, args ...any) {
		if cmd.Progress != nil {
			fmt.Fprintf(cmd.Progress, format+"\n", args...)
		}
	}

	log("Fetching airports from external source...")
	records, err := h.source.Fetch(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("fetch airports: %w", err)
	}
	log("Fetched %d airport records.", len(records))

	log("Fetching Russian airport names from Wikidata...")
	airportNamesRU, err := h.translations.FetchAirportNamesRU(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("fetch airport names ru: %w", err)
	}
	log("Got %d Russian airport name translations.", len(airportNamesRU))

	log("Fetching Russian city names from Wikidata...")
	cityNamesRU, err := h.translations.FetchCityNamesRU(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("fetch city names ru: %w", err)
	}
	log("Got %d Russian city name translations.", len(cityNamesRU))

	// Collect unique countries from source data.
	countries := collectCountries(records)
	log("Syncing %d countries...", len(countries))

	if !cmd.DryRun {
		for iso2 := range countries {
			nameRU := h.countries.NameRU(iso2)
			if nameRU == "" {
				nameRU = iso2 // last-resort fallback
			}
			if err := h.repo.UpsertCountry(ctx, iso2, nameRU); err != nil {
				return Result{}, fmt.Errorf("upsert country %s: %w", iso2, err)
			}
		}
	}

	// Upsert cities and build icao→cityID map.
	type cityKey struct{ name, state, country string }
	cityIDs := make(map[cityKey]int, len(records)/4)

	log("Syncing cities...")
	syncedCities := 0

	for _, r := range records {
		if r.CountryISO2 == "" {
			continue
		}
		k := cityKey{
			name:    strings.ToLower(r.City),
			state:   strings.ToLower(r.State),
			country: strings.ToUpper(r.CountryISO2),
		}
		if _, seen := cityIDs[k]; seen {
			continue
		}

		cityName := r.City
		if ru, ok := cityNamesRU[r.ICAO]; ok && ru != "" {
			cityName = ru
		}

		var statePtr *string
		if r.State != "" {
			s := r.State
			statePtr = &s
		}

		if cmd.DryRun {
			cityIDs[k] = 0
			continue
		}

		id, err := h.repo.UpsertCity(ctx, cityName, statePtr, r.TZ, strings.ToUpper(r.CountryISO2))
		if err != nil {
			return Result{}, fmt.Errorf("upsert city %q: %w", r.City, err)
		}
		cityIDs[k] = id
		syncedCities++
	}

	if cmd.DryRun {
		syncedCities = len(cityIDs)
	}

	log("Syncing %d airports...", len(records))
	syncedAirports := 0

	for _, r := range records {
		if r.CountryISO2 == "" || r.ICAO == "" {
			continue
		}

		k := cityKey{
			name:    strings.ToLower(r.City),
			state:   strings.ToLower(r.State),
			country: strings.ToUpper(r.CountryISO2),
		}
		cityID := cityIDs[k]

		airportName := r.Name
		if ru, ok := airportNamesRU[r.ICAO]; ok && ru != "" {
			airportName = ru
		}

		var iataPtr *string
		if r.IATA != "" {
			s := r.IATA
			iataPtr = &s
		}

		var elevPtr *int
		if r.Elevation != 0 {
			e := r.Elevation
			elevPtr = &e
		}

		if cmd.DryRun {
			syncedAirports++
			continue
		}

		if err := h.repo.UpsertAirport(ctx, r.ICAO, iataPtr, airportName, r.Lat, r.Lon, elevPtr, cityID); err != nil {
			return Result{}, fmt.Errorf("upsert airport %s: %w", r.ICAO, err)
		}
		syncedAirports++
	}

	res := Result{
		Countries: len(countries),
		Cities:    syncedCities,
		Airports:  syncedAirports,
	}
	if cmd.DryRun {
		log("[dry-run] Would sync: %d countries, %d cities, %d airports.", res.Countries, res.Cities, res.Airports)
	} else {
		log("Done. Synced %d countries, %d cities, %d airports.", res.Countries, res.Cities, res.Airports)
	}
	return res, nil
}

// collectCountries returns the unique ISO-2 codes present in records.
func collectCountries(records []AirportRecord) map[string]struct{} {
	m := make(map[string]struct{}, 250)
	for _, r := range records {
		if r.CountryISO2 != "" {
			m[strings.ToUpper(r.CountryISO2)] = struct{}{}
		}
	}
	return m
}

// Compile-time check.
var _ UseCase = (*Handler)(nil)

// logWriter is a helper so nil-safe writes don't need to be repeated.
func logWriter(w io.Writer) io.Writer {
	if w == nil {
		return io.Discard
	}
	return w
}
