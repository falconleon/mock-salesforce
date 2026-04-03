package zaiclient

import (
	"bufio"
	"bytes"
	"context"
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

	"github.com/joho/godotenv"
)

// Error variables
var (
	ErrNoAPIKey     = errors.New("ZAI_API_KEY not found")
	ErrVideoTimeout = errors.New("video generation timed out")
	ErrNoChoices    = errors.New("no choices in API response")
	ErrNoVideoURL   = errors.New("video URL not found in response")
	ErrNoTaskID     = errors.New("task ID not found in response")
)

// APIError wraps an HTTP error response from the Z.AI API.
type APIError struct {
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API error %d: %s", e.StatusCode, e.Body)
}

// Client is the Z.AI API client.
type Client struct {
	apiKey     string
	httpClient *http.Client
}

// FindRepoRoot walks up from startDir looking for a .git directory.
// Returns the directory containing .git, or an error if none found.
// If startDir is empty, starts from the current working directory.
func FindRepoRoot(startDir string) (string, error) {
	// Start from current directory if not specified
	dir := startDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}

	// Walk up directories looking for .git
	for {
		gitPath := filepath.Join(dir, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return dir, nil
		}

		// Move to parent directory
		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", errors.New("repository root not found (no .git directory)")
		}
		dir = parent
	}
}

// LoadEnvFromRepo finds the repo root and loads the .env file from it.
// Returns the repo root path. Silently does nothing if .env doesn't exist.
func LoadEnvFromRepo() (string, error) {
	root, err := FindRepoRoot("")
	if err != nil {
		return "", err
	}

	envPath := filepath.Join(root, ".env")
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		// .env doesn't exist, return root without error
		return root, nil
	}

	if err := godotenv.Load(envPath); err != nil {
		return "", err
	}

	return root, nil
}

// NewClient creates a new Z.AI client.
// If apiKey is empty, it reads from the ZAI_API_KEY environment variable.
func NewClient(apiKey string, opts ...Option) (*Client, error) {
	// If no API key provided, try to load from .env
	if apiKey == "" {
		_, _ = LoadEnvFromRepo() // Ignore error if .env not found
		apiKey = os.Getenv("ZAI_API_KEY")
	}

	if apiKey == "" {
		return nil, ErrNoAPIKey
	}

	client := &Client{
		apiKey:     apiKey,
		httpClient: &http.Client{},
	}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	return client, nil
}

// EncodeImageToBase64 reads an image file and returns its base64 encoding.
func EncodeImageToBase64(imagePath string) (string, error) {
	data, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// EncodeImageToDataURL reads an image file and returns a data URL.
func EncodeImageToDataURL(imagePath string) (string, error) {
	encoded, err := EncodeImageToBase64(imagePath)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("data:image/png;base64,%s", encoded), nil
}

// Chat sends a chat completion request and returns the parsed result.
func (c *Client) Chat(ctx context.Context, opts ChatOptions) (*ChatResult, error) {
	// Apply defaults
	if opts.Model == "" {
		opts.Model = DefaultModel
	}
	if opts.MaxTokens == 0 {
		opts.MaxTokens = DefaultMaxTokens
	}
	if opts.Temperature == 0 {
		opts.Temperature = DefaultTemperature
	}

	// Build thinking config
	thinkingType := "disabled"
	if opts.EnableThinking {
		thinkingType = "enabled"
	}

	// Build request
	request := ChatRequest{
		Model:       opts.Model,
		Messages:    opts.Messages,
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
		Stream:      false,
		Thinking:    &ThinkingConfig{Type: thinkingType},
	}

	// Add timeout if context doesn't have one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 60*time.Second)
		defer cancel()
	}

	// Make request
	body, statusCode, err := c.doRequest(ctx, "POST", EndpointChat, request)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, &APIError{StatusCode: statusCode, Body: string(body)}
	}

	// Parse response
	var response ChatResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return parseChatResponse(&response, opts.EnableThinking), nil
}

