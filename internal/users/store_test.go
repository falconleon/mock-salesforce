package users_test

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/falconleon/mock-salesforce/internal/users"
)

// makeStores returns the two backends side-by-side so each behavioural
// test runs against both — guards against silent divergence.
func makeStores(t *testing.T) []struct {
	name  string
	store users.Store
} {
	t.Helper()
	mem := users.NewMemoryStore()
	dbPath := filepath.Join(t.TempDir(), "users.db")
	sqliteStore, err := users.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	t.Cleanup(func() { sqliteStore.Close() })
	return []struct {
		name  string
		store users.Store
	}{
		{"memory", mem},
		{"sqlite", sqliteStore},
	}
}

func TestStore_CRUD(t *testing.T) {
	for _, c := range makeStores(t) {
		t.Run(c.name, func(t *testing.T) {
			s := c.store

			u, err := s.CreateUser("alice@x.com", "Alice", "alice@x.com", "pw1")
			if err != nil {
				t.Fatalf("CreateUser: %v", err)
			}
			if u.ID == "" || u.Username != "alice@x.com" {
				t.Errorf("unexpected user: %+v", u)
			}
			if u.PasswordHash == "" || u.PasswordHash == "pw1" {
				t.Errorf("password should be hashed, got %q", u.PasswordHash)
			}

			got, err := s.GetUser(u.ID)
			if err != nil || got.Username != u.Username {
				t.Errorf("GetUser: %+v err=%v", got, err)
			}
			byName, err := s.GetUserByUsername("alice@x.com")
			if err != nil || byName.ID != u.ID {
				t.Errorf("GetUserByUsername: %+v err=%v", byName, err)
			}

			if _, err := s.CreateUser("alice@x.com", "", "", "x"); !errors.Is(err, users.ErrUsernameTaken) {
				t.Errorf("expected ErrUsernameTaken, got %v", err)
			}

			newName := "Alice A."
			updated, err := s.UpdateUser(u.ID, users.UserPatch{Name: &newName})
			if err != nil || updated.Name != newName {
				t.Errorf("UpdateUser: %+v err=%v", updated, err)
			}

			list, err := s.ListUsers()
			if err != nil || len(list) != 1 {
				t.Errorf("ListUsers: %d err=%v", len(list), err)
			}

			if err := s.DeleteUser(u.ID); err != nil {
				t.Errorf("DeleteUser: %v", err)
			}
			if _, err := s.GetUser(u.ID); !errors.Is(err, users.ErrNotFound) {
				t.Errorf("expected ErrNotFound after delete, got %v", err)
			}
		})
	}
}

func TestStore_VerifyPassword(t *testing.T) {
	for _, c := range makeStores(t) {
		t.Run(c.name, func(t *testing.T) {
			s := c.store
			if _, err := s.CreateUser("bob@x.com", "Bob", "bob@x.com", "secret"); err != nil {
				t.Fatalf("seed: %v", err)
			}
			if _, ok := users.VerifyPassword(s, "bob@x.com", "secret"); !ok {
				t.Error("expected password to verify")
			}
			if _, ok := users.VerifyPassword(s, "bob@x.com", "wrong"); ok {
				t.Error("expected wrong password to fail")
			}
			if _, ok := users.VerifyPassword(s, "missing@x.com", "secret"); ok {
				t.Error("expected missing user to fail")
			}
		})
	}
}

func TestStore_Tokens(t *testing.T) {
	for _, c := range makeStores(t) {
		t.Run(c.name, func(t *testing.T) {
			s := c.store
			u, err := s.CreateUser("carol@x.com", "", "", "pw")
			if err != nil {
				t.Fatalf("CreateUser: %v", err)
			}

			tok, err := s.CreateToken(u.ID, "ci-runner", time.Hour)
			if err != nil {
				t.Fatalf("CreateToken: %v", err)
			}
			if tok.Token == "" || tok.ID == "" {
				t.Fatal("token should be populated")
			}
			if tok.ExpiresAt.IsZero() {
				t.Error("expected non-zero ExpiresAt for ttl=1h")
			}

			noTTL, err := s.CreateToken(u.ID, "no-ttl", 0)
			if err != nil {
				t.Fatalf("CreateToken: %v", err)
			}
			if !noTTL.ExpiresAt.IsZero() {
				t.Error("expected zero ExpiresAt for ttl=0")
			}

			toks, err := s.ListTokens(u.ID)
			if err != nil || len(toks) != 2 {
				t.Errorf("ListTokens: %d err=%v", len(toks), err)
			}

			if err := s.DeleteToken(u.ID, tok.ID); err != nil {
				t.Errorf("DeleteToken: %v", err)
			}
			if _, err := s.GetToken(u.ID, tok.ID); !errors.Is(err, users.ErrTokenNotFound) {
				t.Errorf("expected ErrTokenNotFound, got %v", err)
			}
		})
	}
}

func TestStore_DeleteUser_CascadesTokens(t *testing.T) {
	for _, c := range makeStores(t) {
		t.Run(c.name, func(t *testing.T) {
			s := c.store
			u, _ := s.CreateUser("dave@x.com", "", "", "pw")
			s.CreateToken(u.ID, "x", 0)
			if err := s.DeleteUser(u.ID); err != nil {
				t.Fatalf("DeleteUser: %v", err)
			}
			if _, err := s.ListTokens(u.ID); !errors.Is(err, users.ErrNotFound) {
				t.Errorf("expected user-not-found after cascade, got %v", err)
			}
		})
	}
}

func TestSQLiteStore_PersistsAcrossOpen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "persist.db")
	s1, err := users.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore #1: %v", err)
	}
	u, err := s1.CreateUser("eve@x.com", "Eve", "eve@x.com", "pw")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	tok, err := s1.CreateToken(u.ID, "persistent", 0)
	if err != nil {
		t.Fatalf("CreateToken: %v", err)
	}
	s1.Close()

	s2, err := users.NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore #2: %v", err)
	}
	defer s2.Close()

	got, err := s2.GetUserByUsername("eve@x.com")
	if err != nil || got.ID != u.ID {
		t.Errorf("expected persisted user, got %+v err=%v", got, err)
	}
	all, err := s2.AllTokens()
	if err != nil || len(all) != 1 || all[0].ID != tok.ID {
		t.Errorf("expected persisted token %q, got %+v err=%v", tok.ID, all, err)
	}
}

func TestSeedFromMap_SkipsExisting(t *testing.T) {
	s := users.NewMemoryStore()
	if err := users.SeedFromMap(s, map[string]string{
		"a@x.com": "p1",
		"b@x.com": "p2",
	}); err != nil {
		t.Fatalf("seed: %v", err)
	}
	list, _ := s.ListUsers()
	if len(list) != 2 {
		t.Fatalf("expected 2 users, got %d", len(list))
	}
	// Re-seeding should not duplicate or error.
	if err := users.SeedFromMap(s, map[string]string{"a@x.com": "p1"}); err != nil {
		t.Fatalf("re-seed: %v", err)
	}
	list2, _ := s.ListUsers()
	if len(list2) != 2 {
		t.Errorf("re-seed should be idempotent, got %d users", len(list2))
	}
}
