package users

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
)

// newID generates an opaque, URL-safe ID with the given prefix.
func newID(prefix string) string {
	var b [12]byte
	if _, err := rand.Read(b[:]); err != nil {
		// rand.Read failure is exceptional; fall back to a static
		// placeholder so callers never see an empty ID.
		return prefix + "_000000000000"
	}
	return prefix + "_" + base64.RawURLEncoding.EncodeToString(b[:])
}

// newToken generates a Salesforce-style opaque bearer token.
func newToken() string {
	var b [24]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "00Dxx0000001gPq!000000000000000000000000"
	}
	return "00Dxx0000001gPq!" + base64.RawURLEncoding.EncodeToString(b[:])
}

// hashPassword returns a deterministic hex-encoded SHA-256 of the
// plaintext password. Matches the lightweight scheme used elsewhere in
// the mock — bcrypt would be overkill for a dev/test fixture.
func hashPassword(pw string) string {
	sum := sha256.Sum256([]byte(pw))
	return hex.EncodeToString(sum[:])
}
