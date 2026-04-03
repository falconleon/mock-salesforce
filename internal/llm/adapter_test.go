package llm

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/falconleon/mock-salesforce/internal/endpoint"
	"github.com/falconleon/mock-salesforce/internal/generator"
)

// mockChatClient is a mock implementation of endpoint.ChatClient for testing.
type mockChatClient struct {
	response       string
	err            error
	callCount      int
	lastModel      string
	lastMsgs       []endpoint.ChatMessage
	lastOpts       *endpoint.ChatOptions
	embedErr       error
	embedCalls     int
	lastEmbedModel string
	lastEmbedTexts []string
}

func (m *mockChatClient) Chat(ctx context.Context, model string, messages []endpoint.ChatMessage, opts *endpoint.ChatOptions) (*endpoint.ChatResponse, error) {
	m.callCount++
	m.lastModel = model
	m.lastMsgs = messages
	m.lastOpts = opts

	if m.err != nil {
		return nil, m.err
	}

	return &endpoint.ChatResponse{
		Content:      m.response,
		Model:        model,
		TokensUsed:   endpoint.TokenUsage{PromptTokens: 10, CompletionTokens: 20, TotalTokens: 30},
		FinishReason: "stop",
	}, nil
}

func (m *mockChatClient) ChatStream(ctx context.Context, model string, messages []endpoint.ChatMessage, opts *endpoint.ChatOptions, callback func(chunk string) error) error {
	return errors.New("streaming not implemented in mock")
}

func (m *mockChatClient) Embed(ctx context.Context, model string, texts []string) ([][]float64, error) {
	m.embedCalls++
	m.lastEmbedModel = model
	m.lastEmbedTexts = texts

	if m.embedErr != nil {
		return nil, m.embedErr
	}

	// Return fake 768-dimensional embeddings (one per text)
	result := make([][]float64, len(texts))
	for i := range texts {
		embedding := make([]float64, 768)
		// Fill with simple pattern based on text content and index
		for j := range embedding {
			embedding[j] = float64((i+1)*(j+1)) * 0.001
		}
		result[i] = embedding
	}
	return result, nil
}

// mockLLMCache is a mock implementation of the LLMCache interface for testing specific behaviors.
type mockLLMCache struct {
	getCalled       bool
	cacheCalled     bool
	lastPromptHash  string
	lastEmbedding   []float64
	lastResponse    *CachedResponse
	shouldReturnHit bool
	hitResponse     *CachedResponse
}

func (m *mockLLMCache) FindSimilar(embedding []float64, threshold float64) (*CachedResponse, error) {
	m.getCalled = true
	m.lastEmbedding = embedding
	if m.shouldReturnHit {
		return m.hitResponse, nil
	}
	return nil, ErrCacheMiss
}

func (m *mockLLMCache) Set(key string, resp *CachedResponse, embedding []float64) error {
	m.cacheCalled = true
	m.lastPromptHash = key
	m.lastResponse = resp
	m.lastEmbedding = embedding
	return nil
}

// createTestCacheAdapter creates an in-memory cache for testing.
func createTestCacheAdapter(t *testing.T) *Cache {
	t.Helper()
	c, err := NewCache("") // Empty string uses :memory:
	if err != nil {
		t.Fatalf("Failed to create cache: %v", err)
	}
	t.Cleanup(func() {
		_ = c.Close()
	})
	return c
}

// --- Mock-based Unit Tests ---

func TestAdapter_Generate_WithMock(t *testing.T) {
	tests := []struct {
		name        string
		mockResp    string
		mockErr     error
		prompt      string
		wantContent string
		wantErr     bool
	}{
		{
			name:        "successful generation",
			mockResp:    "Hello, this is a test response!",
			prompt:      "Say hello",
			wantContent: "Hello, this is a test response!",
			wantErr:     false,
		},
		{
			name:        "JSON response for generator",
			mockResp:    `{"name": "Acme Corp", "industry": "Technology"}`,
			prompt:      "Generate a company",
			wantContent: `{"name": "Acme Corp", "industry": "Technology"}`,
			wantErr:     false,
		},
		{
			name:    "LLM error propagates",
			mockErr: errors.New("API rate limit exceeded"),
			prompt:  "Test",
			wantErr: true,
		},
		{
			name:        "empty prompt handled",
			mockResp:    "Response to empty",
			prompt:      "",
			wantContent: "Response to empty",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatMock := &mockChatClient{
				response: tt.mockResp,
				err:      tt.mockErr,
			}

			cache := createTestCacheAdapter(t)

			adapter := &Adapter{
				client:      chatMock,
				model:       "test-model",
				temperature: 0.7,
				maxTokens:   1000,
				timeout:     30 * time.Second,
				llmCache:    cache,
			}

			got, err := adapter.Generate(tt.prompt)

			if (err != nil) != tt.wantErr {
				t.Errorf("Generate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && got != tt.wantContent {
				t.Errorf("Generate() = %q, want %q", got, tt.wantContent)
			}

			// Verify chat mock was called correctly
			if chatMock.callCount != 1 {
				t.Errorf("expected 1 chat call, got %d", chatMock.callCount)
			}

			if chatMock.lastModel != "test-model" {
				t.Errorf("expected model 'test-model', got %q", chatMock.lastModel)
			}

			if len(chatMock.lastMsgs) != 1 || chatMock.lastMsgs[0].Role != "user" || chatMock.lastMsgs[0].Content != tt.prompt {
				t.Errorf("unexpected messages: %+v", chatMock.lastMsgs)
			}

			// Verify embeddings were generated
			if chatMock.embedCalls != 1 {
				t.Errorf("expected 1 embed call, got %d", chatMock.embedCalls)
			}
		})
	}
}

