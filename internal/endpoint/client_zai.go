// Package endpoint provides LLM endpoint management with multi-provider support.
package endpoint

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ZaiClient implements ChatClientAdapter for Z.ai's GLM models.
// Supports thinking control and vision models with content blocks.
type ZaiClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
	chatPath   string
}

// ZaiMessage represents a message in Z.ai's chat format.
// Content can be a string for text-only or []ZaiContentBlock for vision.
type ZaiMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []ZaiContentBlock
}

// ZaiContentBlock represents a content block for multi-modal messages.
type ZaiContentBlock struct {
	Type     string       `json:"type"`                // "text" or "image_url"
	Text     string       `json:"text,omitempty"`      // for type="text"
	ImageURL *ZaiImageURL `json:"image_url,omitempty"` // for type="image_url"
}

// ZaiImageURL contains the image URL data.
type ZaiImageURL struct {
	URL string `json:"url"` // data:image/png;base64,... or https://...
}

// ZaiThinking controls the thinking mode for reasoning models.
type ZaiThinking struct {
	Type string `json:"type"` // "enabled" or "disabled"
}

// ZaiRequest represents a chat completion request to Z.ai.
type ZaiRequest struct {
	Model       string       `json:"model"`
	Messages    []ZaiMessage `json:"messages"`
	MaxTokens   int          `json:"max_tokens"`
	Temperature float64      `json:"temperature"`
	Stream      bool         `json:"stream"`
	Thinking    *ZaiThinking `json:"thinking,omitempty"`
}

// ZaiResponse represents a chat completion response from Z.ai.
type ZaiResponse struct {
	ID      string      `json:"id"`
	Object  string      `json:"object"`
	Created int64       `json:"created"`
	Model   string      `json:"model"`
	Choices []ZaiChoice `json:"choices"`
	Usage   ZaiUsage    `json:"usage"`
}

// ZaiChoice represents a single choice in the response.
type ZaiChoice struct {
	Index        int        `json:"index"`
	Message      ZaiRespMsg `json:"message"`
	FinishReason string     `json:"finish_reason"`
}

// ZaiRespMsg represents the message content in a response.
type ZaiRespMsg struct {
	Role             string `json:"role"`
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content,omitempty"`
}

// ZaiUsage contains token usage information.
type ZaiUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ZaiStreamChunk represents a streaming chunk from Z.ai.
type ZaiStreamChunk struct {
	ID      string           `json:"id"`
	Object  string           `json:"object"`
	Created int64            `json:"created"`
	Model   string           `json:"model"`
	Choices []ZaiStreamDelta `json:"choices"`
}

// ZaiStreamDelta represents the delta in a streaming chunk.
type ZaiStreamDelta struct {
	Index int `json:"index"`
	Delta struct {
		Content          string `json:"content,omitempty"`
		ReasoningContent string `json:"reasoning_content,omitempty"`
	} `json:"delta"`
	FinishReason string `json:"finish_reason,omitempty"`
}

const (
	// ZaiDefaultEndpoint is the Z.ai API endpoint.
	ZaiDefaultEndpoint = "https://api.z.ai"
	// ZaiChatPath is the chat completions API path.
	ZaiChatPath = "/api/coding/paas/v4/chat/completions"
)

// NewZaiClient creates a new Z.ai client adapter.
func NewZaiClient(apiKey string) *ZaiClient {
	return &ZaiClient{
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
		baseURL: ZaiDefaultEndpoint,
		apiKey:  apiKey,
	}
}

// NewZaiClientWithEndpoint creates a Z.ai client with a custom endpoint.
func NewZaiClientWithEndpoint(apiKey, baseURL string) *ZaiClient {
	client := NewZaiClient(apiKey)
	client.baseURL = strings.TrimSuffix(baseURL, "/")
	return client
}

// effectiveChatPath returns the configured chat path, falling back to ZaiChatPath.
func (c *ZaiClient) effectiveChatPath() string {
	if c.chatPath != "" {
		return c.chatPath
	}
	return ZaiChatPath
}

// ProviderName returns the provider identifier.
func (c *ZaiClient) ProviderName() string {
	return "zai"
}

