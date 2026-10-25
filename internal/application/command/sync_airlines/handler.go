package syncairlines

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"time"

	"api/internal/domain/airline"
	"api/internal/domain/airport"
)

// maxMissingPercent — какая доля записей справочника из источника может не
// прийти в одном прогоне. За месяц в источнике пропадают единицы записей;
// больше — признак усечённого ответа источника, а не настоящих изменений.
const maxMissingPercent = 10

// percent — делитель для maxMissingPercent.
const percent = 100

// pendingID — id записи, которая в dry-run была бы вставлена.
const pendingID = -1

// ErrSourceIncomplete — источник вернул заметно меньше авиакомпаний, чем уже
// загружено в справочник: прогон останавливается до записи, ничего не скрывая.
var ErrSourceIncomplete = errors.New("airline source response looks incomplete")

// AirlineSource — источник авиакомпаний (Wikidata).
type AirlineSource interface {
	// FetchAirlines возвращает все авиакомпании с кодом ICAO — по строке на
	// сочетание значений элемента.
	FetchAirlines(ctx context.Context) ([]AirlineRecord, error)
	// FetchRedirects возвращает для перечисленных идентификаторов источника
	// те, что стали перенаправлениями: старый идентификатор → новый.
	FetchRedirects(ctx context.Context, sourceIDs []string) (map[string]string, error)
}

// CountryNameSource даёт русское название страны по коду ISO-2.
type CountryNameSource interface {
	NameRU(iso2 string) string
}

// UseCase — порт, который вызывает presentation-слой (CLI).
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler выполняет синхронизацию справочника авиакомпаний.
type Handler struct {
	airlines     airline.Repository
	countries    airport.CountryRepository
	source       AirlineSource
	countryNames CountryNameSource
	logger       *slog.Logger
}

// NewHandler собирает обработчик; logger получает журнал аномалий источника.
func NewHandler(
	airlines airline.Repository,
	countries airport.CountryRepository,
	source AirlineSource,
	countryNames CountryNameSource,
	logger *slog.Logger,
) *Handler {
	return &Handler{
		airlines:     airlines,
		countries:    countries,
		source:       source,
		countryNames: countryNames,
		logger:       logger,
	}
}

// Handle загружает авиакомпании из источника, разбирает их, проверяет
// полноту ответа и записывает страны и авиакомпании («вставить или обновить»
// по идентификатору в источнике, id записей сохраняются). Записи, не
// пришедшие из источника, помечаются дублями (если источник перенаправляет
// их элемент на элемент другой записи) или пропавшими. Аномалии источника
// журналируются и считаются, но код выхода не меняют.
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	progress := func(format string, args ...any) {
		if cmd.Progress != nil {
			// Прогресс — необязательный вывод для оператора: ошибка записи в
			// него не должна прерывать синхронизацию справочника.
			_, _ = fmt.Fprintf(cmd.Progress, format+"\n", args...)
		}
	}
	anomalies := newAnomalyLog(ctx, h.logger)

	progress("Fetching airlines from Wikidata...")
	records, err := h.source.FetchAirlines(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("fetch airlines: %w", err)
	}
	parsed, skipped := parseRecords(records, anomalies)
	logSharedICAO(parsed, anomalies)
	progress("Fetched %d records: %d airlines, %d skipped.", len(records), len(parsed), skipped)

	known, err := h.airlines.ListSourceIDs(ctx)
	if err != nil {
		return Result{}, fmt.Errorf("list known airlines: %w", err)
	}
	if err := checkUniqueSourceIDs(known); err != nil {
		return Result{}, err
	}
	arrived := make(map[string]struct{}, len(parsed))
	for _, p := range parsed {
		arrived[p.SourceID] = struct{}{}
	}
	if err := checkCompleteness(known, arrived); err != nil {
		return Result{}, err
	}

	countries := collectCountries(parsed)
	progress("Syncing %d countries...", len(countries))
	if !cmd.DryRun {
		if err := h.upsertCountries(ctx, countries); err != nil {
			return Result{}, err
		}
	}

	progress("Syncing %d airlines...", len(parsed))
	idsBySource, err := h.upsertAirlines(ctx, parsed, known, cmd.DryRun)
	if err != nil {
		return Result{}, err
	}

	if err := h.markGone(ctx, known, arrived, idsBySource, anomalies, cmd.DryRun); err != nil {
		return Result{}, err
	}

	res := Result{
		Countries: len(countries),
		Airlines:  len(parsed),
		Dissolved: countDissolved(parsed),
		Skipped:   skipped,
		Anomalies: anomalies.counts,
	}
	events := make([]string, 0, len(res.Anomalies))
	for event := range res.Anomalies {
		events = append(events, event)
	}
	slices.Sort(events)
	for _, event := range events {
		progress("anomaly %s=%d", event, res.Anomalies[event])
	}
	if !cmd.DryRun {
		progress("Done. Synced %d countries, %d airlines (%d dissolved), %d skipped, %d anomalies.",
			res.Countries, res.Airlines, res.Dissolved, res.Skipped, res.AnomaliesTotal())
	}
	return res, nil
}

