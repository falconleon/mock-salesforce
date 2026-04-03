# API Reference

## Constants

### Endpoints

```go
const (
    EndpointChat        = "https://api.z.ai/api/coding/paas/v4/chat/completions"
    EndpointImage       = "https://api.z.ai/api/paas/v4/images/generations"
    EndpointVideo       = "https://api.z.ai/api/paas/v4/videos/generations"
    EndpointVideoStatus = "https://api.z.ai/api/paas/v4/async-result"
)
```

### Language Models

```go
const (
    ModelAdvanced = "GLM-4.7"       // Most advanced reasoning (recommended)
    ModelFlagship = "GLM-4.6"       // Previous flagship
    ModelStandard = "GLM-4.5"       // Standard, high concurrency
    ModelAir      = "GLM-4.5-Air"   // Lightweight
    ModelFlash    = "GLM-4.5-Flash" // Ultra-fast
)
```

### Vision Models

```go
const (
    ModelVision         = "GLM-4.6V"       // Vision flagship (recommended)
    ModelVisionStandard = "GLM-4.5V"       // Vision standard
    ModelVisionFlash    = "GLM-4.6V-Flash" // Vision fast
)
```

### Image Generation Models

```go
const (
    ModelCogView = "cogView-4-250304" // Text-to-image ($0.01/image)
)
```

### Video Generation Models

```go
const (
    ModelVidu2Image     = "vidu2-image"     // Image-to-video ($0.2/video)
    ModelVidu2StartEnd  = "vidu2-start-end" // Start/end keyframes ($0.2/video)
    ModelVidu2Reference = "vidu2-reference" // Reference-based ($0.4/video)
)
```

### Defaults

```go
const (
    DefaultMaxTokens      = 8000
    DefaultTemperature    = 0.7
    DefaultModel          = ModelAdvanced
    DefaultImageSize      = "1024x1024"
    DefaultVideoSize      = "1280x720"
    DefaultVideoDuration  = 4
    DefaultVideoMovement  = "auto"
    DefaultPollInterval   = 30 * time.Second
    DefaultMaxWait        = 15 * time.Minute
    DefaultOutputDir      = "output"
)
```

## Rate Limits

Z.AI enforces concurrency limits (maximum simultaneous requests) per API key. Choose your model based on your throughput requirements.

### Concurrency Limits by Model (This Client)

| Concurrency Limit | Models |
|---|---|
| 10 concurrent | GLM-4.5 (ModelStandard), GLM-4.5V (ModelVisionStandard), GLM-4.6V (ModelVision) |
| 5 concurrent | GLM-4.5-Air (ModelAir), CogView-4-250304 (ModelCogView), Vidu2 models |
| 2 concurrent | GLM-4.5-Flash (ModelFlash) |
| 1 concurrent | GLM-4.6 (ModelFlagship), GLM-4.7 (ModelAdvanced), GLM-4.6V-Flash (ModelVisionFlash) |

For high-throughput applications, prefer models with higher concurrency limits (GLM-4.5 or GLM-4.5-Air) over GLM-4.7.

## Client

### Constructor

#### NewClient

```go
func NewClient(apiKey string, opts ...Option) (*Client, error)
```

Creates a new Z.AI client. If `apiKey` is empty, reads from `ZAI_API_KEY` environment variable (after attempting to load `.env` from repo root).

**Parameters:**
- `apiKey` - API key (empty string to read from environment)
- `opts` - Optional configuration functions

**Returns:**
- `*Client` - Configured client instance
- `error` - `ErrNoAPIKey` if no API key found

**Example:**
```go
client, err := zaiclient.NewClient("")
```

### Options

#### WithHTTPClient

```go
func WithHTTPClient(c *http.Client) Option
```

Sets a custom HTTP client for all API requests.

**Example:**
```go
httpClient := &http.Client{Timeout: 30 * time.Second}
client, err := zaiclient.NewClient("", zaiclient.WithHTTPClient(httpClient))
```

## Methods

### Chat

```go
func (c *Client) Chat(ctx context.Context, opts ChatOptions) (*ChatResult, error)
```

Sends a chat completion request and returns the parsed result.

**Parameters:**
- `ctx` - Context for cancellation and timeout
- `opts` - Chat configuration (see ChatOptions)

**Returns:**
- `*ChatResult` - Parsed response with content and metadata
- `error` - API error or network error

### ChatStream

```go
func (c *Client) ChatStream(ctx context.Context, opts ChatOptions) (*StreamReader, error)
```

Sends a streaming chat request and returns a stream reader.

**Parameters:**
- `ctx` - Context for cancellation and timeout
- `opts` - Chat configuration (see ChatOptions)

**Returns:**
- `*StreamReader` - Stream reader for incremental response chunks
- `error` - API error or network error

### AnalyzeImage

