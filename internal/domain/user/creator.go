package user

import (
	"context"
	"errors"
	"fmt"

	"api/internal/domain/agency"
	"api/internal/domain/event"
)

// ErrNotPersisted is returned when the repository returns a nil id —
// meaning the row was rejected for a reason caller couldn't predict.
var ErrNotPersisted = errors.New("user was not persisted")

// Creator orchestrates registration: hash credentials, persist, then
// publish a Registered event so async consumers can react.
type Creator struct {
	users    Repository
	agencies agency.Repository
	hasher   PasswordHasher
	eventBus event.Bus
}

// NewCreator wires the collaborators. All four are required.
func NewCreator(
	users Repository,
	agencies agency.Repository,
	hasher PasswordHasher,
	eventBus event.Bus,
) *Creator {
	return &Creator{users: users, agencies: agencies, hasher: hasher, eventBus: eventBus}
}

// Create hashes the user's password, stores the entity, and publishes a
// Registered event. Event-publish failures are returned to the caller
// rather than swallowed: the original PHP project relies on
// transactional outbox / retry at a higher level. If you later add an
// outbox, replace the direct Publish here.
//
// u.AgencyID is required (1 user = 1 agency): the referenced agency
// must exist and be active, or Create fails with agency.ErrNotFound /
// agency.ErrInactive.
func (s *Creator) Create(ctx context.Context, u User) (int, error) {
	a, err := s.agencies.FindByID(ctx, u.AgencyID)
	if err != nil {
		return 0, fmt.Errorf("find agency: %w", err)
	}
	if a == nil {
		return 0, agency.ErrNotFound
	}
	if !a.IsActive() {
		return 0, agency.ErrInactive
	}

	hash, err := s.hasher.Hash(u.Password)
	if err != nil {
		return 0, fmt.Errorf("hash password: %w", err)
	}

	idPtr, err := s.users.Store(ctx, u, hash)
	if err != nil {
		return 0, fmt.Errorf("store user: %w", err)
	}
	if idPtr == nil {
		return 0, ErrNotPersisted
	}

	if err := s.eventBus.Publish(Registered{ID: *idPtr}); err != nil {
		return 0, fmt.Errorf("publish user_registered: %w", err)
	}
	return *idPtr, nil
}
