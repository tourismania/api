package updateoffer

import (
	"context"

	"api/internal/application/apperror"
	"api/internal/application/txmanager"
	"api/internal/domain/money"
	"api/internal/domain/offer"
	"api/internal/domain/offer/flight"
	"api/internal/domain/user"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, cmd Command) (Result, error)
}

// Handler executes the UpdateOffer command by delegating to the domain
// offer.Manager service, which enforces agency ownership and the write
// role. The acting principal is resolved from its uuid via
// user.Finder, not presentation-layer middleware. Every domain
// error is translated to apperror before it leaves this handler.
// Updating the offer and (if the flights key was present) replacing its
// flights is wrapped in a single txManager.WithinTx.
//
// offerManager/offerFlightManager/userFinder — конкретные доменные
// сервисы (*service.X), а не интерфейсы: они не подменяются другой
// реализацией, поэтому абстракция не нужна (единый паттерн для всех
// Command/Query Handler'ов в проекте). txManager — наоборот, интерфейс
// application/txmanager.TxManager: это порт с двумя реализациями
// (реальный Postgres-менеджер в infrastructure и no-op в юнит-тестах),
// поэтому ему, в отличие от доменных сервисов, необходима абстракция.
type Handler struct {
	offerManager       *offer.Manager
	offerFlightManager *flight.Manager
	userFinder         *user.Finder
	txManager          txmanager.TxManager
}

// NewHandler constructs the handler.
func NewHandler(offerManager *offer.Manager, offerFlightManager *flight.Manager, userFinder *user.Finder, txManager txmanager.TxManager) *Handler {
	return &Handler{
		offerManager:       offerManager,
		offerFlightManager: offerFlightManager,
		userFinder:         userFinder,
		txManager:          txManager,
	}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, cmd Command) (Result, error) {
	actor, err := h.userFinder.Resolve(ctx, cmd.CurrentUserUUID)
	if err != nil {
		return Result{}, apperror.FromDomainError(err)
	}

	var flights []flight.Flight
	if cmd.Flights != nil {
		flights, err = buildFlights(*cmd.Flights)
		if err != nil {
			return Result{}, apperror.FromDomainError(err)
		}
	}

	var result Result
	err = h.txManager.WithinTx(ctx, func(txCtx context.Context) error {
		offer, err := h.offerManager.Update(txCtx, cmd.UUID, cmd.Title, cmd.Description, cmd.Status, actor)
		if err != nil {
			return err
		}
		if cmd.Flights != nil {
			if err := h.offerFlightManager.ReplaceForOffer(txCtx, offer.ID, flights); err != nil {
				return err
			}
		}
		result = Result{ID: offer.ID, UUID: offer.UUID}
		return nil
	})
	if err != nil {
		return Result{}, apperror.FromDomainError(err)
	}
	return result, nil
}

// buildFlights разбирает цену каждого перелёта по справочнику валют и
// проверяет структурные инварианты через flight.New before any database
// write happens, so a malformed flight never even opens a transaction.
func buildFlights(inputs []FlightInput) ([]flight.Flight, error) {
	if len(inputs) == 0 {
		return nil, nil
	}
	flights := make([]flight.Flight, 0, len(inputs))
	for _, in := range inputs {
		price, err := money.Parse(in.Price, in.Currency)
		if err != nil {
			return nil, err
		}
		f, err := flight.New(price, toDomainSegments(in.Segments))
		if err != nil {
			return nil, err
		}
		flights = append(flights, f)
	}
	return flights, nil
}

// toDomainSegments конвертирует Application-DTO сегментов в доменный
// flight.Segment. Только Handler знает про domain/offer —
// presentation оперирует исключительно FlightSegmentInput.
func toDomainSegments(in []FlightSegmentInput) []flight.Segment {
	segs := make([]flight.Segment, 0, len(in))
	for _, s := range in {
		segs = append(segs, flight.Segment{
			DepartureAirportICAO: s.DepartureAirportICAO,
			ArrivalAirportICAO:   s.ArrivalAirportICAO,
			DepartureAt:          s.DepartureAt,
			ArrivalAt:            s.ArrivalAt,
		})
	}
	return segs
}
