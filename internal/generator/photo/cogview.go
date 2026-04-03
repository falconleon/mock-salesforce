package photo

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/falconleon/mock-salesforce/internal/generator/seed"
	zaiclient "github.com/falconleon/mock-salesforce/internal/zai"
)

const (
	// DefaultMaxRetries is the default number of retry attempts for API calls.
	DefaultMaxRetries = 3

	// DefaultBaseBackoff is the initial backoff duration.
	DefaultBaseBackoff = 2 * time.Second

	// DefaultMaxBackoff is the maximum backoff duration.
	DefaultMaxBackoff = 30 * time.Second
)

// RetryConfig configures retry behavior for API calls.
type RetryConfig struct {
	MaxRetries  int
	BaseBackoff time.Duration
	MaxBackoff  time.Duration
}

// DefaultRetryConfig returns the default retry configuration.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:  DefaultMaxRetries,
		BaseBackoff: DefaultBaseBackoff,
		MaxBackoff:  DefaultMaxBackoff,
	}
}

// ErrTransient indicates a transient error that may succeed on retry.
var ErrTransient = errors.New("transient error")

const (
	// CogViewModel is the model identifier for CogView-4.
	CogViewModel = "cogview-4"
)

// CogViewRequest represents a request to the CogView-4 API.
// Kept for test compatibility with mock servers.
type CogViewRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

// CogViewResponse represents a response from the CogView-4 API.
// Kept for test compatibility with mock servers.
type CogViewResponse struct {
	Data []CogViewImageData `json:"data"`
}

// CogViewImageData contains the generated image data.
type CogViewImageData struct {
	B64JSON string `json:"b64_json"` // Base64 encoded image
}

// PhotoGenerator generates profile photos using CogView-4.
type PhotoGenerator struct {
	apiKey      string
	outputDir   string // Directory for storing generated images
	httpClient  *http.Client
	endpoint    string      // Test endpoint override (empty = use zaiclient)
	retryConfig RetryConfig // Retry configuration for transient failures (used in test mode)
	zaiClient   *zaiclient.Client
}

// NewPhotoGenerator creates a new CogView-4 photo generator.
// Uses go_zai_client for API calls to the real Z.AI endpoint.
func NewPhotoGenerator(apiKey, outputDir string) *PhotoGenerator {
	// Create the zaiclient (it handles API key validation)
	client, _ := zaiclient.NewClient(apiKey)

	return &PhotoGenerator{
		apiKey:    apiKey,
		outputDir: outputDir,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		endpoint:    "", // Empty means use zaiclient
		retryConfig: DefaultRetryConfig(),
		zaiClient:   client,
	}
}

// WithRetryConfig sets a custom retry configuration (used in test mode).
func (g *PhotoGenerator) WithRetryConfig(cfg RetryConfig) *PhotoGenerator {
	g.retryConfig = cfg
	return g
}

// Generate creates a new profile photo for the given person traits.
// Returns the metadata for the generated photo.
func (g *PhotoGenerator) Generate(ctx context.Context, personSeed seed.PersonSeed) (*PhotoMetadata, error) {
	// Build the prompt
	prompt := BuildPhotoPrompt(personSeed)

	// Generate unique ID for the photo
	id := generateUUID()
	filename := id + ".png"

	var imagePath string

	// Use test endpoint if set (for unit tests), otherwise use zaiclient
	if g.endpoint != "" {
		// Test mode: use legacy HTTP call with mock server
		imageData, err := g.callCogViewAPI(ctx, prompt)
		if err != nil {
			return nil, fmt.Errorf("cogview api: %w", err)
		}

		// Ensure output directory exists
		if err := os.MkdirAll(g.outputDir, 0755); err != nil {
			return nil, fmt.Errorf("create output dir: %w", err)
		}

		// Save image to disk
		imagePath = filepath.Join(g.outputDir, filename)
		if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
			return nil, fmt.Errorf("save image: %w", err)
		}
	} else {
		// Production mode: use go_zai_client
		if g.zaiClient == nil {
			return nil, fmt.Errorf("zai client not initialized (missing API key?)")
		}

		result, err := g.zaiClient.GenerateImage(ctx, zaiclient.GenerateImageOptions{
			Prompt:    prompt,
			Model:     zaiclient.ModelCogView,
			OutputDir: g.outputDir,
			Filename:  filename,
		})
		if err != nil {
			return nil, fmt.Errorf("generate image: %w", err)
		}

		imagePath = result.LocalPath
	}

	// Build metadata
	metadata := &PhotoMetadata{
		ID:        id,
		Filename:  filename,
		Ethnicity: personSeed.Ethnicity,
		Gender:    personSeed.Gender,
		AgeRange:  ageToRange(personSeed.Age),
		HairColor: personSeed.HairColor,
		HairStyle: personSeed.HairStyle,
		EyeColor:  personSeed.EyeColor,
		Glasses:   personSeed.Glasses,
		Build:     personSeed.Build,
		InUseBy:   []string{},
		CreatedAt: time.Now().UTC(),
	}

	// imagePath is used internally but metadata.Filename is the key output
	_ = imagePath

	return metadata, nil
}

