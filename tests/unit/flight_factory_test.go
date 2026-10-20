package unit_test

import (
	"api/internal/domain/money"
	"api/internal/domain/offer/flight"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewFlight_EmptySegments_ReturnsErr(t *testing.T) {
	_, err := flight.New(samplePrice(), "", nil)
	assert.ErrorIs(t, err, flight.ErrSegmentsEmpty)
}

func TestNewFlight_ArrivalNotAfterDeparture_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	_, err := flight.New(samplePrice(), "", []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base},
	})
	assert.ErrorIs(t, err, flight.ErrSegmentChronologyInvalid)
}

func TestNewFlight_ArrivalBeforeDeparture_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	_, err := flight.New(samplePrice(), "", []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(-time.Hour)},
	})
	assert.ErrorIs(t, err, flight.ErrSegmentChronologyInvalid)
}

func TestNewFlight_DiscontinuousRoute_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	_, err := flight.New(samplePrice(), "", []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		// Departs from EDDF, not UUDD (the previous segment's arrival) — the route breaks.
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "EDDF", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(4 * time.Hour), ArrivalAt: base.Add(6 * time.Hour)},
	})
	assert.ErrorIs(t, err, flight.ErrSegmentDiscontinuous)
}

func TestNewFlight_ZeroLayover_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	_, err := flight.New(samplePrice(), "", []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		// Departs exactly when the previous segment arrives: zero layover.
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(2 * time.Hour), ArrivalAt: base.Add(4 * time.Hour)},
	})
	assert.ErrorIs(t, err, flight.ErrLayoverNonPositive)
}

func TestNewFlight_NegativeLayover_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	_, err := flight.New(samplePrice(), "", []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		// Departs before the previous segment even arrives.
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(time.Hour), ArrivalAt: base.Add(4 * time.Hour)},
	})
	assert.ErrorIs(t, err, flight.ErrLayoverNonPositive)
}

func TestNewFlight_ValidNonstop_ReturnsFlight(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	f, err := flight.New(samplePrice(), "", []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour)},
	})
	require.NoError(t, err)
	assert.Len(t, f.Segments, 1)
}

func TestNewFlight_ValidWithLayovers_ReturnsFlight(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	f, err := flight.New(samplePrice(), "", []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "EDDF", DepartureAt: base.Add(3 * time.Hour), ArrivalAt: base.Add(5 * time.Hour)},
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "EDDF", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(6 * time.Hour), ArrivalAt: base.Add(7 * time.Hour)},
	})
	require.NoError(t, err)
	assert.Len(t, f.Segments, 3)
}

func TestNewFlight_ZeroPrice_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	_, err := flight.New(money.Money{}, "", []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(time.Hour)},
	})
	assert.ErrorIs(t, err, money.ErrCurrencyUnsupported)
}

func TestNewFlight_NegativePrice_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	price := samplePrice()
	price.AmountMinor = -1
	_, err := flight.New(price, "", []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(time.Hour)},
	})
	assert.ErrorIs(t, err, money.ErrAmountInvalid)
}

func TestNewFlight_Valid_KeepsPrice(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	price := mustMoney("12345.5", money.CodeEUR)
	f, err := flight.New(price, "", []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(time.Hour)},
	})
	require.NoError(t, err)
	assert.Equal(t, price, f.Price)
}

// nonstopSegments — один корректный сегмент для тестов комментария.
func nonstopSegments() []flight.Segment {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	return []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(time.Hour)},
	}
}

func TestNewFlight_ManagerComment_KeptAsIs(t *testing.T) {
	cases := map[string]string{
		"пустой":                  "",
		"пробелы по краям":        "  Багаж оплачивается отдельно  ",
		"только пробелы":          "   ",
		"табуляция и переводы":    "Вылет:\tночью\nПересадка:\r\nтранзит",
		"кавычки и обратный слэш": `Rock'n'roll \ "VIP"'; DROP TABLE offers; --`,
		"разметка":                "<b>A & B</b> <script>",
		"эмодзи и юникод":         "Отличный рейс ✈️🧳 — ок",
		"символ замены":           "�",
		"C1 после DEL":            "\u0080",
	}
	for name, comment := range cases {
		t.Run(name, func(t *testing.T) {
			f, err := flight.New(samplePrice(), comment, nonstopSegments())

			require.NoError(t, err)
			assert.Equal(t, comment, f.ManagerComment)
		})
	}
}