// ChatStream sends a streaming chat request and returns a stream reader.
func (c *Client) ChatStream(ctx context.Context, opts ChatOptions) (*StreamReader, error) {
	// Apply defaults
	if opts.Model == "" {
		opts.Model = DefaultModel
	}
	if opts.MaxTokens == 0 {
		opts.MaxTokens = DefaultMaxTokens
	}
	if opts.Temperature == 0 {
		opts.Temperature = DefaultTemperature
	}

	// Build thinking config
	thinkingType := "disabled"
	if opts.EnableThinking {
		thinkingType = "enabled"
	}

	// Build request
	request := ChatRequest{
		Model:       opts.Model,
		Messages:    opts.Messages,
		MaxTokens:   opts.MaxTokens,
		Temperature: opts.Temperature,
		Stream:      true,
		Thinking:    &ThinkingConfig{Type: thinkingType},
	}

	// Marshal request
	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, err
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", EndpointChat, bytes.NewReader(jsonData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, &APIError{StatusCode: resp.StatusCode, Body: string(body)}
	}

	return &StreamReader{
		scanner:        bufio.NewScanner(resp.Body),
		body:           resp.Body,
		enableThinking: opts.EnableThinking,
		done:           false,
	}, nil
}

// Read returns the next text chunk from the stream. Returns io.EOF when done.
func (sr *StreamReader) Read() (string, error) {
	if sr.done {
		return "", io.EOF
	}

	for sr.scanner.Scan() {
		line := sr.scanner.Text()

		// Skip empty lines
		if line == "" {
			continue
		}

		// Skip non-data lines
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		// Extract JSON after "data: " prefix
		jsonStr := strings.TrimPrefix(line, "data: ")

		// Check for terminator
		if strings.TrimSpace(jsonStr) == "[DONE]" {
			sr.done = true
			return "", io.EOF
		}

		// Parse JSON
		var chunk struct {
			Choices []struct {
				Delta MessageContent `json:"delta"`
			} `json:"choices"`
		}

		if err := json.Unmarshal([]byte(jsonStr), &chunk); err != nil {
			continue
		}

		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		// Extract text based on thinking mode
		var text string
		if sr.enableThinking {
			// When thinking enabled, prefer reasoning_content
			text = delta.ReasoningContent
			if text == "" {
				text = delta.Content
			}
		} else {
			// When thinking disabled, prefer content
			text = delta.Content
			if text == "" {
				text = delta.ReasoningContent
			}
		}

		if text != "" {
			return text, nil
		}
	}

	// Scanner finished
	sr.done = true
	return "", io.EOF
}

// Close releases resources.
func (sr *StreamReader) Close() error {
	return sr.body.Close()
}

// AnalyzeImage sends an image to the vision model for analysis.
func (c *Client) AnalyzeImage(ctx context.Context, opts AnalyzeImageOptions) (*ChatResult, error) {
	// Encode image to data URL
	dataURL, err := EncodeImageToDataURL(opts.ImagePath)
	if err != nil {
		return nil, err
	}

	// Build multimodal message
	message := Message{
		Role: "user",
		Content: []ContentPart{
			{Type: "text", Text: opts.Question},
			{Type: "image_url", ImageURL: &ImageURL{URL: dataURL}},
		},
	}

	// Delegate to Chat
	chatOpts := ChatOptions{
		Messages:       []Message{message},
		Model:          ModelVision,
		MaxTokens:      opts.MaxTokens,
		EnableThinking: opts.EnableThinking,
	}

	if chatOpts.MaxTokens == 0 {
		chatOpts.MaxTokens = DefaultMaxTokens
	}

	return c.Chat(ctx, chatOpts)
}

