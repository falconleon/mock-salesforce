package handlers

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/internal/users"
)

const (
	scopeAdminUser = "admin user-token"
	adminClientID  = "admin-cli"
)

// AdminUsersHandler exposes runtime user CRUD and per-user bearer
// token issuance/revocation under /admin/users.
//
// All endpoints require the X-Admin-Token header to match the
// configured AdminToken. When AdminToken is empty the handler refuses
// every request with 401 — this is the "default to disabled" mode.
type AdminUsersHandler struct {
	store      users.Store
	adminToken string
	logger     zerolog.Logger
}

// NewAdminUsersHandler constructs an AdminUsersHandler.
func NewAdminUsersHandler(store users.Store, adminToken string, logger zerolog.Logger) *AdminUsersHandler {
	return &AdminUsersHandler{
		store:      store,
		adminToken: adminToken,
		logger:     logger.With().Str("handler", "admin_users").Logger(),
	}
}

// authorize enforces the X-Admin-Token gate. Returns true if the
// request may proceed; otherwise it has already written a 401.
func (h *AdminUsersHandler) authorize(w http.ResponseWriter, r *http.Request) bool {
	if h.adminToken == "" {
		writeAdminError(w, http.StatusUnauthorized, "admin endpoints disabled (set ADMIN_TOKEN)")
		return false
	}
	got := r.Header.Get("X-Admin-Token")
	if subtle.ConstantTimeCompare([]byte(got), []byte(h.adminToken)) != 1 {
		writeAdminError(w, http.StatusUnauthorized, "invalid or missing X-Admin-Token")
		return false
	}
	return true
}

// HandleUsers dispatches GET (list) and POST (create) on /admin/users.
func (h *AdminUsersHandler) HandleUsers(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	switch r.Method {
	case http.MethodGet:
		h.listUsers(w, r)
	case http.MethodPost:
		h.createUser(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleUser dispatches PATCH and DELETE on /admin/users/{id}.
func (h *AdminUsersHandler) HandleUser(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	id := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		u, err := h.store.GetUser(id)
		if err != nil {
			h.writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, u)
	case http.MethodPatch:
		h.updateUser(w, r, id)
	case http.MethodDelete:
		if err := h.store.DeleteUser(id); err != nil {
			h.writeStoreError(w, err)
			return
		}
		// Best-effort: revoke any tokens owned by this user from the
		// in-memory OAuth registry. The store has already removed them
		// from durable state via cascade.
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleTokens dispatches GET (list) and POST (mint) on
// /admin/users/{id}/tokens.
func (h *AdminUsersHandler) HandleTokens(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	id := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		toks, err := h.store.ListTokens(id)
		if err != nil {
			h.writeStoreError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"tokens": toks})
	case http.MethodPost:
		h.mintToken(w, r, id)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleToken handles DELETE on /admin/users/{id}/tokens/{tokenId}.
func (h *AdminUsersHandler) HandleToken(w http.ResponseWriter, r *http.Request) {
	if !h.authorize(w, r) {
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := r.PathValue("id")
	tokenID := r.PathValue("tokenId")
	tok, err := h.store.GetToken(userID, tokenID)
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	if err := h.store.DeleteToken(userID, tokenID); err != nil {
		h.writeStoreError(w, err)
		return
	}
	// Pull from the live OAuth validator so subsequent /services/data
	// calls return 401.
	middleware.RevokeToken(tok.Token)
	w.WriteHeader(http.StatusNoContent)
}


// listUsers writes the JSON-encoded user list (no secrets).
func (h *AdminUsersHandler) listUsers(w http.ResponseWriter, _ *http.Request) {
	all, err := h.store.ListUsers()
	if err != nil {
		writeAdminError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": all})
}

type createUserRequest struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AdminUsersHandler) createUser(w http.ResponseWriter, r *http.Request) {
	var body createUserRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	u, err := h.store.CreateUser(body.Username, body.Name, body.Email, body.Password)
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	h.logger.Info().Str("user_id", u.ID).Str("username", u.Username).Msg("user created")
	writeJSON(w, http.StatusCreated, u)
}

type updateUserRequest struct {
	Username *string `json:"username,omitempty"`
	Name     *string `json:"name,omitempty"`
	Email    *string `json:"email,omitempty"`
	Password *string `json:"password,omitempty"`
}

func (h *AdminUsersHandler) updateUser(w http.ResponseWriter, r *http.Request, id string) {
	var body updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	patch := users.UserPatch{
		Username: body.Username,
		Name:     body.Name,
		Email:    body.Email,
		Password: body.Password,
	}
	u, err := h.store.UpdateUser(id, patch)
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

type mintTokenRequest struct {
	Label      string `json:"label"`
	TTLSeconds int64  `json:"ttl_seconds"`
}

// mintTokenResponse echoes the standard token metadata plus the
// plaintext token value — returned exactly once.
type mintTokenResponse struct {
	ID         string    `json:"id"`
	UserID     string    `json:"user_id"`
	Label      string    `json:"label"`
	Token      string    `json:"token"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at,omitempty"`
}

func (h *AdminUsersHandler) mintToken(w http.ResponseWriter, r *http.Request, userID string) {
	var body mintTokenRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeAdminError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	u, err := h.store.GetUser(userID)
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	ttl := time.Duration(body.TTLSeconds) * time.Second
	tok, err := h.store.CreateToken(userID, body.Label, ttl)
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	// Register with the live OAuth validator so the token authenticates
	// /services/data calls through the same path as password-grant
	// tokens (T11).
	info := &middleware.TokenInfo{
		Token:    tok.Token,
		Type:     "access",
		Username: u.Username,
		UserID:   u.ID,
		ClientID: adminClientID,
		Scope:    scopeAdminUser,
		IssuedAt: tok.CreatedAt.Unix(),
	}
	if !tok.ExpiresAt.IsZero() {
		info.ExpiresAt = tok.ExpiresAt.Unix()
	}
	middleware.RegisterTokenInfo(info)

	h.logger.Info().Str("user_id", u.ID).Str("token_id", tok.ID).Msg("admin-minted token issued")
	writeJSON(w, http.StatusCreated, mintTokenResponse{
		ID:        tok.ID,
		UserID:    tok.UserID,
		Label:     tok.Label,
		Token:     tok.Token,
		CreatedAt: tok.CreatedAt,
		ExpiresAt: tok.ExpiresAt,
	})
}

// writeStoreError maps users package errors to HTTP responses.
func (h *AdminUsersHandler) writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, users.ErrNotFound):
		writeAdminError(w, http.StatusNotFound, "user not found")
	case errors.Is(err, users.ErrTokenNotFound):
		writeAdminError(w, http.StatusNotFound, "token not found")
	case errors.Is(err, users.ErrUsernameTaken):
		writeAdminError(w, http.StatusConflict, "username already exists")
	case errors.Is(err, users.ErrEmptyUsername):
		writeAdminError(w, http.StatusBadRequest, "username is required")
	case errors.Is(err, users.ErrEmptyPassword):
		writeAdminError(w, http.StatusBadRequest, "password is required")
	default:
		writeAdminError(w, http.StatusInternalServerError, err.Error())
	}
}

// writeAdminError writes a small JSON error envelope used by the admin
// API. Distinct from the Salesforce SF-array shape because this isn't
// a Salesforce-spec endpoint.
func writeAdminError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}


