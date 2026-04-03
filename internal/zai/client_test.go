package zaiclient

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Test helper functions

func testSetup(t *testing.T) *Client {
	t.Helper()
	LoadEnvFromRepo()
	key := os.Getenv("ZAI_API_KEY")
	if key == "" {
		t.Skip("ZAI_API_KEY not set, skipping integration test")
	}
	client, err := NewClient(key)
	if err != nil {
		t.Fatalf("failed to create client: %v", err)
	}
	return client
}

func isQuotaError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "insufficient_quota") ||
		   strings.Contains(msg, "余额不足") ||
		   strings.Contains(msg, "file not exist") // Image generation quota/payment issue
}

// Base64-encoded 1x1 red pixel PNG for vision tests
const redPixelPNG = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8DwHwAFBQIAX8jx0gAAAABJRU5ErkJggg=="

// Tests from test_all_endpoints.py

func TestChatText(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	result, err := client.Chat(ctx, ChatOptions{
		Messages: []Message{
			{Role: "user", Content: "Say 'Hello' in one word"},
		},
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if result.Content == "" {
		t.Errorf("expected non-empty response, got empty")
	}

	t.Logf("Response: %s", result.Content)
	writeTestReport(t, "chat_text.txt", fmt.Sprintf("Test: ChatText\nTimestamp: %s\nResponse: %s\n", testTimestamp(), result.Content))
}

func TestChatVision(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	// Build data URL for 1x1 red pixel
	dataURL := "data:image/png;base64," + redPixelPNG

	// Build multimodal message
	message := Message{
		Role: "user",
		Content: []ContentPart{
			{Type: "text", Text: "What color is this pixel?"},
			{Type: "image_url", ImageURL: &ImageURL{URL: dataURL}},
		},
	}

	result, err := client.Chat(ctx, ChatOptions{
		Messages: []Message{message},
		Model:    ModelVision,
	})

	if err != nil {
		t.Fatalf("Vision chat failed: %v", err)
	}

	if result.Content == "" {
		t.Errorf("expected non-empty response, got empty")
	}

	t.Logf("Vision response: %s", result.Content)
	writeTestReport(t, "chat_vision.txt", fmt.Sprintf("Test: ChatVision\nTimestamp: %s\nResponse: %s\n", testTimestamp(), result.Content))
}

func TestImageGeneration(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	result, err := client.GenerateImage(ctx, GenerateImageOptions{
		Prompt:    "A simple red circle on white background",
		OutputDir: testOutputDir(t),
		Filename:  testTimestamp() + "_test_image.png",
	})

	if err != nil {
		if isQuotaError(err) {
			t.Skipf("Skipped (requires payment): %v", err)
		}
		t.Fatalf("GenerateImage failed: %v", err)
	}

	if result.LocalPath == "" {
		t.Errorf("expected non-empty LocalPath, got empty")
	}
	if result.URL == "" {
		t.Errorf("expected non-empty URL, got empty")
	}

	// Check file exists
	if _, err := os.Stat(result.LocalPath); os.IsNotExist(err) {
		t.Errorf("generated file does not exist: %s", result.LocalPath)
	}

	t.Logf("Image generated:")
	t.Logf("  Local: %s", result.LocalPath)
	t.Logf("  URL: %s", result.URL)
	writeTestReport(t, "image_generation.txt", fmt.Sprintf("Test: ImageGeneration\nTimestamp: %s\nLocal: %s\nURL: %s\n", testTimestamp(), result.LocalPath, result.URL))
}

// Tests from test_zai_final_verification.py

func TestSimpleQuestion(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	result, err := client.Chat(ctx, ChatOptions{
		Messages: []Message{
			{Role: "user", Content: "What is the capital of France?"},
		},
		EnableThinking: false, // explicitly disabled (default)
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if result.Content == "" {
		t.Errorf("expected non-empty Content, got empty")
	}

	t.Logf("Response: %s", result.Content)
	writeTestReport(t, "simple_question.txt", fmt.Sprintf("Test: SimpleQuestion\nTimestamp: %s\nResponse: %s\n", testTimestamp(), result.Content))
}

func TestCodeGeneration(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	result, err := client.Chat(ctx, ChatOptions{
		Messages: []Message{
			{Role: "user", Content: "Write a Python one-liner to sum numbers 1-10"},
		},
		EnableThinking: false,
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if result.Content == "" {
		t.Errorf("expected non-empty Content, got empty")
	}

	// Log first 200 chars
	response := result.Content
	if len(response) > 200 {
		response = response[:200] + "..."
	}
	t.Logf("Response: %s", response)
	writeTestReport(t, "code_generation.txt", fmt.Sprintf("Test: CodeGeneration\nTimestamp: %s\nResponse: %s\n", testTimestamp(), result.Content))
}

func TestMultiTurnConversation(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	result, err := client.Chat(ctx, ChatOptions{
		Messages: []Message{
			{Role: "user", Content: "I like Python"},
			{Role: "assistant", Content: "Python is a great language!"},
			{Role: "user", Content: "What language do I like?"},
		},
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if result.Content == "" {
		t.Errorf("expected non-empty Content, got empty")
	}

	// Check that response mentions "Python"
	responseLower := strings.ToLower(result.Content)
	if !strings.Contains(responseLower, "python") {
		t.Errorf("expected response to mention 'Python', got: %s", result.Content)
	}

	t.Logf("Response: %s", result.Content)
	writeTestReport(t, "multi_turn.txt", fmt.Sprintf("Test: MultiTurnConversation\nTimestamp: %s\nResponse: %s\n", testTimestamp(), result.Content))
}

func TestThinkingEnabledBasic(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	result, err := client.Chat(ctx, ChatOptions{
		Messages: []Message{
			{Role: "user", Content: "What is 5+3?"},
		},
		EnableThinking: true,
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if result.Content == "" {
		t.Errorf("expected non-empty Content, got empty")
	}

	if result.ReasoningContent == "" {
		t.Errorf("expected non-empty ReasoningContent when thinking enabled, got empty")
	}

	// Log first 300 chars
	fullResponse := result.ReasoningContent + "\n\n" + result.Content
	if len(fullResponse) > 300 {
		fullResponse = fullResponse[:300] + "..."
	}
	t.Logf("Response (first 300 chars):\n%s", fullResponse)
	writeTestReport(t, "thinking_enabled_basic.txt", fmt.Sprintf("Test: ThinkingEnabledBasic\nTimestamp: %s\nReasoning (first 300): %s\nContent: %s\n", testTimestamp(), fullResponse, result.Content))
}

func TestVisionWithAnalyzeImage(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	// Create a temporary 1x1 red pixel PNG file
	tmpDir := t.TempDir()
	imagePath := filepath.Join(tmpDir, "red_pixel.png")

	// Decode the same base64 PNG that works in TestChatVision
	imageData, err := base64.StdEncoding.DecodeString(redPixelPNG)
	if err != nil {
		t.Fatalf("failed to decode base64: %v", err)
	}

	if err := os.WriteFile(imagePath, imageData, 0644); err != nil {
		t.Fatalf("failed to write temp image: %v", err)
	}

	result, err := client.AnalyzeImage(ctx, AnalyzeImageOptions{
		ImagePath: imagePath,
		Question:  "What color is this pixel?",
	})

	if err != nil {
		t.Fatalf("AnalyzeImage failed: %v", err)
	}

	if result.Content == "" {
		t.Errorf("expected non-empty response, got empty")
	}

	t.Logf("Vision analysis response: %s", result.Content)
	writeTestReport(t, "vision_analyze.txt", fmt.Sprintf("Test: VisionWithAnalyzeImage\nTimestamp: %s\nResponse: %s\n", testTimestamp(), result.Content))
}
