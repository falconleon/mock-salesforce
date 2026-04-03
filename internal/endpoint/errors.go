package endpoint

import "errors"

// Error types for endpoint operations.
var (
	// ErrNoEndpointsAvailable is returned when no healthy endpoints are available.
	ErrNoEndpointsAvailable = errors.New("no healthy endpoints available")

	// ErrProviderNotFound is returned when a requested provider is not registered.
	ErrProviderNotFound = errors.New("provider not found")

	// ErrHealthCheckFailed is returned when a health check fails.
	ErrHealthCheckFailed = errors.New("health check failed")

	// ErrModelNotFound is returned when a requested model is not available.
	ErrModelNotFound = errors.New("model not found")

	// ErrAuthenticationRequired is returned when authentication is required but not provided.
	ErrAuthenticationRequired = errors.New("authentication required")

	// ErrInvalidEndpoint is returned when an endpoint URL is invalid.
	ErrInvalidEndpoint = errors.New("invalid endpoint URL")

	// ErrRateLimited is returned when the endpoint rate limit is exceeded.
	ErrRateLimited = errors.New("rate limited")

	// ErrProviderNotConfigured is returned when a provider is not configured.
	ErrProviderNotConfigured = errors.New("provider not configured")

	// ErrInvalidConfiguration is returned when configuration is invalid.
	ErrInvalidConfiguration = errors.New("invalid configuration")
)

// EndpointError wraps an error with additional context about the endpoint.
type EndpointError struct {
	Endpoint string
	Provider string
	Op       string // operation that failed
	Err      error
}

// Error implements the error interface.
func (e *EndpointError) Error() string {
	if e.Provider != "" {
		return e.Provider + ": " + e.Op + " on " + e.Endpoint + ": " + e.Err.Error()
	}
	return e.Op + " on " + e.Endpoint + ": " + e.Err.Error()
}

// Unwrap returns the underlying error.
func (e *EndpointError) Unwrap() error {
	return e.Err
}

// NewEndpointError creates a new EndpointError.
func NewEndpointError(endpoint, provider, op string, err error) *EndpointError {
	return &EndpointError{
		Endpoint: endpoint,
		Provider: provider,
		Op:       op,
		Err:      err,
	}
}

