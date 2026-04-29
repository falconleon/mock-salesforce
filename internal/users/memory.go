package users

import (
	"sort"
	"sync"
	"time"
)

// MemoryStore is an in-memory implementation of Store.
type MemoryStore struct {
	mu     sync.RWMutex
	users  map[string]*User      // id -> user
	tokens map[string][]*Token   // userID -> tokens
}

// NewMemoryStore returns an empty in-memory user store.
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		users:  make(map[string]*User),
		tokens: make(map[string][]*Token),
	}
}

// ListUsers returns all users sorted by username.
func (s *MemoryStore) ListUsers() ([]User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]User, 0, len(s.users))
	for _, u := range s.users {
		out = append(out, *u)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out, nil
}

// GetUser fetches a user by ID.
func (s *MemoryStore) GetUser(id string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return *u, nil
}

// GetUserByUsername looks up a user by their unique username.
func (s *MemoryStore) GetUserByUsername(username string) (User, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, u := range s.users {
		if u.Username == username {
			return *u, nil
		}
	}
	return User{}, ErrNotFound
}

// CreateUser inserts a new user. Returns ErrUsernameTaken if the
// username already exists.
func (s *MemoryStore) CreateUser(username, name, email, password string) (User, error) {
	if username == "" {
		return User{}, ErrEmptyUsername
	}
	if password == "" {
		return User{}, ErrEmptyPassword
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, u := range s.users {
		if u.Username == username {
			return User{}, ErrUsernameTaken
		}
	}
	now := time.Now().UTC()
	u := &User{
		ID:           newID("usr"),
		Username:     username,
		Name:         name,
		Email:        email,
		PasswordHash: hashPassword(password),
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	s.users[u.ID] = u
	return *u, nil
}

// UpdateUser applies a patch to an existing user.
func (s *MemoryStore) UpdateUser(id string, patch UserPatch) (User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	u, ok := s.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	if patch.Username != nil && *patch.Username != u.Username {
		for _, other := range s.users {
			if other.ID != id && other.Username == *patch.Username {
				return User{}, ErrUsernameTaken
			}
		}
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
	return *u, nil
}

// DeleteUser removes a user and all their tokens.
func (s *MemoryStore) DeleteUser(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[id]; !ok {
		return ErrNotFound
	}
	delete(s.users, id)
	delete(s.tokens, id)
	return nil
}

// ListTokens returns all tokens for a user.
func (s *MemoryStore) ListTokens(userID string) ([]Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if _, ok := s.users[userID]; !ok {
		return nil, ErrNotFound
	}
	toks := s.tokens[userID]
	out := make([]Token, len(toks))
	for i, t := range toks {
		out[i] = *t
	}
	return out, nil
}


// CreateToken mints a new bearer token for a user.
func (s *MemoryStore) CreateToken(userID, label string, ttl time.Duration) (Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.users[userID]; !ok {
		return Token{}, ErrNotFound
	}
	now := time.Now().UTC()
	t := &Token{
		ID:        newID("tok"),
		UserID:    userID,
		Token:     newToken(),
		Label:     label,
		CreatedAt: now,
	}
	if ttl > 0 {
		t.ExpiresAt = now.Add(ttl)
	}
	s.tokens[userID] = append(s.tokens[userID], t)
	return *t, nil
}

// GetToken returns metadata for a single token.
func (s *MemoryStore) GetToken(userID, tokenID string) (Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, t := range s.tokens[userID] {
		if t.ID == tokenID {
			return *t, nil
		}
	}
	return Token{}, ErrTokenNotFound
}

// DeleteToken removes a token from a user's token list.
func (s *MemoryStore) DeleteToken(userID, tokenID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	toks := s.tokens[userID]
	for i, t := range toks {
		if t.ID == tokenID {
			s.tokens[userID] = append(toks[:i], toks[i+1:]...)
			return nil
		}
	}
	return ErrTokenNotFound
}

// AllTokens returns every token across all users.
func (s *MemoryStore) AllTokens() ([]Token, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []Token
	for _, toks := range s.tokens {
		for _, t := range toks {
			out = append(out, *t)
		}
	}
	return out, nil
}
