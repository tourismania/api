package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/repository"

	"github.com/google/uuid"
)

// ErrOfferNotFound is returned when an offer lookup finds no matching
// non-deleted row.
var ErrOfferNotFound = errors.New("offer not found")

// ErrOfferForbidden is returned when the acting user does not belong to
// the offer's owning agency. 1 user = 1 agency: there is no bypass —
// every actor, including ROLE_SUPER_ADMIN, may only manage offers of
// their own agency.
var ErrOfferForbidden = errors.New("forbidden: offer belongs to another agency")

// ErrOfferTitleInvalid is returned when the title is empty or exceeds
// entity.OfferTitleMaxLength.
var ErrOfferTitleInvalid = errors.New("offer title is required and must be at most 200 characters")

// ErrOfferStatusInvalid is returned when the supplied status is not one
// of the known enum.OfferStatus values.
var ErrOfferStatusInvalid = errors.New("invalid offer status")

// ErrOfferNotPersisted is returned when the repository does not persist
// an offer for a reason the caller could not predict.
var ErrOfferNotPersisted = errors.New("offer was not persisted")

// Actor is the identity of the user performing an offer operation. It is
// a plain domain value — no HTTP/JWT knowledge leaks in here. AgencyID
// is required: every user belongs to exactly one agency (1 user = 1
// agency, enforced at the database level).
type Actor struct {
	UserID   int
	AgencyID int
}

// OfferManager orchestrates offer lifecycle: creation, update and soft
// deletion. It enforces invariants and strict agency ownership — an
// offer may only be managed by an actor belonging to its owning agency.
type OfferManager struct {
	offers   repository.OfferRepository
	agencies repository.AgencyRepository
}

// NewOfferManager wires the collaborators.
func NewOfferManager(offers repository.OfferRepository, agencies repository.AgencyRepository) *OfferManager {
	return &OfferManager{offers: offers, agencies: agencies}
}

// Insert creates a new offer under the actor's own agency. AgencyID is
// never taken from caller input — it is always derived from the actor.
func (m *OfferManager) Insert(ctx context.Context, title, description string, status enum.OfferStatus, actor Actor) (entity.Offer, error) {
	if err := validateOfferTitle(title); err != nil {
		return entity.Offer{}, err
	}
	if !status.IsValid() {
		return entity.Offer{}, ErrOfferStatusInvalid
	}

	agency, err := m.agencies.FindByID(ctx, actor.AgencyID)
	if err != nil {
		return entity.Offer{}, fmt.Errorf("find agency: %w", err)
	}
	if agency == nil {
		return entity.Offer{}, ErrAgencyNotFound
	}
	if !agency.IsActive() {
		return entity.Offer{}, ErrAgencyInactive
	}

	now := time.Now()
	offer := entity.Offer{
		UUID:        uuid.New(),
		Title:       title,
		Description: description,
		AgencyID:    actor.AgencyID,
		CreatedBy:   actor.UserID,
		Status:      status,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	id, err := m.offers.Store(ctx, offer)
	if err != nil {
		return entity.Offer{}, fmt.Errorf("store offer: %w", err)
	}
	if id == 0 {
		return entity.Offer{}, ErrOfferNotPersisted
	}
	offer.ID = id
	return offer, nil
}

// Update applies partial changes to an existing offer. Only non-nil
// fields are modified. Ownership is enforced: the actor must belong to
// the offer's agency.
func (m *OfferManager) Update(ctx context.Context, id uuid.UUID, title, description *string, status *enum.OfferStatus, actor Actor) (entity.Offer, error) {
	offer, err := m.findOwned(ctx, id, actor)
	if err != nil {
		return entity.Offer{}, err
	}

	if title != nil {
		if err := validateOfferTitle(*title); err != nil {
			return entity.Offer{}, err
		}
		offer.Title = *title
	}
	if description != nil {
		offer.Description = *description
	}
	if status != nil {
		if !status.IsValid() {
			return entity.Offer{}, ErrOfferStatusInvalid
		}
		offer.Status = *status
	}
	offer.UpdatedAt = time.Now()

	if err := m.offers.Update(ctx, *offer); err != nil {
		return entity.Offer{}, fmt.Errorf("update offer: %w", err)
	}
	return *offer, nil
}

// Delete soft-deletes an offer. Ownership is enforced the same way as
// Update.
func (m *OfferManager) Delete(ctx context.Context, id uuid.UUID, actor Actor) error {
	if _, err := m.findOwned(ctx, id, actor); err != nil {
		return err
	}
	if err := m.offers.SoftDelete(ctx, id); err != nil {
		return fmt.Errorf("soft delete offer: %w", err)
	}
	return nil
}

// findOwned fetches an offer and verifies the actor belongs to its
// owning agency. 1 user = 1 agency: there is no role-based bypass.
func (m *OfferManager) findOwned(ctx context.Context, id uuid.UUID, actor Actor) (*entity.Offer, error) {
	offer, err := m.offers.FindByUUID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find offer: %w", err)
	}
	if offer == nil {
		return nil, ErrOfferNotFound
	}
	if offer.AgencyID != actor.AgencyID {
		return nil, ErrOfferForbidden
	}
	return offer, nil
}

func validateOfferTitle(title string) error {
	if title == "" || len([]rune(title)) > entity.OfferTitleMaxLength {
		return ErrOfferTitleInvalid
	}
	return nil
}
