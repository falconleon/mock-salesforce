// Package llm provides an adapter to connect the endpoint module to generators.
package llm

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"time"

	"github.com/falconleon/mock-salesforce/internal/endpoint"
	"github.com/falconleon/mock-salesforce/internal/generator"
)

// LLMCache defines the interface for LLM response caching.
type LLMCache interface {
	FindSimilar(embedding []float64, threshold float64) (*CachedResponse, error)
	Set(key string, resp *CachedResponse, embedding []float64) error
}

// Config holds configuration for the LLM adapter.
type Config struct {
	// Provider is the LLM provider: "zai", "ollama", "openai"
	Provider string

	// Model is the model name (e.g., "GLM-4.7" for Z.ai)
	Model string

	// APIKey is the API key (loaded from environment if empty)
	APIKey string

	// EndpointURL overrides the default endpoint URL
	EndpointURL string

	// Temperature controls randomness (0.0-2.0)
	Temperature float64

	// MaxTokens limits response length
	MaxTokens int

	// Timeout for LLM requests
	Timeout time.Duration

	// CachePath is the path to the SQLite cache database
	CachePath string
}

// DefaultConfig returns sensible defaults for Z.ai.
func DefaultConfig() Config {
	return Config{
		Provider:    "zai",
		Model:       "GLM-4.7",
		Temperature: 0.7,
		MaxTokens:   4096,
		Timeout:     120 * time.Second,
		CachePath:   "./data/llm_cache.db",
	}
}

// Adapter wraps an endpoint.ChatClient to implement generator.LLM.
type Adapter struct {
	client      endpoint.ChatClient
	model       string
	temperature float64
	maxTokens   int
	timeout     time.Duration
	llmCache    LLMCache
}

// New creates a new LLM adapter from config.
func New(cfg Config) (*Adapter, error) {
	apiKey := cfg.APIKey
	if apiKey == "" {
		apiKey = loadAPIKey(cfg.Provider)
	}

	endpointURL := cfg.EndpointURL
	if endpointURL == "" {
		endpointURL = defaultEndpoint(cfg.Provider)
	}

	client, err := endpoint.NewChatClient(endpoint.ChatClientConfig{
		EndpointURL: endpointURL,
		Provider:    cfg.Provider,
		APIKey:      apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("create chat client: %w", err)
	}

	cachePath := cfg.CachePath
	if cachePath == "" {
		cachePath = "./data/llm_cache.db"
	}

	llmCache, err := NewCache(cachePath)
	if err != nil {
		return nil, fmt.Errorf("initialize cache: %w", err)
	}

	return &Adapter{
		client:      client,
		model:       cfg.Model,
		temperature: cfg.Temperature,
		maxTokens:   cfg.MaxTokens,
		timeout:     cfg.Timeout,
		llmCache:    llmCache,
	}, nil
}

// Generate implements generator.LLM interface.
func (a *Adapter) Generate(prompt string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), a.timeout)
	defer cancel()

	// Record start time for latency calculation
	startTime := time.Now()

	// Generate embedding for the prompt
	embeddings, err := a.client.Embed(ctx, "qwen3-embedding:0.6b", []string{prompt})
	if err != nil {
		return "", fmt.Errorf("generate embedding: %w", err)
	}

	if len(embeddings) != 1 {
		return "", fmt.Errorf("expected 1 embedding, got %d", len(embeddings))
	}

	queryEmbedding := embeddings[0]

	// Generate hash of the prompt
	promptHash := hashPrompt(prompt)

	// Check cache for similar response with 0.95 threshold
	cachedResp, err := a.llmCache.FindSimilar(queryEmbedding, 0.95)
	if err == nil && cachedResp != nil {
		// Cache hit: return cached response
		return cachedResp.Response, nil
	}
	if err != nil && err != ErrCacheMiss {
		// Log unexpected errors but continue with LLM call
		fmt.Fprintf(os.Stderr, "cache lookup error: %v\n", err)
	}

	// Cache miss: call LLM to generate response
	resp, err := a.client.Chat(ctx, a.model, []endpoint.ChatMessage{
		{Role: "user", Content: prompt},
	}, &endpoint.ChatOptions{
		MaxTokens:   a.maxTokens,
		Temperature: a.temperature,
	})
	if err != nil {
		return "", fmt.Errorf("llm chat: %w", err)
	}

	// Calculate latency
	latencyMs := int(time.Since(startTime).Milliseconds())

	// Cache the response with embedding
	cacheEntry := &CachedResponse{
		Model:      a.model,
		Response:   resp.Content,
		TokensUsed: resp.TokensUsed.PromptTokens + resp.TokensUsed.CompletionTokens,
		LatencyMs:  latencyMs,
		Metadata: map[string]interface{}{
			"temperature": a.temperature,
			"max_tokens":  a.maxTokens,
		},
	}

	if err := a.llmCache.Set(promptHash, cacheEntry, queryEmbedding); err != nil {
		// Log caching errors but don't fail the response
		fmt.Fprintf(os.Stderr, "cache storage error: %v\n", err)
	}

	return resp.Content, nil
}

// hashPrompt generates a SHA256 hash of the prompt for use as a cache key.
func hashPrompt(prompt string) string {
	hash := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("%x", hash)
}

// loadAPIKey loads API key from environment based on provider.
func loadAPIKey(provider string) string {
	switch provider {
	case "zai":
		return os.Getenv("ZAI_API_KEY")
	case "openai":
		return os.Getenv("OPENAI_API_KEY")
	case "anthropic":
		return os.Getenv("ANTHROPIC_API_KEY")
	default:
		return ""
	}
}

// defaultEndpoint returns the default endpoint URL for a provider.
func defaultEndpoint(provider string) string {
	switch provider {
	case "zai":
		return endpoint.ZaiDefaultEndpoint
	case "openai":
		return "https://api.openai.com"
	case "anthropic":
		return "https://api.anthropic.com"
	case "ollama":
		return "http://localhost:11434"
	default:
		return ""
	}
}

// Compile-time verification that Adapter implements generator.LLM
var _ generator.LLM = (*Adapter)(nil)
