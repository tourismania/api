package wikidata

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	syncairlines "api/internal/application/command/sync_airlines"
)

const (
	// redirectBatchSize — сколько QID идёт в один VALUES запроса
	// перенаправлений: сотни элементов разрешаются на WDQS быстро.
	redirectBatchSize = 200

	// Точность даты Wikidata (wikibase:timePrecision): 9 — год, 10 — месяц,
	// 11 — день. Менее точные даты (десятилетие, век) приводятся к году.
	precisionYear  = 9
	precisionMonth = 10

	// dateLen — длина префикса YYYY-MM-DD в значении даты.
	dateLen = len("2006-01-02")
)

// qidPattern — короткий идентификатор элемента Wikidata. Значения из БД
// подставляются в SPARQL только после этой проверки.
var qidPattern = regexp.MustCompile(`^Q[0-9]+$`)

// airlinesQuery — все элементы с кодом ICAO авиакомпании (P230): коды IATA
// (P229), английская и русская метки, страна (P17 → ISO-2 через P297) и
// дата прекращения деятельности (P576) лучшего ранга с её точностью. По
// строке на сочетание значений; сортировка по всем переменным делает
// постраничную выдачу LIMIT/OFFSET устойчивой. Фильтра «экземпляр класса
// авиакомпания» нет: P230 есть практически только у авиакомпаний, а обход
// иерархии классов на публичном WDQS приводит к таймаутам.
const airlinesQuery = `
SELECT ?item ?icao ?iata ?nameEn ?nameRu ?iso2 ?dissolved ?precision WHERE {
  ?item wdt:P230 ?icao .
  OPTIONAL { ?item wdt:P229 ?iata . }
  OPTIONAL { ?item rdfs:label ?nameEn . FILTER(LANG(?nameEn) = "en") }
  OPTIONAL { ?item rdfs:label ?nameRu . FILTER(LANG(?nameRu) = "ru") }
  OPTIONAL { ?item wdt:P17 ?country . ?country wdt:P297 ?iso2 . }
  OPTIONAL {
    ?item p:P576 ?dissolvedStatement .
    ?dissolvedStatement a wikibase:BestRank ;
                        psv:P576 ?dissolvedValue .
    ?dissolvedValue wikibase:timeValue ?dissolved ;
                    wikibase:timePrecision ?precision .
  }
}
ORDER BY ?item ?icao ?iata ?nameEn ?nameRu ?iso2 ?dissolved
LIMIT %d
OFFSET %d`

// FetchAirlines загружает авиакомпании постранично (повторы при временных
// сбоях — в sparql). Дата прекращения деятельности с точностью до года — 1
// января, до месяца — 1-е число.
func (c *Client) FetchAirlines(ctx context.Context) ([]syncairlines.AirlineRecord, error) {
	var out []syncairlines.AirlineRecord
	for offset := 0; ; offset += pageSize {
		bindings, err := c.sparql(ctx, fmt.Sprintf(airlinesQuery, pageSize, offset))
		if err != nil {
			return nil, fmt.Errorf("fetch airlines (offset %d): %w", offset, err)
		}
		for _, b := range bindings {
			out = append(out, toAirlineRecord(b))
		}
		if len(bindings) < pageSize {
			return out, nil
		}
	}
}

// toAirlineRecord разбирает одну строку ответа. Проверку форматов кодов
// делает синхронизация — здесь значения только извлекаются.
func toAirlineRecord(b sparqlBinding) syncairlines.AirlineRecord {
	return syncairlines.AirlineRecord{
		SourceID:    strings.TrimPrefix(b["item"].Value, wikidataEntityPrefix),
		ICAO:        b["icao"].Value,
		IATA:        b["iata"].Value,
		NameEN:      b["nameEn"].Value,
		NameRU:      b["nameRu"].Value,
		CountryISO2: strings.ToUpper(b["iso2"].Value),
		DissolvedOn: parseWikidataDate(b["dissolved"].Value, b["precision"].Value),
	}
}

// parseWikidataDate превращает значение даты Wikidata в дату с учётом
// точности. Пустое, до нашей эры или неразборчивое значение — nil.
func parseWikidataDate(value, precision string) *time.Time {
	if len(value) < dateLen || strings.HasPrefix(value, "-") {
		return nil
	}
	parts := strings.Split(value[:dateLen], "-")
	if len(parts) != 3 {
		return nil
	}
	year, errY := strconv.Atoi(parts[0])
	month, errM := strconv.Atoi(parts[1])
	day, errD := strconv.Atoi(parts[2])
	if errY != nil || errM != nil || errD != nil {
		return nil
	}

	p, err := strconv.Atoi(precision)
	if err != nil {
		p = precisionYear
	}
	if p <= precisionYear {
		month, day = 1, 1
	} else if p == precisionMonth {
		day = 1
	}
	// Wikidata хранит неизвестные месяц и день как 00.
	month, day = max(month, 1), max(day, 1)

	d := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	return &d
}

// FetchRedirects возвращает для перечисленных QID те, что стали
// перенаправлениями (при слиянии элементов Wikidata оставляет старый QID
// перенаправлением, owl:sameAs): старый QID → новый. Запросы — пачками по
// redirectBatchSize; некорректные QID пропускаются.
func (c *Client) FetchRedirects(ctx context.Context, qids []string) (map[string]string, error) {
	valid := make([]string, 0, len(qids))
	for _, q := range qids {
		if qidPattern.MatchString(q) {
			valid = append(valid, q)
		}
	}

	out := make(map[string]string)
	for _, batch := range chunkStrings(valid, redirectBatchSize) {
		bindings, err := c.sparql(ctx, buildRedirectsQuery(batch))
		if err != nil {
			return nil, fmt.Errorf("fetch redirects: %w", err)
		}
		for _, b := range bindings {
			from := strings.TrimPrefix(b["old"].Value, wikidataEntityPrefix)
			to := strings.TrimPrefix(b["new"].Value, wikidataEntityPrefix)
			if from != "" && to != "" {
				out[from] = to
			}
		}
	}
	return out, nil
}

// buildRedirectsQuery — запрос перенаправлений для пачки корректных QID.
func buildRedirectsQuery(qids []string) string {
	values := make([]string, len(qids))
	for i, q := range qids {
		values[i] = "wd:" + q
	}
	return fmt.Sprintf(`
SELECT ?old ?new WHERE {
  VALUES ?old { %s }
  ?old owl:sameAs ?new .
}`, strings.Join(values, " "))
}

// Compile-time check.
var _ syncairlines.AirlineSource = (*Client)(nil)
