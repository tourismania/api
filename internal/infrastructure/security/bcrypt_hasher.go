// Package security contains adapters for the domain's PasswordHasher
// interface. Keeping bcrypt out of the domain lets us swap algorithms
// (argon2id, etc.) without touching business logic.
package security

import (
	"errors"

	"golang.org/x/crypto/bcrypt"
)

// BcryptHasher implements domain user.PasswordHasher using bcrypt with
// a configurable cost. Match Symfony's default cost (12) for behavioral
// parity with the PHP project.
type BcryptHasher struct {
	cost int
}

// NewBcryptHasher constructs a hasher with the given cost. A cost below
// bcrypt.MinCost falls back to the default.
func NewBcryptHasher(cost int) *BcryptHasher {
	if cost < bcrypt.MinCost {
		cost = bcrypt.DefaultCost
	}
	return &BcryptHasher{cost: cost}
}

// Hash produces a bcrypt hash for the supplied plain-text password.
func (h *BcryptHasher) Hash(password string) (string, error) {
	if password == "" {
		return "", errors.New("password must not be empty")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), h.cost)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// Verify checks a plain-text password against a stored hash. Returns nil
// on success; bcrypt.ErrMismatchedHashAndPassword on mismatch.
func (h *BcryptHasher) Verify(hashed, plain string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(plain))
}
