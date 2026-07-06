package repository

import (
	"context"

	"api/internal/domain/entity"
	"api/internal/domain/enum"
)

// AgencyRepository persists Agency aggregates. 1 entity = 1 repository.
type AgencyRepository interface {
	// Store inserts a new agency and returns its id.
	Store(ctx context.Context, a entity.Agency) (int, error)
	// FindByID fetches an agency by primary key. Reads filter
	// deleted_at IS NULL. Returns nil if not found.
	FindByID(ctx context.Context, id int) (*entity.Agency, error)
	// SetStatus updates the agency lifecycle status (active/inactive).
	SetStatus(ctx context.Context, id int, status enum.AgencyStatus) error
	// Exists reports whether an agency with the given id exists
	// (deleted_at IS NULL).
	Exists(ctx context.Context, id int) (bool, error)
}
