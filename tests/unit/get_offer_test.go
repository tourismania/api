package unit_test

import (
	"context"
	"testing"
	"time"

	"api/internal/application/apperror"
	getoffer "api/internal/application/query/get_offer"
	"api/internal/domain/offer"
	"api/internal/domain/offer/flight"
	"api/internal/domain/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubUserFinder is a hand-written test double for the domain
// user.Repository port, shared by tests that resolve the
// acting principal from its uuid via user.NewFinder. Store is a
// no-op: these tests only ever exercise the read path.
type stubUserFinder struct {
	record *user.Record
	err    error
}

func (s stubUserFinder) FindByUuid(_ context.Context, _ uuid.UUID) (*user.Record, error) {
	return s.record, s.err
}

func (s stubUserFinder) Store(_ context.Context, _ user.User, _ string) (*int, error) {
	return nil, nil
}

func userRecordWithAgency(agencyID int) *user.Finder {
	return user.NewFinder(stubUserFinder{record: &user.Record{ID: 1, AgencyID: agencyID}})
}

func noUserFound() *user.Finder {
	return user.NewFinder(stubUserFinder{record: nil})
}

func TestGetOffer_MatchingAgency_SeesDraftOffer(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: o}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, stubFlightFinder{}, &mockAirlineRepo{}, &mockAirportRepo{}, userRecordWithAgency(7))

	res, err := h.Handle(context.Background(), getoffer.Query{
		UUID: o.UUID,
	})

	require.NoError(t, err)
	assert.Equal(t, o.UUID, res.UUID)
}

func TestGetOffer_MatchingAgency_SeesPublishedOffer(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusPublished}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: o}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, stubFlightFinder{}, &mockAirlineRepo{}, &mockAirportRepo{}, userRecordWithAgency(7))

	res, err := h.Handle(context.Background(), getoffer.Query{
		UUID: o.UUID,
	})

	require.NoError(t, err)
	assert.Equal(t, o.UUID, res.UUID)
}

func TestGetOffer_DifferentAgency_DraftOffer_NotFound(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: o}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, stubFlightFinder{}, &mockAirlineRepo{}, &mockAirportRepo{}, userRecordWithAgency(1))

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID: o.UUID,
	})

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestGetOffer_DifferentAgency_PublishedOffer_StillNotFound(t *testing.T) {
	// 1 user = 1 agency: even a published offer of another agency is
	// invisible on the private endpoint — get_published_offer serves
	// cross-agency published reads separately, with no identity at all.
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusPublished}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: o}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, stubFlightFinder{}, &mockAirlineRepo{}, &mockAirportRepo{}, userRecordWithAgency(1))

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID: o.UUID,
	})

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestGetOffer_NotFound_ReturnsErrNotFound(t *testing.T) {
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: nil}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, stubFlightFinder{}, &mockAirlineRepo{}, &mockAirportRepo{}, userRecordWithAgency(1))

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrNotFound)
}

func TestGetOffer_ActorNotFound_ReturnsErrUnauthenticated(t *testing.T) {
	mgr := offer.NewManager(&mockOfferRepo{}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, stubFlightFinder{}, &mockAirlineRepo{}, &mockAirportRepo{}, noUserFound())

	_, err := h.Handle(context.Background(), getoffer.Query{
		UUID: uuid.New(),
	})

	assert.ErrorIs(t, err, apperror.ErrUnauthenticated)
}

// cabinClassFlight — сохранённый перелёт из двух сегментов: первый в
// бизнесе, у второго класс неизвестен.
func cabinClassFlight() flight.Flight {
	f := withCabinClasses(twoSegmentFlight(time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)), 11, flight.CabinClassBusiness)
	return f
}

// flightNumberFlight — сохранённый перелёт из двух сегментов: у первого номер
// рейса в произвольном написании, у второго номер неизвестен.
func flightNumberFlight() flight.Flight {
	return withFlightNumbers(twoSegmentFlight(time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC)), 11, "su 1234")
}

func TestGetOffer_SegmentFlightNumber_ReturnedIncludingEmpty(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: o}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, stubFlightFinder{flights: []flight.Flight{flightNumberFlight()}}, &mockAirlineRepo{}, &mockAirportRepo{}, userRecordWithAgency(7))

	res, err := h.Handle(context.Background(), getoffer.Query{UUID: o.UUID})

	require.NoError(t, err)
	require.Len(t, res.Flights, 1)
	require.Len(t, res.Flights[0].Segments, 2)
	assert.Equal(t, "su 1234", res.Flights[0].Segments[0].FlightNumber)
	assert.Equal(t, "", res.Flights[0].Segments[1].FlightNumber)
}

func TestGetOffer_SegmentCabinClass_ReturnedIncludingEmpty(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: o}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, stubFlightFinder{flights: []flight.Flight{cabinClassFlight()}}, &mockAirlineRepo{}, &mockAirportRepo{}, userRecordWithAgency(7))

	res, err := h.Handle(context.Background(), getoffer.Query{UUID: o.UUID})

	require.NoError(t, err)
	require.Len(t, res.Flights, 1)
	require.Len(t, res.Flights[0].Segments, 2)
	assert.Equal(t, "business", res.Flights[0].Segments[0].CabinClass)
	assert.Equal(t, "", res.Flights[0].Segments[1].CabinClass)
}
