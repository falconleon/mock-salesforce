package handlers_test

import (
	"testing"
	"time"

	"github.com/falconleon/mock-salesforce/internal/handlers"
)

func TestAuthCodeStore_IssueAndConsume(t *testing.T) {
	s := handlers.NewAuthCodeStore()
	ac := s.Issue("client", "https://app/cb", "api", "challenge", "S256", "demo@falcon.local")
	if ac.Code == "" {
		t.Fatal("Issue returned empty code")
	}
	if ac.ExpiresAt.Sub(ac.IssuedAt) < 9*time.Minute || ac.ExpiresAt.Sub(ac.IssuedAt) > 11*time.Minute {
		t.Errorf("expected ~10m TTL, got %v", ac.ExpiresAt.Sub(ac.IssuedAt))
	}

	got := s.Consume(ac.Code)
	if got == nil {
		t.Fatal("first Consume returned nil")
	}
	if got.ClientID != "client" || got.RedirectURI != "https://app/cb" ||
		got.CodeChallenge != "challenge" || got.CodeChallengeMethod != "S256" ||
		got.Username != "demo@falcon.local" || got.Scope != "api" {
		t.Errorf("Consume returned unexpected payload: %+v", got)
	}
}

func TestAuthCodeStore_SingleUse(t *testing.T) {
	s := handlers.NewAuthCodeStore()
	ac := s.Issue("client", "https://app/cb", "", "c", "S256", "u")
	if s.Consume(ac.Code) == nil {
		t.Fatal("first Consume returned nil")
	}
	if s.Consume(ac.Code) != nil {
		t.Error("second Consume of same code must return nil (single-use)")
	}
}

func TestAuthCodeStore_UnknownCode(t *testing.T) {
	s := handlers.NewAuthCodeStore()
	if s.Consume("does-not-exist") != nil {
		t.Error("Consume of unknown code must return nil")
	}
	if s.Lookup("does-not-exist") != nil {
		t.Error("Lookup of unknown code must return nil")
	}
}

func TestAuthCodeStore_LookupDoesNotConsume(t *testing.T) {
	s := handlers.NewAuthCodeStore()
	ac := s.Issue("client", "https://app/cb", "", "c", "S256", "u")
	if got := s.Lookup(ac.Code); got == nil || got.Used {
		t.Fatalf("Lookup should return unused entry, got %+v", got)
	}
	if s.Consume(ac.Code) == nil {
		t.Error("Consume after Lookup must still succeed")
	}
}