// checkUniqueSourceIDs останавливает прогон, если у идентификатора источника
// несколько записей: какую обновлять, решает человек.
func checkUniqueSourceIDs(known map[string][]airline.SourceRecord) error {
	var dups []string
	for sourceID, recs := range known {
		if len(recs) > 1 {
			dups = append(dups, sourceID)
		}
	}
	if len(dups) == 0 {
		return nil
	}
	slices.Sort(dups)
	return fmt.Errorf("source ids with several airline records, resolve manually before sync: %v", dups)
}

// checkCompleteness сравнивает видимые в поиске записи справочника из
// источника с пришедшими: не пришло больше maxMissingPercent — ответ
// источника считается неполным. Уже скрытые записи (дубли, пропавшие) не
// учитываются: они и не должны приходить.
func checkCompleteness(known map[string][]airline.SourceRecord, arrived map[string]struct{}) error {
	visible, absent := 0, 0
	for sourceID, recs := range known {
		for _, rec := range recs {
			if rec.Merged || rec.Missing {
				continue
			}
			visible++
			if _, ok := arrived[sourceID]; !ok {
				absent++
			}
		}
	}
	if absent*percent > visible*maxMissingPercent {
		return fmt.Errorf("%w: %d of %d known airlines did not arrive (more than %d%%), nothing written; rerun the sync",
			ErrSourceIncomplete, absent, visible, maxMissingPercent)
	}
	return nil
}

// collectCountries — коды стран авиакомпаний по алфавиту.
func collectCountries(parsed []parsedAirline) []string {
	var codes []string
	for _, p := range parsed {
		if p.Airline.Country != nil {
			codes = append(codes, p.Airline.Country.ISO2)
		}
	}
	slices.Sort(codes)
	return slices.Compact(codes)
}

// upsertCountries записывает страны с русскими названиями (неизвестный код —
// сам код).
func (h *Handler) upsertCountries(ctx context.Context, countries []string) error {
	for _, iso2 := range countries {
		name := h.countryNames.NameRU(iso2)
		if name == "" {
			name = iso2
		}
		if err := h.countries.Upsert(ctx, iso2, name); err != nil {
			return fmt.Errorf("upsert country %s: %w", iso2, err)
		}
	}
	return nil
}

// countDissolved — сколько авиакомпаний источника прекратили деятельность.
func countDissolved(parsed []parsedAirline) int {
	n := 0
	for _, p := range parsed {
		if p.Airline.DissolvedOn != nil {
			n++
		}
	}
	return n
}

