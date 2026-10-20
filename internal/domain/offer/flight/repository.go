package flight

import (
	"context"

	"api/internal/domain/money"
)

// Repository persists the Flight child aggregate of an Offer.
// 1 entity = 1 repository; Flight is a child of Offer, so every
// method is scoped by offerID rather than exposing a standalone Flight
// identifier lookup.
type Repository interface {
	// FindByOfferID returns the offer's flights in the order they were
	// saved (flights by ascending id, segments within a flight by
	// ascending sequence).
	FindByOfferID(ctx context.Context, offerID int) ([]Flight, error)
	// ReplaceForOffer atomically deletes every flight/segment currently
	// stored for offerID and inserts the given set in its place. Callers
	// invoke this only from within a txmanager.TxManager.WithinTx
	// together with the write to the owning offer, so the offer and its
	// flights always commit or roll back together.
	ReplaceForOffer(ctx context.Context, offerID int, flights []Flight) error
	// UpdateDetails перезаписывает цену, валюту и комментарий менеджера у
	// перечисленных перелётов offerID и классы обслуживания и авиакомпании их
	// сегментов (по порядку сегментов), не трогая аэропорты и время сегментов
	// и идентификаторы перелётов. Вызывается, как и ReplaceForOffer, только внутри
	// txmanager.TxManager.WithinTx вместе с записью оффера.
	UpdateDetails(ctx context.Context, offerID int, updates []DetailsUpdate) error
}

// DetailsUpdate — новые цена, комментарий менеджера, классы и авиакомпании
// сегментов одного уже сохранённого перелёта (поля, не входящие в маршрут).
// SegmentCabinClasses и SegmentAirlineIDs — значения всех сегментов перелёта
// по порядку: маршрут совпал, поэтому длина равна числу сохранённых сегментов.
type DetailsUpdate struct {
	FlightID            int
	Price               money.Money
	ManagerComment      string
	SegmentCabinClasses []CabinClass
	SegmentAirlineIDs   []int
}
