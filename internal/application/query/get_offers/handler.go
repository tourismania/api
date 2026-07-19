package getoffers

import (
	"context"
	"fmt"

	"api/internal/domain/enum"
	"api/internal/domain/repository"
)

// UseCase is the port the presentation layer depends on.
type UseCase interface {
	Handle(ctx context.Context, q Query) (Result, error)
}

// OfferLister is the read-port consumed by this use-case. Defined here
// to invert the dependency: infrastructure implements this.
type OfferLister interface {
	List(ctx context.Context, f repository.OfferFilter) (repository.OfferListResult, error)
}

// Handler orchestrates the offer listing use-case and enforces
// read-side visibility by role:
//   - ROLE_SUPER_ADMIN — every offer, filters applied as requested.
//   - ROLE_AGENT       — every offer of their own agency (any status);
//     the agency filter is always forced to their own agency.
//   - everyone else    — published offers only.
type Handler struct {
	offers OfferLister
}

// NewHandler constructs the handler.
func NewHandler(offers OfferLister) *Handler {
	return &Handler{offers: offers}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	filter := repository.OfferFilter{
		AgencyID:  q.AgencyID,
		Status:    q.Status,
		CreatedBy: q.CreatedBy,
		Limit:     q.Limit,
		Offset:    q.Offset,
	}
	applyVisibility(&filter, q.CurrentAgencyID, q.CurrentRoles)

	res, err := h.offers.List(ctx, filter)
	if err != nil {
		return Result{}, fmt.Errorf("list offers: %w", err)
	}

	out := make([]OfferResult, 0, len(res.Offers))
	for _, o := range res.Offers {
		out = append(out, OfferResult{
			ID:          o.ID,
			UUID:        o.UUID,
			Title:       o.Title,
			Description: o.Description,
			AgencyID:    o.AgencyID,
			CreatedBy:   o.CreatedBy,
			Status:      o.Status,
			CreatedAt:   o.CreatedAt,
			UpdatedAt:   o.UpdatedAt,
		})
	}
	return Result{Offers: out, TotalCount: res.TotalCount}, nil
}

func applyVisibility(filter *repository.OfferFilter, currentAgencyID *int, roles []enum.Role) {
	for _, r := range roles {
		if r == enum.RoleSuperAdmin {
			return
		}
	}
	for _, r := range roles {
		if r == enum.RoleAgent && currentAgencyID != nil {
			filter.AgencyID = currentAgencyID
			return
		}
	}
	published := enum.OfferStatusPublished
	filter.Status = &published
}
