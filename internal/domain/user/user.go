// Package user is the user/identity aggregate: the User entity, its
// read-model, roles, the acting principal (Actor), rights description
// and the domain services around registration and identity resolution.
// Pure domain — no ORM or transport tags.
package user

import "github.com/google/uuid"

// User is the immutable domain entity. Persistence-layer ORM model
// (infrastructure/persistence/postgres/model.User) is kept separate
// to preserve dependency direction: Domain never knows about storage.
type User struct {
	LastName  string
	FirstName string
	Email     string
	// Password is plain-text at the domain boundary; hashing is delegated
	// to the domain service so the entity itself never carries credentials
	// past Creator.
	Password string
	// AgencyID links the user to exactly one agency (1 user = 1 agency).
	// Required for every user, regardless of role.
	AgencyID int
}

// Record is a read-model returned by the persistence layer. It is
// intentionally separate from User (the write aggregate) to keep the
// read and write paths independent.
type Record struct {
	ID        int
	Uuid      uuid.UUID
	Email     string
	Phone     string
	FirstName string
	LastName  string
	Roles     []string
	AgencyID  int
}
