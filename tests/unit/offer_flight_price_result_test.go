package unit_test

import (
	"context"
	"testing"
	"time"

	getoffer "api/internal/application/query/get_offer"
	getpublishedoffer "api/internal/application/query/get_published_offer"
	"api/internal/domain/money"
	"api/internal/domain/offer"
	"api/internal/domain/offer/flight"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pricedFlight — сохранённый перелёт с ценой 15000 RUB.
func pricedFlight() flight.Flight {
	f := sampleFlight("UUEE", "LFPG", time.Date(2026, 8, 1, 10, 0, 0, 0, time.UTC))
	f.ID = 11
	f.Price = mustMoney("15000", money.CodeRUB)
	return f
}

func TestGetOffer_FlightWithPrice_ReturnsFormattedPriceAndCurrency(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusDraft}
	mgr := offer.NewManager(&mockOfferRepo{findByUUIDOffer: o}, &mockAgencyRepo{})
	h := getoffer.NewHandler(mgr, stubFlightFinder{flights: []flight.Flight{pricedFlight()}}, userRecordWithAgency(7))

	res, err := h.Handle(context.Background(), getoffer.Query{UUID: o.UUID})

	require.NoError(t, err)
	require.Len(t, res.Flights, 1)
	assert.Equal(t, 11, res.Flights[0].ID)
	assert.Equal(t, "15000.00", res.Flights[0].Price)
	assert.Equal(t, "RUB", res.Flights[0].Currency)
}

func TestGetPublishedOffer_FlightWithPrice_ReturnsFormattedPriceAndCurrency(t *testing.T) {
	o := &offer.Offer{UUID: uuid.New(), AgencyID: 7, Status: offer.StatusPublished}
	h := getpublishedoffer.NewHandler(stubOfferFinder{offer: o}, stubFlightFinder{flights: []flight.Flight{pricedFlight()}})

	res, err := h.Handle(context.Background(), getpublishedoffer.Query{UUID: o.UUID})

	require.NoError(t, err)
	require.Len(t, res.Flights, 1)
	assert.Equal(t, "15000.00", res.Flights[0].Price)
	assert.Equal(t, "RUB", res.Flights[0].Currency)
}
