# Getting Started with go-zai-client

A Go client library for Z.AI's GLM models (language, vision, image generation, and video generation).

## Installation

```bash
go get github.com/user/go-zai-client
```

Note: Replace `github.com/user/go-zai-client` with the actual module path when published.

## API Key Setup

### Option 1: Environment Variable

```bash
export ZAI_API_KEY=your_api_key_here
```

### Option 2: .env File

Create a `.env` file in your repository root:

```
ZAI_API_KEY=your_api_key_here
```

The client automatically discovers the repo root by walking up directories to find `.git`, then loads the `.env` file from there.

### Option 3: Direct Parameter

```go
client, err := zaiclient.NewClient("your_api_key_here")
```

Get your API key from: https://open.bigmodel.cn/usercenter/apikeys

## Basic Chat Example

```go
package main

import (
    "context"
    "fmt"
    "log"

    zaiclient "github.com/user/go-zai-client"
)

func main() {
    // Create client (reads ZAI_API_KEY from environment or .env)
    client, err := zaiclient.NewClient("")
    if err != nil {
        log.Fatal(err)
    }

    // Send a chat message
    result, err := client.Chat(context.Background(), zaiclient.ChatOptions{
        Messages: []zaiclient.Message{
            {Role: "user", Content: "What is 2+2?"},
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    fmt.Println(result.Content)
    fmt.Printf("Tokens used: %d\n", result.Usage.TotalTokens)
}
```

## Vision Analysis Example

```go
result, err := client.AnalyzeImage(context.Background(), zaiclient.AnalyzeImageOptions{
    ImagePath: "/path/to/image.png",
    Question:  "What's in this image?",
})
if err != nil {
    log.Fatal(err)
}

fmt.Println(result.Content)
```

The `AnalyzeImage` method handles local file paths, automatically encoding images to base64 and constructing the multimodal request.

## Image Generation Example

```go
result, err := client.GenerateImage(context.Background(), zaiclient.GenerateImageOptions{
    Prompt: "A serene mountain landscape at sunset",
    Size:   "1024x1024", // optional, defaults to 1024x1024
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Image saved to: %s\n", result.LocalPath)
fmt.Printf("Remote URL: %s\n", result.URL)
```

Images are automatically downloaded to the `output/` directory with timestamped filenames. Both the local path and remote URL are returned.

Note: Image generation requires payment ($0.01 per image).

## Video Generation Example

```go
// First generate an image to use as the video source
imageResult, err := client.GenerateImage(context.Background(), zaiclient.GenerateImageOptions{
    Prompt: "A calm ocean scene",
})
if err != nil {
    log.Fatal(err)
}

// Generate video from the image (uses async polling)
videoResult, err := client.GenerateVideo(context.Background(), zaiclient.GenerateVideoOptions{
    ImageURL: imageResult.URL, // Use the URL from image generation
    Prompt:   "Gentle waves moving",
    Duration: 4, // seconds
})
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Video saved to: %s\n", videoResult.LocalPath)
```

Video generation is asynchronous and typically takes 30-60 seconds. The client automatically polls the status endpoint every 30 seconds (configurable) and downloads the video when ready.

Note: Video generation requires payment ($0.2-$0.4 per video depending on model).

## Thinking Control

The `EnableThinking` parameter controls whether reasoning models show their internal thought process.

### Disabled (Default) - Efficient

```go
result, err := client.Chat(context.Background(), zaiclient.ChatOptions{
    Messages: []zaiclient.Message{
        {Role: "user", Content: "What is 2+2?"},
    },
    EnableThinking: false, // default
})

fmt.Println(result.Content) // "4"
// Token usage: ~50 tokens
```

When disabled, the model returns direct answers with minimal tokens (typically 2-50 tokens for simple questions).

### Enabled - Shows Reasoning

```go
result, err := client.Chat(context.Background(), zaiclient.ChatOptions{
    Messages: []zaiclient.Message{
        {Role: "user", Content: "What is 2+2?"},
    },
    EnableThinking: true,
})

fmt.Println("Reasoning:", result.ReasoningContent) // Shows step-by-step thinking
fmt.Println("Answer:", result.Content)             // "4"
// Token usage: ~1000+ tokens
```

When enabled, the model returns both the reasoning process (`ReasoningContent`) and the final answer (`Content`). This uses significantly more tokens (typically 74x more) but provides transparency into the model's thinking.

**Best Practice:** Use `EnableThinking: false` (default) for production to save costs. Enable it only when you need to debug or understand the model's reasoning.

## Streaming Example

```go
stream, err := client.ChatStream(context.Background(), zaiclient.ChatOptions{
    Messages: []zaiclient.Message{
        {Role: "user", Content: "Write a short poem about Go"},
    },
})
if err != nil {
    log.Fatal(err)
}
defer stream.Close()

// Read chunks as they arrive
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
fmt.Println()
```

Streaming allows you to display responses incrementally as they're generated.

## Multi-Turn Conversations

```go
messages := []zaiclient.Message{
    {Role: "user", Content: "I like Python"},
    {Role: "assistant", Content: "Python is a great language!"},
    {Role: "user", Content: "What language do I like?"},
}

result, err := client.Chat(context.Background(), zaiclient.ChatOptions{
    Messages: messages,
})
```

Include previous messages to maintain conversation context.

## Rate Limits

Z.AI enforces concurrency limits (maximum simultaneous requests) per API key. The "Concurrency Limit" indicates how many requests can run at the same time before receiving rate limit errors.

