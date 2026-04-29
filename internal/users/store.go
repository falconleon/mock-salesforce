// Package users provides runtime user CRUD and per-user bearer token
// issuance/revocation for the mock Salesforce server. It augments the
// build-time MOCK_USERS env var so testers can mint API tokens without
// going through the OAuth password grant.
package users

import (
	"errors"
	"time"
)

// Common errors.
var (
	ErrNotFound       = errors.New("user not found")
	ErrTokenNotFound  = errors.New("token not found")
	ErrUsernameTaken  = errors.New("username already exists")
	ErrEmptyUsername  = errors.New("username required")
	ErrEmptyPassword  = errors.New("password required")
)

// User is a runtime-mutable mock Salesforce user.
type User struct {
	ID           string    `json:"id"`
	Username     string    `json:"username"`
	Name         string    `json:"name"`
	Email        string    `json:"email"`
	PasswordHash string    `json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// UserPatch is the set of fields that can be updated on a user. Nil
// fields are left unchanged.
type UserPatch struct {
	Username *string
	Name     *string
	Email    *string
	Password *string
}

// Token is an active per-user bearer token. The plaintext value is
// returned exactly once at mint time; subsequent reads only surface
// metadata.
type Token struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Token      string    `json:"-"`
	Label      string    `json:"label"`
	CreatedAt  time.Time `json:"created_at"`
	LastUsedAt time.Time `json:"last_used_at,omitempty"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
}

// Store is the persistence interface for users and their tokens.
type Store interface {
	ListUsers() ([]User, error)
	GetUser(id string) (User, error)
	GetUserByUsername(username string) (User, error)
	CreateUser(username, name, email, password string) (User, error)
	UpdateUser(id string, patch UserPatch) (User, error)
	DeleteUser(id string) error

	ListTokens(userID string) ([]Token, error)
	CreateToken(userID, label string, ttl time.Duration) (Token, error)
	GetToken(userID, tokenID string) (Token, error)
	DeleteToken(userID, tokenID string) error

	// AllTokens returns every active token across all users — used at
	// startup to re-register tokens with the OAuth Bearer validator.
	AllTokens() ([]Token, error)
}

// VerifyPassword reports whether the plaintext password matches the
// stored hash for the given username. It is a convenience helper that
// works against any Store implementation.
func VerifyPassword(s Store, username, password string) (User, bool) {
	u, err := s.GetUserByUsername(username)
	if err != nil {
		return User{}, false
	}
	if u.PasswordHash == hashPassword(password) {
		return u, true
	}
	return User{}, false
}
