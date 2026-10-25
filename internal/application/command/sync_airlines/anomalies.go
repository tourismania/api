package syncairlines

import (
	"context"
	"log/slog"
	"slices"
)

// anomalyLog пишет аномалии источника в журнал и считает их по видам. Имена
// событий — в events.go.
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
	slices.Sort(codes)
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
