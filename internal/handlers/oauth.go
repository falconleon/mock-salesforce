// Package handlers provides HTTP handlers for the Salesforce mock API.
package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"

	"github.com/falconleon/mock-salesforce/internal/config"
	"github.com/falconleon/mock-salesforce/internal/server/middleware"
	"github.com/falconleon/mock-salesforce/pkg/models"
)

// OAuthHandler handles OAuth authentication requests.
type OAuthHandler struct {
	config *config.Config
	logger zerolog.Logger
}

// NewOAuthHandler creates a new OAuth handler.
func NewOAuthHandler(cfg *config.Config, logger zerolog.Logger) *OAuthHandler {
	return &OAuthHandler{
		config: cfg,
		logger: logger.With().Str("handler", "oauth").Logger(),
	}
}

// HandleToken processes OAuth token requests.
// POST /services/oauth2/token
func (h *OAuthHandler) HandleToken(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_request", "Unable to parse request body")
		return
	}

	grantType := r.FormValue("grant_type")
	clientID := r.FormValue("client_id")
	clientSecret := r.FormValue("client_secret")
	username := r.FormValue("username")
	password := r.FormValue("password")

	h.logger.Debug().
		Str("grant_type", grantType).
		Str("client_id", clientID).
		Str("username", username).
		Msg("OAuth token request received")

	// Validate grant type
	if grantType != "password" {
		h.writeOAuthError(w, http.StatusBadRequest, "unsupported_grant_type",
			fmt.Sprintf("Grant type '%s' not supported. Use 'password'.", grantType))
		return
	}

	// Validate credentials (constant-time for all comparisons)
	if subtle.ConstantTimeCompare([]byte(clientID), []byte(h.config.MockClientID)) != 1 {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_client_id", "Invalid client_id")
		return
	}

	if subtle.ConstantTimeCompare([]byte(clientSecret), []byte(h.config.MockClientSecret)) != 1 {
		h.writeOAuthError(w, http.StatusBadRequest, "invalid_client", "Invalid client credentials")
		return
	}

	if len(h.config.MockUsers) > 0 {
		// Multi-user validation
		expected, ok := h.config.MockUsers[username]
		if !ok || subtle.ConstantTimeCompare([]byte(expected), []byte(password)) != 1 {
			h.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "authentication failure")
			return
		}
	} else {
		// Fallback to single credential (backwards compat)
		if subtle.ConstantTimeCompare([]byte(username), []byte(h.config.MockUsername)) != 1 ||
			subtle.ConstantTimeCompare([]byte(password), []byte(h.config.MockPassword)) != 1 {
			h.writeOAuthError(w, http.StatusBadRequest, "invalid_grant", "authentication failure")
			return
		}
	}

	// Generate mock token response
	now := time.Now()
	issuedAt := fmt.Sprintf("%d", now.UnixMilli())
	accessToken := h.generateAccessToken(username, now)
	instanceURL := h.config.InstanceURL
	if strings.HasPrefix(instanceURL, ":") {
		instanceURL = "http://localhost" + instanceURL
	}

	// Generate signature (mock implementation)
	signature := h.generateSignature(accessToken, issuedAt)

	response := models.OAuthResponse{
		AccessToken: accessToken,
		InstanceURL: instanceURL,
		ID:          fmt.Sprintf("https://login.salesforce.com/id/00Dxx0000001gPq/005xx000001SwiUAAS"),
		TokenType:   "Bearer",
		IssuedAt:    issuedAt,
		Signature:   signature,
		Scope:       "api refresh_token",
	}

	// Register the token as valid
	middleware.RegisterToken(accessToken)

	h.logger.Info().
		Str("username", username).
		Str("access_token", accessToken[:20]+"...").
		Msg("OAuth token issued")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// generateAccessToken creates a mock access token.
func (h *OAuthHandler) generateAccessToken(username string, t time.Time) string {
	// Create a deterministic but unique-looking token
	data := fmt.Sprintf("%s:%d", username, t.UnixNano())
	hash := sha256.Sum256([]byte(data))
	return fmt.Sprintf("00Dxx0000001gPq!%s", base64.RawURLEncoding.EncodeToString(hash[:24]))
}

// generateSignature creates a mock signature matching Salesforce format.
func (h *OAuthHandler) generateSignature(token, issuedAt string) string {
	mac := hmac.New(sha256.New, []byte(h.config.MockClientSecret))
	mac.Write([]byte(token + issuedAt))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// writeOAuthError writes an OAuth error response.
func (h *OAuthHandler) writeOAuthError(w http.ResponseWriter, status int, errorCode, description string) {
	h.logger.Warn().
		Str("error", errorCode).
		Str("description", description).
		Msg("OAuth error")

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(models.OAuthError{
		Error:            errorCode,
		ErrorDescription: description,
	})
}
