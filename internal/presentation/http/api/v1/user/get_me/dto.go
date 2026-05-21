// Package getmehttp is the HTTP boundary for the GetMe query.
package getmehttp

import "github.com/google/uuid"

// GetMeDto is the transport view of the authenticated user, populated by
// the resolver from JWT claims. Only the immutable identity is carried
// here; mutable data comes from the DB inside the use-case.
type GetMeDto struct {
	Uuid uuid.UUID
}

// GetMeResponse is what we serialise back to the client.
type GetMeResponse struct {
	UUID      uuid.UUID `json:"uid"`
	Email     string    `json:"email"`
	Phone     string    `json:"phone,omitempty"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Rights    Rights    `json:"rights"`
}

// Rights is the public projection of valueobject.RightsDescribe.
type Rights struct {
	IsSuperAdmin bool `json:"is_super_admin"`
}
