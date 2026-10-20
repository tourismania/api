package syncairlines

// Имена событий журнала аномалий источника. Это стабильный контракт для
// фильтров и алертов (см. docs/cli/sync_airlines.md, «Мониторинг»):
// переименование — ломающее изменение.
const (
	// EventICAOShared — один код ICAO у нескольких действующих авиакомпаний
	// источника (вероятный дубль; в поиске видны обе записи).
	EventICAOShared = "airline_sync.icao_shared"
	// EventICAOMultiple — несколько кодов ICAO у одной авиакомпании;
	// сохраняется первый по алфавиту.
	EventICAOMultiple = "airline_sync.icao_multiple"
	// EventIATAMultiple — несколько кодов IATA у одной авиакомпании;
	// сохраняются все.
	EventIATAMultiple = "airline_sync.iata_multiple"
	// EventNameENMissing — нет английской метки; авиакомпания пропущена.
	EventNameENMissing = "airline_sync.name_en_missing"
	// EventICAOInvalid — нет ни одного корректного кода ICAO; авиакомпания
	// пропущена.
	EventICAOInvalid = "airline_sync.icao_invalid"
	// EventCountryMultiple — несколько стран; страна не заполняется.
	EventCountryMultiple = "airline_sync.country_multiple"
	// EventMerged — запись оказалась дублем: источник слил её элемент с
	// элементом другой записи справочника.
	EventMerged = "airline_sync.merged"
	// EventMissing — запись пропала из источника без слияния.
	EventMissing = "airline_sync.missing"
	// EventMergeTargetUnknown — элемент записи перенаправлен на элемент,
	// которого нет в справочнике; запись считается пропавшей.
	EventMergeTargetUnknown = "airline_sync.merge_target_unknown"
)
