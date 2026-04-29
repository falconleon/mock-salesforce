package middleware_test

import (
	"strings"
	"testing"
	"time"

	"github.com/falconleon/mock-salesforce/internal/server/middleware"
)

func TestJWT_RoundTrip(t *testing.T) {
	c := middleware.SessionClaims{
		Sub:   "user-1",
		Name:  "Demo User",
		Email: "demo@falcon.local",
		Iat:   time.Now().Unix(),
		Exp:   time.Now().Add(time.Hour).Unix(),
	}
	tok, err := middleware.MintSessionJWT(c, "secret")
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if strings.Count(tok, ".") != 2 {
		t.Errorf("expected 3-segment JWT, got %q", tok)
	}
	got, err := middleware.ParseSessionJWT(tok, "secret")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got.Email != c.Email || got.Sub != c.Sub || got.Name != c.Name {
		t.Errorf("claims mismatch: %+v != %+v", got, c)
	}
}

func TestJWT_BadSignature(t *testing.T) {
	c := middleware.SessionClaims{Email: "x", Exp: time.Now().Add(time.Hour).Unix()}
	tok, _ := middleware.MintSessionJWT(c, "secret")
	if _, err := middleware.ParseSessionJWT(tok, "different-secret"); err == nil {
		t.Error("expected signature mismatch to fail")
	}
}

func TestJWT_Expired(t *testing.T) {
	c := middleware.SessionClaims{Email: "x", Exp: time.Now().Add(-time.Minute).Unix()}
	tok, _ := middleware.MintSessionJWT(c, "secret")
	if _, err := middleware.ParseSessionJWT(tok, "secret"); err == nil {
		t.Error("expected expired token to fail")
	}
}

func TestJWT_Malformed(t *testing.T) {
	if _, err := middleware.ParseSessionJWT("not-a-jwt", "secret"); err == nil {
		t.Error("expected malformed token to fail")
	}
	if _, err := middleware.ParseSessionJWT("a.b", "secret"); err == nil {
		t.Error("expected 2-segment token to fail")
	}
}

func TestJWT_TamperedPayload(t *testing.T) {
	c := middleware.SessionClaims{Email: "x", Exp: time.Now().Add(time.Hour).Unix()}
	tok, _ := middleware.MintSessionJWT(c, "secret")
	parts := strings.Split(tok, ".")
	// Replace payload with a different valid base64 segment but keep old sig.
	parts[1] = "eyJlbWFpbCI6ImhheCJ9"
	if _, err := middleware.ParseSessionJWT(strings.Join(parts, "."), "secret"); err == nil {
		t.Error("expected tampered payload to fail signature check")
	}
}
