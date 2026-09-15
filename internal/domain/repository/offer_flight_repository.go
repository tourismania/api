package repository

import (
	"context"

	"api/internal/domain/entity"
)

// OfferFlightRepository persists the Flight child aggregate of an
// Offer. 1 entity = 1 repository; Flight is a child of Offer, so every
// method is scoped by offerID rather than exposing a standalone Flight
// identifier lookup.
type OfferFlightRepository interface {
	// FindByOfferID returns the offer's flights in the order they were
	// saved (flights by ascending id, segments within a flight by
	// ascending sequence).
	FindByOfferID(ctx context.Context, offerID int) ([]entity.Flight, error)
	// ReplaceForOffer atomically deletes every flight/segment currently
	// stored for offerID and inserts the given set in its place. Callers
	// invoke this only from within a txmanager.TxManager.WithinTx
	// together with the write to the owning offer, so the offer and its
	// flights always commit or roll back together.
	ReplaceForOffer(ctx context.Context, offerID int, flights []entity.Flight) error
}
