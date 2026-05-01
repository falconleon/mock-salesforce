package middleware

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"
)

// SessionClaims is the payload of an HS256 JWT used for the UI session
// cookie. The claim set matches the standard JWT names where applicable
// (sub, iat, exp) plus name/email for convenience.
type SessionClaims struct {
	Sub   string `json:"sub"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Iat   int64  `json:"iat"`
	Exp   int64  `json:"exp"`
}

// jwtHeader is the fixed JOSE header for HS256.
var jwtHeader = []byte(`{"alg":"HS256","typ":"JWT"}`)

// MintSessionJWT serializes the given claims into a signed HS256 JWT
// using secret as the HMAC key.
func MintSessionJWT(c SessionClaims, secret string) (string, error) {
	payload, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	h := base64.RawURLEncoding.EncodeToString(jwtHeader)
	p := base64.RawURLEncoding.EncodeToString(payload)
	signing := h + "." + p
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return signing + "." + sig, nil
}

// ParseSessionJWT verifies signature, header alg, and exp on token.
// Returns the decoded claims on success.
func ParseSessionJWT(token, secret string) (*SessionClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("jwt: malformed token")
	}
	signing := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(signing))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(parts[2])) {
		return nil, errors.New("jwt: bad signature")
	}
	hdrBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return nil, errors.New("jwt: bad header encoding")
	}
	var hdr struct {
		Alg string `json:"alg"`
		Typ string `json:"typ"`
	}
	if err := json.Unmarshal(hdrBytes, &hdr); err != nil {
		return nil, errors.New("jwt: bad header json")
	}
	if hdr.Alg != "HS256" {
		return nil, errors.New("jwt: unsupported alg")
	}
	payloadBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, errors.New("jwt: bad payload encoding")
	}
	var c SessionClaims
	if err := json.Unmarshal(payloadBytes, &c); err != nil {
		return nil, errors.New("jwt: bad payload json")
	}
	if c.Exp != 0 && time.Now().Unix() > c.Exp {
		return nil, errors.New("jwt: expired")
	}
	return &c, nil
}
