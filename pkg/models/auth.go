// Package models defines the data structures for Salesforce API responses.
package models

// OAuthRequest represents a Salesforce OAuth token request.
type OAuthRequest struct {
	GrantType    string `json:"grant_type"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Username     string `json:"username"`
	Password     string `json:"password"`
}

// OAuthResponse represents a successful Salesforce OAuth token response.
type OAuthResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	InstanceURL  string `json:"instance_url"`
	ID           string `json:"id"`
	TokenType    string `json:"token_type"`
	IssuedAt     string `json:"issued_at"`
	Signature    string `json:"signature"`
	Scope        string `json:"scope"`
}

// OAuthError represents a Salesforce OAuth error response.
type OAuthError struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// APIError represents a Salesforce REST API error.
// Errors are returned as arrays, even for single errors.
type APIError struct {
	Message   string   `json:"message"`
	ErrorCode string   `json:"errorCode"`
	Fields    []string `json:"fields,omitempty"`
}

// UserinfoPhotos holds picture URLs for the userinfo response.
type UserinfoPhotos struct {
	Picture   string `json:"picture"`
	Thumbnail string `json:"thumbnail"`
}

// UserinfoURLs holds the per-user service endpoint URLs.
type UserinfoURLs struct {
	Enterprise   string `json:"enterprise"`
	Metadata     string `json:"metadata"`
	Partner      string `json:"partner"`
	Rest         string `json:"rest"`
	Sobjects     string `json:"sobjects"`
	Search       string `json:"search"`
	Query        string `json:"query"`
	Recent       string `json:"recent"`
	Profile      string `json:"profile"`
	Feeds        string `json:"feeds"`
	Groups       string `json:"groups"`
	Users        string `json:"users"`
	FeedItems    string `json:"feed_items"`
	FeedElements string `json:"feed_elements"`
	CustomDomain string `json:"custom_domain,omitempty"`
}

// UserinfoResponse models the OpenID-style claims at /services/oauth2/userinfo.
type UserinfoResponse struct {
	Sub               string         `json:"sub"`
	UserID            string         `json:"user_id"`
	OrganizationID    string         `json:"organization_id"`
	PreferredUsername string         `json:"preferred_username"`
	Nickname          string         `json:"nickname"`
	Name              string         `json:"name"`
	Email             string         `json:"email"`
	EmailVerified     bool           `json:"email_verified"`
	GivenName         string         `json:"given_name"`
	FamilyName        string         `json:"family_name"`
	Zoneinfo          string         `json:"zoneinfo"`
	Photos            UserinfoPhotos `json:"photos"`
	Profile           string         `json:"profile"`
	Picture           string         `json:"picture"`
	Address           map[string]any `json:"address"`
	URLs              UserinfoURLs   `json:"urls"`
	Active            bool           `json:"active"`
	UserType          string         `json:"user_type"`
	Language          string         `json:"language"`
	Locale            string         `json:"locale"`
	UTCOffset         int            `json:"utcOffset"`
	UpdatedAt         string         `json:"updated_at"`
}

// IntrospectResponse models the RFC 7662 token introspection response.
type IntrospectResponse struct {
	Active    bool   `json:"active"`
	Scope     string `json:"scope,omitempty"`
	ClientID  string `json:"client_id,omitempty"`
	Username  string `json:"username,omitempty"`
	Exp       int64  `json:"exp,omitempty"`
	Iat       int64  `json:"iat,omitempty"`
	Nbf       int64  `json:"nbf,omitempty"`
	Sub       string `json:"sub,omitempty"`
	Aud       string `json:"aud,omitempty"`
	Iss       string `json:"iss,omitempty"`
	TokenType string `json:"token_type,omitempty"`
}
