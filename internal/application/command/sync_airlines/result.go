package syncairlines

// Result — итог синхронизации (в dry-run — что было бы записано).
type Result struct {
	// Countries — число стран авиакомпаний.
	Countries int
	// Airlines — число записанных (вставленных или обновлённых) авиакомпаний.
	Airlines int
	// Dissolved — сколько из них прекратили деятельность.
	Dissolved int
	// Skipped — авиакомпании источника, пропущенные при разборе (нет
	// английского названия или корректного кода ICAO).
	Skipped int
	// Anomalies — число аномалий источника по имени события (см. events.go).
	Anomalies map[string]int
}

// AnomaliesTotal — общее число аномалий.
func (r Result) AnomaliesTotal() int {
	total := 0
	for _, n := range r.Anomalies {
		total += n
	}
	return total
}
