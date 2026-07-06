package unit_test

import (
	"testing"
	"time"

	"api/internal/domain/entity"
	"api/internal/domain/enum"

	"github.com/stretchr/testify/assert"
)

func TestAgency_IsActive_StatusAndSoftDeleteTruthTable(t *testing.T) {
	past := time.Now().Add(-time.Hour)

	cases := []struct {
		name      string
		status    enum.AgencyStatus
		deletedAt *time.Time
		want      bool
	}{
		{name: "active, not deleted", status: enum.AgencyStatusActive, deletedAt: nil, want: true},
		{name: "inactive, not deleted", status: enum.AgencyStatusInactive, deletedAt: nil, want: false},
		{name: "active, but soft-deleted", status: enum.AgencyStatusActive, deletedAt: &past, want: false},
		{name: "inactive, soft-deleted", status: enum.AgencyStatusInactive, deletedAt: &past, want: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			a := entity.Agency{Status: tc.status, DeletedAt: tc.deletedAt}
			assert.Equal(t, tc.want, a.IsActive())
		})
	}
}
