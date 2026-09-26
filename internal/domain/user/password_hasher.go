package user

// PasswordHasher abstracts the password hashing algorithm so the domain
// layer does not pin itself to bcrypt. The concrete bcrypt-backed
// implementation lives in infrastructure.
type PasswordHasher interface {
	Hash(password string) (string, error)
	Verify(hashed, plain string) error
}
