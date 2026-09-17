package integration_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"api/internal/domain/agency"
	"api/internal/domain/event"
	"api/internal/domain/user"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubHasher returns the input as-is, prefixed so we can assert it
// arrived at the repository.
type stubHasher struct{}

func (stubHasher) Hash(p string) (string, error) { return "hashed:" + p, nil }
func (stubHasher) Verify(_, _ string) error      { return nil }

// fakeAgencyRepo returns a fixed agency (or nil) for FindByID.
type fakeAgencyRepo struct{ agency *agency.Agency }

func (r fakeAgencyRepo) Store(_ context.Context, _ agency.Agency) (int, error) { return 0, nil }
func (r fakeAgencyRepo) FindByID(_ context.Context, _ int) (*agency.Agency, error) {
	return r.agency, nil
}
func (fakeAgencyRepo) SetStatus(_ context.Context, _ int, _ agency.Status) error { return nil }
func (fakeAgencyRepo) Exists(_ context.Context, _ int) (bool, error)             { return false, nil }

// nilStoringRepo simulates a repository that "stores" without producing
// an id — same path as the PHP integration test, where Store returns
// null and the service must surface a hard error.
type nilStoringRepo struct {
	called      bool
	savedHash   string
	savedEntity user.User
}

func (r *nilStoringRepo) Store(_ context.Context, u user.User, hash string) (*int, error) {
	r.called = true
	r.savedEntity = u
	r.savedHash = hash
	return nil, nil
}

func (r *nilStoringRepo) FindByUuid(_ context.Context, _ uuid.UUID) (*user.Record, error) {
	return nil, nil
}

// inMemoryBus captures publishes for assertion.
type inMemoryBus struct{ events []event.DomainEvent }

func (b *inMemoryBus) Publish(e event.DomainEvent) error {
	b.events = append(b.events, e)
	return nil
}

func TestUserCreator_StoreReturnsNilID_ProducesError(t *testing.T) {
	repo := &nilStoringRepo{}
	bus := &inMemoryBus{}
	active := agency.Agency{ID: 1, Status: agency.StatusActive, CreatedAt: time.Now()}
	svc := user.NewCreator(repo, fakeAgencyRepo{agency: &active}, stubHasher{}, bus)

	_, err := svc.Create(context.Background(), user.User{
		FirstName: "Ada", LastName: "Lovelace",
		Email: "ada@example.com", Password: "secret", AgencyID: 1,
	})

	assert.True(t, repo.called, "repository should be called")
	assert.Equal(t, "hashed:secret", repo.savedHash, "hashed password should be forwarded")
	assert.ErrorIs(t, err, user.ErrNotPersisted)
	assert.Empty(t, bus.events, "no event should be published when persist fails")
}

// repoOK returns a fixed id.
type repoOK struct{}

func (repoOK) Store(_ context.Context, _ user.User, _ string) (*int, error) {
	id := 42
	return &id, nil
}

func (repoOK) FindByUuid(_ context.Context, _ uuid.UUID) (*user.Record, error) {
	return nil, nil
}

// publishErrBus simulates kafka being unreachable.
type publishErrBus struct{}

func (publishErrBus) Publish(_ event.DomainEvent) error { return errors.New("broker down") }

func TestUserCreator_PublishFailure_PropagatesError(t *testing.T) {
	active := agency.Agency{ID: 1, Status: agency.StatusActive, CreatedAt: time.Now()}
	svc := user.NewCreator(repoOK{}, fakeAgencyRepo{agency: &active}, stubHasher{}, publishErrBus{})
	_, err := svc.Create(context.Background(), user.User{
		Email: "a@b.c", Password: "p", AgencyID: 1,
	})
	assert.Error(t, err)
}

func TestUserCreator_AgencyNotFound_ReturnsError(t *testing.T) {
	svc := user.NewCreator(repoOK{}, fakeAgencyRepo{agency: nil}, stubHasher{}, &inMemoryBus{})

	_, err := svc.Create(context.Background(), user.User{
		Email: "agent@example.com", Password: "secret", AgencyID: 99,
	})

	assert.ErrorIs(t, err, agency.ErrNotFound)
}

func TestUserCreator_AgencyInactive_ReturnsError(t *testing.T) {
	agencyID := 1
	inactive := agency.Agency{ID: agencyID, Status: agency.StatusInactive, CreatedAt: time.Now()}
	svc := user.NewCreator(repoOK{}, fakeAgencyRepo{agency: &inactive}, stubHasher{}, &inMemoryBus{})

	_, err := svc.Create(context.Background(), user.User{
		Email: "agent@example.com", Password: "secret", AgencyID: agencyID,
	})

	assert.ErrorIs(t, err, agency.ErrInactive)
}

func TestUserCreator_ActiveAgency_Succeeds(t *testing.T) {
	agencyID := 1
	active := agency.Agency{ID: agencyID, Status: agency.StatusActive, CreatedAt: time.Now()}
	bus := &inMemoryBus{}
	svc := user.NewCreator(repoOK{}, fakeAgencyRepo{agency: &active}, stubHasher{}, bus)

	id, err := svc.Create(context.Background(), user.User{
		Email: "agent@example.com", Password: "secret", AgencyID: agencyID,
	})

	require.NoError(t, err)
	assert.Equal(t, 42, id)
	assert.Len(t, bus.events, 1)
}
