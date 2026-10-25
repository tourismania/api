package unit_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"testing"
	"time"

	syncairlines "api/internal/application/command/sync_airlines"
	"api/internal/domain/airline"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Двойники
// ---------------------------------------------------------------------------

type fakeAirlineSource struct {
	records   []syncairlines.AirlineRecord
	redirects map[string]string
	fetchErr  error
	// redirectCalls — запрошенные QID по вызовам FetchRedirects.
	redirectCalls [][]string
}

func (f *fakeAirlineSource) FetchAirlines(_ context.Context) ([]syncairlines.AirlineRecord, error) {
	return f.records, f.fetchErr
}

func (f *fakeAirlineSource) FetchRedirects(_ context.Context, ids []string) (map[string]string, error) {
	f.redirectCalls = append(f.redirectCalls, ids)
	out := map[string]string{}
	for _, id := range ids {
		if to, ok := f.redirects[id]; ok {
			out[id] = to
		}
	}
	return out, nil
}

// storedAirline — запись фейкового справочника.
type storedAirline struct {
	sourceID string
	airline  airline.Airline
	mergedTo int
	missing  bool
}

// fakeAirlineStore — справочник в памяти, реализует airline.Repository.
type fakeAirlineStore struct {
	rows   map[int]*storedAirline
	nextID int
	// insertErrFor — source_id, на вставке которого вернуть ошибку.
	insertErrFor string
	writes       int
}

func newFakeAirlineStore() *fakeAirlineStore {
	return &fakeAirlineStore{rows: map[int]*storedAirline{}, nextID: 1}
}

// seed добавляет запись из источника и возвращает её id.
func (s *fakeAirlineStore) seed(sourceID, icao, name string) int {
	s.nextID++
	code := icao
	s.rows[s.nextID] = &storedAirline{sourceID: sourceID, airline: airline.Airline{ID: s.nextID, ICAO: &code, Name: name}}
	return s.nextID
}

func (s *fakeAirlineStore) bySource(sourceID string) []*storedAirline {
	var out []*storedAirline
	for _, r := range s.rows {
		if r.sourceID == sourceID {
			out = append(out, r)
		}
	}
	return out
}

func (s *fakeAirlineStore) one(t *testing.T, sourceID string) *storedAirline {
	t.Helper()
	rows := s.bySource(sourceID)
	require.Len(t, rows, 1, sourceID)
	return rows[0]
}

func (s *fakeAirlineStore) Search(_ context.Context, _ airline.Filter) (airline.SearchResult, error) {
	return airline.SearchResult{}, nil
}

func (s *fakeAirlineStore) FindByIDs(_ context.Context, ids []int) ([]airline.Airline, error) {
	var out []airline.Airline
	for _, id := range ids {
		if r, ok := s.rows[id]; ok {
			out = append(out, r.airline)
		}
	}
	return out, nil
}

func (s *fakeAirlineStore) Insert(_ context.Context, sourceID string, a airline.Airline) (int, error) {
	if sourceID == s.insertErrFor {
		return 0, errors.New("db down")
	}
	s.writes++
	s.nextID++
	a.ID = s.nextID
	s.rows[s.nextID] = &storedAirline{sourceID: sourceID, airline: a}
	return s.nextID, nil
}

func (s *fakeAirlineStore) Update(_ context.Context, id int, a airline.Airline) error {
	s.writes++
	a.ID = id
	s.rows[id].airline = a
	return nil
}

func (s *fakeAirlineStore) ListSourceIDs(_ context.Context) (map[string][]airline.SourceRecord, error) {
	out := map[string][]airline.SourceRecord{}
	for id, r := range s.rows {
		if r.sourceID == "" {
			continue // как в БД: служебная запись без source_id
		}
		out[r.sourceID] = append(out[r.sourceID], airline.SourceRecord{ID: id, Merged: r.mergedTo != 0, Missing: r.missing})
	}
	return out, nil
}

func (s *fakeAirlineStore) MarkMerged(_ context.Context, fromID, toID int) error {
	s.writes++
	for id, r := range s.rows {
		if (id == fromID || r.mergedTo == fromID) && id != toID {
			r.mergedTo = toID
		}
	}
	return nil
}

func (s *fakeAirlineStore) MarkMissing(_ context.Context, ids []int, _ time.Time) error {
	if len(ids) > 0 {
		s.writes++
	}
	for _, id := range ids {
		s.rows[id].missing = true
	}
	return nil
}

