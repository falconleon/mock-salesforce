# Go Z.AI Client

A Go client library for [Z.AI](https://open.bigmodel.cn/)'s GLM model family, providing access to chat completions, vision analysis, image generation, and video generation APIs.

## Features

- **Chat Completions** - Text conversations using GLM-4.5 through GLM-4.7 models
- **Vision Analysis** - Image understanding via GLM-4.6V (supports local files and URLs)
- **Image Generation** - Text-to-image via CogView-4 ($0.01/image)
- **Video Generation** - Image-to-video via vidu2 with async polling ($0.2-0.4/video)
- **Thinking Control** - Toggle reasoning visibility for 87.5x token savings
- **Streaming** - Server-Sent Events streaming for chat responses
- **Auto-configuration** - Finds `.env` files by walking up to the repo root (`.git`)

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    zaiclient "github.com/user/go-zai-client"
)

func main() {
    zaiclient.LoadEnvFromRepo()

    client, err := zaiclient.NewClient("")
    if err != nil {
        log.Fatal(err)
    }

    result, err := client.Chat(context.Background(), zaiclient.ChatOptions{
        Messages: []zaiclient.Message{
            {Role: "user", Content: "Hello!"},
        },
    })
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(result.Content)
}
```

## Available Models

| Category | Model | Constant | Notes |
|----------|-------|----------|-------|
| Language | GLM-4.7 | `ModelAdvanced` | Most capable, recommended |
| Language | GLM-4.6 | `ModelFlagship` | Previous flagship |
| Language | GLM-4.5 | `ModelStandard` | High concurrency (10) |
| Language | GLM-4.5-Air | `ModelAir` | Lightweight |
| Language | GLM-4.5-Flash | `ModelFlash` | Ultra-fast |
| Vision | GLM-4.6V | `ModelVision` | Vision flagship, recommended |
| Vision | GLM-4.5V | `ModelVisionStandard` | Vision standard |
| Vision | GLM-4.6V-Flash | `ModelVisionFlash` | Vision fast |
| Image Gen | CogView-4-250304 | `ModelCogView` | $0.01/image |
| Video Gen | vidu2-image | `ModelVidu2Image` | $0.2/video |
| Video Gen | vidu2-start-end | `ModelVidu2StartEnd` | $0.2/video |
| Video Gen | vidu2-reference | `ModelVidu2Reference` | $0.4/video |

## API Overview

### Chat

```go
result, err := client.Chat(ctx, zaiclient.ChatOptions{
    Messages: []zaiclient.Message{
        {Role: "user", Content: "What is the capital of France?"},
    },
    Model:          zaiclient.ModelStandard,  // optional, defaults to ModelAdvanced
    EnableThinking: false,                     // optional, saves tokens
})
// result.Content = "Paris"
```

### Vision

```go
result, err := client.AnalyzeImage(ctx, zaiclient.AnalyzeImageOptions{
    ImagePath: "photo.jpg",
    Question:  "What is in this image?",
})
```

### Image Generation

```go
result, err := client.GenerateImage(ctx, zaiclient.GenerateImageOptions{
    Prompt:    "A duck pond with lily pads",
    OutputDir: "output",
})
// result.LocalPath = "output/generated_image.png"
// result.URL = "https://mfile.z.ai/..."
```

### Video Generation

```go
result, err := client.GenerateVideo(ctx, zaiclient.GenerateVideoOptions{
    ImageURL: imageResult.URL,  // from GenerateImage
    Prompt:   "A duck swimming across the pond",
    MaxWait:  3 * time.Minute,
})
// result.LocalPath = "output/generated_video.mp4"
```

### Thinking Control

Thinking mode exposes the model's reasoning process but uses significantly more tokens:

```go
// Disabled (default): 2 tokens, direct answer
result, _ := client.Chat(ctx, zaiclient.ChatOptions{
    Messages:       messages,
    EnableThinking: false,
})
// result.Content = "4"

