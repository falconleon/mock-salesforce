// Package endpoint provides LLM endpoint management with multi-provider support.
package endpoint

import (
	"context"
	"fmt"
	"strings"
)

// ChatClientConfig configures the UnifiedChatClient.
type ChatClientConfig struct {
	// EndpointURL is the base URL for the LLM endpoint
	EndpointURL string

	// Provider explicitly sets the provider type. If empty, auto-detected from URL.
	// Valid values: "openai", "anthropic", "zai", "ollama"
	Provider string

	// APIKey is the authentication key for the provider
	APIKey string

	// ModelRegistry is an optional registry for canonical model name resolution
	ModelRegistry *ModelRegistry

	// ChatPath overrides the default chat completions path for this provider.
	// If empty, the adapter's default is used (e.g., "/v1/chat/completions" for OpenAI).
	ChatPath string

	// ExtraHeaders are additional HTTP headers sent with every request.
	// Used for provider-specific requirements (e.g., OpenRouter's HTTP-Referer).
	ExtraHeaders map[string]string
}

// UnifiedChatClient wraps provider-specific adapters with automatic detection and model resolution.
type UnifiedChatClient struct {
	adapter       ChatClientAdapter
	modelRegistry *ModelRegistry
}

// NewChatClient creates a new UnifiedChatClient with automatic provider detection.
func NewChatClient(cfg ChatClientConfig) (*UnifiedChatClient, error) {
	if cfg.EndpointURL == "" {
		return nil, fmt.Errorf("EndpointURL is required")
	}

	// Determine provider
	provider := cfg.Provider
	if provider == "" {
		provider = detectProvider(cfg.EndpointURL)
	}

	// Create appropriate adapter
	adapter, err := createAdapter(provider, cfg.EndpointURL, cfg.APIKey, cfg.ChatPath, cfg.ExtraHeaders)
	if err != nil {
		return nil, err
	}

	return &UnifiedChatClient{
		adapter:       adapter,
		modelRegistry: cfg.ModelRegistry,
	}, nil
}

// detectProvider determines the provider from URL patterns.
func detectProvider(url string) string {
	lower := strings.ToLower(url)

	// Anthropic detection
	if strings.Contains(lower, "api.anthropic.com") {
		return "anthropic"
	}

	// Z.ai detection
	if strings.Contains(lower, "api.z.ai") || strings.Contains(lower, "z.ai") {
		return "zai"
	}

	// OpenRouter detection (OpenAI-compatible API)
	if strings.Contains(lower, "openrouter.ai") {
		return "openai"
	}

	// Ollama detection (common ports)
	if strings.Contains(lower, ":11434") || strings.Contains(lower, ":11445") {
		return "ollama"
	}

	// Default to OpenAI-compatible (works for OpenAI, vLLM, DeepSeek, Groq)
	return "openai"
}

// createAdapter creates the appropriate ChatClientAdapter for the provider.
func createAdapter(provider, url, apiKey, chatPath string, extraHeaders map[string]string) (ChatClientAdapter, error) {
	switch provider {
	case "anthropic":
		return NewAnthropicClient(url, apiKey), nil
	case "zai":
		client := NewZaiClientWithEndpoint(apiKey, url)
		client.chatPath = chatPath
		return client, nil
	case "ollama":
		return NewOllamaClient(url, chatPath), nil
	case "openai", "openai-compat", "vllm", "deepseek", "groq", "openrouter":
		return NewOpenAICompatClient(url, apiKey, chatPath, extraHeaders, provider), nil
	default:
		// Unknown provider - use OpenAI-compatible as fallback
		return NewOpenAICompatClient(url, apiKey, chatPath, extraHeaders, provider), nil
	}
}

// resolveModel translates canonical model names to provider-specific names.
func (c *UnifiedChatClient) resolveModel(model string) string {
	if c.modelRegistry == nil {
		return model
	}

	providerName := c.adapter.ProviderName()
	resolved, err := c.modelRegistry.ResolveModel(model, providerName)
	if err != nil {
		// Model not in registry - use as-is (likely already provider-specific)
		return model
	}
	return resolved
}

// Chat sends a chat completion request and returns the response.
func (c *UnifiedChatClient) Chat(ctx context.Context, model string, messages []ChatMessage, opts *ChatOptions) (*ChatResponse, error) {
	resolvedModel := c.resolveModel(model)

	resp, err := c.adapter.Chat(ctx, resolvedModel, messages, opts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.adapter.ProviderName(), err)
	}

	return resp, nil
}

// ChatStream sends a streaming chat request and invokes callback for each chunk.
func (c *UnifiedChatClient) ChatStream(ctx context.Context, model string, messages []ChatMessage, opts *ChatOptions, callback func(chunk string) error) error {
	resolvedModel := c.resolveModel(model)

	if err := c.adapter.ChatStream(ctx, resolvedModel, messages, opts, callback); err != nil {
		return fmt.Errorf("%s: %w", c.adapter.ProviderName(), err)
	}

	return nil
}

// Embed generates embeddings for the given texts.
func (c *UnifiedChatClient) Embed(ctx context.Context, model string, texts []string) ([][]float64, error) {
	resolvedModel := c.resolveModel(model)

	embeddings, err := c.adapter.Embed(ctx, resolvedModel, texts)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", c.adapter.ProviderName(), err)
	}

	return embeddings, nil
}

// ProviderName returns the name of the underlying provider.
func (c *UnifiedChatClient) ProviderName() string {
	return c.adapter.ProviderName()
}

// Compile-time interface verification
var _ ChatClient = (*UnifiedChatClient)(nil)

