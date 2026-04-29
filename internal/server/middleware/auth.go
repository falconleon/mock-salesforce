// Package middleware provides HTTP middleware for the mock server.
package middleware

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/pkg/models"
)

const (
	sessionCookieName = "sf_session"
	sessionDuration   = 12 * time.Hour
)

// publicPaths are paths that don't require authentication.
// Endpoints that perform their own auth (revoke/introspect) or are
// intentionally unauthenticated (token) are listed here.
var publicPaths = map[string]bool{
	"/services/oauth2/token":      true,
	"/services/oauth2/revoke":     true,
	"/services/oauth2/introspect": true,
	"/health":                     true,
	"/":                           true,
	"/login":                      true,
	"/logout":                     true,
}

// TokenInfo carries metadata about an issued OAuth token. Used by the
// auth middleware for validation and by the OAuth handlers for
// introspection and userinfo.
type TokenInfo struct {
	Token     string
	Type      string // "access" or "refresh"
	Username  string
	UserID    string
	ClientID  string
	Scope     string
	IssuedAt  int64 // unix seconds
	ExpiresAt int64 // unix seconds; 0 = no expiry
	Refresh   string
}

var (
	mu         sync.RWMutex
	mockTokens = map[string]*TokenInfo{
		"mock-access-token": {Token: "mock-access-token", Type: "access"},
	}
)

// RegisterToken adds a token to the valid tokens set with no metadata.
// Retained for backwards compatibility with existing callers/tests.
func RegisterToken(token string) {
	mu.Lock()
	mockTokens[token] = &TokenInfo{Token: token, Type: "access"}
	mu.Unlock()
}

// RegisterTokenInfo registers a token along with its full metadata.
func RegisterTokenInfo(info *TokenInfo) {
	if info == nil || info.Token == "" {
		return
	}
	mu.Lock()
	mockTokens[info.Token] = info
	mu.Unlock()
}

// LookupToken returns the metadata for a token, or nil if unknown/revoked.
func LookupToken(token string) *TokenInfo {
	mu.RLock()
	defer mu.RUnlock()
	return mockTokens[token]
}

// RevokeToken removes a token (access or refresh) from the valid set.
// Returns true if the token existed and was removed, false otherwise.
func RevokeToken(token string) bool {
	mu.Lock()
	defer mu.Unlock()
	if _, ok := mockTokens[token]; !ok {
		return false
	}
	delete(mockTokens, token)
	return true
}

// Auth validates Bearer tokens and session cookies on protected routes.
func Auth(logger zerolog.Logger, sessionSecret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path

			// Skip auth for public paths
			if publicPaths[path] {
				next.ServeHTTP(w, r)
				return
			}
			// Admin paths are unauthenticated (internal use by demo controller)
			if strings.HasPrefix(path, "/admin/") {
				next.ServeHTTP(w, r)
				return
			}
			// Skip auth for static assets
			if strings.HasPrefix(path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}

			// Try session cookie first (for UI browsing)
			if claims, ok := validateSessionCookie(r, sessionSecret); ok {
				ctx := context.WithValue(r.Context(), contextKeyAuthUser, claims.Email)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Try Bearer token (for API calls)
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				if info := LookupToken(token); info != nil && info.Type != "refresh" {
					username := info.Username
					if username == "" {
						username = "bearer-user"
					}
					SetSessionCookie(w, username, sessionSecret)
					next.ServeHTTP(w, r)
					return
				}
			}

			// Auth failed
			if isHTMLRequest(r) {
				params := url.Values{}
				if r.URL.Path != "" && r.URL.Path != "/" {
					nextURL := r.URL.Path
					if r.URL.RawQuery != "" {
						nextURL += "?" + r.URL.RawQuery
					}
					params.Set("next", nextURL)
				}
				if fr := ExtractFalconReturn(r); fr != "" {
					params.Set(falconReturnParam, fr)
				}
				redirectURL := "/login"
				if encoded := params.Encode(); encoded != "" {
					redirectURL += "?" + encoded
				}
				http.Redirect(w, r, redirectURL, http.StatusFound)
				return
			}
			writeAuthError(w, logger, "Session expired or invalid")
		})
	}
}

type contextKey string

const contextKeyAuthUser contextKey = "auth_user"

// SetSessionCookie mints an HS256 JWT for the given email and writes it
// as the sf_session cookie. Sub/Name are derived from email when not
// supplied externally; expiry is sessionDuration.
func SetSessionCookie(w http.ResponseWriter, email, secret string) {
	now := time.Now()
	claims := SessionClaims{
		Sub:   email,
		Name:  email,
		Email: email,
		Iat:   now.Unix(),
		Exp:   now.Add(sessionDuration).Unix(),
	}
	tok, err := MintSessionJWT(claims, secret)
	if err != nil {
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int(sessionDuration / time.Second),
	})
}

