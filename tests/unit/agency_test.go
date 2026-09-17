package unit_test

import (
	"api/internal/domain/agency"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestAgency_IsActive_StatusAndSoftDeleteTruthTable(t *testing.T) {
	past := time.Now().Add(-time.Hour)

	cases := []struct {
		name      string
		status    agency.Status
		deletedAt *time.Time
		want      bool
	}{
		{name: "active, not deleted", status: agency.StatusActive, deletedAt: nil, want: true},
		{name: "inactive, not deleted", status: agency.StatusInactive, deletedAt: nil, want: false},
		{name: "active, but soft-deleted", status: agency.StatusActive, deletedAt: &past, want: false},
		{name: "inactive, soft-deleted", status: agency.StatusInactive, deletedAt: &past, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := agency.Agency{Status: tc.status, DeletedAt: tc.deletedAt}
			assert.Equal(t, tc.want, a.IsActive())
		})
	}
}
