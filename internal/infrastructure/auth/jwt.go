// Package auth wraps RSA-backed JWT issue/verify so the rest of the app
// never touches go-jwt directly.
package auth

import (
	"crypto/rsa"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Claims Only immutable identifiers are embedded;
// mutable profile data (roles, phone, name) is fetched from the DB on
// each request so stale tokens can never carry outdated permissions.
type Claims struct {
	jwt.RegisteredClaims
}

// Service signs and validates JWT tokens using an RS256 keypair loaded
// from disk. Passphrase, if non-empty, is used to decrypt the private
// key (PKCS#8 PEM with encryption header).
type Service struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
	ttl        time.Duration
}

// NewService loads the keypair from disk. The public key is mandatory
// (verify path); the private key is optional (verify-only deployments).
func NewService(privateKeyPath, publicKeyPath, passphrase string, ttl time.Duration) (*Service, error) {
	pubBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("read public key: %w", err)
	}
	pub, err := jwt.ParseRSAPublicKeyFromPEM(pubBytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	s := &Service{publicKey: pub, ttl: ttl}

	if privateKeyPath != "" {
		privBytes, err := os.ReadFile(privateKeyPath)
		if err != nil {
			return nil, fmt.Errorf("read private key: %w", err)
		}
		var priv *rsa.PrivateKey
		if passphrase != "" {
			priv, err = jwt.ParseRSAPrivateKeyFromPEMWithPassword(privBytes, passphrase)
		} else {
			priv, err = jwt.ParseRSAPrivateKeyFromPEM(privBytes)
		}
		if err != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		s.privateKey = priv
	}
	return s, nil
}

// Issue signs a token for the given user.
func (s *Service) Issue(uuid uuid.UUID) (string, error) {
	if s.privateKey == nil {
		return "", errors.New("jwt: private key not configured")
	}
	now := time.Now()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.ttl)),
			Subject:   uuid.String(),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	return token.SignedString(s.privateKey)
}

// ErrInvalidToken is returned by Verify when the token fails any
// validation step (signature, expiry, algorithm).
var ErrInvalidToken = errors.New("invalid token")

// Verify parses and validates a raw token string.
func (s *Service) Verify(raw string) (*Claims, error) {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Alg()}))
	var claims Claims
	tok, err := parser.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		return s.publicKey, nil
	})
	if err != nil || !tok.Valid {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}
	return &claims, nil
}
