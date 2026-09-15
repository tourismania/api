// Package txmanager defines the application-layer port for running a
// use case's writes atomically. Modeled after application/apperror: a
// small, standalone top-level Application package rather than something
// tucked inside a single command/*. It lives in Application, not the
// domain — atomicity across OfferRepository and OfferFlightRepository
// is only ever needed by the use-case orchestration in
// create_offer/update_offer; neither OfferManager nor
// OfferFlightManager call WithinTx themselves or know it exists. The
// concrete implementation lives in
// infrastructure/persistence/postgres/txmanager.
package txmanager

import "context"

// TxManager runs fn within a single atomic unit of work. If fn returns
// an error, every write fn made through it is rolled back. Repositories
// invoked from within fn detect the active transaction via context and
// use it transparently instead of the connection pool.
type TxManager interface {
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}
