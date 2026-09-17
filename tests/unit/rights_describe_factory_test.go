package unit_test

import (
	"api/internal/domain/user"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Mirrors the original PHP unit test: roles → IsSuperAdmin truth table.
func TestRightsDescribeFactory_ByRoles(t *testing.T) {
	f := user.NewRightsDescribeFactory()

	cases := []struct {
		name  string
		roles []string
		want  bool
	}{
		{name: "empty", roles: []string{}, want: false},
		{name: "only super admin", roles: []string{string(user.RoleSuperAdmin)}, want: true},
		{name: "only user", roles: []string{string(user.RoleUser)}, want: false},
		{name: "mixed includes super", roles: []string{string(user.RoleUser), string(user.RoleSuperAdmin)}, want: true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := f.ByRoles(tc.roles)
			assert.Equal(t, tc.want, got.IsSuperAdmin)
		})
	}
}
