package syncairlines

import (
	"regexp"
	"slices"
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
	// anyValue — любое непустое значение (названия).
	anyValue = regexp.MustCompile(`.`)
)

// AirlineRecord — строка источника: одно сочетание значений элемента. SPARQL
// отдаёт декартово произведение необязательных свойств, поэтому элемент с
// двумя кодами IATA и двумя странами приходит четырьмя строками. Пустая
// строка — значения нет; DissolvedOn nil — даты нет.
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

// parseRecords разбирает ответ источника в два шага. Сначала строки
// собираются по элементу (groupBySource): по одной строке нельзя решить, какой
// код ICAO сохранить, одна ли у авиакомпании страна и какая дата закрытия
// последняя, — для этого нужны все строки элемента. Затем каждый элемент
// разбирается целиком (parseAirline). Возвращает авиакомпании в порядке
// SourceID (детерминированный порядок записи и журнала) и число пропущенных.
func parseRecords(records []AirlineRecord, log *anomalyLog) ([]parsedAirline, int) {
	groups := groupBySource(records)

	sourceIDs := make([]string, 0, len(groups))
	for id := range groups {
		sourceIDs = append(sourceIDs, id)
	}
	slices.Sort(sourceIDs)

	parsed := make([]parsedAirline, 0, len(sourceIDs))
	skipped := 0
	for _, id := range sourceIDs {
		p, ok := parseAirline(id, groups[id], log)
		if !ok {
			skipped++
			continue
		}
		parsed = append(parsed, p)
	}
	return parsed, skipped
}

// groupBySource собирает строки источника по идентификатору элемента.
func groupBySource(records []AirlineRecord) map[string][]AirlineRecord {
	groups := make(map[string][]AirlineRecord)
	for _, r := range records {
		groups[r.SourceID] = append(groups[r.SourceID], r)
	}
	return groups
}

// parseAirline разбирает все строки одного элемента по правилам требования
// «однозначно разбирать данные источника»; false — элемент пропущен.
func parseAirline(sourceID string, rows []AirlineRecord, log *anomalyLog) (parsedAirline, bool) {
	icaos, invalidICAOs := distinct(rows, func(r AirlineRecord) string { return r.ICAO }, icaoPattern)
	names, _ := distinct(rows, func(r AirlineRecord) string { return r.NameEN }, anyValue)
	nameEN := first(names)

	if len(icaos) == 0 {
		log.warn(EventICAOInvalid, sourceID, "", nameEN, "codes", invalidICAOs)
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

	iatas, _ := distinct(rows, func(r AirlineRecord) string { return r.IATA }, iataPattern)
	if len(iatas) > 1 {
		log.warn(EventIATAMultiple, sourceID, icao, nameEN, "codes", iatas)
	}

	var country *airline.Country
	countries, _ := distinct(rows, func(r AirlineRecord) string { return r.CountryISO2 }, iso2Pattern)
	switch len(countries) {
	case 0:
	case 1:
		country = &airline.Country{ISO2: countries[0]}
	default:
		log.warn(EventCountryMultiple, sourceID, icao, nameEN, "countries", countries)
	}

	nameRU := ""
	if country != nil && country.ISO2 == countryRU {
		namesRU, _ := distinct(rows, func(r AirlineRecord) string { return r.NameRU }, anyValue)
		nameRU = first(namesRU)
	}

	return parsedAirline{
		SourceID: sourceID,
		Airline: airline.Airline{
			ICAO:        &icao,
			Name:        nameEN,
			NameRU:      nameRU,
			Country:     country,
			DissolvedOn: latestDate(rows),
			IATACodes:   iatas,
		},
	}, true
}

// distinct возвращает различные непустые значения поля строк по алфавиту:
// подходящие под pattern и отброшенные.
func distinct(rows []AirlineRecord, field func(AirlineRecord) string, pattern *regexp.Regexp) (valid, invalid []string) {
	for _, r := range rows {
		v := field(r)
		switch {
		case v == "":
		case pattern.MatchString(v):
			valid = append(valid, v)
		default:
			invalid = append(invalid, v)
		}
	}
	slices.Sort(valid)
	slices.Sort(invalid)
	return slices.Compact(valid), slices.Compact(invalid)
}

// latestDate — самая поздняя дата прекращения деятельности среди строк.
func latestDate(rows []AirlineRecord) *time.Time {
	var latest *time.Time
	for _, r := range rows {
		if r.DissolvedOn != nil && (latest == nil || r.DissolvedOn.After(*latest)) {
			d := *r.DissolvedOn
			latest = &d
		}
	}
	return latest
}

func first(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
