package handlers

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// authCodeTTL is the lifetime of an issued authorization code per
// RFC 6749 §4.1.2 ("a maximum authorization code lifetime of 10
// minutes is RECOMMENDED").
const authCodeTTL = 10 * time.Minute

// AuthCode is the persisted record for a one-time OAuth authorization
// code. All fields are populated at issue time and consumed exactly
// once by the token endpoint when exchanging the code.
type AuthCode struct {
	Code                string
	ClientID            string
	RedirectURI         string
	Scope               string
	CodeChallenge       string
	CodeChallengeMethod string
	Username            string
	IssuedAt            time.Time
	ExpiresAt           time.Time
	Used                bool
}

// AuthCodeStore is the in-memory registry of outstanding authorization
// codes. Safe for concurrent use.
type AuthCodeStore struct {
	mu    sync.Mutex
	codes map[string]*AuthCode
}

// NewAuthCodeStore returns an empty AuthCodeStore.
func NewAuthCodeStore() *AuthCodeStore {
	return &AuthCodeStore{codes: make(map[string]*AuthCode)}
}

// Issue mints a fresh authorization code, registers it, and returns it.
// The returned pointer references the stored entry; callers must not
// mutate it.
func (s *AuthCodeStore) Issue(clientID, redirectURI, scope, codeChallenge, codeChallengeMethod, username string) *AuthCode {
	now := time.Now()
	ac := &AuthCode{
		Code:                generateAuthCode(),
		ClientID:            clientID,
		RedirectURI:         redirectURI,
		Scope:               scope,
		CodeChallenge:       codeChallenge,
		CodeChallengeMethod: codeChallengeMethod,
		Username:            username,
		IssuedAt:            now,
		ExpiresAt:           now.Add(authCodeTTL),
	}
	s.mu.Lock()
	s.codes[ac.Code] = ac
	s.mu.Unlock()
	return ac
}

// Consume atomically marks the code as used and returns a copy of the
// stored entry. Returns nil if the code is unknown, expired, or has
// already been consumed.
func (s *AuthCodeStore) Consume(code string) *AuthCode {
	s.mu.Lock()
	defer s.mu.Unlock()
	ac, ok := s.codes[code]
	if !ok {
		return nil
	}
	if ac.Used {
		return nil
	}
	if time.Now().After(ac.ExpiresAt) {
		return nil
	}
	ac.Used = true
	cp := *ac
	return &cp
}

// Lookup returns a copy of the code entry without consuming it. Useful
// for tests and diagnostic checks. Returns nil if the code is unknown.
func (s *AuthCodeStore) Lookup(code string) *AuthCode {
	s.mu.Lock()
	defer s.mu.Unlock()
	ac, ok := s.codes[code]
	if !ok {
		return nil
	}
	cp := *ac
	return &cp
}

// generateAuthCode produces a 256-bit URL-safe random code.
func generateAuthCode() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
