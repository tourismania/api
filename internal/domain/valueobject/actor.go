package valueobject

import "api/internal/domain/enum"

// Actor is the identity of the user performing a domain operation. It
// belongs to entity.User's identity, not to any single Manager: every
// domain service that needs to know "who is doing this" (not just
// offers) resolves and passes the same Actor. Callers resolve it once
// per request from the DB, keyed by the immutable uuid carried in the
// JWT — no HTTP/JWT knowledge leaks into this type itself.
//
// AgencyID is required: every user belongs to exactly one agency (1
// user = 1 agency, enforced at the database level) — never optional.
type Actor struct {
	UserID   int
	AgencyID int
	Roles    []enum.Role
}

// HasRole reports whether the actor carries the given role.
func (a Actor) HasRole(role enum.Role) bool {
	for _, r := range a.Roles {
		if r == role {
			return true
		}
	}
	return false
}
