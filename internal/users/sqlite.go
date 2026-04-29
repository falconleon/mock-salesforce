package users

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

// SQLiteStore is a SQLite-backed Store. Schema is auto-created on
// first open; safe to point at an existing DB file as long as the
// `users` and `user_tokens` table names aren't taken.
type SQLiteStore struct {
	mu sync.RWMutex
	db *sql.DB
}

const userSchema = `
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    email TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS user_tokens (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    label TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    last_used_at INTEGER NOT NULL DEFAULT 0,
    expires_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_user_tokens_user_id ON user_tokens(user_id);
`

// NewSQLiteStore opens (or creates) a SQLite-backed user store at the
// given path. Empty path uses a shared in-memory DB.
func NewSQLiteStore(dbPath string) (*SQLiteStore, error) {
	dsn := dbPath
	if dbPath == "" {
		dsn = "file::memory:?cache=shared"
	} else {
		dir := filepath.Dir(dbPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, fmt.Errorf("create db directory: %w", err)
		}
		dsn = dbPath + "?_foreign_keys=on&_journal_mode=WAL"
	}
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("open users db: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping users db: %w", err)
	}
	if _, err := db.Exec(userSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init users schema: %w", err)
	}
	return &SQLiteStore{db: db}, nil
}

// Close releases the underlying database handle.
func (s *SQLiteStore) Close() error { return s.db.Close() }

func scanUser(row interface{ Scan(...any) error }) (User, error) {
	var u User
	var created, updated int64
	if err := row.Scan(&u.ID, &u.Username, &u.Name, &u.Email, &u.PasswordHash, &created, &updated); err != nil {
		return User{}, err
	}
	u.CreatedAt = time.Unix(created, 0).UTC()
	u.UpdatedAt = time.Unix(updated, 0).UTC()
	return u, nil
}