```go
func (c *Client) AnalyzeImage(ctx context.Context, opts AnalyzeImageOptions) (*ChatResult, error)
```

Analyzes an image using the vision model.

**Parameters:**
- `ctx` - Context for cancellation and timeout
- `opts` - Image analysis configuration (see AnalyzeImageOptions)

**Returns:**
- `*ChatResult` - Parsed response with image analysis
- `error` - API error or file read error

### GenerateImage

```go
func (c *Client) GenerateImage(ctx context.Context, opts GenerateImageOptions) (*ImageResult, error)
```

Generates an image from a text prompt and downloads it.

**Parameters:**
- `ctx` - Context for cancellation and timeout
- `opts` - Image generation configuration (see GenerateImageOptions)

**Returns:**
- `*ImageResult` - Local file path and remote URL
- `error` - API error, network error, or file write error

### GenerateVideo

```go
func (c *Client) GenerateVideo(ctx context.Context, opts GenerateVideoOptions) (*VideoResult, error)
```

Generates a video with async polling and downloads it when ready.

**Parameters:**
- `ctx` - Context for cancellation and timeout
- `opts` - Video generation configuration (see GenerateVideoOptions)

**Returns:**
- `*VideoResult` - Local file path to downloaded video
- `error` - API error, `ErrVideoTimeout`, or network error

## Options Types

### ChatOptions

```go
type ChatOptions struct {
    Messages       []Message
    Model          string  // defaults to ModelAdvanced
    MaxTokens      int     // defaults to DefaultMaxTokens (8000)
    Temperature    float64 // defaults to DefaultTemperature (0.7)
    EnableThinking bool    // defaults to false
}
```

**Fields:**
- `Messages` - Conversation messages (required)
- `Model` - Model to use (defaults to "GLM-4.7")
- `MaxTokens` - Maximum tokens in response
- `Temperature` - Sampling temperature (0.0-1.0)
- `EnableThinking` - Show reasoning process (uses ~74x more tokens)

### Message

```go
type Message struct {
    Role    string // "user" or "assistant"
    Content any    // string or []ContentPart for multimodal
}
```

**Fields:**
- `Role` - Message role ("user", "assistant", "system")
- `Content` - Message content (string for text, []ContentPart for vision)

### ContentPart

```go
type ContentPart struct {
    Type     string    // "text" or "image_url"
    Text     string    // for type="text"
    ImageURL *ImageURL // for type="image_url"
}
```

### AnalyzeImageOptions

```go
type AnalyzeImageOptions struct {
    ImagePath      string // local file path (required)
    Question       string // question about the image (required)
    MaxTokens      int    // defaults to DefaultMaxTokens
    EnableThinking bool   // defaults to false
}
```

### GenerateImageOptions

```go
type GenerateImageOptions struct {
    Prompt    string // text description (required)
    Size      string // defaults to "1024x1024"
    OutputDir string // defaults to "output"
    Filename  string // auto-generated if empty
    Model     string // defaults to ModelCogView
}
```

**Fields:**
- `Prompt` - Text description of the image (required)
- `Size` - Image dimensions (e.g., "1024x1024", "512x512")
- `OutputDir` - Directory to save downloaded image
- `Filename` - Custom filename (auto-generated with timestamp if empty)
- `Model` - Image generation model

### GenerateVideoOptions

```go
type GenerateVideoOptions struct {
    ImageURL          any           // string or []string (required)
    Prompt            string        // video description (required)
    OutputDir         string        // defaults to "output"
    Filename          string        // auto-generated if empty
    Model             string        // defaults to ModelVidu2Image
    Duration          int           // defaults to 4 seconds
    Size              string        // defaults to "1280x720"
    MovementAmplitude string        // defaults to "auto"
    AspectRatio       string        // optional
    WithAudio         bool          // optional (reference model only)
    MaxWait           time.Duration // defaults to 15 minutes
    PollInterval      time.Duration // defaults to 30 seconds
}
```

**Fields:**
- `ImageURL` - Image URL or array of URLs (for start-end/reference models)
- `Prompt` - Text description of desired video motion
- `Duration` - Video length in seconds
- `Size` - Video resolution (e.g., "1280x720")
- `MovementAmplitude` - Movement intensity ("auto", "low", "medium", "high")
- `MaxWait` - Maximum time to wait for video generation
- `PollInterval` - How often to check video status

## Result Types

### ChatResult

```go
type ChatResult struct {
    Content          string // The direct answer
    ReasoningContent string // The reasoning process (when thinking enabled)
    FinishReason     string // "stop", "length", or error code
    Usage            *Usage // Token usage statistics
}
```

### ImageResult

```go
type ImageResult struct {
    LocalPath string // Path to downloaded file
    URL       string // Remote URL from Z.AI CDN
}
```

### VideoResult

