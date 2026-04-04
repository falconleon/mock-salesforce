// Package middleware provides HTTP middleware for the mock server.
package middleware

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/pkg/models"
)

const sessionCookieName = "sf_session"

// publicPaths are paths that don't require authentication.
var publicPaths = map[string]bool{
	"/services/oauth2/token": true,
	"/health":                true,
	"/":                      true,
	"/login":                 true,
}

var (
	mu              sync.RWMutex
	mockValidTokens = map[string]bool{
		"mock-access-token": true,
	}
)

// RegisterToken adds a token to the valid tokens set.
func RegisterToken(token string) {
	mu.Lock()
	mockValidTokens[token] = true
	mu.Unlock()
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
			// Admin paths require Bearer token (not session cookie)
			if strings.HasPrefix(path, "/admin/") {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					token := strings.TrimPrefix(authHeader, "Bearer ")
					mu.RLock()
					valid := mockValidTokens[token]
					mu.RUnlock()
					if valid {
						next.ServeHTTP(w, r)
						return
					}
				}
				writeAuthError(w, logger, "Admin access requires valid Bearer token")
				return
			}
			// Skip auth for static assets
			if strings.HasPrefix(path, "/static/") {
				next.ServeHTTP(w, r)
				return
			}

			// Try session cookie first (for UI browsing)
			if email, ok := validateSessionCookie(r, sessionSecret); ok {
				ctx := context.WithValue(r.Context(), contextKeyAuthUser, email)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Try Bearer token (for API calls)
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token := strings.TrimPrefix(authHeader, "Bearer ")
				mu.RLock()
				valid := mockValidTokens[token]
				mu.RUnlock()
				if valid {
					SetSessionCookie(w, "bearer-user", sessionSecret)
					next.ServeHTTP(w, r)
					return
				}
			}

			// Auth failed
			if isHTMLRequest(r) {
				redirectURL := "/"
				if fr := ExtractFalconReturn(r); fr != "" {
					redirectURL = "/?falcon_return=" + url.QueryEscape(fr)
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

// SetSessionCookie creates a signed session cookie.
func SetSessionCookie(w http.ResponseWriter, email, secret string) {
	sig := signSession(email, secret)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    email + "|" + sig,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   3600 * 8, // 8 hours
	})
}

func signSession(email, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(email))
	return hex.EncodeToString(mac.Sum(nil))
}

// ValidateSession checks if the request has a valid session cookie.
func ValidateSession(r *http.Request, secret string) (string, bool) {
	return validateSessionCookie(r, secret)
}

func validateSessionCookie(r *http.Request, secret string) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	parts := strings.SplitN(cookie.Value, "|", 2)
	if len(parts) != 2 {
		return "", false
	}
	email, sig := parts[0], parts[1]
	expected := signSession(email, secret)
	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return "", false
	}
	return email, true
}

func isHTMLRequest(r *http.Request) bool {
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "text/html") ||
		strings.HasPrefix(r.URL.Path, "/lightning/")
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

// LoginHandler handles POST /login for UI session auth.
func LoginHandler(users map[string]string, sessionSecret, basePath string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		email := r.FormValue("email")
		password := r.FormValue("password")
		falconReturn := ValidateFalconReturn(r.FormValue(falconReturnParam))

		// Validate against multi-user store
		expected, ok := users[email]
		if !ok || subtle.ConstantTimeCompare([]byte(expected), []byte(password)) != 1 {
			redirectURL := basePath + "/?error=invalid"
			if falconReturn != "" {
				redirectURL += "&falcon_return=" + url.QueryEscape(falconReturn)
			}
			http.Redirect(w, r, redirectURL, http.StatusFound)
			return
		}

		SetSessionCookie(w, email, sessionSecret)

		// Store falcon_return in cookie if present
		if falconReturn != "" {
			SetFalconReturnCookie(w, falconReturn)
		}

		// Redirect to case list
		http.Redirect(w, r, basePath+"/lightning/o/Case/list", http.StatusFound)
	}
}