func TestAdapter_Generate_OptionsPassedCorrectly(t *testing.T) {
	chatMock := &mockChatClient{response: "OK"}
	cache := createTestCacheAdapter(t)

	adapter := &Adapter{
		client:      chatMock,
		model:       "custom-model",
		temperature: 0.5,
		maxTokens:   2048,
		timeout:     60 * time.Second,
		llmCache:    cache,
	}

	_, err := adapter.Generate("Test prompt")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify options were passed
	if chatMock.lastOpts == nil {
		t.Fatal("expected options to be passed")
	}

	if chatMock.lastOpts.MaxTokens != 2048 {
		t.Errorf("MaxTokens = %d, want 2048", chatMock.lastOpts.MaxTokens)
	}

	if chatMock.lastOpts.Temperature != 0.5 {
		t.Errorf("Temperature = %v, want 0.5", chatMock.lastOpts.Temperature)
	}
}

func TestAdapter_Generate_Timeout(t *testing.T) {
	// Test that timeout is respected (we can't easily test actual timeout without a real delay)
	cache := createTestCacheAdapter(t)

	adapter := &Adapter{
		client:      &mockChatClient{response: "OK"},
		model:       "model",
		temperature: 0.7,
		maxTokens:   100,
		timeout:     5 * time.Second,
		llmCache:    cache,
	}

	// Just verify it works with the timeout set
	_, err := adapter.Generate("Test")
	if err != nil {
		t.Errorf("Generate() error = %v", err)
	}
}

func TestAdapter_ImplementsGeneratorLLM(t *testing.T) {
	// Compile-time check that Adapter implements generator.LLM
	var _ generator.LLM = (*Adapter)(nil)
}

// --- Embedding and Cache Tests ---

func TestAdapter_Generate_WithEmbeddings_CacheMiss(t *testing.T) {
	prompt := "Test prompt for embedding"
	expectedResponse := "Generated response"

	chatMock := &mockChatClient{
		response: expectedResponse,
	}

	cache := createTestCacheAdapter(t)

	adapter := &Adapter{
		client:      chatMock,
		model:       "test-model",
		temperature: 0.7,
		maxTokens:   1000,
		timeout:     30 * time.Second,
		llmCache:    cache,
	}

	got, err := adapter.Generate(prompt)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if got != expectedResponse {
		t.Errorf("Generate() = %q, want %q", got, expectedResponse)
	}

	// Verify embedding was generated
	if chatMock.embedCalls != 1 {
		t.Errorf("expected 1 embed call, got %d", chatMock.embedCalls)
	}

	if chatMock.lastEmbedModel != "qwen3-embedding:0.6b" {
		t.Errorf("expected embedding model 'qwen3-embedding:0.6b', got %q", chatMock.lastEmbedModel)
	}

	if len(chatMock.lastEmbedTexts) != 1 || chatMock.lastEmbedTexts[0] != prompt {
		t.Errorf("unexpected embed texts: %+v", chatMock.lastEmbedTexts)
	}

	// Verify LLM was called (cache miss - no match found)
	if chatMock.callCount != 1 {
		t.Errorf("expected 1 LLM call on cache miss, got %d", chatMock.callCount)
	}
}