// GenerateImage generates an image from a text prompt and downloads it.
func (c *Client) GenerateImage(ctx context.Context, opts GenerateImageOptions) (*ImageResult, error) {
	// Apply defaults
	if opts.Size == "" {
		opts.Size = DefaultImageSize
	}
	if opts.OutputDir == "" {
		opts.OutputDir = DefaultOutputDir
	}
	if opts.Model == "" {
		opts.Model = ModelCogView
	}

	// Add timeout if context doesn't have one
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 120*time.Second)
		defer cancel()
	}

	// Build request
	request := ImageRequest{
		Model:  opts.Model,
		Prompt: opts.Prompt,
		Size:   opts.Size,
	}

	// Make request
	body, statusCode, err := c.doRequest(ctx, "POST", EndpointImage, request)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, &APIError{StatusCode: statusCode, Body: string(body)}
	}

	// Parse response
	var response ImageResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	if len(response.Data) == 0 {
		return nil, errors.New("no image data in response")
	}

	imageURL := response.Data[0].URL

	// Generate filename if empty
	filename := opts.Filename
	if filename == "" {
		filename = fmt.Sprintf("zai_image_%d.png", time.Now().Unix())
	}

	// Create output directory
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, err
	}

	// Download file
	filePath := filepath.Join(opts.OutputDir, filename)
	if err := c.downloadFile(ctx, imageURL, filePath, 3); err != nil {
		return nil, err
	}

	return &ImageResult{
		LocalPath: filePath,
		URL:       imageURL,
	}, nil
}

// GenerateVideo generates a video with async polling and downloads it.
func (c *Client) GenerateVideo(ctx context.Context, opts GenerateVideoOptions) (*VideoResult, error) {
	// Apply defaults
	if opts.Model == "" {
		opts.Model = ModelVidu2Image
	}
	if opts.Duration == 0 {
		opts.Duration = DefaultVideoDuration
	}
	if opts.Size == "" {
		opts.Size = DefaultVideoSize
	}
	if opts.MovementAmplitude == "" {
		opts.MovementAmplitude = DefaultVideoMovement
	}
	if opts.OutputDir == "" {
		opts.OutputDir = DefaultOutputDir
	}
	if opts.MaxWait == 0 {
		opts.MaxWait = DefaultMaxWait
	}
	if opts.PollInterval == 0 {
		opts.PollInterval = DefaultPollInterval
	}

	// Build request
	request := VideoRequest{
		Model:             opts.Model,
		ImageURL:          opts.ImageURL,
		Prompt:            opts.Prompt,
		Duration:          opts.Duration,
		Size:              opts.Size,
		MovementAmplitude: opts.MovementAmplitude,
	}

	if opts.AspectRatio != "" {
		request.AspectRatio = opts.AspectRatio
	}
	if opts.WithAudio {
		request.WithAudio = opts.WithAudio
	}

	// Submit video generation (with timeout for submission only)
	submitCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		submitCtx, cancel = context.WithTimeout(ctx, 300*time.Second)
		defer cancel()
	}

	body, statusCode, err := c.doRequest(submitCtx, "POST", EndpointVideo, request)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, &APIError{StatusCode: statusCode, Body: string(body)}
	}

	// Parse submission response
	var submitResponse VideoSubmitResponse
	if err := json.Unmarshal(body, &submitResponse); err != nil {
		return nil, err
	}

	// Extract task ID
	var taskID string
	if submitResponse.Data != nil && submitResponse.Data.ID != "" {
		taskID = submitResponse.Data.ID
	} else if submitResponse.ID != "" {
		taskID = submitResponse.ID
	} else {
		return nil, ErrNoTaskID
	}

	// Generate filename if empty
	filename := opts.Filename
	if filename == "" {
		filename = fmt.Sprintf("zai_video_%d.mp4", time.Now().Unix())
	}

	// Create output directory
	if err := os.MkdirAll(opts.OutputDir, 0755); err != nil {
		return nil, err
	}

	// Polling loop — first check is immediate, then on interval
	deadline := time.NewTimer(opts.MaxWait)
	defer deadline.Stop()

	// Buffered channel triggers the immediate first check
	immediate := make(chan struct{}, 1)
	immediate <- struct{}{}

	ticker := time.NewTicker(opts.PollInterval)
	defer ticker.Stop()

	for {
		// Wait for a trigger: immediate first check, ticker, deadline, or cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-deadline.C:
			return nil, ErrVideoTimeout
		case <-immediate:
			// First poll — falls through to check below
		case <-ticker.C:
			// Subsequent polls — falls through to check below
		}

		// Check video status
		statusResp, err := c.checkVideoStatus(ctx, taskID)
		if err != nil {
			return nil, err
		}

		switch statusResp.TaskStatus {
		case "SUCCESS":
			if len(statusResp.VideoResult) == 0 {
				return nil, ErrNoVideoURL
			}
			videoURL := statusResp.VideoResult[0].URL
			if videoURL == "" {
				return nil, ErrNoVideoURL
			}

			filePath := filepath.Join(opts.OutputDir, filename)
			if err := c.downloadFile(ctx, videoURL, filePath, 3); err != nil {
				return nil, err
			}
			return &VideoResult{LocalPath: filePath}, nil

		case "FAIL":
			errMsg := statusResp.Error
			if errMsg == "" {
				errMsg = "unknown error"
			}
			return nil, fmt.Errorf("video generation failed: %s", errMsg)
		}
		// Still processing — loop back and wait for next tick
	}
}