// callCogViewAPI sends a request to CogView-4 and returns the image data.
// Includes retry logic for transient failures with exponential backoff.
func (g *PhotoGenerator) callCogViewAPI(ctx context.Context, prompt string) ([]byte, error) {
	var lastErr error

	for attempt := 0; attempt <= g.retryConfig.MaxRetries; attempt++ {
		// If not the first attempt, wait with exponential backoff
		if attempt > 0 {
			backoff := g.calculateBackoff(attempt)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				// Continue with retry
			}
		}

		imageData, err := g.doAPICall(ctx, prompt)
		if err == nil {
			return imageData, nil
		}

		// Check if error is retryable
		if !g.isRetryableError(err) {
			return nil, err
		}

		lastErr = err
	}

	return nil, fmt.Errorf("api call failed after %d retries: %w", g.retryConfig.MaxRetries+1, lastErr)
}

// doAPICall performs a single API call to CogView-4.
func (g *PhotoGenerator) doAPICall(ctx context.Context, prompt string) ([]byte, error) {
	reqBody := CogViewRequest{
		Model:  CogViewModel,
		Prompt: prompt,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", g.endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+g.apiKey)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		// Network errors are typically transient
		return nil, fmt.Errorf("%w: http request: %v", ErrTransient, err)
	}
	defer resp.Body.Close()

	// Handle non-OK status codes
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// 429 (rate limit), 500, 502, 503, 504 are typically transient
		if isTransientStatusCode(resp.StatusCode) {
			return nil, fmt.Errorf("%w: api error %d: %s", ErrTransient, resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("api error %d: %s", resp.StatusCode, string(body))
	}

	var cogResp CogViewResponse
	if err := json.NewDecoder(resp.Body).Decode(&cogResp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}

	if len(cogResp.Data) == 0 {
		return nil, fmt.Errorf("no image data in response")
	}

	// Decode base64 image
	imageData, err := base64.StdEncoding.DecodeString(cogResp.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("decode base64: %w", err)
	}

	return imageData, nil
}

// calculateBackoff computes the backoff duration for a given attempt.
func (g *PhotoGenerator) calculateBackoff(attempt int) time.Duration {
	// Exponential backoff: base * 2^(attempt-1)
	backoff := g.retryConfig.BaseBackoff * time.Duration(1<<(attempt-1))
	if backoff > g.retryConfig.MaxBackoff {
		backoff = g.retryConfig.MaxBackoff
	}
	return backoff
}

// isRetryableError returns true if the error is transient and should be retried.
func (g *PhotoGenerator) isRetryableError(err error) bool {
	return errors.Is(err, ErrTransient)
}

// isTransientStatusCode returns true for HTTP status codes that indicate transient failures.
func isTransientStatusCode(code int) bool {
	switch code {
	case http.StatusTooManyRequests,      // 429
		http.StatusInternalServerError,   // 500
		http.StatusBadGateway,            // 502
		http.StatusServiceUnavailable,    // 503
		http.StatusGatewayTimeout:        // 504
		return true
	}
	return false
}

// BuildPhotoPrompt creates a CogView-4 prompt from person traits.
func BuildPhotoPrompt(personSeed seed.PersonSeed) string {
	var parts []string

	// Base: Professional corporate headshot of a [age]-year-old [ethnicity] [gender]
	parts = append(parts, fmt.Sprintf(
		"Professional corporate headshot of a %d-year-old %s %s",
		personSeed.Age,
		personSeed.Ethnicity,
		strings.ToLower(personSeed.Gender),
	))

	// Hair description
	parts = append(parts, fmt.Sprintf(
		"with %s %s %s hair",
		strings.ToLower(personSeed.HairColor),
		strings.ToLower(personSeed.HairTexture),
		strings.ToLower(personSeed.HairStyle),
	))

	// Eyes
	parts = append(parts, fmt.Sprintf("and %s eyes", strings.ToLower(personSeed.EyeColor)))

	// Facial hair (male only)
	if personSeed.FacialHair != "" && personSeed.FacialHair != "clean-shaven" {
		parts = append(parts, fmt.Sprintf("with %s", strings.ToLower(personSeed.FacialHair)))
	}

	// Glasses
	if personSeed.Glasses {
		parts = append(parts, "wearing glasses")
	}

	// Build
	parts = append(parts, fmt.Sprintf("%s build", strings.ToLower(personSeed.Build)))

	// Standard ending for professional photos
	parts = append(parts, "clean background, professional lighting, business attire, natural smile, high resolution portrait photo")

	return strings.Join(parts, ", ")
}

// generateUUID creates a simple UUID-like string.
func generateUUID() string {
	// Use crypto/rand for better randomness
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

