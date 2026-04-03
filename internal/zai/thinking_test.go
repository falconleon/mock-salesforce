package zaiclient

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// truncate truncates a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// TestThinkingDefault sends a question with default options (EnableThinking=false is default).
func TestThinkingDefault(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	result, err := client.Chat(ctx, ChatOptions{
		Messages: []Message{
			{Role: "user", Content: "What is 2+2? Just give the answer."},
		},
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if result.Content == "" {
		t.Fatal("Expected Content to be non-empty")
	}

	if result.Usage == nil {
		t.Fatal("Expected Usage to be non-nil")
	}

	t.Logf("Content: %s", result.Content)
	t.Logf("Completion tokens: %d", result.Usage.CompletionTokens)
	t.Logf("Total tokens: %d", result.Usage.TotalTokens)
	writeTestReport(t, "thinking_default.txt", fmt.Sprintf("Test: ThinkingDefault\nTimestamp: %s\nContent: %s\nCompletion tokens: %d\nTotal tokens: %d\n", testTimestamp(), result.Content, result.Usage.CompletionTokens, result.Usage.TotalTokens))
}

// TestThinkingExplicitlyDisabled sends a question with EnableThinking explicitly set to false.
func TestThinkingExplicitlyDisabled(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	result, err := client.Chat(ctx, ChatOptions{
		Messages: []Message{
			{Role: "user", Content: "What is 2+2? Just give the answer."},
		},
		EnableThinking: false,
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if result.Content == "" {
		t.Fatal("Expected Content to be non-empty")
	}

	t.Logf("Content: %s", result.Content)
	t.Logf("Completion tokens: %d", result.Usage.CompletionTokens)
	t.Logf("Total tokens: %d", result.Usage.TotalTokens)
	writeTestReport(t, "thinking_disabled.txt", fmt.Sprintf("Test: ThinkingExplicitlyDisabled\nTimestamp: %s\nContent: %s\nCompletion tokens: %d\nTotal tokens: %d\n", testTimestamp(), result.Content, result.Usage.CompletionTokens, result.Usage.TotalTokens))
}

// TestThinkingExplicitlyEnabled sends a question with EnableThinking=true.
func TestThinkingExplicitlyEnabled(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	result, err := client.Chat(ctx, ChatOptions{
		Messages: []Message{
			{Role: "user", Content: "What is 2+2? Just give the answer."},
		},
		EnableThinking: true,
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if result.Content == "" {
		t.Fatal("Expected Content to be non-empty")
	}

	if result.ReasoningContent == "" {
		t.Fatal("Expected ReasoningContent to be non-empty when thinking is enabled")
	}

	t.Logf("ReasoningContent: %s", truncate(result.ReasoningContent, 300))
	t.Logf("Content: %s", result.Content)
	t.Logf("Completion tokens: %d", result.Usage.CompletionTokens)
	t.Logf("Total tokens: %d", result.Usage.TotalTokens)
	writeTestReport(t, "thinking_enabled.txt", fmt.Sprintf("Test: ThinkingExplicitlyEnabled\nTimestamp: %s\nReasoning: %s\nContent: %s\nCompletion tokens: %d\nTotal tokens: %d\n", testTimestamp(), truncate(result.ReasoningContent, 300), result.Content, result.Usage.CompletionTokens, result.Usage.TotalTokens))
}

// TestThinkingDisabledComplexQuestion tests a more complex question with thinking disabled.
func TestThinkingDisabledComplexQuestion(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	result, err := client.Chat(ctx, ChatOptions{
		Messages: []Message{
			{Role: "user", Content: "Write a Python function to calculate factorial. Just the code."},
		},
		EnableThinking: false,
	})

	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	if result.Content == "" {
		t.Fatal("Expected Content to be non-empty")
	}

	// Check if content contains code-like elements
	hasCode := strings.Contains(result.Content, "def ") || strings.Contains(result.Content, "factorial")
	if !hasCode {
		t.Logf("Warning: Content doesn't appear to contain code: %s", truncate(result.Content, 200))
	}

	t.Logf("Content (first 200 chars): %s", truncate(result.Content, 200))
	t.Logf("Completion tokens: %d", result.Usage.CompletionTokens)
	t.Logf("Total tokens: %d", result.Usage.TotalTokens)
	writeTestReport(t, "thinking_complex.txt", fmt.Sprintf("Test: ThinkingDisabledComplexQuestion\nTimestamp: %s\nContent: %s\nCompletion tokens: %d\nTotal tokens: %d\n", testTimestamp(), result.Content, result.Usage.CompletionTokens, result.Usage.TotalTokens))
}

// TestThinkingTokenEfficiency compares token usage between enabled and disabled thinking.
func TestThinkingTokenEfficiency(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()

	question := "What is 2+2? Just give the answer."

	// Test with thinking disabled
	resultDisabled, err := client.Chat(ctx, ChatOptions{
		Messages: []Message{
			{Role: "user", Content: question},
		},
		EnableThinking: false,
	})

	if err != nil {
		t.Fatalf("Chat with thinking disabled failed: %v", err)
	}

	if resultDisabled.Usage == nil {
		t.Fatal("Expected Usage to be non-nil for disabled thinking")
	}

	// Test with thinking enabled
	resultEnabled, err := client.Chat(ctx, ChatOptions{
		Messages: []Message{
			{Role: "user", Content: question},
		},
		EnableThinking: true,
	})

	if err != nil {
		t.Fatalf("Chat with thinking enabled failed: %v", err)
	}

	if resultEnabled.Usage == nil {
		t.Fatal("Expected Usage to be non-nil for enabled thinking")
	}

	tokensDisabled := resultDisabled.Usage.CompletionTokens
	tokensEnabled := resultEnabled.Usage.CompletionTokens

	t.Logf("Disabled: %d tokens", tokensDisabled)
	t.Logf("Enabled: %d tokens", tokensEnabled)

	if tokensEnabled <= tokensDisabled {
		t.Logf("Warning: Expected enabled thinking to use MORE tokens than disabled, got enabled=%d, disabled=%d", tokensEnabled, tokensDisabled)
	} else {
		t.Logf("Token efficiency confirmed: disabled used %d fewer tokens (%.1fx reduction)",
			tokensEnabled-tokensDisabled,
			float64(tokensEnabled)/float64(tokensDisabled))
	}
	report := fmt.Sprintf("Test: ThinkingTokenEfficiency\nTimestamp: %s\nQuestion: %s\nThinking Disabled: %d completion tokens\nThinking Enabled: %d completion tokens\n", testTimestamp(), question, tokensDisabled, tokensEnabled)
	if tokensEnabled > tokensDisabled {
		report += fmt.Sprintf("Efficiency: disabled used %d fewer tokens (%.1fx reduction)\n", tokensEnabled-tokensDisabled, float64(tokensEnabled)/float64(tokensDisabled))
	}
	writeTestReport(t, "thinking_efficiency.txt", report)
}