func (s *fakeAirlineStore) ClearMissing(_ context.Context, ids []int) error {
	for _, id := range ids {
		s.rows[id].missing = false
	}
	return nil
}

type fakeCountryUpserts struct{ names map[string]string }

func (f *fakeCountryUpserts) Upsert(_ context.Context, iso2, name string) error {
	if f.names == nil {
		f.names = map[string]string{}
	}
	f.names[iso2] = name
	return nil
}

type staticCountryNames map[string]string

func (s staticCountryNames) NameRU(iso2 string) string { return s[iso2] }

// syncEnv — обработчик синхронизации поверх двойников и журнал в буфере.
type syncEnv struct {
	source    *fakeAirlineSource
	store     *fakeAirlineStore
	countries *fakeCountryUpserts
	log       *bytes.Buffer
	handler   *syncairlines.Handler
}

func newSyncEnv(records ...syncairlines.AirlineRecord) *syncEnv {
	e := &syncEnv{
		source:    &fakeAirlineSource{records: records},
		store:     newFakeAirlineStore(),
		countries: &fakeCountryUpserts{},
		log:       &bytes.Buffer{},
	}
	e.handler = syncairlines.NewHandler(e.store, e.countries, e.source,
		staticCountryNames{"RU": "Россия", "TR": "Турция"},
		slog.New(slog.NewJSONHandler(e.log, nil)))
	return e
}

func (e *syncEnv) run(t *testing.T, dryRun bool) (syncairlines.Result, string) {
	t.Helper()
	var progress bytes.Buffer
	res, err := e.handler.Handle(context.Background(), syncairlines.Command{DryRun: dryRun, Progress: &progress})
	require.NoError(t, err)
	return res, progress.String()
}

// events — записи журнала по порядку.
func (e *syncEnv) events(t *testing.T) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(e.log.String()), "\n") {
		if line == "" {
			continue
		}
		var m map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &m))
		out = append(out, m)
	}
	return out
}

func (e *syncEnv) eventsNamed(t *testing.T, name string) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, ev := range e.events(t) {
		if ev["event"] == name {
			out = append(out, ev)
		}
	}
	return out
}

func rec(sourceID, icao, nameEN string) syncairlines.AirlineRecord {
	return syncairlines.AirlineRecord{SourceID: sourceID, ICAO: icao, NameEN: nameEN}
}

func date(y int, m time.Month, d int) *time.Time {
	t := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	return &t
}

// ---------------------------------------------------------------------------
// Разбор источника
// ---------------------------------------------------------------------------

func TestSyncAirlines_RussianName_OnlyForRussianAirlines(t *testing.T) {
	ruWithLabel := rec("Q1", "AFL", "Aeroflot")
	ruWithLabel.NameRU, ruWithLabel.CountryISO2 = "Аэрофлот", "RU"
	ruNoLabel := rec("Q2", "RUN", "Russian No Label")
	ruNoLabel.CountryISO2 = "RU"
	foreign := rec("Q3", "THY", "Turkish Airlines")
	foreign.NameRU, foreign.CountryISO2 = "Турецкие авиалинии", "TR"
	noCountry := rec("Q4", "NOC", "No Country")
	noCountry.NameRU = "Без страны"
	twoCountries1 := rec("Q5", "TWO", "Two Countries")
	twoCountries1.NameRU, twoCountries1.CountryISO2 = "Две страны", "RU"
	twoCountries2 := twoCountries1
	twoCountries2.CountryISO2 = "KZ"

	e := newSyncEnv(ruWithLabel, ruNoLabel, foreign, noCountry, twoCountries1, twoCountries2)
	e.run(t, false)

	assert.Equal(t, "Аэрофлот", e.store.one(t, "Q1").airline.NameRU)
	assert.Equal(t, "", e.store.one(t, "Q2").airline.NameRU, "у российской без метки — пустая строка")
	assert.Equal(t, "", e.store.one(t, "Q3").airline.NameRU, "у иностранной — пустая строка")
	assert.Equal(t, "", e.store.one(t, "Q4").airline.NameRU, "без страны — пустая строка")
	two := e.store.one(t, "Q5").airline
	assert.Equal(t, "", two.NameRU, "две страны, в т. ч. Россия, — пустая строка")
	assert.Nil(t, two.Country, "две страны — страна не заполняется")
	assert.Equal(t, "RU", e.store.one(t, "Q1").airline.Country.ISO2)
	assert.Equal(t, map[string]string{"RU": "Россия", "TR": "Турция"}, e.countries.names)
}

