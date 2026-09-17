package user

// RightsDescribe is a Value Object describing the privilege flags derived
// from a user's role set. Always constructed via RightsDescribeFactory;
// consumers must not mutate fields directly.
type RightsDescribe struct {
	IsSuperAdmin bool
}

// Equals returns true when both VOs carry the same privilege bits.
func (r RightsDescribe) Equals(other RightsDescribe) bool {
	return r.IsSuperAdmin == other.IsSuperAdmin
}