// ClearSessionCookie expires the sf_session cookie at the browser.
func ClearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
	})
}

// ValidateSession reports whether the request carries a valid session
// cookie, returning the authenticated email on success. Retained for
// callers that only care about the email; use ValidateSessionClaims
// when full claim access is required.
func ValidateSession(r *http.Request, secret string) (string, bool) {
	c, ok := validateSessionCookie(r, secret)
	if !ok {
		return "", false
	}
	return c.Email, true
}

// ValidateSessionClaims returns the full decoded claim set for a valid
// sf_session cookie, or false if the cookie is missing/invalid/expired.
func ValidateSessionClaims(r *http.Request, secret string) (*SessionClaims, bool) {
	return validateSessionCookie(r, secret)
}

func validateSessionCookie(r *http.Request, secret string) (*SessionClaims, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, false
	}
	claims, err := ParseSessionJWT(cookie.Value, secret)
	if err != nil {
		return nil, false
	}
	return claims, true
}

// isHTMLRequest decides whether a failed-auth response should be a 302
// redirect to /login (browser flow) or a 401 JSON error (API flow).
//
// Negotiation rule:
//   - Explicit Bearer Authorization → API caller → 401 JSON.
//   - /services/* and /admin/* are API surfaces → 401 JSON regardless of Accept.
//   - Accept header that excludes text/html (e.g. application/json) → 401 JSON.
//   - Otherwise: path is a known UI route AND Accept allows HTML
//     (text/html, */*, or absent) → 302 redirect.
//   - Default → 401 JSON.
func isHTMLRequest(r *http.Request) bool {
	if strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		return false
	}
	path := r.URL.Path
	if strings.HasPrefix(path, "/services/") || strings.HasPrefix(path, "/admin/") {
		return false
	}
	accept := r.Header.Get("Accept")
	acceptHTML := accept == "" ||
		strings.Contains(accept, "text/html") ||
		strings.Contains(accept, "*/*")
	if !acceptHTML {
		return false
	}
	return isUIRoute(path)
}

// isUIRoute reports whether the path is one of the browser-facing
// routes registered in router.go (Lightning pages, /home, settings,
// playground). New UI route prefixes should be added here so unauth
// hits redirect to /login instead of returning JSON 401.
func isUIRoute(path string) bool {
	if path == "/home" {
		return true
	}
	return strings.HasPrefix(path, "/lightning/") ||
		strings.HasPrefix(path, "/settings") ||
		strings.HasPrefix(path, "/playground")
}

func writeAuthError(w http.ResponseWriter, logger zerolog.Logger, msg string) {
	logger.Warn().Str("error", msg).Msg("Authentication failed")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)

	// Salesforce returns errors as an array
	errors := []models.APIError{
		{
			Message:   msg,
			ErrorCode: "INVALID_SESSION_ID",
		},
	}
	json.NewEncoder(w).Encode(errors)
}

// LoginHandler handles POST /login for UI session auth. On success it
// mints a JWT session cookie and redirects to ?next=<path> when that
// is a same-origin relative path, or /home otherwise.
func LoginHandler(users map[string]string, sessionSecret, basePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		email := r.FormValue("email")
		password := r.FormValue("password")
		falconReturn := ValidateFalconReturn(r.FormValue(falconReturnParam))
		next := sanitizeNext(r.FormValue("next"))

		expected, ok := users[email]
		if !ok || subtle.ConstantTimeCompare([]byte(expected), []byte(password)) != 1 {
			params := url.Values{}
			params.Set("error", "invalid")
			if next != "" {
				params.Set("next", next)
			}
			if falconReturn != "" {
				params.Set(falconReturnParam, falconReturn)
			}
			http.Redirect(w, r, basePath+"/login?"+params.Encode(), http.StatusFound)
			return
		}

		SetSessionCookie(w, email, sessionSecret)

		if falconReturn != "" {
			SetFalconReturnCookie(w, falconReturn)
		}

		dest := basePath + "/home"
		if next != "" {
			dest = basePath + next
		}
		http.Redirect(w, r, dest, http.StatusFound)
	}
}

// LogoutHandler clears the session cookie and redirects to /login.
func LogoutHandler(basePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ClearSessionCookie(w)
		http.Redirect(w, r, basePath+"/login", http.StatusFound)
	}
}

// sanitizeNext rejects absolute URLs and protocol-relative paths so the
// next parameter cannot be used as an open redirect.
func sanitizeNext(s string) string {
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "/") {
		return ""
	}
	if strings.HasPrefix(s, "//") {
		return ""
	}
	return s
}
