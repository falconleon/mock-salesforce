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
	AccessToken string `json:"access_token"`
	InstanceURL string `json:"instance_url"`
	ID          string `json:"id"`
	TokenType   string `json:"token_type"`
	IssuedAt    string `json:"issued_at"`
	Signature   string `json:"signature"`
	Scope       string `json:"scope"`
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
