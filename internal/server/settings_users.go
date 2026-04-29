package server

import (
	"errors"
	"html/template"
	"net/http"
	"strconv"
	"time"

	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/internal/users"
)

// settingsClientID and settingsScope tag tokens minted from the
// settings UI so they can be distinguished from password-grant or
// admin-CLI tokens in the OAuth registry.
const (
	settingsClientID = "settings-ui"
	settingsScope    = "settings user-token"
)

// SettingsUsersHandler renders the /settings/users browser pages and
// handles the form-driven create/update/delete and per-user token
// mint/revoke flows. It calls into the same users.Store the
// AdminUsersHandler does and registers minted tokens with the OAuth
// validator so they authenticate /services/data calls identically.
type SettingsUsersHandler struct {
	store         users.Store
	basePath      string
	sessionSecret string
	listTpl       *template.Template
	detailTpl     *template.Template
}

// NewSettingsUsersHandler builds the handler with parsed templates.
func NewSettingsUsersHandler(s users.Store, basePath, sessionSecret string) *SettingsUsersHandler {
	funcMap := template.FuncMap{
		"basePath": func() string { return basePath },
		"fmtTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.UTC().Format("2006-01-02 15:04 UTC")
		},
	}
	listTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS,
		"templates/layout.html", "templates/users.html"))
	detailTpl := template.Must(template.New("").Funcs(funcMap).ParseFS(templateFS,
		"templates/layout.html", "templates/user_detail.html"))
	return &SettingsUsersHandler{
		store:         s,
		basePath:      basePath,
		sessionSecret: sessionSecret,
		listTpl:       listTpl,
		detailTpl:     detailTpl,
	}
}

// currentUser extracts the authenticated user's email from the session
// cookie for layout rendering. Mirrors UIHandler.currentUser.
func (h *SettingsUsersHandler) currentUser(r *http.Request) string {
	if h.sessionSecret == "" {
		return ""
	}
	email, _ := middleware.ValidateSession(r, h.sessionSecret)
	return email
}

