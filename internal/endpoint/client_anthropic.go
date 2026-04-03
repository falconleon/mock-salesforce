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

// anthropicClient implements ChatClientAdapter for the native Anthropic Messages API.
type anthropicClient struct {
	httpClient *http.Client
	baseURL    string
	apiKey     string
}

// NewAnthropicClient creates a new Anthropic client adapter.
func NewAnthropicClient(baseURL, apiKey string) ChatClientAdapter {
	return &anthropicClient{
		httpClient: &http.Client{Timeout: 300 * time.Second},
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		apiKey:     apiKey,
	}
}

// ProviderName returns "anthropic".
func (c *anthropicClient) ProviderName() string {
	return "anthropic"
}

// AnthropicRequest is the native Anthropic API request format.
type AnthropicRequest struct {
	Model       string              `json:"model"`
	Messages    []AnthropicMessage  `json:"messages"`
	MaxTokens   int                 `json:"max_tokens"`
	System      string              `json:"system,omitempty"`
	Temperature float64             `json:"temperature,omitempty"`
	Stream      bool                `json:"stream,omitempty"`
	StopSeqs    []string            `json:"stop_sequences,omitempty"`
	Thinking    *ThinkingConfig     `json:"thinking,omitempty"`
}

// AnthropicMessage represents a message in Anthropic format.
type AnthropicMessage struct {
	Role    string                  `json:"role"`    // "user" or "assistant" only
	Content []AnthropicContentBlock `json:"content"` // Always array of content blocks
}

// AnthropicContentBlock represents a content block in a message.
type AnthropicContentBlock struct {
	Type      string `json:"type"`                // "text", "thinking"
	Text      string `json:"text,omitempty"`
	Thinking  string `json:"thinking,omitempty"`
	Signature string `json:"signature,omitempty"`
}

// ThinkingConfig configures extended thinking mode.
type ThinkingConfig struct {
	Type         string `json:"type"`          // "enabled" or "disabled"
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// AnthropicResponse is the native Anthropic API response format.
type AnthropicResponse struct {
	ID         string                  `json:"id"`
	Type       string                  `json:"type"`        // "message"
	Role       string                  `json:"role"`        // "assistant"
	Content    []AnthropicContentBlock `json:"content"`
	Model      string                  `json:"model"`
	StopReason string                  `json:"stop_reason"`
	Usage      AnthropicUsage          `json:"usage"`
}

// AnthropicUsage contains token usage information.
type AnthropicUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// AnthropicErrorResponse represents an API error.
type AnthropicErrorResponse struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// convertToAnthropicFormat converts unified ChatMessages to Anthropic format.
// It extracts system messages to the top-level system field and converts
// user/assistant messages to the Anthropic content block format.
func convertToAnthropicFormat(messages []ChatMessage) (systemPrompt string, anthropicMsgs []AnthropicMessage) {
	var systemParts []string
	anthropicMsgs = make([]AnthropicMessage, 0, len(messages))

	for _, msg := range messages {
		switch msg.Role {
		case "system":
			// Collect system messages into a single system prompt
			systemParts = append(systemParts, msg.Content)
		case "user", "assistant":
			// Convert to Anthropic message with content blocks
			anthropicMsgs = append(anthropicMsgs, AnthropicMessage{
				Role: msg.Role,
				Content: []AnthropicContentBlock{
					{Type: "text", Text: msg.Content},
				},
			})
		}
	}

	// Join multiple system messages with newlines
	if len(systemParts) > 0 {
		systemPrompt = strings.Join(systemParts, "\n\n")
	}

	return systemPrompt, anthropicMsgs
}

// Chat sends a chat completion request using the native Anthropic Messages API.
func (c *anthropicClient) Chat(ctx context.Context, model string, messages []ChatMessage, opts *ChatOptions) (*ChatResponse, error) {
	systemPrompt, anthropicMsgs := convertToAnthropicFormat(messages)

	// Build request
	reqBody := AnthropicRequest{
		Model:     model,
		Messages:  anthropicMsgs,
		MaxTokens: 4096, // Anthropic requires max_tokens
		System:    systemPrompt,
	}

	if opts != nil {
		if opts.MaxTokens > 0 {
			reqBody.MaxTokens = opts.MaxTokens
		}
		if opts.Temperature > 0 {
			reqBody.Temperature = opts.Temperature
		}
		if len(opts.Stop) > 0 {
			reqBody.StopSeqs = opts.Stop
		}
		if opts.EnableThinking {
			budget := opts.ThinkingBudget
			if budget == 0 {
				budget = 10000 // Default thinking budget
			}
			reqBody.Thinking = &ThinkingConfig{Type: "enabled", BudgetTokens: budget}
		}
	}

	return c.doRequest(ctx, reqBody)
}

// doRequest performs the HTTP request and parses the response.
func (c *anthropicClient) doRequest(ctx context.Context, reqBody AnthropicRequest) (*ChatResponse, error) {
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, c.handleError(resp)
	}

	var apiResp AnthropicResponse
	if err := json.NewDecoder(resp.Body).Decode(&apiResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return c.convertResponse(&apiResp), nil
}

// setHeaders sets the required Anthropic API headers.
func (c *anthropicClient) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
}

