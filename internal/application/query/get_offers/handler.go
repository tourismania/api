package getoffers

import (
	"context"
	"fmt"

	"api/internal/application/apperror"
	"api/internal/domain/repository"
	"api/internal/domain/service"
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

// Handler orchestrates the offer listing use-case. The list is always
// scoped to the caller's own agency, any status — the role has no
// bearing on visibility, only on write access (enforced by the domain
// OfferManager). The caller's own agency is resolved from its uuid via
// service.UserFinder, not presentation-layer middleware.
type Handler struct {
	offers     OfferLister
	userFinder *service.UserFinder
}

// NewHandler constructs the handler.
func NewHandler(offers OfferLister, userFinder *service.UserFinder) *Handler {
	return &Handler{offers: offers, userFinder: userFinder}
}

// Handle satisfies UseCase.
func (h *Handler) Handle(ctx context.Context, q Query) (Result, error) {
	actor, err := h.userFinder.Resolve(ctx, q.CurrentUserUUID)
	if err != nil {
		return Result{}, apperror.FromDomainError(err)
	}

	filter := repository.OfferFilter{
		AgencyID:  &actor.AgencyID,
		Status:    q.Status,
		CreatedBy: q.CreatedBy,
		Limit:     q.Limit,
		Offset:    q.Offset,
	}

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
