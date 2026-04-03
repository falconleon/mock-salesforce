package zaiclient

import (
	"bufio"
	"io"
	"net/http"
	"time"
)

// ThinkingConfig controls whether reasoning models show their work.
type ThinkingConfig struct {
	Type string `json:"type"` // "enabled" or "disabled"
}

// ChatRequest is the payload for chat completions.
type ChatRequest struct {
	Model       string          `json:"model"`
	Messages    []Message       `json:"messages"`
	MaxTokens   int             `json:"max_tokens"`
	Temperature float64         `json:"temperature"`
	Stream      bool            `json:"stream"`
	Thinking    *ThinkingConfig `json:"thinking,omitempty"`
}

// Message represents a single chat message.
// Content can be a plain string or a slice of ContentPart (for vision).
type Message struct {
	Role    string      `json:"role"`
	Content any `json:"content"` // string or []ContentPart
}

// ContentPart represents a multimodal content element (text or image).
type ContentPart struct {
	Type     string    `json:"type"`               // "text" or "image_url"
	Text     string    `json:"text,omitempty"`     // for type="text"
	ImageURL *ImageURL `json:"image_url,omitempty"` // for type="image_url"
}

// ImageURL wraps an image URL or data URL.
type ImageURL struct {
	URL string `json:"url"`
}

// ImageRequest is the payload for image generation.
type ImageRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Size   string `json:"size"`
}

// VideoRequest is the payload for video generation.
type VideoRequest struct {
	Model             string      `json:"model"`
	ImageURL          any `json:"image_url"` // string or []string
	Prompt            string      `json:"prompt"`
	Duration          int         `json:"duration"`
	Size              string      `json:"size"`
	MovementAmplitude string      `json:"movement_amplitude"`
	AspectRatio       string      `json:"aspect_ratio,omitempty"`
	WithAudio         bool        `json:"with_audio,omitempty"`
}

// ChatResponse is the full response from chat completions.
type ChatResponse struct {
	Choices []Choice `json:"choices"`
	Usage   *Usage   `json:"usage,omitempty"`
}

// Choice represents a single completion choice.
type Choice struct {
	Message      MessageContent  `json:"message"`
	Delta        *MessageContent `json:"delta,omitempty"` // for streaming
	FinishReason string          `json:"finish_reason"`
}

// MessageContent holds the response text fields.
type MessageContent struct {
	Content          string `json:"content"`
	ReasoningContent string `json:"reasoning_content"`
}

// Usage contains token usage statistics.
type Usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ImageResponse is the response from image generation.
type ImageResponse struct {
	Data []ImageData `json:"data"`
}

// ImageData contains a generated image URL.
type ImageData struct {
	URL string `json:"url"`
}

// VideoSubmitResponse is the response from video generation submission.
// Handles both {"data": {"id": "..."}} and {"id": "..."} formats.
type VideoSubmitResponse struct {
	Data *struct {
		ID string `json:"id"`
	} `json:"data,omitempty"`
	ID string `json:"id,omitempty"`
}

// VideoStatusResponse is the response from the video status endpoint.
type VideoStatusResponse struct {
	TaskStatus  string      `json:"task_status"`
	VideoResult []VideoData `json:"video_result,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// VideoData contains a generated video URL.
type VideoData struct {
	URL string `json:"url"`
}

// ChatResult is the parsed result returned to callers of Chat().
type ChatResult struct {
	Content          string // The direct answer
	ReasoningContent string // The reasoning process (when thinking enabled)
	FinishReason     string
	Usage            *Usage
}

// ImageResult is the result returned to callers of GenerateImage().
type ImageResult struct {
	LocalPath string // Path to downloaded file
	URL       string // Remote URL from Z.AI CDN
}

// VideoResult is the result returned to callers of GenerateVideo().
type VideoResult struct {
	LocalPath string // Path to downloaded video file
}

// ChatOptions configures a chat request.
type ChatOptions struct {
	Messages       []Message
	Model          string  // defaults to ModelAdvanced
	MaxTokens      int     // defaults to DefaultMaxTokens
	Temperature    float64 // defaults to DefaultTemperature
	EnableThinking bool    // defaults to false
}

// AnalyzeImageOptions configures an image analysis request.
type AnalyzeImageOptions struct {
	ImagePath      string // local file path
	Question       string
	MaxTokens      int  // defaults to DefaultMaxTokens
	EnableThinking bool
}

// GenerateImageOptions configures image generation.
type GenerateImageOptions struct {
	Prompt    string
	Size      string // defaults to DefaultImageSize
	OutputDir string // defaults to DefaultOutputDir
	Filename  string // auto-generated if empty
	Model     string // defaults to ModelCogView
}

// GenerateVideoOptions configures video generation.
type GenerateVideoOptions struct {
	ImageURL          any   // string or []string
	Prompt            string
	OutputDir         string        // defaults to DefaultOutputDir
	Filename          string        // auto-generated if empty
	Model             string        // defaults to ModelVidu2Image
	Duration          int           // defaults to DefaultVideoDuration
	Size              string        // defaults to DefaultVideoSize
	MovementAmplitude string        // defaults to DefaultVideoMovement
	AspectRatio       string        // optional
	WithAudio         bool          // optional (reference model only)
	MaxWait           time.Duration // defaults to DefaultMaxWait
	PollInterval      time.Duration // defaults to DefaultPollInterval
}

// StreamReader reads chunks from a streaming response.
type StreamReader struct {
	scanner        *bufio.Scanner
	body           io.ReadCloser
	enableThinking bool
	done           bool
}

// Option configures the client.
type Option func(*Client)

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(c *http.Client) Option {
	return func(client *Client) {
		client.httpClient = c
	}
}