// HandleList dispatches GET (render) and POST (create) on /settings/users.
func (h *SettingsUsersHandler) HandleList(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.renderList(w, r, "", nil)
	case http.MethodPost:
		h.createUser(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleDetail dispatches GET (render) and POST (update or delete) on
// /settings/users/{id}. The form's _action hidden field selects between
// update (default) and delete to keep all forms POST without method
// override middleware.
func (h *SettingsUsersHandler) HandleDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	switch r.Method {
	case http.MethodGet:
		h.renderDetail(w, r, id, "", users.Token{})
	case http.MethodPost:
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		switch r.FormValue("_action") {
		case "delete":
			h.deleteUser(w, r, id)
		default:
			h.updateUser(w, r, id)
		}
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// HandleTokens handles POST /settings/users/{id}/tokens (mint).
func (h *SettingsUsersHandler) HandleTokens(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.mintToken(w, r, r.PathValue("id"))
}

// HandleTokenRevoke handles POST /settings/users/{id}/tokens/{tokenId}/revoke.
func (h *SettingsUsersHandler) HandleTokenRevoke(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	userID := r.PathValue("id")
	tokenID := r.PathValue("tokenId")
	tok, err := h.store.GetToken(userID, tokenID)
	if err == nil {
		if delErr := h.store.DeleteToken(userID, tokenID); delErr == nil {
			middleware.RevokeToken(tok.Token)
		}
	}
	http.Redirect(w, r, h.basePath+"/settings/users/"+userID, http.StatusSeeOther)
}

func (h *SettingsUsersHandler) renderList(w http.ResponseWriter, r *http.Request, errMsg string, form map[string]string) {
	all, _ := h.store.ListUsers()
	if form == nil {
		form = map[string]string{}
	}
	_ = h.listTpl.ExecuteTemplate(w, "users.html", map[string]any{
		"Title":       "Settings — Users",
		"BasePath":    h.basePath,
		"CurrentUser": h.currentUser(r),
		"Users":       all,
		"Total":       len(all),
		"Error":       errMsg,
		"Form":        form,
	})
}

func (h *SettingsUsersHandler) renderDetail(w http.ResponseWriter, r *http.Request, id, errMsg string, newTok users.Token) {
	u, err := h.store.GetUser(id)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	toks, _ := h.store.ListTokens(id)
	_ = h.detailTpl.ExecuteTemplate(w, "user_detail.html", map[string]any{
		"Title":       "Settings — " + u.Username,
		"BasePath":    h.basePath,
		"CurrentUser": h.currentUser(r),
		"User":        u,
		"Tokens":      toks,
		"NewToken":    newTok,
		"Error":       errMsg,
	})
}

func (h *SettingsUsersHandler) createUser(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	username := r.FormValue("username")
	name := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")
	if _, err := h.store.CreateUser(username, name, email, password); err != nil {
		h.renderList(w, r, settingsErrorMessage(err), map[string]string{
			"username": username, "name": name, "email": email,
		})
		return
	}
	http.Redirect(w, r, h.basePath+"/settings/users", http.StatusSeeOther)
}

func (h *SettingsUsersHandler) updateUser(w http.ResponseWriter, r *http.Request, id string) {
	patch := users.UserPatch{}
	if v := r.FormValue("username"); v != "" {
		patch.Username = &v
	}
	if v := r.FormValue("name"); v != "" {
		patch.Name = &v
	}
	if v := r.FormValue("email"); v != "" {
		patch.Email = &v
	}
	if v := r.FormValue("password"); v != "" {
		patch.Password = &v
	}
	if _, err := h.store.UpdateUser(id, patch); err != nil {
		if errors.Is(err, users.ErrNotFound) {
			http.Error(w, "user not found", http.StatusNotFound)
			return
		}
		h.renderDetail(w, r, id, settingsErrorMessage(err), users.Token{})
		return
	}
	http.Redirect(w, r, h.basePath+"/settings/users/"+id, http.StatusSeeOther)
}

func (h *SettingsUsersHandler) deleteUser(w http.ResponseWriter, r *http.Request, id string) {
	// Revoke any tokens this user owns from the OAuth registry before
	// removing them from the store so subsequent /services/data calls
	// fail. The store cascade only handles durable rows.
	if toks, err := h.store.ListTokens(id); err == nil {
		for _, t := range toks {
			middleware.RevokeToken(t.Token)
		}
	}
	if err := h.store.DeleteUser(id); err != nil && !errors.Is(err, users.ErrNotFound) {
		h.renderDetail(w, r, id, settingsErrorMessage(err), users.Token{})
		return
	}
	http.Redirect(w, r, h.basePath+"/settings/users", http.StatusSeeOther)
}

func (h *SettingsUsersHandler) mintToken(w http.ResponseWriter, r *http.Request, userID string) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form", http.StatusBadRequest)
		return
	}
	u, err := h.store.GetUser(userID)
	if err != nil {
		http.Error(w, "user not found", http.StatusNotFound)
		return
	}
	label := r.FormValue("label")
	ttlSecs, _ := strconv.ParseInt(r.FormValue("ttl_seconds"), 10, 64)
	ttl := time.Duration(ttlSecs) * time.Second
	tok, err := h.store.CreateToken(userID, label, ttl)
	if err != nil {
		h.renderDetail(w, r, userID, settingsErrorMessage(err), users.Token{})
		return
	}
	info := &middleware.TokenInfo{
		Token:    tok.Token,
		Type:     "access",
		Username: u.Username,
		UserID:   u.ID,
		ClientID: settingsClientID,
		Scope:    settingsScope,
		IssuedAt: tok.CreatedAt.Unix(),
	}
	if !tok.ExpiresAt.IsZero() {
		info.ExpiresAt = tok.ExpiresAt.Unix()
	}
	middleware.RegisterTokenInfo(info)
	// Render the detail page directly so the plaintext token is shown
	// exactly once, in the same response that minted it.
	h.renderDetail(w, r, userID, "", tok)
}

// settingsErrorMessage maps users-package errors to short, human
// readable messages suitable for surfacing in the form.
func settingsErrorMessage(err error) string {
	switch {
	case errors.Is(err, users.ErrUsernameTaken):
		return "username already exists"
	case errors.Is(err, users.ErrEmptyUsername):
		return "username is required"
	case errors.Is(err, users.ErrEmptyPassword):
		return "password is required"
	case errors.Is(err, users.ErrNotFound):
		return "user not found"
	case errors.Is(err, users.ErrTokenNotFound):
		return "token not found"
	default:
		return err.Error()
	}
}