func TestAdapter_Generate_WithEmbeddings_CacheHit(t *testing.T) {
	ctx := context.Background()

	prompt := "Test prompt for cache hit"
	cachedResponse := "Cached response from database"

	// First, pre-populate the cache with a response
	cacheAdapter := createTestCacheAdapter(t)

	// Generate embedding using the mock client
	chatMock := &mockChatClient{response: cachedResponse}
	embeddings, err := chatMock.Embed(ctx, "qwen3-embedding:0.6b", []string{prompt})
	if err != nil {
		t.Fatalf("Embed() error = %v", err)
	}

	// Cache the response with the embedding
	promptHash := hashPrompt(prompt)
	cacheEntry := &CachedResponse{
		Model:      "test-model",
		Response:   cachedResponse,
		TokensUsed: 30,
		LatencyMs:  50,
	}

	err = cacheAdapter.Set(promptHash, cacheEntry, embeddings[0])
	if err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// Now test that Generate returns the cached response
	chatMock2 := &mockChatClient{
		response: "This should not be called",
	}

	adapter := &Adapter{
		client:      chatMock2,
		model:       "test-model",
		temperature: 0.7,
		maxTokens:   1000,
		timeout:     30 * time.Second,
		llmCache:    cacheAdapter,
	}

	got, err := adapter.Generate(prompt)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if got != cachedResponse {
		t.Errorf("Generate() = %q, want %q (should return cached response)", got, cachedResponse)
	}

	// Verify embedding was still generated for similarity search
	if chatMock2.embedCalls != 1 {
		t.Errorf("expected 1 embed call, got %d", chatMock2.embedCalls)
	}

	// Verify LLM was NOT called (cache hit)
	if chatMock2.callCount != 0 {
		t.Errorf("expected 0 LLM calls on cache hit, got %d", chatMock2.callCount)
	}
}

func TestAdapter_Generate_EmbeddingError(t *testing.T) {
	prompt := "Test prompt"
	embeddingErr := errors.New("embedding service error")

	chatMock := &mockChatClient{
		response: "Should not be called",
		embedErr: embeddingErr,
	}

	cacheMock := &mockLLMCache{}

	adapter := &Adapter{
		client:      chatMock,
		model:       "test-model",
		temperature: 0.7,
		maxTokens:   1000,
		timeout:     30 * time.Second,
		llmCache:    cacheMock,
	}

	_, err := adapter.Generate(prompt)
	if err == nil {
		t.Error("Generate() error = nil, wantErr true")
	}

	if !strings.Contains(err.Error(), "embedding") {
		t.Errorf("expected embedding error, got: %v", err)
	}

	// Verify LLM was never called
	if chatMock.callCount != 0 {
		t.Errorf("expected 0 LLM calls on embedding error, got %d", chatMock.callCount)
	}

	// Verify response was not cached
	if cacheMock.cacheCalled {
		t.Error("cache Set should not have been called on embedding error")
	}
}

func TestAdapter_Generate_EmbeddingCorrectDimensions(t *testing.T) {
	tests := []struct {
		name       string
		prompts    []string
		wantCached bool
	}{
		{
			name:       "single prompt returns 768 dimensions",
			prompts:    []string{"Hello world"},
			wantCached: true,
		},
		{
			name:       "multiple prompts each 768 dimensions",
			prompts:    []string{"Prompt 1", "Prompt 2", "Prompt 3"},
			wantCached: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chatMock := &mockChatClient{response: "OK"}
			cacheMock := &mockLLMCache{}

			adapter := &Adapter{
				client:      chatMock,
				model:       "test-model",
				temperature: 0.7,
				maxTokens:   1000,
				timeout:     30 * time.Second,
				llmCache:    cacheMock,
			}

			// Generate with first prompt only (Adapter.Generate takes single prompt)
			_, err := adapter.Generate(tt.prompts[0])
			if err != nil {
				t.Fatalf("Generate() error = %v", err)
			}

			// Verify embedding dimensions
			if len(cacheMock.lastEmbedding) != 768 {
				t.Errorf("embedding dimensions = %d, want 768", len(cacheMock.lastEmbedding))
			}

			// Verify embedding values are within expected range
			for i, val := range cacheMock.lastEmbedding {
				if val < 0 || val > 1 {
					t.Errorf("embedding[%d] = %v, want value between 0 and 1", i, val)
				}
			}
		})
	}
}

