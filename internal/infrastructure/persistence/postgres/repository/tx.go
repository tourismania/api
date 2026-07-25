package repository

import (
	"context"

	"api/internal/infrastructure/persistence/postgres/db"
	pgtx "api/internal/infrastructure/persistence/postgres/txmanager"
)

// queriesFor returns q bound to the pgx.Tx stashed in ctx by
// txmanager.Manager.WithinTx, if one is active, otherwise q unchanged
// (bound to the pool). This is what lets a repository's queries
// transparently join an ambient transaction opened by the application
// handler without the repository knowing anything about transactions
// itself.
func queriesFor(ctx context.Context, q *db.Queries) *db.Queries {
	if tx, ok := pgtx.FromContext(ctx); ok {
		return q.WithTx(tx)
	}
	return q
}
