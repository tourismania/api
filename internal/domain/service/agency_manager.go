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

// ErrAgencyNotFound is returned when an agency lookup finds no matching
// non-deleted row.
var ErrAgencyNotFound = errors.New("agency not found")

// ErrAgencyInactive is returned when an operation requires an active
// agency but the agency's status is inactive.
var ErrAgencyInactive = errors.New("agency is inactive")

// AgencyManager orchestrates agency lifecycle: creation and deactivation.
// It backs the `agency create`/`agency deactivate` CLI commands — there
// is no HTTP CRUD for agencies in this iteration.
type AgencyManager struct {
	agencies repository.AgencyRepository
}

// NewAgencyManager wires the collaborator.
func NewAgencyManager(agencies repository.AgencyRepository) *AgencyManager {
	return &AgencyManager{agencies: agencies}
}

// Create persists a new agency with a generated UUID, active status and
// the current timestamp.
func (m *AgencyManager) Create(ctx context.Context, name string) (entity.Agency, error) {
	if name == "" {
		return entity.Agency{}, errors.New("agency name is required")
	}

	agency := entity.Agency{
		UUID:      uuid.New(),
		Name:      name,
		Status:    enum.AgencyStatusActive,
		CreatedAt: time.Now(),
	}

	id, err := m.agencies.Store(ctx, agency)
	if err != nil {
		return entity.Agency{}, fmt.Errorf("store agency: %w", err)
	}
	agency.ID = id
	return agency, nil
}

// Deactivate transitions an agency to the inactive status.
func (m *AgencyManager) Deactivate(ctx context.Context, id int) error {
	return m.setStatus(ctx, id, enum.AgencyStatusInactive)
}

// Activate transitions an agency back to the active status.
func (m *AgencyManager) Activate(ctx context.Context, id int) error {
	return m.setStatus(ctx, id, enum.AgencyStatusActive)
}

func (m *AgencyManager) setStatus(ctx context.Context, id int, status enum.AgencyStatus) error {
	exists, err := m.agencies.Exists(ctx, id)
	if err != nil {
		return fmt.Errorf("check agency existence: %w", err)
	}
	if !exists {
		return ErrAgencyNotFound
	}

	if err := m.agencies.SetStatus(ctx, id, status); err != nil {
		return fmt.Errorf("set agency status: %w", err)
	}
	return nil
}
