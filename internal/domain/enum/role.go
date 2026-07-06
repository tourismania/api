// Package enum contains domain enumerations.
package enum

// Role is the domain role identifier. It mirrors Symfony's role naming
// (ROLE_* prefix) so persisted values are directly portable from the
// original PHP project.
type Role string

const (
	RoleUser       Role = "ROLE_USER"
	RoleSuperAdmin Role = "ROLE_SUPER_ADMIN"
	// RoleAgent identifies a travel agent — belongs to exactly one agency
	// (User.AgencyID) and owns the offers created under it.
	RoleAgent Role = "ROLE_AGENT"
)

// String implements fmt.Stringer.
func (r Role) String() string { return string(r) }