### Full Rate Limits Table

| Model Type | Model Name | Concurrency Limit |
|---|---|---|
| Language Model | GLM-4-Plus | 20 |
| Language Model | GLM-4-32B-0414-128K | 15 |
| Language Model | GLM-4.5 | 10 |
| Language Model | GLM-4.5V | 10 |
| Language Model | GLM-4.6V | 10 |
| Language Model | AutoGLM-Phone-Multilingual | 5 |
| Language Model | GLM-4.5-Air / AirX | 5 |
| Image Generation | CogView-4-250304 | 5 |
| Real-time Audio-Video | GLM-ASR-2512 | 5 |
| Video Generation | ViduQ1 / Vidu2 (various) | 5 |
| Language Model | GLM-4.6V-FlashX | 3 |
| Language Model | GLM-4.7-FlashX | 3 |
| Language Model | GLM-4.5-Flash | 2 |
| Language Model | GLM-4.6 | 1 |
| Language Model | GLM-4.7 | 1 |
| Language Model | GLM-4.7-Flash | 1 |
| Language Model | GLM-4.6V-Flash | 1 |
| Image Generation | GLM-Image | 1 |
| Video Generation | CogVideoX-3 | 1 |

### Models Available in This Client

- **GLM-4.5** (ModelStandard): 10 concurrent requests
- **GLM-4.5V** (ModelVisionStandard): 10 concurrent requests
- **GLM-4.6V** (ModelVision): 10 concurrent requests
- **GLM-4.5-Air** (ModelAir): 5 concurrent requests
- **CogView-4-250304** (ModelCogView): 5 concurrent requests
- **Vidu2** models (ModelVidu2Image, ModelVidu2StartEnd, ModelVidu2Reference): 5 concurrent requests
- **GLM-4.5-Flash** (ModelFlash): 2 concurrent requests
- **GLM-4.6** (ModelFlagship): 1 concurrent request
- **GLM-4.7** (ModelAdvanced): 1 concurrent request
- **GLM-4.6V-Flash** (ModelVisionFlash): 1 concurrent request

### Practical Tip

For high-throughput applications, prefer **GLM-4.5** (10 concurrent) or **GLM-4.5-Air** (5 concurrent) over **GLM-4.7** (1 concurrent). While GLM-4.7 offers the most advanced reasoning, it can only handle one request at a time per API key.

## Running Tests

All tests are integration tests that hit the live Z.AI API.

```bash
# Set API key
export ZAI_API_KEY=your_key

# Run all tests
cd go_zai_client
go test -v ./...

# Run specific test
go test -v -run TestChatText
```

Tests are automatically skipped if `ZAI_API_KEY` is not set.

## Error Handling

```go
result, err := client.Chat(context.Background(), zaiclient.ChatOptions{
    Messages: []zaiclient.Message{
        {Role: "user", Content: "Hello"},
    },
})
if err != nil {
    // Check for specific error types
    var apiErr *zaiclient.APIError
    if errors.As(err, &apiErr) {
        fmt.Printf("API error %d: %s\n", apiErr.StatusCode, apiErr.Body)
    } else if errors.Is(err, zaiclient.ErrNoAPIKey) {
        fmt.Println("API key not configured")
    } else {
        fmt.Printf("Error: %v\n", err)
    }
    return
}
```

## Advanced Configuration

### Custom HTTP Client

```go
import "net/http"
import "time"

httpClient := &http.Client{
    Timeout: 30 * time.Second,
}

client, err := zaiclient.NewClient("", zaiclient.WithHTTPClient(httpClient))
```

### Context with Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
defer cancel()

result, err := client.Chat(ctx, zaiclient.ChatOptions{
    Messages: []zaiclient.Message{
        {Role: "user", Content: "Complex analysis here"},
    },
})
```

### Custom Output Directory

```go
result, err := client.GenerateImage(context.Background(), zaiclient.GenerateImageOptions{
    Prompt:    "A beautiful sunset",
    OutputDir: "/path/to/custom/output",
    Filename:  "my_sunset.png",
})
```

## Available Models

### Language Models
- `zaiclient.ModelAdvanced` - "GLM-4.7" (most advanced reasoning, recommended)
- `zaiclient.ModelFlagship` - "GLM-4.6" (previous flagship)
- `zaiclient.ModelStandard` - "GLM-4.5" (high concurrency)
- `zaiclient.ModelAir` - "GLM-4.5-Air" (lightweight)
- `zaiclient.ModelFlash` - "GLM-4.5-Flash" (ultra-fast)

### Vision Models
- `zaiclient.ModelVision` - "GLM-4.6V" (recommended)
- `zaiclient.ModelVisionStandard` - "GLM-4.5V"
- `zaiclient.ModelVisionFlash` - "GLM-4.6V-Flash"

### Image Generation
- `zaiclient.ModelCogView` - "cogView-4-250304"

### Video Generation
- `zaiclient.ModelVidu2Image` - "vidu2-image" (single keyframe)
- `zaiclient.ModelVidu2StartEnd` - "vidu2-start-end" (start/end keyframes)
- `zaiclient.ModelVidu2Reference` - "vidu2-reference" (reference-based)

## Next Steps

- See [API Reference](api-reference.md) for detailed method documentation
- Check the `examples/` directory for more usage examples
- Review `client_test.go` and `thinking_test.go` for comprehensive test examples