// handleError parses and returns an appropriate error from the API response.
func (c *anthropicClient) handleError(resp *http.Response) error {
	bodyBytes, _ := io.ReadAll(resp.Body)

	var errResp AnthropicErrorResponse
	if err := json.Unmarshal(bodyBytes, &errResp); err == nil && errResp.Error.Message != "" {
		return fmt.Errorf("anthropic API error (%d): %s", resp.StatusCode, errResp.Error.Message)
	}

	// Handle specific status codes
	switch resp.StatusCode {
	case http.StatusUnauthorized:
		return NewEndpointError(c.baseURL, "anthropic", "Chat", ErrAuthenticationRequired)
	case http.StatusTooManyRequests:
		return NewEndpointError(c.baseURL, "anthropic", "Chat", ErrRateLimited)
	default:
		return fmt.Errorf("anthropic API error: status %d, body: %s", resp.StatusCode, string(bodyBytes))
	}
}

// convertResponse converts an Anthropic response to the unified ChatResponse format.
func (c *anthropicClient) convertResponse(apiResp *AnthropicResponse) *ChatResponse {
	var content, thinking string

	// Extract text and thinking content from blocks
	for _, block := range apiResp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "thinking":
			thinking += block.Thinking
		}
	}

	return &ChatResponse{
		Content: content,
		Model:   apiResp.Model,
		TokensUsed: TokenUsage{
			PromptTokens:     apiResp.Usage.InputTokens,
			CompletionTokens: apiResp.Usage.OutputTokens,
			TotalTokens:      apiResp.Usage.InputTokens + apiResp.Usage.OutputTokens,
		},
		FinishReason: apiResp.StopReason,
		Thinking:     thinking,
	}
}

// ChatStream sends a streaming chat request using the native Anthropic Messages API.
func (c *anthropicClient) ChatStream(ctx context.Context, model string, messages []ChatMessage, opts *ChatOptions, callback func(chunk string) error) error {
	systemPrompt, anthropicMsgs := convertToAnthropicFormat(messages)

	// Build request with streaming enabled
	reqBody := AnthropicRequest{
		Model:     model,
		Messages:  anthropicMsgs,
		MaxTokens: 4096,
		System:    systemPrompt,
		Stream:    true,
	}

	if opts != nil {
		if opts.MaxTokens > 0 {
			reqBody.MaxTokens = opts.MaxTokens
		}
		if opts.Temperature > 0 {
			reqBody.Temperature = opts.Temperature
		}
		if len(opts.Stop) > 0 {
			reqBody.StopSeqs = opts.Stop
		}
		if opts.EnableThinking {
			budget := opts.ThinkingBudget
			if budget == 0 {
				budget = 10000
			}
			reqBody.Thinking = &ThinkingConfig{Type: "enabled", BudgetTokens: budget}
		}
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	c.setHeaders(httpReq)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return c.handleError(resp)
	}

	return c.handleStreamResponse(resp.Body, callback)
}

// anthropicStreamEvent represents a streaming SSE event.
type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Delta *struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
}

// handleStreamResponse processes Anthropic SSE streaming events.
func (c *anthropicClient) handleStreamResponse(body io.Reader, callback func(chunk string) error) error {
	scanner := bufio.NewScanner(body)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip empty lines and event type lines
		if line == "" || strings.HasPrefix(line, "event:") {
			continue
		}

		// Parse data lines
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")

			// Check for end of stream
			if data == "[DONE]" {
				break
			}

			var event anthropicStreamEvent
			if err := json.Unmarshal([]byte(data), &event); err != nil {
				continue // Skip malformed events
			}

			// Handle content_block_delta events with text
			if event.Type == "content_block_delta" && event.Delta != nil && event.Delta.Type == "text_delta" {
				if err := callback(event.Delta.Text); err != nil {
					return err
				}
			}

			// Stop on message_stop
			if event.Type == "message_stop" {
				break
			}
		}
	}

	return scanner.Err()
}

// Embed generates embeddings - Anthropic does not support embeddings.
func (c *anthropicClient) Embed(_ context.Context, _ string, _ []string) ([][]float64, error) {
	return nil, fmt.Errorf("anthropic does not support embeddings")
}