func TestAdapter_Generate_CacheResponseMetadata(t *testing.T) {
	prompt := "Test prompt"
	expectedResponse := "Test response"

	chatMock := &mockChatClient{
		response: expectedResponse,
	}

	cacheMock := &mockLLMCache{}

	adapter := &Adapter{
		client:      chatMock,
		model:       "gpt-4-turbo",
		temperature: 0.8,
		maxTokens:   2048,
		timeout:     30 * time.Second,
		llmCache:    cacheMock,
	}

	_, err := adapter.Generate(prompt)
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	// Verify cached response has correct metadata
	if cacheMock.lastResponse == nil {
		t.Fatal("lastResponse is nil")
	}

	if cacheMock.lastResponse.Model != "gpt-4-turbo" {
		t.Errorf("cached model = %q, want %q", cacheMock.lastResponse.Model, "gpt-4-turbo")
	}

	if cacheMock.lastResponse.Response != expectedResponse {
		t.Errorf("cached response = %q, want %q", cacheMock.lastResponse.Response, expectedResponse)
	}

	// Verify metadata contains temperature and max_tokens
	if cacheMock.lastResponse.Metadata == nil {
		t.Fatal("metadata is nil")
	}

	if temp, ok := cacheMock.lastResponse.Metadata["temperature"]; !ok {
		t.Error("metadata missing 'temperature' key")
	} else if temp != 0.8 {
		t.Errorf("metadata temperature = %v, want 0.8", temp)
	}

	if maxTokens, ok := cacheMock.lastResponse.Metadata["max_tokens"]; !ok {
		t.Error("metadata missing 'max_tokens' key")
	} else if maxTokens != 2048 {
		t.Errorf("metadata max_tokens = %v, want 2048", maxTokens)
	}
}

// --- Integration Test (with real API, skipped by default) ---

func TestAdapter_Generate_Zai(t *testing.T) {
	// Skip if no API key is set
	apiKey := os.Getenv("ZAI_API_KEY")
	if apiKey == "" {
		t.Skip("ZAI_API_KEY not set - skipping integration test")
	}

	cfg := DefaultConfig()
	cfg.APIKey = apiKey
	cfg.MaxTokens = 100 // Keep responses short for testing

	adapter, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	// Simple test prompt
	resp, err := adapter.Generate("What is 2+2? Answer with just the number.")
	if err != nil {
		t.Fatalf("Generate() error = %v", err)
	}

	if resp == "" {
		t.Error("Generate() returned empty response")
	}

	// Check if response contains expected answer
	if !strings.Contains(resp, "4") {
		t.Logf("Response: %s", resp)
		t.Error("Generate() response does not contain '4'")
	}

	t.Logf("Z.ai response: %s", resp)
}

// --- Config Tests ---

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Provider != "zai" {
		t.Errorf("DefaultConfig().Provider = %q, want %q", cfg.Provider, "zai")
	}

	if cfg.Model != "GLM-4.7" {
		t.Errorf("DefaultConfig().Model = %q, want %q", cfg.Model, "GLM-4.7")
	}

	if cfg.Temperature != 0.7 {
		t.Errorf("DefaultConfig().Temperature = %v, want %v", cfg.Temperature, 0.7)
	}

	if cfg.MaxTokens != 4096 {
		t.Errorf("DefaultConfig().MaxTokens = %d, want 4096", cfg.MaxTokens)
	}

	if cfg.Timeout != 120*time.Second {
		t.Errorf("DefaultConfig().Timeout = %v, want %v", cfg.Timeout, 120*time.Second)
	}
}

func TestLoadAPIKey(t *testing.T) {
	tests := []struct {
		provider string
		envVar   string
	}{
		{"zai", "ZAI_API_KEY"},
		{"openai", "OPENAI_API_KEY"},
		{"anthropic", "ANTHROPIC_API_KEY"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			if tt.envVar != "" {
				// Set a test value
				original := os.Getenv(tt.envVar)
				os.Setenv(tt.envVar, "test-key-"+tt.provider)
				defer os.Setenv(tt.envVar, original)

				key := loadAPIKey(tt.provider)
				if key != "test-key-"+tt.provider {
					t.Errorf("loadAPIKey(%q) = %q, want %q", tt.provider, key, "test-key-"+tt.provider)
				}
			} else {
				key := loadAPIKey(tt.provider)
				if key != "" {
					t.Errorf("loadAPIKey(%q) = %q, want empty", tt.provider, key)
				}
			}
		})
	}
}

func TestDefaultEndpoint(t *testing.T) {
	tests := []struct {
		provider string
		want     string
	}{
		{"zai", endpoint.ZaiDefaultEndpoint},
		{"openai", "https://api.openai.com"},
		{"anthropic", "https://api.anthropic.com"},
		{"ollama", "http://localhost:11434"},
		{"unknown", ""},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			got := defaultEndpoint(tt.provider)
			if got != tt.want {
				t.Errorf("defaultEndpoint(%q) = %q, want %q", tt.provider, got, tt.want)
			}
		})
	}
}
