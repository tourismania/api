package repository

import (
	"context"
	"fmt"

	"api/internal/domain/entity"
	domainrepo "api/internal/domain/repository"
	"api/internal/infrastructure/persistence/postgres/db"
	"api/internal/infrastructure/persistence/postgres/mapper"
)

// OfferFlightRepository persists the Flight child aggregate of an Offer
// via pgx/sqlc.
type OfferFlightRepository struct {
	queries *db.Queries
}

// NewOfferFlightRepository wires the queries layer.
func NewOfferFlightRepository(queries *db.Queries) *OfferFlightRepository {
	return &OfferFlightRepository{queries: queries}
}

// Ensure compile-time interface compliance.
var _ domainrepo.OfferFlightRepository = (*OfferFlightRepository)(nil)

// FindByOfferID returns the offer's flights in saved order (flights by
// ascending id, segments within a flight by ascending sequence).
func (r *OfferFlightRepository) FindByOfferID(ctx context.Context, offerID int) ([]entity.Flight, error) {
	q := queriesFor(ctx, r.queries)

	flightRows, err := q.ListOfferFlightsByOfferID(ctx, int32(offerID))
	if err != nil {
		return nil, fmt.Errorf("list offer flights: %w", err)
	}
	if len(flightRows) == 0 {
		return nil, nil
	}

	flightIDs := make([]int32, len(flightRows))
	for i, f := range flightRows {
		flightIDs[i] = f.ID
	}

	segRows, err := q.ListOfferFlightSegmentsByFlightIDs(ctx, flightIDs)
	if err != nil {
		return nil, fmt.Errorf("list offer flight segments: %w", err)
	}

	return mapper.ToFlightsDomain(flightRows, segRows), nil
}

// ReplaceForOffer atomically deletes every flight/segment currently
// stored for offerID and inserts the given set. The caller (an
// application handler running inside txmanager.TxManager.WithinTx)
// guarantees this and the sibling write to the owning offer share one
// transaction — this method issues its statements against whatever
// queriesFor(ctx, ...) resolves to and does not open a transaction of
// its own.
func (r *OfferFlightRepository) ReplaceForOffer(ctx context.Context, offerID int, flights []entity.Flight) error {
	q := queriesFor(ctx, r.queries)

	if err := q.DeleteOfferFlightsByOfferID(ctx, int32(offerID)); err != nil {
		return fmt.Errorf("delete existing offer flights: %w", err)
	}

	for _, f := range flights {
		flightID, err := q.CreateOfferFlight(ctx, int32(offerID))
		if err != nil {
			return fmt.Errorf("insert offer flight: %w", err)
		}

		for i, seg := range f.Segments {
			if err := q.CreateOfferFlightSegment(ctx, db.CreateOfferFlightSegmentParams{
				FlightID:             flightID,
				Sequence:             int16(i + 1),
				DepartureAirportICAO: seg.DepartureAirportICAO,
				ArrivalAirportICAO:   seg.ArrivalAirportICAO,
				DepartureAt:          seg.DepartureAt,
				ArrivalAt:            seg.ArrivalAt,
			}); err != nil {
				return fmt.Errorf("insert offer flight segment: %w", err)
			}
		}
	}
	return nil
}