// Enabled: 175 tokens, shows reasoning
result, _ := client.Chat(ctx, zaiclient.ChatOptions{
    Messages:       messages,
    EnableThinking: true,
})
// result.ReasoningContent = "The user is asking 2+2. Let me calculate..."
// result.Content = "4"
```

**Measured result**: Disabling thinking provides an **87.5x token reduction** (2 vs 175 completion tokens for the same question).

## Performance & Concurrency

All benchmarks measured against live Z.AI API endpoints.

### Single Request Latency

| Operation | Typical Latency |
|-----------|----------------|
| Chat (simple question) | 0.7 - 1.0s |
| Chat (complex, 500 tokens) | 8 - 12s |
| Vision analysis | 0.8 - 1.2s |
| Image generation | 10 - 15s |
| Video generation | 40 - 55s (async polling) |

### GLM-4.5 Chat Concurrency (ModelStandard)

Documented limit: 10 concurrent requests. Tested with medium-complexity prompts (500 max tokens):

| Concurrent | Succeeded | Failed | Wall Time | Speedup |
|------------|-----------|--------|-----------|---------|
| 1 | 1 | 0 | 8.7s | 1.0x |
| 2 | 2 | 0 | 11.4s | 1.5x |
| 5 | 5 | 0 | 9.1s | 4.8x |
| 8 | 8 | 0 | 10.9s | 6.4x |
| 10 | 8 | **2** | 11.3s | - |

**Safe maximum: 8 concurrent requests.** At 10, the API returns HTTP 429 rate limit errors. Speedup is near-linear up to 5 concurrent requests.

### CogView Image Generation Concurrency (ModelCogView)

Documented limit: 5 concurrent requests. Tested with profile-image-style prompts:

| Concurrent | Succeeded | Failed | Wall Time | Speedup |
|------------|-----------|--------|-----------|---------|
| 1 | 1 | 0 | 12.2s | 1.0x |
| 2 | 2 | 0 | 10.8s | 2.3x |
| 3 | 3 | 0 | 10.3s | 3.6x |
| 5 | 5 | 0 | 12.1s | 5.1x |

**All 5 concurrent requests succeed.** Near-linear speedup confirms true parallel processing on the API side.

### Concurrency Recommendations

| Model | Documented Limit | Safe Limit | Notes |
|-------|-----------------|------------|-------|
| GLM-4.7 (Advanced) | 1 | 1 | Sequential only |
| GLM-4.6 (Flagship) | 5 | 5 | Not benchmarked |
| GLM-4.5 (Standard) | 10 | **8** | Rate-limited at 10 |
| GLM-4.5-Flash | 10 | 10 | Not benchmarked |
| CogView-4 | 5 | **5** | Full limit works |
| vidu2 | 1 | 1 | Async polling |

## Running Tests

```bash
# Set up API key
cp .env.example .env
# Edit .env with your key from https://open.bigmodel.cn/usercenter/apikeys

# Run fast tests (chat, vision, thinking) - ~30s
go test -v -run "TestChat|TestSimple|TestCode|TestMultiTurn|TestVision|TestThinking" -timeout 5m

# Run image generation test - ~15s, requires payment
go test -v -run "TestImageGeneration$" -timeout 5m

# Run video generation test (duck pond) - ~60s, requires payment
go test -v -run "TestVideoGeneration" -timeout 10m

# Run GLM-4.5 concurrency benchmark - ~60s
go test -v -run "TestGLM45ConcurrencyLimits" -timeout 10m

# Run CogView concurrency benchmark - ~60s, requires payment
go test -v -run "TestCogViewConcurrencyLimits" -timeout 20m

# Run everything
go test -v -timeout 20m
```

All tests write timestamped output (reports, images, videos) to the `output/` directory.

## Project Structure

```
go_zai_client/
  constants.go              # API endpoints, model names, defaults
  models.go                 # Request/response types, options structs
  client.go                 # Client implementation (~720 lines)
  client_test.go            # Core tests (chat, vision, image)
  thinking_test.go          # Thinking control tests
  concurrency_test.go       # GLM-4.5 concurrency benchmark
  simple_image_gen_test.go  # CogView concurrency benchmark
  video_test.go             # Video generation (duck pond scenario)
  testutil_test.go          # Shared test helpers
  output/                   # Generated test artifacts (gitignored)
  docs/
    getting-started.md
    api-reference.md
```