func TestNewFlight_ManagerCommentControlChar_ReturnsErr(t *testing.T) {
	cases := map[string]string{
		"NUL":                  "до\u0000после",
		"ESC":                  "\u001b[31mкрасный",
		"DEL":                  "текст\u007f",
		"первый C0":            "\u0001",
		"последний C0":         "\u001f",
		"вертикальная таб":     "\v",
		"рядом с разрешёнными": "\t\n\r\u000c",
	}
	for name, comment := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := flight.New(samplePrice(), comment, nonstopSegments())

			assert.ErrorIs(t, err, flight.ErrManagerCommentInvalid)
		})
	}
}

// segmentsWithClasses — два непрерывных сегмента UUEE→LFPG→KJFK с
// заданными классами обслуживания.
func segmentsWithClasses(first, second flight.CabinClass) []flight.Segment {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	return []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(4 * time.Hour), CabinClass: first},
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "LFPG", ArrivalAirportICAO: "KJFK", DepartureAt: base.Add(6 * time.Hour), ArrivalAt: base.Add(14 * time.Hour), CabinClass: second},
	}
}

func TestNewFlight_CabinClassValid_KeptAsIs(t *testing.T) {
	for _, class := range []flight.CabinClass{"", "economy", "comfort", "comfort_plus", "business"} {
		t.Run(string(class), func(t *testing.T) {
			f, err := flight.New(samplePrice(), "", segmentsWithClasses(class, flight.CabinClassUnknown))

			require.NoError(t, err)
			assert.Equal(t, class, f.Segments[0].CabinClass)
			assert.Equal(t, flight.CabinClassUnknown, f.Segments[1].CabinClass)
		})
	}
}

func TestNewFlight_DifferentCabinClassesPerSegment_KeptInOrder(t *testing.T) {
	f, err := flight.New(samplePrice(), "", segmentsWithClasses(flight.CabinClassEconomy, flight.CabinClassComfort))

	require.NoError(t, err)
	assert.Equal(t, flight.CabinClassEconomy, f.Segments[0].CabinClass)
	assert.Equal(t, flight.CabinClassComfort, f.Segments[1].CabinClass)
}

func TestNewFlight_CabinClassInvalid_ReturnsErr(t *testing.T) {
	for _, class := range []flight.CabinClass{"first", "premium_economy", "comfort+", "Business", " economy", "economy "} {
		t.Run(string(class), func(t *testing.T) {
			// Недопустимый класс во втором сегменте: проверяется каждый сегмент.
			_, err := flight.New(samplePrice(), "", segmentsWithClasses(flight.CabinClassEconomy, class))

			assert.ErrorIs(t, err, flight.ErrCabinClassInvalid)
		})
	}
}

func TestCabinClasses_ReturnsFourKnownClasses(t *testing.T) {
	assert.Equal(t, []flight.CabinClass{
		flight.CabinClassEconomy,
		flight.CabinClassComfort,
		flight.CabinClassComfortPlus,
		flight.CabinClassBusiness,
	}, flight.CabinClasses())
}

func TestCabinClasses_ReturnsCopy(t *testing.T) {
	classes := flight.CabinClasses()
	classes[0] = "first"

	assert.Equal(t, flight.CabinClassEconomy, flight.CabinClasses()[0])
	assert.False(t, flight.CabinClass("first").IsValid())
}

func TestCabinClass_IsValid_KnownAndUnknown(t *testing.T) {
	for _, class := range flight.CabinClasses() {
		assert.True(t, class.IsValid(), class)
	}
	assert.True(t, flight.CabinClassUnknown.IsValid())
	assert.False(t, flight.CabinClass("ECONOMY").IsValid())
}
