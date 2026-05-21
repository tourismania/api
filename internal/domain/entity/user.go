// Package entity contains pure domain entities — no ORM or transport tags.
package entity

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
	// past UserCreator.
	Password string
}

// UserRecord is a read-model returned by the persistence layer. It is
// intentionally separate from User (the write aggregate) to keep the
// read and write paths independent.
type UserRecord struct {
	Uuid      uuid.UUID
	Email     string
	Phone     string
	FirstName string
	LastName  string
	Roles     []string
}