func TestSyncAirlines_SourceIDWithSeveralRecords_ErrorBeforeAnyWrite(t *testing.T) {
	e := newSyncEnv(rec("Q1", "AFL", "Aeroflot"), rec("Q9", "NEW", "New"))
	e.store.seed("Q1", "AFL", "Aeroflot")
	e.store.seed("Q1", "AFL", "Aeroflot copy")

	_, err := e.handler.Handle(context.Background(), syncairlines.Command{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Q1")
	assert.Equal(t, 0, e.store.writes, "ни одной записи")
	assert.Empty(t, e.countries.names)
}

func TestSyncAirlines_NoEnglishName_Skipped(t *testing.T) {
	noName := rec("Q1", "AAA", "")
	noName.NameRU = "Только русское"
	e := newSyncEnv(noName, rec("Q2", "BBB", "Bravo"))

	res, _ := e.run(t, false)

	assert.Empty(t, e.store.bySource("Q1"))
	assert.Equal(t, 1, res.Skipped)
	assert.Equal(t, 1, res.Airlines)
	require.Len(t, e.eventsNamed(t, syncairlines.EventNameENMissing), 1)
	assert.Equal(t, 1, res.Anomalies[syncairlines.EventNameENMissing])
}

func TestSyncAirlines_TwoICAOs_FirstAlphabeticallyKept(t *testing.T) {
	e := newSyncEnv(rec("Q1", "XBB", "Two Codes"), rec("Q1", "XAB", "Two Codes"))

	e.run(t, false)

	assert.Equal(t, "XAB", *e.store.one(t, "Q1").airline.ICAO)
	events := e.eventsNamed(t, syncairlines.EventICAOMultiple)
	require.Len(t, events, 1)
	assert.Equal(t, []any{"XAB", "XBB"}, events[0]["codes"])
	assert.Equal(t, "XAB", events[0]["kept"])
	assert.Equal(t, "Q1", events[0]["source_id"])
}

func TestSyncAirlines_TwoIATAs_BothKept(t *testing.T) {
	a := rec("Q1", "AFL", "Aeroflot")
	a.IATA = "SU"
	b := a
	b.IATA = "S7"
	e := newSyncEnv(a, b)

	res, _ := e.run(t, false)

	assert.Equal(t, []string{"S7", "SU"}, e.store.one(t, "Q1").airline.IATACodes)
	require.Len(t, e.eventsNamed(t, syncairlines.EventIATAMultiple), 1)
	assert.Equal(t, 1, res.Anomalies[syncairlines.EventIATAMultiple])
}

func TestSyncAirlines_InvalidCodes_Dropped(t *testing.T) {
	badIATA := rec("Q1", "AFL", "Aeroflot")
	badIATA.IATA = "S-"
	lowerICAO := rec("Q1", "afl", "Aeroflot")
	onlyBad := rec("Q2", "AB", "Bad Code")
	onlyBad2 := rec("Q2", "ABCD", "Bad Code")
	e := newSyncEnv(badIATA, lowerICAO, onlyBad, onlyBad2)

	res, _ := e.run(t, false)

	got := e.store.one(t, "Q1").airline
	assert.Equal(t, "AFL", *got.ICAO)
	assert.Empty(t, got.IATACodes, "IATA неверного формата отброшен")
	assert.Empty(t, e.store.bySource("Q2"), "без корректного ICAO — пропуск")
	assert.Equal(t, 1, res.Skipped)
	events := e.eventsNamed(t, syncairlines.EventICAOInvalid)
	require.Len(t, events, 1)
	assert.Equal(t, "Q2", events[0]["source_id"])
	assert.Equal(t, []any{"AB", "ABCD"}, events[0]["codes"])
}

func TestSyncAirlines_TwoCountries_NoCountryAndAnomaly(t *testing.T) {
	a := rec("Q1", "AAA", "Alpha")
	a.CountryISO2 = "TR"
	b := a
	b.CountryISO2 = "GR"
	e := newSyncEnv(a, b)

	e.run(t, false)

	assert.Nil(t, e.store.one(t, "Q1").airline.Country)
	events := e.eventsNamed(t, syncairlines.EventCountryMultiple)
	require.Len(t, events, 1)
	assert.Equal(t, []any{"GR", "TR"}, events[0]["countries"])
}

func TestSyncAirlines_SeveralDissolvedDates_LatestKept(t *testing.T) {
	a := rec("Q1", "AAA", "Alpha")
	a.DissolvedOn = date(2010, 1, 1)
	b := a
	b.DissolvedOn = date(2015, 10, 26)
	e := newSyncEnv(a, b)

	res, _ := e.run(t, false)

	assert.True(t, date(2015, 10, 26).Equal(*e.store.one(t, "Q1").airline.DissolvedOn))
	assert.Equal(t, 1, res.Dissolved)
}

// ---------------------------------------------------------------------------
// Запись, повторный прогон, дубли и пропавшие
// ---------------------------------------------------------------------------

func TestSyncAirlines_Rerun_UpdatesKeepingIDs(t *testing.T) {
	e := newSyncEnv(rec("Q1", "AFL", "Aeroflot"))
	e.run(t, false)
	id := e.store.one(t, "Q1").airline.ID

	dissolved := rec("Q1", "AFL", "Aeroflot Renamed")
	dissolved.DissolvedOn = date(2020, 1, 1)
	e.source.records = []syncairlines.AirlineRecord{dissolved}
	e.run(t, false)

	got := e.store.one(t, "Q1")
	assert.Equal(t, id, got.airline.ID, "id не меняется, дубликата нет")
	assert.Equal(t, "Aeroflot Renamed", got.airline.Name)
	assert.NotNil(t, got.airline.DissolvedOn)
}

func TestSyncAirlines_DissolvedCodeTakenByNewAirline_TwoRecords(t *testing.T) {
	old := rec("QA", "XAA", "Old Air")
	old.DissolvedOn = date(2015, 1, 1)
	e := newSyncEnv(old)
	e.run(t, false)
	oldRow := *e.store.one(t, "QA")

	e.source.records = []syncairlines.AirlineRecord{old, rec("QB", "XAA", "New Air")}
	e.run(t, false)

	assert.Equal(t, oldRow.airline, e.store.one(t, "QA").airline, "A не меняется")
	newRow := e.store.one(t, "QB")
	assert.NotEqual(t, oldRow.airline.ID, newRow.airline.ID)
	assert.Equal(t, "XAA", *newRow.airline.ICAO)
	assert.Empty(t, e.eventsNamed(t, syncairlines.EventICAOShared), "у закрывшейся и действующей код не считается общим")
}

func TestSyncAirlines_SharedICAOAmongActive_BothWrittenOneAnomaly(t *testing.T) {
	e := newSyncEnv(rec("Q1", "XAA", "Brand Air"), rec("Q2", "XAA", "Brand Air LLC"))

	res, _ := e.run(t, false)

	assert.Len(t, e.store.bySource("Q1"), 1)
	assert.Len(t, e.store.bySource("Q2"), 1)
	events := e.eventsNamed(t, syncairlines.EventICAOShared)
	require.Len(t, events, 1)
	assert.Equal(t, []any{"Q1", "Q2"}, events[0]["source_ids"])
	assert.Equal(t, "XAA", events[0]["icao"])
	assert.Equal(t, 1, res.Anomalies[syncairlines.EventICAOShared])
}

// bigCatalog — n авиакомпаний Q100…, уже загруженных в справочник и
// пришедших в прогоне; нужен, чтобы пропажа единиц записей не превышала
// порог полноты.
func bigCatalog(e *syncEnv, n int) {
	for i := range n {
		id := fmt.Sprintf("Q%d", 100+i)
		code := fillerICAO(i)
		e.store.seed(id, code, "Filler "+id)
		e.source.records = append(e.source.records, rec(id, code, "Filler "+id))
	}
}

// fillerICAO — уникальный код ICAO из 3 букв для i-й записи.
func fillerICAO(i int) string {
	const letters = 26
	return string([]byte{'F', byte('A' + i/letters%letters), byte('A' + i%letters)})
}

func TestSyncAirlines_GoneAndRedirectedToKnown_MarkedMerged(t *testing.T) {
	e := newSyncEnv(rec("QB", "AZM", "Azimuth Airlines"))
	bigCatalog(e, 20)
	a := e.store.seed("QA", "AZM", "Azimuth")
	b := e.store.seed("QB", "AZM", "Azimuth Airlines")
	e.source.redirects = map[string]string{"QA": "QB"}

	res, _ := e.run(t, false)

	assert.Equal(t, b, e.store.rows[a].mergedTo, "A — дубль B")
	assert.False(t, e.store.rows[a].missing)
	events := e.eventsNamed(t, syncairlines.EventMerged)
	require.Len(t, events, 1)
	assert.Equal(t, "QA", events[0]["source_id"])
	assert.Equal(t, "QB", events[0]["merged_into_source_id"])
	assert.Equal(t, "AZM", events[0]["icao"])
	assert.Equal(t, "Azimuth", events[0]["name"])
	assert.Equal(t, 1, res.Anomalies[syncairlines.EventMerged])
}

func TestSyncAirlines_GoneWithoutRedirect_MarkedMissing(t *testing.T) {
	e := newSyncEnv()
	bigCatalog(e, 20)
	gone := e.store.seed("QG", "GON", "Gone Air")

	res, _ := e.run(t, false)

	assert.True(t, e.store.rows[gone].missing)
	assert.Equal(t, 0, e.store.rows[gone].mergedTo)
	require.Len(t, e.eventsNamed(t, syncairlines.EventMissing), 1)
	assert.Equal(t, 1, res.Anomalies[syncairlines.EventMissing])
}

func TestSyncAirlines_GoneRedirectedToUnknownTarget_MarkedMissingWithAnomaly(t *testing.T) {
	e := newSyncEnv()
	bigCatalog(e, 20)
	gone := e.store.seed("QG", "GON", "Gone Air")
	e.source.redirects = map[string]string{"QG": "Q999999"}

	e.run(t, false)

	assert.True(t, e.store.rows[gone].missing)
	events := e.eventsNamed(t, syncairlines.EventMergeTargetUnknown)
	require.Len(t, events, 1)
	assert.Equal(t, "QG", events[0]["source_id"])
	assert.Equal(t, "Q999999", events[0]["merged_into_source_id"])
	assert.Empty(t, e.eventsNamed(t, syncairlines.EventMissing))
}

func TestSyncAirlines_MissingAirlineBack_ClearMissing(t *testing.T) {
	e := newSyncEnv(rec("QG", "GON", "Gone Air"))
	bigCatalog(e, 20)
	gone := e.store.seed("QG", "GON", "Gone Air")
	e.store.rows[gone].missing = true

	e.run(t, false)

	assert.False(t, e.store.rows[gone].missing, "вернулась — снова видна")
	assert.Equal(t, gone, e.store.one(t, "QG").airline.ID)
}

func TestSyncAirlines_AlreadyMissing_NotLoggedAgain(t *testing.T) {
	e := newSyncEnv()
	bigCatalog(e, 20)
	gone := e.store.seed("QG", "GON", "Gone Air")
	e.store.rows[gone].missing = true

	res, _ := e.run(t, false)

	assert.True(t, e.store.rows[gone].missing)
	assert.Empty(t, e.eventsNamed(t, syncairlines.EventMissing))
	assert.Equal(t, 0, res.AnomaliesTotal())
}

func TestSyncAirlines_ElevenPercentAbsent_ErrorNothingWritten(t *testing.T) {
	e := newSyncEnv()
	// 100 известных, пришли 89 — не пришло 11%.
	for i := range 100 {
		id := fmt.Sprintf("Q%d", i)
		e.store.seed(id, fillerICAO(i), "Air "+id)
		if i >= 11 {
			e.source.records = append(e.source.records, rec(id, fillerICAO(i), "Air "+id))
		}
	}

	_, err := e.handler.Handle(context.Background(), syncairlines.Command{})

	require.ErrorIs(t, err, syncairlines.ErrSourceIncomplete)
	assert.Equal(t, 0, e.store.writes, "ни одной записи и пометки")
	for _, r := range e.store.rows {
		assert.False(t, r.missing)
	}
	assert.Empty(t, e.source.redirectCalls)
}

func TestSyncAirlines_TenPercentAbsent_Proceeds(t *testing.T) {
	e := newSyncEnv()
	for i := range 100 {
		id := fmt.Sprintf("Q%d", i)
		e.store.seed(id, fillerICAO(i), "Air "+id)
		if i >= 10 {
			e.source.records = append(e.source.records, rec(id, fillerICAO(i), "Air "+id))
		}
	}

	res, _ := e.run(t, false)

	assert.Equal(t, 10, res.Anomalies[syncairlines.EventMissing])
}

func TestSyncAirlines_DryRun_NoWritesButCountsAndLogs(t *testing.T) {
	ru := rec("Q1", "AFL", "Aeroflot")
	ru.CountryISO2 = "RU"
	old := rec("Q2", "OLD", "Old Air")
	old.DissolvedOn = date(2001, 1, 1)
	e := newSyncEnv(ru, old, rec("Q3", "AB", "Bad"), rec("Q4", "XAA", "Brand"), rec("Q5", "XAA", "Brand LLC"))

	res, out := e.run(t, true)

	assert.Equal(t, 0, e.store.writes)
	assert.Empty(t, e.store.rows)
	assert.Empty(t, e.countries.names)
	assert.Equal(t, syncairlines.Result{
		Countries: 1, Airlines: 4, Dissolved: 1, Skipped: 1,
		Anomalies: map[string]int{syncairlines.EventICAOInvalid: 1, syncairlines.EventICAOShared: 1},
	}, res)
	assert.Len(t, e.events(t), 2, "журнал ведётся и в dry-run")
	assert.NotContains(t, out, "Done.")
}

func TestSyncAirlines_UpsertError_StopsWithSourceIDAndName(t *testing.T) {
	e := newSyncEnv(rec("Q1", "AAA", "Alpha"), rec("Q2", "BBB", "Bravo Air"), rec("Q3", "CCC", "Charlie"))
	e.store.insertErrFor = "Q2"

	_, err := e.handler.Handle(context.Background(), syncairlines.Command{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "Q2")
	assert.Contains(t, err.Error(), "Bravo Air")
	assert.Len(t, e.store.bySource("Q1"), 1, "записанное до ошибки остаётся")
	assert.Empty(t, e.store.bySource("Q3"))
}

func TestSyncAirlines_UnknownRecordNeverTouched(t *testing.T) {
	e := newSyncEnv()
	// В фейке у служебной записи нет source_id — как в БД она не попадает в
	// ListSourceIDs и не может стать пропавшей.
	e.store.rows[airline.UnknownID] = &storedAirline{airline: airline.Airline{ID: airline.UnknownID, Name: "Unknown"}}
	bigCatalog(e, 3)

	e.run(t, false)

	assert.False(t, e.store.rows[airline.UnknownID].missing)
	assert.Empty(t, e.eventsNamed(t, syncairlines.EventMissing))
}

// ---------------------------------------------------------------------------
// Журнал аномалий и итог
// ---------------------------------------------------------------------------

func TestSyncAirlines_EachAnomaly_HasCommonAttributes(t *testing.T) {
	noName := rec("Q2", "BBB", "")
	twoIATA1 := rec("Q3", "CCC", "Charlie")
	twoIATA1.IATA = "C1"
	twoIATA2 := twoIATA1
	twoIATA2.IATA = "C2"
	e := newSyncEnv(rec("Q1", "AAA", "Alpha"), rec("Q1", "AAB", "Alpha"), noName, twoIATA1, twoIATA2)

	res, out := e.run(t, false)

	for _, ev := range e.events(t) {
		assert.Equal(t, "WARN", ev["level"])
		for _, attr := range []string{"event", "source_id", "icao", "name"} {
			assert.Contains(t, ev, attr, ev["event"])
		}
	}
	counts := map[string]int{}
	for _, ev := range e.events(t) {
		counts[ev["event"].(string)]++
	}
	assert.Equal(t, counts, res.Anomalies, "счётчики итога совпадают с журналом")

	events := make([]string, 0, len(res.Anomalies))
	for ev := range res.Anomalies {
		events = append(events, ev)
	}
	sort.Strings(events)
	for _, ev := range events {
		assert.Contains(t, out, fmt.Sprintf("anomaly %s=%d", ev, res.Anomalies[ev]))
	}
	assert.Contains(t, out, "Done. Synced 0 countries, 2 airlines (0 dissolved), 1 skipped, 3 anomalies.")
}

func TestSyncAirlines_CleanSource_NoWarnings(t *testing.T) {
	a := rec("Q1", "AFL", "Aeroflot")
	a.IATA, a.CountryISO2 = "SU", "RU"
	e := newSyncEnv(a, rec("Q2", "THY", "Turkish Airlines"))

	res, out := e.run(t, false)

	assert.Empty(t, e.events(t))
	assert.Equal(t, 0, res.AnomaliesTotal())
	assert.Contains(t, out, "Done. Synced 1 countries, 2 airlines (0 dissolved), 0 skipped, 0 anomalies.")
	assert.NotContains(t, out, "anomaly ")
}

func TestSyncAirlines_FetchError_Wrapped(t *testing.T) {
	e := newSyncEnv()
	e.source.fetchErr = errors.New("sparql: all 4 attempts failed")

	_, err := e.handler.Handle(context.Background(), syncairlines.Command{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "fetch airlines")
}
