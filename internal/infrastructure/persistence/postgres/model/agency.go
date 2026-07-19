// Package model holds persistence-only structs. They mirror the database
// schema and are intentionally separate from domain entities.
package model

import (
	"time"

	"github.com/google/uuid"
)

// Agency is the ORM/row representation of the agencies table.
type Agency struct {
	ID        int
	UUID      uuid.UUID
	Name      string
	Status    string
	CreatedAt time.Time
	DeletedAt *time.Time
}
