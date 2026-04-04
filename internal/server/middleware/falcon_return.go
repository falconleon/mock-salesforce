package middleware

import (
	"net/http"
	"net/url"
	"os"
	"strings"
)

const (
	falconReturnParam  = "falcon_return"
	falconReturnCookie = "falcon_return"
)

// defaultAllowedPatterns lists host patterns accepted for falcon_return URLs.
// Patterns starting with "*." match any subdomain of that suffix.
var defaultAllowedPatterns = []string{
	"*.orb.local",
	"localhost",
	"127.0.0.1",
}

// allowedPatterns returns the configured allowed-origin patterns, falling back
// to defaults if the env var is unset.
func allowedPatterns() []string {
	env := os.Getenv("FALCON_RETURN_ALLOWED_ORIGINS")
	if env == "" {
		return defaultAllowedPatterns
	}
	var patterns []string
	for _, p := range strings.Split(env, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			patterns = append(patterns, p)
		}
	}
	return patterns
}

// ValidateFalconReturn checks that rawURL is a valid HTTP(S) URL whose host
// matches one of the allowed patterns. Returns the cleaned URL or "" if invalid.
func ValidateFalconReturn(rawURL string) string {
	if rawURL == "" {
		return ""
	}
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	host := u.Hostname() // strips port
	for _, pattern := range allowedPatterns() {
		if strings.HasPrefix(pattern, "*.") {
			suffix := pattern[1:] // e.g. ".orb.local"
			if strings.HasSuffix(host, suffix) {
				return rawURL
			}
		} else if host == pattern {
			return rawURL
		}
	}
	return ""
}

// SetFalconReturnCookie stores the validated falcon_return URL in a cookie.
func SetFalconReturnCookie(w http.ResponseWriter, returnURL string) {
	http.SetCookie(w, &http.Cookie{
		Name:     falconReturnCookie,
		Value:    returnURL,
		Path:     "/",
		HttpOnly: false, // readable by JS for clear-on-click
		SameSite: http.SameSiteLaxMode,
		MaxAge:   3600 * 8,
	})
}

// ClearFalconReturnCookie removes the falcon_return cookie.
func ClearFalconReturnCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:   falconReturnCookie,
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	})
}

// GetFalconReturn reads the falcon_return value from the request cookie.
func GetFalconReturn(r *http.Request) string {
	c, err := r.Cookie(falconReturnCookie)
	if err != nil || c.Value == "" {
		return ""
	}
	return c.Value
}

// ExtractFalconReturn reads and validates falcon_return from a query parameter.
func ExtractFalconReturn(r *http.Request) string {
	return ValidateFalconReturn(r.URL.Query().Get(falconReturnParam))
}

// CaptureFalconReturn is middleware that captures the falcon_return query
// parameter on any request and stores it in a cookie. This handles the case
// where an already-authenticated user arrives with falcon_return on a deep link.
func CaptureFalconReturn() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if fr := ExtractFalconReturn(r); fr != "" {
				SetFalconReturnCookie(w, fr)
			}
			next.ServeHTTP(w, r)
		})
	}
}
