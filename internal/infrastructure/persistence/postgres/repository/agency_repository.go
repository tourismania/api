package repository

import (
	"context"
	"errors"
	"fmt"

	"api/internal/domain/entity"
	"api/internal/domain/enum"
	domainrepo "api/internal/domain/repository"
	"api/internal/infrastructure/persistence/postgres/db"
	"api/internal/infrastructure/persistence/postgres/mapper"

	"github.com/jackc/pgx/v5"
)

// AgencyRepository persists domain.Agency aggregates via pgx/sqlc.
type AgencyRepository struct {
	queries *db.Queries
}

// NewAgencyRepository wires the queries layer.
func NewAgencyRepository(queries *db.Queries) *AgencyRepository {
	return &AgencyRepository{queries: queries}
}

// Ensure compile-time interface compliance.
var _ domainrepo.AgencyRepository = (*AgencyRepository)(nil)

// Store inserts a new agency and returns its id.
func (r *AgencyRepository) Store(ctx context.Context, a entity.Agency) (int, error) {
	id, err := r.queries.CreateAgency(ctx, db.CreateAgencyParams{
		Uuid:      a.UUID,
		Name:      a.Name,
		Status:    string(a.Status),
		CreatedAt: a.CreatedAt,
	})
	if err != nil {
		return 0, fmt.Errorf("insert agency: %w", err)
	}
	return int(id), nil
}

// FindByID fetches a non-deleted agency by primary key.
func (r *AgencyRepository) FindByID(ctx context.Context, id int) (*entity.Agency, error) {
	row, err := r.queries.GetAgencyByID(ctx, int32(id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find agency by id: %w", err)
	}
	agency := mapper.ToAgencyDomain(row)
	return &agency, nil
}

// SetStatus updates the agency lifecycle status.
func (r *AgencyRepository) SetStatus(ctx context.Context, id int, status enum.AgencyStatus) error {
	if err := r.queries.SetAgencyStatus(ctx, int32(id), string(status)); err != nil {
		return fmt.Errorf("set agency status: %w", err)
	}
	return nil
}

// Exists reports whether a non-deleted agency exists for id.
func (r *AgencyRepository) Exists(ctx context.Context, id int) (bool, error) {
	exists, err := r.queries.AgencyExists(ctx, int32(id))
	if err != nil {
		return false, fmt.Errorf("check agency existence: %w", err)
	}
	return exists, nil
}