// Chat sends a chat completion request to Z.ai.
func (c *ZaiClient) Chat(ctx context.Context, model string, messages []ChatMessage, opts *ChatOptions) (*ChatResponse, error) {
	if opts == nil {
		opts = &ChatOptions{}
	}

	// Convert messages to Z.ai format
	zaiMessages := c.convertMessages(messages)

	// Build request with thinking control
	req := ZaiRequest{
		Model:       model,
		Messages:    zaiMessages,
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
		Stream:      false,
		Thinking:    c.buildThinking(opts.EnableThinking),
	}

	// Set defaults
	if req.MaxTokens == 0 {
		req.MaxTokens = 8000
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("zai: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+c.effectiveChatPath(), bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("zai: failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("zai: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("zai: API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var zaiResp ZaiResponse
	if err := json.NewDecoder(resp.Body).Decode(&zaiResp); err != nil {
		return nil, fmt.Errorf("zai: failed to decode response: %w", err)
	}

	return c.convertResponse(&zaiResp, opts.EnableThinking), nil
}

// ChatStream sends a streaming chat request to Z.ai.
func (c *ZaiClient) ChatStream(ctx context.Context, model string, messages []ChatMessage, opts *ChatOptions, callback func(chunk string) error) error {
	if opts == nil {
		opts = &ChatOptions{}
	}

	// Convert messages to Z.ai format
	zaiMessages := c.convertMessages(messages)

	// Build request with thinking control
	req := ZaiRequest{
		Model:       model,
		Messages:    zaiMessages,
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
		Stream:      true,
		Thinking:    c.buildThinking(opts.EnableThinking),
	}

	// Set defaults
	if req.MaxTokens == 0 {
		req.MaxTokens = 8000
	}

	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("zai: failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+c.effectiveChatPath(), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("zai: failed to create request: %w", err)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("zai: request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("zai: API error %d: %s", resp.StatusCode, string(bodyBytes))
	}

	return c.parseStream(resp.Body, opts.EnableThinking, callback)
}

// Embed generates embeddings (not supported by Z.ai).
func (c *ZaiClient) Embed(_ context.Context, _ string, _ []string) ([][]float64, error) {
	return nil, fmt.Errorf("zai: embeddings not supported")
}

// buildThinking creates the thinking control parameter.
// Default to disabled for production efficiency (74x token savings).
func (c *ZaiClient) buildThinking(enabled bool) *ZaiThinking {
	if enabled {
		return &ZaiThinking{Type: "enabled"}
	}
	return &ZaiThinking{Type: "disabled"}
}

// convertMessages converts ChatMessage to ZaiMessage format.
func (c *ZaiClient) convertMessages(messages []ChatMessage) []ZaiMessage {
	zaiMessages := make([]ZaiMessage, len(messages))
	for i, msg := range messages {
		zaiMessages[i] = ZaiMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}
	return zaiMessages
}

// convertResponse converts ZaiResponse to ChatResponse.
func (c *ZaiClient) convertResponse(resp *ZaiResponse, enableThinking bool) *ChatResponse {
	result := &ChatResponse{
		Model: resp.Model,
		TokensUsed: TokenUsage{
			PromptTokens:     resp.Usage.PromptTokens,
			CompletionTokens: resp.Usage.CompletionTokens,
			TotalTokens:      resp.Usage.TotalTokens,
		},
	}

	if len(resp.Choices) > 0 {
		choice := resp.Choices[0]
		result.Content = choice.Message.Content
		result.FinishReason = choice.FinishReason

		// Include reasoning content if thinking was enabled
		if enableThinking && choice.Message.ReasoningContent != "" {
			result.Thinking = choice.Message.ReasoningContent
		}
	}

	return result
}

// parseStream parses SSE stream and invokes callback for each chunk.
func (c *ZaiClient) parseStream(body io.Reader, enableThinking bool, callback func(chunk string) error) error {
	scanner := bufio.NewScanner(body)
	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Check for SSE data prefix
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		// Extract JSON payload
		jsonStr := strings.TrimPrefix(line, "data: ")

		// Check for stream end
		if jsonStr == "[DONE]" {
			break
		}

		// Parse chunk
		var chunk ZaiStreamChunk
		if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
			// Skip malformed chunks
			continue
		}

		// Extract content from delta
		if len(chunk.Choices) > 0 {
			delta := chunk.Choices[0].Delta
			var text string

			// When thinking enabled, prefer reasoning_content, then content
			// When thinking disabled, prefer content, then reasoning_content
			if enableThinking {
				text = delta.ReasoningContent
				if text == "" {
					text = delta.Content
				}
			} else {
				text = delta.Content
				if text == "" {
					text = delta.ReasoningContent
				}
			}

			if text != "" {
				if err := callback(text); err != nil {
					return err
				}
			}
		}
	}

	return scanner.Err()
}

// BuildVisionMessage creates a ZaiMessage with text and image content blocks.
// Use this for vision model requests (e.g., GLM-4.6V).
func BuildVisionMessage(role, text, imageURL string) ZaiMessage {
	return ZaiMessage{
		Role: role,
		Content: []ZaiContentBlock{
			{Type: "text", Text: text},
			{Type: "image_url", ImageURL: &ZaiImageURL{URL: imageURL}},
		},
	}
}

// BuildVisionMessageMultiImage creates a ZaiMessage with text and multiple images.
func BuildVisionMessageMultiImage(role, text string, imageURLs []string) ZaiMessage {
	blocks := make([]ZaiContentBlock, 0, len(imageURLs)+1)
	blocks = append(blocks, ZaiContentBlock{Type: "text", Text: text})
	for _, url := range imageURLs {
		blocks = append(blocks, ZaiContentBlock{
			Type:     "image_url",
			ImageURL: &ZaiImageURL{URL: url},
		})
	}
	return ZaiMessage{
		Role:    role,
		Content: blocks,
	}
}
