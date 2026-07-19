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

// ErrOfferForbidden is returned when the acting user does not own the
// offer's agency and is not a super admin.
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

// ErrActorHasNoAgency is returned when the acting user has no agency
// attached — required to create or own offers.
var ErrActorHasNoAgency = errors.New("actor has no agency")

// Actor is the identity of the user performing an offer operation. It is
// a plain domain value — no HTTP/JWT knowledge leaks in here.
type Actor struct {
	UserID   int
	AgencyID *int
	Roles    []enum.Role
}

// IsSuperAdmin reports whether the actor carries the super-admin role.
func (a Actor) IsSuperAdmin() bool {
	for _, r := range a.Roles {
		if r == enum.RoleSuperAdmin {
			return true
		}
	}
	return false
}

// OfferManager orchestrates offer lifecycle: creation, update and soft
// deletion. It enforces invariants and agency ownership; super admins
// bypass the ownership check.
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
	if actor.AgencyID == nil {
		return entity.Offer{}, ErrActorHasNoAgency
	}

	agency, err := m.agencies.FindByID(ctx, *actor.AgencyID)
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
		AgencyID:    *actor.AgencyID,
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
// the offer's agency, unless they are a super admin.
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

// findOwned fetches an offer and verifies the actor may act on it:
// super admins may act on any offer; everyone else must belong to the
// offer's owning agency.
func (m *OfferManager) findOwned(ctx context.Context, id uuid.UUID, actor Actor) (*entity.Offer, error) {
	offer, err := m.offers.FindByUUID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find offer: %w", err)
	}
	if offer == nil {
		return nil, ErrOfferNotFound
	}
	if !actor.IsSuperAdmin() {
		if actor.AgencyID == nil || offer.AgencyID != *actor.AgencyID {
			return nil, ErrOfferForbidden
		}
	}
	return offer, nil
}

func validateOfferTitle(title string) error {
	if title == "" || len([]rune(title)) > entity.OfferTitleMaxLength {
		return ErrOfferTitleInvalid
	}
	return nil
}
