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

func TestCheckChronology_ArrivalNotAfterDeparture_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	err := flight.CheckChronology(flight.Flight{Segments: []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base},
	}}, nil)
	assert.ErrorIs(t, err, flight.ErrSegmentChronologyInvalid)
}

func TestCheckChronology_ArrivalBeforeDeparture_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	err := flight.CheckChronology(flight.Flight{Segments: []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "LFPG", DepartureAt: base, ArrivalAt: base.Add(-time.Hour)},
	}}, nil)
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

func TestCheckChronology_ZeroLayover_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	err := flight.CheckChronology(flight.Flight{Segments: []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		// Departs exactly when the previous segment arrives: zero layover.
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(2 * time.Hour), ArrivalAt: base.Add(4 * time.Hour)},
	}}, nil)
	assert.ErrorIs(t, err, flight.ErrLayoverNonPositive)
}

func TestCheckChronology_NegativeLayover_ReturnsErr(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	err := flight.CheckChronology(flight.Flight{Segments: []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUEE", ArrivalAirportICAO: "UUDD", DepartureAt: base, ArrivalAt: base.Add(2 * time.Hour)},
		// Departs before the previous segment even arrives.
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "UUDD", ArrivalAirportICAO: "LFPG", DepartureAt: base.Add(time.Hour), ArrivalAt: base.Add(4 * time.Hour)},
	}}, nil)
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

func TestNewFlight_FlightNumber_KeptAsIs(t *testing.T) {
	for _, number := range []string{"", "SU1234", "su 1234", " U6-773 ", "ЮТ-773", "AFL 1234A"} {
		t.Run(number, func(t *testing.T) {
			segs := segmentsWithClasses(flight.CabinClassUnknown, flight.CabinClassUnknown)
			segs[0].FlightNumber = number

			f, err := flight.New(samplePrice(), "", segs)

			require.NoError(t, err)
			assert.Equal(t, number, f.Segments[0].FlightNumber, "number must not be validated or normalized")
			assert.Equal(t, "", f.Segments[1].FlightNumber)
		})
	}
}

// Хронология — не дело New: для неё нужны таймзоны аэропортов.
func TestNewFlight_ClockOrderNotChecked(t *testing.T) {
	base := time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)
	_, err := flight.New(samplePrice(), "", []flight.Segment{
		{AirlineID: sampleAirlineID, DepartureAirportICAO: "USSS", ArrivalAirportICAO: "UUEE", DepartureAt: base, ArrivalAt: base.Add(-time.Hour)},
	})
	assert.NoError(t, err)
}

func TestCheckChronology_ByRealMoments(t *testing.T) {
	cases := map[string]struct {
		segments []flight.Segment
		want     error
	}{
		"Екатеринбург 10:00 → Москва 09:00 принят": {
			[]flight.Segment{localSegment(t, "USSS", "2026-05-14T10:00:00", "UUEE", "2026-05-14T09:00:00")}, nil,
		},
		"Москва 10:00 → Екатеринбург 11:00 отклонён": {
			[]flight.Segment{localSegment(t, "UUEE", "2026-05-14T10:00:00", "USSS", "2026-05-14T11:00:00")}, flight.ErrSegmentChronologyInvalid,
		},
		"Москва 10:00 → Екатеринбург 12:00 — ноль в полёте": {
			[]flight.Segment{localSegment(t, "UUEE", "2026-05-14T10:00:00", "USSS", "2026-05-14T12:00:00")}, flight.ErrSegmentChronologyInvalid,
		},
		"пересадка в одном аэропорту положительная": {
			[]flight.Segment{
				localSegment(t, "USSS", "2026-05-14T10:00:00", "UUEE", "2026-05-14T11:00:00"),
				localSegment(t, "UUEE", "2026-05-14T12:00:00", "LTFM", "2026-05-14T15:00:00"),
			}, nil,
		},
		"пересадка нулевая": {
			[]flight.Segment{
				localSegment(t, "USSS", "2026-05-14T10:00:00", "UUEE", "2026-05-14T11:00:00"),
				localSegment(t, "UUEE", "2026-05-14T11:00:00", "LTFM", "2026-05-14T15:00:00"),
			}, flight.ErrLayoverNonPositive,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := flight.CheckChronology(flight.Flight{Segments: tc.segments}, testZones())

			if tc.want == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tc.want)
		})
	}
}