// ListUsers returns all users sorted by username.
func (s *SQLiteStore) ListUsers() ([]User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT id, username, name, email, password_hash, created_at, updated_at FROM users ORDER BY username`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out, rows.Err()
}

// GetUser fetches a user by ID.
func (s *SQLiteStore) GetUser(id string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRow(`SELECT id, username, name, email, password_hash, created_at, updated_at FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// GetUserByUsername looks up a user by their unique username.
func (s *SQLiteStore) GetUserByUsername(username string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRow(`SELECT id, username, name, email, password_hash, created_at, updated_at FROM users WHERE username = ?`, username)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

// CreateUser inserts a new user.
func (s *SQLiteStore) CreateUser(username, name, email, password string) (User, error) {
	if username == "" {
		return User{}, ErrEmptyUsername
	}
	if password == "" {
		return User{}, ErrEmptyPassword
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	u := User{
		ID:           newID("usr"),
		Username:     username,
		Name:         name,
		Email:        email,
		PasswordHash: hashPassword(password),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	_, err := s.db.Exec(
		`INSERT INTO users (id, username, name, email, password_hash, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Username, u.Name, u.Email, u.PasswordHash, u.CreatedAt.Unix(), u.UpdatedAt.Unix(),
	)
	if err != nil {
		// SQLite UNIQUE violation surfaces as a constraint error; map to
		// the package-level sentinel so handlers can branch cleanly.
		return User{}, ErrUsernameTaken
	}
	return u, nil
}

// UpdateUser applies the patch to an existing user.
func (s *SQLiteStore) UpdateUser(id string, patch UserPatch) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	row := s.db.QueryRow(`SELECT id, username, name, email, password_hash, created_at, updated_at FROM users WHERE id = ?`, id)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, ErrNotFound
	}
	if err != nil {
		return User{}, err
	}
	if patch.Username != nil {
		u.Username = *patch.Username
	}
	if patch.Name != nil {
		u.Name = *patch.Name
	}
	if patch.Email != nil {
		u.Email = *patch.Email
	}
	if patch.Password != nil && *patch.Password != "" {
		u.PasswordHash = hashPassword(*patch.Password)
	}
	u.UpdatedAt = time.Now().UTC()
	_, err = s.db.Exec(
		`UPDATE users SET username=?, name=?, email=?, password_hash=?, updated_at=? WHERE id=?`,
		u.Username, u.Name, u.Email, u.PasswordHash, u.UpdatedAt.Unix(), u.ID,
	)
	if err != nil {
		return User{}, ErrUsernameTaken
	}
	return u, nil
}

// DeleteUser removes a user; ON DELETE CASCADE drops their tokens.
func (s *SQLiteStore) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`DELETE FROM users WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func scanToken(row interface{ Scan(...any) error }) (Token, error) {
	var t Token
	var created, lastUsed, expires int64
	if err := row.Scan(&t.ID, &t.UserID, &t.Token, &t.Label, &created, &lastUsed, &expires); err != nil {
		return Token{}, err
	}
	t.CreatedAt = time.Unix(created, 0).UTC()
	if lastUsed > 0 {
		t.LastUsedAt = time.Unix(lastUsed, 0).UTC()
	}
	if expires > 0 {
		t.ExpiresAt = time.Unix(expires, 0).UTC()
	}
	return t, nil
}

// ListTokens returns all tokens for a user.
func (s *SQLiteStore) ListTokens(userID string) ([]Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, err := s.userExists(userID); err != nil {
		return nil, err
	}
	rows, err := s.db.Query(`SELECT id, user_id, token, label, created_at, last_used_at, expires_at FROM user_tokens WHERE user_id = ? ORDER BY created_at`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Token
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (s *SQLiteStore) userExists(id string) (bool, error) {
	var found string
	err := s.db.QueryRow(`SELECT id FROM users WHERE id = ?`, id).Scan(&found)
	if errors.Is(err, sql.ErrNoRows) {
		return false, ErrNotFound
	}
	return err == nil, err
}


// CreateToken mints a new bearer token for a user.
func (s *SQLiteStore) CreateToken(userID, label string, ttl time.Duration) (Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, err := s.userExists(userID); err != nil {
		return Token{}, err
	}
	now := time.Now().UTC()
	t := Token{
		ID:        newID("tok"),
		UserID:    userID,
		Token:     newToken(),
		Label:     label,
		CreatedAt: now,
	}
	var expiresUnix int64
	if ttl > 0 {
		t.ExpiresAt = now.Add(ttl)
		expiresUnix = t.ExpiresAt.Unix()
	}
	_, err := s.db.Exec(
		`INSERT INTO user_tokens (id, user_id, token, label, created_at, last_used_at, expires_at) VALUES (?, ?, ?, ?, ?, 0, ?)`,
		t.ID, t.UserID, t.Token, t.Label, t.CreatedAt.Unix(), expiresUnix,
	)
	if err != nil {
		return Token{}, err
	}
	return t, nil
}

// GetToken returns metadata for a single token.
func (s *SQLiteStore) GetToken(userID, tokenID string) (Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	row := s.db.QueryRow(`SELECT id, user_id, token, label, created_at, last_used_at, expires_at FROM user_tokens WHERE user_id = ? AND id = ?`, userID, tokenID)
	t, err := scanToken(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Token{}, ErrTokenNotFound
	}
	return t, err
}

// DeleteToken removes a token from a user's token list.
func (s *SQLiteStore) DeleteToken(userID, tokenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.Exec(`DELETE FROM user_tokens WHERE user_id = ? AND id = ?`, userID, tokenID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrTokenNotFound
	}
	return nil
}

// AllTokens returns every token across all users.
func (s *SQLiteStore) AllTokens() ([]Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	rows, err := s.db.Query(`SELECT id, user_id, token, label, created_at, last_used_at, expires_at FROM user_tokens`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Token
	for rows.Next() {
		t, err := scanToken(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}
