package endpoint

import (
	"errors"
	"os"
	"strings"
	"sync"
)

// Model registry errors.
var (
	// ErrModelProfileNotFound is returned when a model profile is not registered.
	ErrModelProfileNotFound = errors.New("model profile not found")

	// ErrProviderNotSupported is returned when a provider mapping is not defined for a model.
	ErrProviderNotSupported = errors.New("provider not supported for this model")

	// ErrDuplicateModelProfile is returned when attempting to register a duplicate profile.
	ErrDuplicateModelProfile = errors.New("model profile already registered")

	// ErrInvalidModelProfile is returned when a model profile has invalid configuration.
	ErrInvalidModelProfile = errors.New("invalid model profile")
)

// ModelProfile defines a canonical model with provider-specific mappings.
type ModelProfile struct {
	// CanonicalID is the unified model identifier (e.g., "qwen3-30b-instruct")
	CanonicalID string `json:"canonical_id"`

	// DisplayName is the human-readable model name
	DisplayName string `json:"display_name"`

	// MaxContextTokens is the maximum context window size
	MaxContextTokens int `json:"max_context_tokens"`

	// MaxOutputTokens is the maximum output token limit
	MaxOutputTokens int `json:"max_output_tokens"`

	// SupportsStreaming indicates if the model supports streaming responses
	SupportsStreaming bool `json:"supports_streaming"`

	// SupportsEmbedding indicates if the model supports embedding generation
	SupportsEmbedding bool `json:"supports_embedding"`

	// SupportsReasoning indicates if the model supports extended thinking/reasoning
	SupportsReasoning bool `json:"supports_reasoning"`

	// SupportsVision indicates if the model supports vision/image input
	SupportsVision bool `json:"supports_vision"`

	// ProviderModels maps provider names to provider-specific model identifiers
	// e.g., {"ollama": "qwen3:30b-a3b-instruct", "vllm": "Qwen/Qwen3-30B-A3B-Instruct"}
	ProviderModels map[string]string `json:"provider_models"`

	// DefaultTemperature is the recommended temperature for this model
	DefaultTemperature float64 `json:"default_temperature"`
}

// Validate checks if the model profile has valid configuration.
func (p *ModelProfile) Validate() error {
	if p.CanonicalID == "" {
		return ErrInvalidModelProfile
	}
	if p.ProviderModels == nil || len(p.ProviderModels) == 0 {
		return ErrInvalidModelProfile
	}
	return nil
}

// ModelRegistry manages model profiles and provides translation between
// canonical model names and provider-specific identifiers.
type ModelRegistry struct {
	mu       sync.RWMutex
	profiles map[string]*ModelProfile

	// reverseIndex maps provider model names to canonical IDs for AutoDetect
	reverseIndex map[string]string
}

// NewModelRegistry creates a new ModelRegistry with pre-registered default models.
func NewModelRegistry() *ModelRegistry {
	r := &ModelRegistry{
		profiles:     make(map[string]*ModelProfile),
		reverseIndex: make(map[string]string),
	}
	r.registerDefaults()
	return r
}

// NewEmptyModelRegistry creates a new ModelRegistry without pre-registered models.
func NewEmptyModelRegistry() *ModelRegistry {
	return &ModelRegistry{
		profiles:     make(map[string]*ModelProfile),
		reverseIndex: make(map[string]string),
	}
}

// Register adds a new model profile to the registry.
func (r *ModelRegistry) Register(profile *ModelProfile) error {
	if err := profile.Validate(); err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.profiles[profile.CanonicalID]; exists {
		return ErrDuplicateModelProfile
	}

	r.profiles[profile.CanonicalID] = profile

	// Build reverse index for AutoDetect
	for _, providerModel := range profile.ProviderModels {
		normalizedModel := strings.ToLower(providerModel)
		r.reverseIndex[normalizedModel] = profile.CanonicalID
	}

	return nil
}

// ResolveModel translates a canonical model ID to a provider-specific model name.
func (r *ModelRegistry) ResolveModel(canonicalID, provider string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	profile, exists := r.profiles[canonicalID]
	if !exists {
		return "", ErrModelProfileNotFound
	}

	providerModel, exists := profile.ProviderModels[provider]
	if !exists {
		return "", ErrProviderNotSupported
	}

	return providerModel, nil
}

// GetProfile retrieves a model profile by canonical ID.
func (r *ModelRegistry) GetProfile(canonicalID string) (*ModelProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	profile, exists := r.profiles[canonicalID]
	if !exists {
		return nil, ErrModelProfileNotFound
	}

	return profile, nil
}

// AutoDetect attempts to find a model profile from a provider-specific model name.
func (r *ModelRegistry) AutoDetect(providerModel string) (*ModelProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	normalizedModel := strings.ToLower(providerModel)
	canonicalID, exists := r.reverseIndex[normalizedModel]
	if !exists {
		return nil, ErrModelProfileNotFound
	}

	return r.profiles[canonicalID], nil
}

// ListProfiles returns all registered model profiles.
func (r *ModelRegistry) ListProfiles() []*ModelProfile {
	r.mu.RLock()
	defer r.mu.RUnlock()

	profiles := make([]*ModelProfile, 0, len(r.profiles))
	for _, p := range r.profiles {
		profiles = append(profiles, p)
	}
	return profiles
}

// HasProvider checks if the canonical model supports a specific provider.
func (r *ModelRegistry) HasProvider(canonicalID, provider string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	profile, exists := r.profiles[canonicalID]
	if !exists {
		return false
	}

	_, hasProvider := profile.ProviderModels[provider]
	return hasProvider
}

