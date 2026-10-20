package syncairlines

import (
	"context"
	"log/slog"
	"regexp"
	"slices"
	"sort"
	"time"

	"api/internal/domain/airline"
)

// countryRU — код России: русское название авиакомпании сохраняется только у
// российских, иностранные клиенту показываются по-английски.
const countryRU = "RU"

var (
	// icaoPattern — код ICAO авиакомпании: 3 заглавные латинские буквы.
	icaoPattern = regexp.MustCompile(`^[A-Z]{3}$`)
	// iataPattern — код IATA авиакомпании: 2 заглавные латинские буквы или цифры.
	iataPattern = regexp.MustCompile(`^[A-Z0-9]{2}$`)
	// iso2Pattern — код страны ISO 3166-1 alpha-2.
	iso2Pattern = regexp.MustCompile(`^[A-Z]{2}$`)
)

// AirlineRecord — строка источника: одно сочетание значений элемента
// (элемент с двумя кодами IATA даёт две строки). Пустая строка — значения
// нет; DissolvedOn nil — даты нет.
type AirlineRecord struct {
	SourceID    string
	ICAO        string
	IATA        string
	NameEN      string
	NameRU      string
	CountryISO2 string
	DissolvedOn *time.Time
}

// parsedAirline — авиакомпания источника после разбора, готовая к записи.
type parsedAirline struct {
	SourceID string
	Airline  airline.Airline
}

// ICAO — сохранённый код ICAO.
func (p parsedAirline) ICAO() string {
	return *p.Airline.ICAO
}

// anomalyLog пишет аномалии источника в журнал и считает их по видам.
type anomalyLog struct {
	ctx    context.Context
	logger *slog.Logger
	counts map[string]int
}

func newAnomalyLog(ctx context.Context, logger *slog.Logger) *anomalyLog {
	return &anomalyLog{ctx: ctx, logger: logger, counts: make(map[string]int)}
}

// warn записывает предупреждение event с общими атрибутами записи и
// специфичными attrs.
func (l *anomalyLog) warn(event, sourceID, icao, name string, attrs ...any) {
	l.counts[event]++
	args := append([]any{"event", event, "source_id", sourceID, "icao", icao, "name", name}, attrs...)
	l.logger.WarnContext(l.ctx, "airline source anomaly", args...)
}

// sourceGroup — все строки одного элемента источника.
type sourceGroup struct {
	icao, iata, nameEN, nameRU, countries map[string]struct{}
	invalidICAO                           []string
	dissolvedOn                           *time.Time
}

// parseRecords группирует строки источника по SourceID и разбирает каждую
// авиакомпанию по правилам требования «однозначно разбирать данные
// источника». Возвращает авиакомпании в порядке SourceID и число
// пропущенных.
func parseRecords(records []AirlineRecord, log *anomalyLog) ([]parsedAirline, int) {
	groups := make(map[string]*sourceGroup)
	for _, r := range records {
		g, ok := groups[r.SourceID]
		if !ok {
			g = &sourceGroup{
				icao: map[string]struct{}{}, iata: map[string]struct{}{},
				nameEN: map[string]struct{}{}, nameRU: map[string]struct{}{},
				countries: map[string]struct{}{},
			}
			groups[r.SourceID] = g
		}
		switch {
		case icaoPattern.MatchString(r.ICAO):
			g.icao[r.ICAO] = struct{}{}
		case r.ICAO != "":
			g.invalidICAO = append(g.invalidICAO, r.ICAO)
		}
		if iataPattern.MatchString(r.IATA) {
			g.iata[r.IATA] = struct{}{}
		}
		addNonEmpty(g.nameEN, r.NameEN)
		addNonEmpty(g.nameRU, r.NameRU)
		if iso2Pattern.MatchString(r.CountryISO2) {
			g.countries[r.CountryISO2] = struct{}{}
		}
		if r.DissolvedOn != nil && (g.dissolvedOn == nil || r.DissolvedOn.After(*g.dissolvedOn)) {
			d := *r.DissolvedOn
			g.dissolvedOn = &d
		}
	}

	sourceIDs := make([]string, 0, len(groups))
	for id := range groups {
		sourceIDs = append(sourceIDs, id)
	}
	sort.Strings(sourceIDs)

	parsed := make([]parsedAirline, 0, len(sourceIDs))
	skipped := 0
	for _, id := range sourceIDs {
		p, ok := parseGroup(id, groups[id], log)
		if !ok {
			skipped++
			continue
		}
		parsed = append(parsed, p)
	}
	return parsed, skipped
}

// parseGroup разбирает один элемент источника; false — элемент пропущен.
func parseGroup(sourceID string, g *sourceGroup, log *anomalyLog) (parsedAirline, bool) {
	icaos := sortedKeys(g.icao)
	nameEN := first(sortedKeys(g.nameEN))
	if len(icaos) == 0 {
		slices.Sort(g.invalidICAO)
		log.warn(EventICAOInvalid, sourceID, "", nameEN, "codes", slices.Compact(g.invalidICAO))
		return parsedAirline{}, false
	}
	icao := icaos[0]
	if nameEN == "" {
		log.warn(EventNameENMissing, sourceID, icao, "")
		return parsedAirline{}, false
	}
	if len(icaos) > 1 {
		log.warn(EventICAOMultiple, sourceID, icao, nameEN, "codes", icaos, "kept", icao)
	}
	iatas := sortedKeys(g.iata)
	if len(iatas) > 1 {
		log.warn(EventIATAMultiple, sourceID, icao, nameEN, "codes", iatas)
	}

	var country *airline.Country
	countries := sortedKeys(g.countries)
	switch len(countries) {
	case 1:
		country = &airline.Country{ISO2: countries[0]}
	case 0:
	default:
		log.warn(EventCountryMultiple, sourceID, icao, nameEN, "countries", countries)
	}

	nameRU := ""
	if country != nil && country.ISO2 == countryRU {
		nameRU = first(sortedKeys(g.nameRU))
	}

	return parsedAirline{
		SourceID: sourceID,
		Airline: airline.Airline{
			ICAO:        &icao,
			Name:        nameEN,
			NameRU:      nameRU,
			Country:     country,
			DissolvedOn: g.dissolvedOn,
			IATACodes:   iatas,
		},
	}, true
}

// logSharedICAO журналирует коды ICAO, общие для нескольких действующих
// авиакомпаний (только журнал: обе записываются и обе видны в поиске).
func logSharedICAO(parsed []parsedAirline, log *anomalyLog) {
	byICAO := make(map[string][]parsedAirline)
	for _, p := range parsed {
		if p.Airline.DissolvedOn == nil {
			byICAO[p.ICAO()] = append(byICAO[p.ICAO()], p)
		}
	}
	codes := make([]string, 0, len(byICAO))
	for code, group := range byICAO {
		if len(group) > 1 {
			codes = append(codes, code)
		}
	}
	sort.Strings(codes)
	for _, code := range codes {
		group := byICAO[code]
		sourceIDs := make([]string, 0, len(group))
		names := make([]string, 0, len(group))
		for _, p := range group {
			sourceIDs = append(sourceIDs, p.SourceID)
			names = append(names, p.Airline.Name)
		}
		log.warn(EventICAOShared, group[0].SourceID, code, group[0].Airline.Name,
			"source_ids", sourceIDs, "names", names)
	}
}

func addNonEmpty(set map[string]struct{}, v string) {
	if v != "" {
		set[v] = struct{}{}
	}
}

func sortedKeys(set map[string]struct{}) []string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
