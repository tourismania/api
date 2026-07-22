package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"api/internal/domain/entity"
	"api/internal/domain/enum"
	"api/internal/domain/repository"
	"api/internal/domain/valueobject"

	"github.com/google/uuid"
)

// ErrOfferNotFound is returned when an offer lookup finds no matching
// non-deleted row.
var ErrOfferNotFound = errors.New("offer not found")

// ErrActorNotFound is returned when the identity resolved from a valid
// JWT subject no longer has a matching user row — e.g. the account was
// deleted after the token was issued. Application-layer handlers use
// this whenever they resolve the acting principal by uuid before
// delegating to a domain service.
var ErrActorNotFound = errors.New("actor not found")

// ErrOfferTitleInvalid is returned when the title is empty or exceeds
// entity.OfferTitleMaxLength.
var ErrOfferTitleInvalid = errors.New("offer title is required and must be at most 200 characters")

// ErrOfferStatusInvalid is returned when the supplied status is not one
// of the known enum.OfferStatus values.
var ErrOfferStatusInvalid = errors.New("invalid offer status")

// ErrOfferNotPersisted is returned when the repository does not persist
// an offer for a reason the caller could not predict.
var ErrOfferNotPersisted = errors.New("offer was not persisted")

// ErrInsufficientRole is returned when the actor is authenticated (and,
// for Update/Delete, may even belong to the right agency) but lacks the
// role required to write an offer. Unlike ErrOfferNotFound this never
// depends on any specific offer's existence, so returning it does not
// leak anything about a particular resource.
var ErrInsufficientRole = errors.New("actor lacks the role required to manage offers")

// OfferManager orchestrates offer lifecycle: creation, update and soft
// deletion. It enforces invariants, strict agency ownership, and the
// role required to write an offer — an offer may only be managed by an
// actor belonging to its owning agency and carrying ROLE_AGENT or
// ROLE_SUPER_ADMIN. Reads have no role restriction (see
// application/query/get_offer(s)).
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
func (m *OfferManager) Insert(ctx context.Context, title, description string, status enum.OfferStatus, actor valueobject.Actor) (entity.Offer, error) {
	if !canWriteOffers(actor) {
		return entity.Offer{}, ErrInsufficientRole
	}
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
func (m *OfferManager) Update(ctx context.Context, id uuid.UUID, title, description *string, status *enum.OfferStatus, actor valueobject.Actor) (entity.Offer, error) {
	if !canWriteOffers(actor) {
		return entity.Offer{}, ErrInsufficientRole
	}
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
func (m *OfferManager) Delete(ctx context.Context, id uuid.UUID, actor valueobject.Actor) error {
	if !canWriteOffers(actor) {
		return ErrInsufficientRole
	}
	if _, err := m.findOwned(ctx, id, actor); err != nil {
		return err
	}
	if err := m.offers.SoftDelete(ctx, id); err != nil {
		return fmt.Errorf("soft delete offer: %w", err)
	}
	return nil
}

// findOwned fetches an offer and verifies the actor belongs to its
// owning agency. 1 user = 1 agency: there is no role-based bypass. An
// offer of another agency is reported as ErrOfferNotFound, not a
// forbidden error — for the actor it simply does not exist.
func (m *OfferManager) findOwned(ctx context.Context, id uuid.UUID, actor valueobject.Actor) (*entity.Offer, error) {
	offer, err := m.offers.FindByUUID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("find offer: %w", err)
	}
	if offer == nil || offer.AgencyID != actor.AgencyID {
		return nil, ErrOfferNotFound
	}
	return offer, nil
}

// canWriteOffers reports whether the actor's roles permit creating,
// updating or deleting offers.
func canWriteOffers(actor valueobject.Actor) bool {
	return actor.HasRole(enum.RoleAgent) || actor.HasRole(enum.RoleSuperAdmin)
}

func validateOfferTitle(title string) error {
	if title == "" || len([]rune(title)) > entity.OfferTitleMaxLength {
		return ErrOfferTitleInvalid
	}
	return nil
}