// upsertAirlines обновляет известные записи по id и вставляет новые, снимает
// пометку «пропала» с вернувшихся. Возвращает id не слитых записей по
// идентификатору источника (с новыми; в dry-run у новых pendingID).
func (h *Handler) upsertAirlines(
	ctx context.Context,
	parsed []parsedAirline,
	known map[string][]airline.SourceRecord,
	dryRun bool,
) (map[string]int, error) {
	idsBySource := make(map[string]int, len(known)+len(parsed))
	for sourceID, recs := range known {
		if !recs[0].Merged {
			idsBySource[sourceID] = recs[0].ID
		}
	}

	var returned []int
	for _, p := range parsed {
		recs, ok := known[p.SourceID]
		if ok && recs[0].Missing {
			returned = append(returned, recs[0].ID)
		}
		if dryRun {
			if !ok {
				idsBySource[p.SourceID] = pendingID
			}
			continue
		}

		var err error
		if ok {
			err = h.airlines.Update(ctx, recs[0].ID, p.Airline)
		} else {
			var id int
			id, err = h.airlines.Insert(ctx, p.SourceID, p.Airline)
			idsBySource[p.SourceID] = id
		}
		if err != nil {
			return nil, fmt.Errorf("upsert airline %s (%s): %w", p.SourceID, p.Airline.Name, err)
		}
	}

	if !dryRun {
		if err := h.airlines.ClearMissing(ctx, returned); err != nil {
			return nil, fmt.Errorf("clear missing airlines: %w", err)
		}
	}
	return idsBySource, nil
}

// markGone разбирает записи, не пришедшие из источника (кроме уже слитых):
// элемент перенаправлен на элемент записи справочника — запись становится
// её дублем; иначе — пропавшей с сегодняшнего дня. Уже пропавшие повторно
// не журналируются, но могут стать дублями, если источник их слил позже.
func (h *Handler) markGone(
	ctx context.Context,
	known map[string][]airline.SourceRecord,
	arrived map[string]struct{},
	idsBySource map[string]int,
	anomalies *anomalyLog,
	dryRun bool,
) error {
	var gone []string
	for sourceID, recs := range known {
		if _, ok := arrived[sourceID]; !ok && !recs[0].Merged {
			gone = append(gone, sourceID)
		}
	}
	if len(gone) == 0 {
		return nil
	}
	slices.Sort(gone)

	redirects, err := h.source.FetchRedirects(ctx, gone)
	if err != nil {
		return fmt.Errorf("fetch redirects: %w", err)
	}
	details, err := h.describe(ctx, known, gone)
	if err != nil {
		return err
	}

	var missing []int
	for _, sourceID := range gone {
		rec := known[sourceID][0]
		d := details[rec.ID]
		target, redirected := redirects[sourceID]
		targetID, targetKnown := idsBySource[target]

		switch {
		case redirected && targetKnown && targetID != rec.ID:
			anomalies.warn(EventMerged, sourceID, d.icao, d.name, "merged_into_source_id", target)
			if dryRun {
				continue
			}
			if err := h.airlines.MarkMerged(ctx, rec.ID, targetID); err != nil {
				return fmt.Errorf("mark airline %s merged into %s: %w", sourceID, target, err)
			}
		case rec.Missing:
			// Уже скрыта как пропавшая — пометка и журнал не повторяются.
		case redirected:
			anomalies.warn(EventMergeTargetUnknown, sourceID, d.icao, d.name, "merged_into_source_id", target)
			missing = append(missing, rec.ID)
		default:
			anomalies.warn(EventMissing, sourceID, d.icao, d.name)
			missing = append(missing, rec.ID)
		}
	}

	if dryRun {
		return nil
	}
	today := time.Now().UTC().Truncate(24 * time.Hour)
	if err := h.airlines.MarkMissing(ctx, missing, today); err != nil {
		return fmt.Errorf("mark airlines missing: %w", err)
	}
	return nil
}

// recordDetails — код и название записи для журнала.
type recordDetails struct {
	icao, name string
}

// describe читает коды и названия не пришедших записей: в ответе источника их
// уже нет, а журналу они нужны.
func (h *Handler) describe(ctx context.Context, known map[string][]airline.SourceRecord, sourceIDs []string) (map[int]recordDetails, error) {
	ids := make([]int, 0, len(sourceIDs))
	for _, sourceID := range sourceIDs {
		ids = append(ids, known[sourceID][0].ID)
	}
	found, err := h.airlines.FindByIDs(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("find gone airlines: %w", err)
	}
	out := make(map[int]recordDetails, len(found))
	for _, a := range found {
		d := recordDetails{name: a.Name}
		if a.ICAO != nil {
			d.icao = *a.ICAO
		}
		out[a.ID] = d
	}
	return out, nil
}

// Compile-time check.
var _ UseCase = (*Handler)(nil)