```go
type VideoResult struct {
    LocalPath string // Path to downloaded video file
}
```

### Usage

```go
type Usage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}
```

## Stream Reader

### StreamReader

```go
type StreamReader struct {
    // private fields
}
```

#### Read

```go
func (sr *StreamReader) Read() (string, error)
```

Returns the next text chunk from the stream. Returns `io.EOF` when done.

**Example:**
```go
for {
    chunk, err := stream.Read()
    if err == io.EOF {
        break
    }
    if err != nil {
        log.Fatal(err)
    }
    fmt.Print(chunk)
}
```

#### Close

```go
func (sr *StreamReader) Close() error
```

Releases resources. Always call when done reading.

## Errors

### Sentinel Errors

```go
var (
    ErrNoAPIKey     = errors.New("ZAI_API_KEY not found")
    ErrVideoTimeout = errors.New("video generation timed out")
    ErrNoChoices    = errors.New("no choices in API response")
    ErrNoVideoURL   = errors.New("video URL not found in response")
    ErrNoTaskID     = errors.New("task ID not found in response")
)
```

### APIError

```go
type APIError struct {
    StatusCode int
    Body       string
}

func (e *APIError) Error() string
```

Wraps HTTP error responses from the Z.AI API.

**Example:**
```go
if err != nil {
    var apiErr *zaiclient.APIError
    if errors.As(err, &apiErr) {
        fmt.Printf("API error %d: %s\n", apiErr.StatusCode, apiErr.Body)
    }
}
```

## Utility Functions

### FindRepoRoot

```go
func FindRepoRoot(startDir string) (string, error)
```

Walks up from `startDir` looking for a `.git` directory. Returns the directory containing `.git`. If `startDir` is empty, starts from the current working directory.

### LoadEnvFromRepo

```go
func LoadEnvFromRepo() (string, error)
```

Finds the repo root and loads the `.env` file from it. Returns the repo root path. Silently does nothing if `.env` doesn't exist (environment variables may be set externally).

### EncodeImageToBase64

```go
func EncodeImageToBase64(imagePath string) (string, error)
```

Reads an image file and returns its base64 encoding.

### EncodeImageToDataURL

```go
func EncodeImageToDataURL(imagePath string) (string, error)
```

Reads an image file and returns a data URL (e.g., `data:image/png;base64,...`).

## Usage Examples

### Basic Chat

```go
client, _ := zaiclient.NewClient("")
result, err := client.Chat(context.Background(), zaiclient.ChatOptions{
    Messages: []zaiclient.Message{
        {Role: "user", Content: "Hello!"},
    },
})
fmt.Println(result.Content)
```

### Streaming Chat

```go
stream, err := client.ChatStream(context.Background(), zaiclient.ChatOptions{
    Messages: []zaiclient.Message{
        {Role: "user", Content: "Write a poem"},
    },
})
defer stream.Close()

for {
    chunk, err := stream.Read()
    if err == io.EOF {
        break
    }
    fmt.Print(chunk)
}
```

### Vision Analysis

```go
result, err := client.AnalyzeImage(context.Background(), zaiclient.AnalyzeImageOptions{
    ImagePath: "photo.jpg",
    Question:  "What's in this image?",
})
fmt.Println(result.Content)
```

### Image Generation

```go
result, err := client.GenerateImage(context.Background(), zaiclient.GenerateImageOptions{
    Prompt: "A serene lake",
})
fmt.Printf("Saved to: %s\n", result.LocalPath)
```

### Video Generation

```go
video, err := client.GenerateVideo(context.Background(), zaiclient.GenerateVideoOptions{
    ImageURL: "https://example.com/image.png",
    Prompt:   "Gentle motion",
    Duration: 4,
})
fmt.Printf("Video saved to: %s\n", video.LocalPath)
```

### Multi-Turn Conversation

```go
messages := []zaiclient.Message{
    {Role: "user", Content: "I like Python"},
    {Role: "assistant", Content: "Python is great!"},
    {Role: "user", Content: "What language do I like?"},
}

result, err := client.Chat(context.Background(), zaiclient.ChatOptions{
    Messages: messages,
})
```

### Thinking Control Comparison

```go
// Efficient (default)
result1, _ := client.Chat(ctx, zaiclient.ChatOptions{
    Messages:       messages,
    EnableThinking: false,
})
fmt.Printf("Tokens: %d\n", result1.Usage.TotalTokens) // ~50 tokens

// Show reasoning
result2, _ := client.Chat(ctx, zaiclient.ChatOptions{
    Messages:       messages,
    EnableThinking: true,
})
fmt.Printf("Reasoning: %s\n", result2.ReasoningContent)
fmt.Printf("Answer: %s\n", result2.Content)
fmt.Printf("Tokens: %d\n", result2.Usage.TotalTokens) // ~1000+ tokens
```