// registerDefaults registers the built-in model profiles.
func (r *ModelRegistry) registerDefaults() {
	// Claude models (Anthropic provider)
	r.registerNoLock(&ModelProfile{
		CanonicalID:       "claude-sonnet",
		DisplayName:       "Claude 4.5 Sonnet",
		MaxContextTokens:  200000,
		MaxOutputTokens:   8192,
		SupportsStreaming: true,
		SupportsReasoning: true,
		ProviderModels:    map[string]string{"anthropic": "claude-sonnet-4-5"},
		DefaultTemperature: 0.7,
	})

	r.registerNoLock(&ModelProfile{
		CanonicalID:       "claude-opus",
		DisplayName:       "Claude 4.5 Opus",
		MaxContextTokens:  200000,
		MaxOutputTokens:   8192,
		SupportsStreaming: true,
		SupportsReasoning: true,
		ProviderModels:    map[string]string{"anthropic": "claude-opus-4-5"},
		DefaultTemperature: 0.7,
	})

	r.registerNoLock(&ModelProfile{
		CanonicalID:       "claude-haiku",
		DisplayName:       "Claude 3.5 Haiku",
		MaxContextTokens:  200000,
		MaxOutputTokens:   8192,
		SupportsStreaming: true,
		SupportsReasoning: false,
		ProviderModels:    map[string]string{"anthropic": "claude-haiku-3-5"},
		DefaultTemperature: 0.7,
	})

	// GLM models (Z.ai provider)
	r.registerNoLock(&ModelProfile{
		CanonicalID:       "glm-4.7",
		DisplayName:       "GLM-4.7 (Z.ai Flagship)",
		MaxContextTokens:  128000,
		MaxOutputTokens:   8000,
		SupportsStreaming: true,
		SupportsReasoning: true,
		ProviderModels:    map[string]string{"zai": "GLM-4.7"},
		DefaultTemperature: 0.7,
	})

	r.registerNoLock(&ModelProfile{
		CanonicalID:       "glm-vision",
		DisplayName:       "GLM-4.6V Vision",
		MaxContextTokens:  128000,
		MaxOutputTokens:   8000,
		SupportsStreaming: true,
		SupportsVision:    true,
		ProviderModels:    map[string]string{"zai": "GLM-4.6V"},
		DefaultTemperature: 0.7,
	})

	r.registerNoLock(&ModelProfile{
		CanonicalID:        "glm-5",
		DisplayName:        "GLM-5",
		MaxContextTokens:   128000,
		MaxOutputTokens:    8000,
		SupportsStreaming:   true,
		SupportsEmbedding:  false,
		SupportsReasoning:  true,
		SupportsVision:     false,
		ProviderModels: map[string]string{
			"zai": "GLM-5",
		},
		DefaultTemperature: 0.7,
	})

	// Qwen models (Ollama, vLLM providers)
	r.registerNoLock(&ModelProfile{
		CanonicalID:       "qwen3-30b-instruct",
		DisplayName:       "Qwen3 30B Instruct",
		MaxContextTokens:  32768,
		MaxOutputTokens:   8192,
		SupportsStreaming: true,
		SupportsReasoning: true,
		ProviderModels: map[string]string{
			"ollama": ollamaQwenModel(),
			"vllm":   "Qwen/Qwen3-30B-A3B-Instruct-2507",
		},
		DefaultTemperature: 0.7,
	})

	// Embedding models
	r.registerNoLock(&ModelProfile{
		CanonicalID:       "nomic-embed-text",
		DisplayName:       "Nomic Embed Text",
		MaxContextTokens:  8192,
		MaxOutputTokens:   0,
		SupportsEmbedding: true,
		ProviderModels: map[string]string{
			"ollama": "nomic-embed-text:137m-v1.5-fp16",
		},
	})
}

func ollamaQwenModel() string {
	if m := os.Getenv("OLLAMA_QWEN_MODEL"); m != "" {
		return m
	}
	return "qwen3:30b-a3b-instruct-2507-q4_K_M"
}

// ModelProfileData carries model profile data from an external source (DB).
// Used by LoadFromDB to register profiles without importing localstore.
type ModelProfileData struct {
	CanonicalID       string
	DisplayName       string
	ProviderModels    map[string]string
	MaxContext        int
	MaxOutput         int
	Temperature       float64
	SupportsStreaming  bool
	SupportsReasoning bool
	SupportsEmbedding bool
}

// LoadFromDB registers model profiles from external data, replacing any
// existing profiles with the same CanonicalID. Call after registerDefaults
// to override hardcoded entries with DB-sourced ones.
func (r *ModelRegistry) LoadFromDB(profiles []ModelProfileData) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	loaded := 0
	for _, p := range profiles {
		r.registerNoLock(&ModelProfile{
			CanonicalID:        p.CanonicalID,
			DisplayName:        p.DisplayName,
			MaxContextTokens:   p.MaxContext,
			MaxOutputTokens:    p.MaxOutput,
			SupportsStreaming:   p.SupportsStreaming,
			SupportsReasoning:  p.SupportsReasoning,
			SupportsEmbedding:  p.SupportsEmbedding,
			ProviderModels:     p.ProviderModels,
			DefaultTemperature: p.Temperature,
		})
		loaded++
	}
	return loaded
}

// registerNoLock registers a profile without acquiring the lock.
// Used internally during initialization.
func (r *ModelRegistry) registerNoLock(profile *ModelProfile) {
	r.profiles[profile.CanonicalID] = profile

	for _, providerModel := range profile.ProviderModels {
		normalizedModel := strings.ToLower(providerModel)
		r.reverseIndex[normalizedModel] = profile.CanonicalID
	}
}