// checkVideoStatus polls the video task status endpoint.
func (c *Client) checkVideoStatus(ctx context.Context, taskID string) (*VideoStatusResponse, error) {
	url := fmt.Sprintf("%s/%s", EndpointVideoStatus, taskID)

	body, statusCode, err := c.doRequest(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	if statusCode != http.StatusOK {
		return nil, &APIError{StatusCode: statusCode, Body: string(body)}
	}

	var response VideoStatusResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// doRequest sends an HTTP request and returns the response body.
func (c *Client) doRequest(ctx context.Context, method, url string, body any) ([]byte, int, error) {
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, 0, err
		}
		reqBody = bytes.NewReader(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return nil, 0, err
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}

	return responseBody, resp.StatusCode, nil
}

// downloadFile downloads a URL to a local file path with retry logic.
func (c *Client) downloadFile(ctx context.Context, url, filePath string, maxRetries int) error {
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		// Handle 404 with retry
		if resp.StatusCode == http.StatusNotFound && attempt < maxRetries-1 {
			resp.Body.Close()
			time.Sleep(2 * time.Second)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return &APIError{StatusCode: resp.StatusCode, Body: string(body)}
		}

		// Read and write file
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return err
		}

		if err := os.WriteFile(filePath, data, 0644); err != nil {
			return err
		}

		return nil
	}

	return lastErr
}

// parseChatResponse extracts text from a chat response based on thinking mode.
func parseChatResponse(data *ChatResponse, enableThinking bool) *ChatResult {
	if len(data.Choices) == 0 {
		return &ChatResult{}
	}

	choice := data.Choices[0]
	content := strings.TrimSpace(choice.Message.Content)
	reasoning := strings.TrimSpace(choice.Message.ReasoningContent)

	// Warn if tokens exhausted
	if choice.FinishReason == "length" {
		fmt.Fprintf(os.Stderr, "Warning: max_tokens exhausted. Consider increasing max_tokens.\n")
	}

	result := &ChatResult{
		FinishReason: choice.FinishReason,
		Usage:        data.Usage,
	}

	if enableThinking {
		// When thinking enabled, return both
		result.Content = content
		result.ReasoningContent = reasoning
	} else {
		// When thinking disabled, only return the direct answer
		if content != "" {
			result.Content = content
		} else {
			result.Content = reasoning
		}
	}

	return result
}
