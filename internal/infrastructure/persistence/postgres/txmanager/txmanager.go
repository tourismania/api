// Package txmanager implements application/txmanager.TxManager on top of
// pgxpool. The active pgx.Tx is stashed in the context under a private
// key; repositories pull it out via FromContext and, when present, run
// their queries against it instead of the pool — see db.Queries.WithTx.
package txmanager

import (
	"context"
	"fmt"

	apptx "api/internal/application/txmanager"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type txKey struct{}

// Manager implements apptx.TxManager over a pgxpool.Pool.
type Manager struct {
	pool *pgxpool.Pool
}

// New wires the pool.
func New(pool *pgxpool.Pool) *Manager {
	return &Manager{pool: pool}
}

// Ensure compile-time interface compliance.
var _ apptx.TxManager = (*Manager)(nil)

// WithinTx begins a transaction, stashes it in the context passed to
// fn, and commits on success or rolls back on error (including a panic,
// which is re-raised after rollback).
func (m *Manager) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := m.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	txCtx := context.WithValue(ctx, txKey{}, tx)

	defer func() {
		if p := recover(); p != nil {
			_ = tx.Rollback(ctx)
			panic(p)
		}
	}()

	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(ctx); rbErr != nil {
			return fmt.Errorf("%w (rollback also failed: %v)", err, rbErr)
		}
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}

// FromContext returns the active transaction stashed by WithinTx, if
// any. Repositories use this to run their sqlc queries against the tx
// instead of the pool when one is present.
func FromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}
