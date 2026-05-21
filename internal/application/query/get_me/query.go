// Package getme contains the "current user profile" read-side use case.
package getme

import "github.com/google/uuid"

// Query carries only the immutable identity token provided by the JWT.
// All mutable profile data is fetched from the DB inside the handler.
type Query struct {
	Uuid uuid.UUID
}
