package agency

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// ErrNotFound is returned when an agency lookup finds no matching
// non-deleted row.
var ErrNotFound = errors.New("agency not found")

// ErrInactive is returned when an operation requires an active
// agency but the agency's status is inactive.
var ErrInactive = errors.New("agency is inactive")

// Manager orchestrates agency lifecycle: creation and deactivation.
// It backs the `agency create`/`agency deactivate` CLI commands — there
// is no HTTP CRUD for agencies in this iteration.
type Manager struct {
	agencies Repository
}

// NewManager wires the collaborator.
func NewManager(agencies Repository) *Manager {
	return &Manager{agencies: agencies}
}

// Create persists a new agency with a generated UUID, active status and
// the current timestamp.
func (m *Manager) Create(ctx context.Context, name string) (Agency, error) {
	if name == "" {
		return Agency{}, errors.New("agency name is required")
	}

	a := Agency{
		UUID:      uuid.New(),
		Name:      name,
		Status:    StatusActive,
		CreatedAt: time.Now(),
	}

	id, err := m.agencies.Store(ctx, a)
	if err != nil {
		return Agency{}, fmt.Errorf("store agency: %w", err)
	}
	a.ID = id
	return a, nil
}

// Deactivate transitions an agency to the inactive status.
func (m *Manager) Deactivate(ctx context.Context, id int) error {
	return m.setStatus(ctx, id, StatusInactive)
}

// Activate transitions an agency back to the active status.
func (m *Manager) Activate(ctx context.Context, id int) error {
	return m.setStatus(ctx, id, StatusActive)
}

func (m *Manager) setStatus(ctx context.Context, id int, status Status) error {
	exists, err := m.agencies.Exists(ctx, id)
	if err != nil {
		return fmt.Errorf("check agency existence: %w", err)
	}
	if !exists {
		return ErrNotFound
	}

	if err := m.agencies.SetStatus(ctx, id, status); err != nil {
		return fmt.Errorf("set agency status: %w", err)
	}
	return nil
}
