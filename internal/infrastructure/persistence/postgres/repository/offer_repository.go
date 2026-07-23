package repository

import (
	"context"
	"errors"
	"fmt"

	"api/internal/domain/entity"
	domainrepo "api/internal/domain/repository"
	"api/internal/infrastructure/persistence/postgres/db"
	"api/internal/infrastructure/persistence/postgres/mapper"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// OfferRepository persists domain.Offer aggregates via pgx/sqlc.
type OfferRepository struct {
	queries *db.Queries
}

// NewOfferRepository wires the queries layer.
func NewOfferRepository(queries *db.Queries) *OfferRepository {
	return &OfferRepository{queries: queries}
}

// Ensure compile-time interface compliance.
var _ domainrepo.OfferRepository = (*OfferRepository)(nil)

// Store inserts a new offer and returns its id.
func (r *OfferRepository) Store(ctx context.Context, o entity.Offer) (int, error) {
	id, err := r.queries.CreateOffer(ctx, db.CreateOfferParams{
		Uuid:        o.UUID,
		Title:       o.Title,
		Description: o.Description,
		AgencyID:    int32(o.AgencyID),
		CreatedBy:   int32(o.CreatedBy),
		Status:      string(o.Status),
		CreatedAt:   o.CreatedAt,
		UpdatedAt:   o.UpdatedAt,
	})
	if err != nil {
		return 0, fmt.Errorf("insert offer: %w", err)
	}
	return int(id), nil
}

// FindByUUID fetches a non-deleted offer by its public identifier.
func (r *OfferRepository) FindByUUID(ctx context.Context, id uuid.UUID) (*entity.Offer, error) {
	row, err := r.queries.GetOfferByUUID(ctx, id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find offer by uuid: %w", err)
	}
	offer := mapper.ToOfferDomain(row)
	return &offer, nil
}

// List returns a filtered, paginated page of non-deleted offers.
func (r *OfferRepository) List(ctx context.Context, f domainrepo.OfferFilter) (domainrepo.OfferListResult, error) {
	var agencyID *int32
	if f.AgencyID != nil {
		v := int32(*f.AgencyID)
		agencyID = &v
	}
	var status *string
	if f.Status != nil {
		v := string(*f.Status)
		status = &v
	}
	var createdBy *int32
	if f.CreatedBy != nil {
		v := int32(*f.CreatedBy)
		createdBy = &v
	}

	rows, err := r.queries.ListOffers(ctx, db.ListOffersParams{
		AgencyID:  agencyID,
		Status:    status,
		CreatedBy: createdBy,
		LimitVal:  int32(f.Limit),
		OffsetVal: int32(f.Offset),
	})
	if err != nil {
		return domainrepo.OfferListResult{}, fmt.Errorf("list offers: %w", err)
	}

	offers := make([]entity.Offer, 0, len(rows))
	var total int64
	for _, row := range rows {
		offers = append(offers, mapper.ToOfferDomainFromListRow(row))
		total = row.TotalCount
	}
	return domainrepo.OfferListResult{Offers: offers, TotalCount: total}, nil
}

// Update persists changes to an existing offer's title/description/status.
func (r *OfferRepository) Update(ctx context.Context, o entity.Offer) error {
	if err := r.queries.UpdateOffer(ctx, db.UpdateOfferParams{
		Uuid:        o.UUID,
		Title:       o.Title,
		Description: o.Description,
		Status:      string(o.Status),
		UpdatedAt:   o.UpdatedAt,
	}); err != nil {
		return fmt.Errorf("update offer: %w", err)
	}
	return nil
}

// SoftDelete marks the offer as deleted.
func (r *OfferRepository) SoftDelete(ctx context.Context, id uuid.UUID) error {
	if err := r.queries.SoftDeleteOffer(ctx, id); err != nil {
		return fmt.Errorf("soft delete offer: %w", err)
	}
	return nil
}
