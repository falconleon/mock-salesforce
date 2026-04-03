package zaiclient

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"
)

// concurrencyResult captures the outcome of a single concurrent request.
type concurrencyResult struct {
	index   int
	success bool
	err     error
	elapsed time.Duration
}

// TestGLM45ConcurrencyLimits tests the documented concurrency limit of 10
// for GLM-4.5 (ModelStandard) by sending 1, 2, 5, 8, and 10 simultaneous
// medium-complexity prompts and measuring response times.
func TestGLM45ConcurrencyLimits(t *testing.T) {
	client := testSetup(t)

	// Medium-complexity prompts to avoid caching and demonstrate concurrency
	prompts := []string{
		"Explain the difference between a stack and a queue in computer science. Give an example of when you'd use each.",
		"Write a Go function that checks if a string is a palindrome. Include error handling.",
		"What are the main differences between TCP and UDP? When would you choose one over the other?",
		"Explain how a hash table works internally. What happens during a collision?",
		"Write a Python function to find the longest common subsequence of two strings.",
		"Describe the CAP theorem in distributed systems. Give a real-world example.",
		"Explain how garbage collection works in Go. What are the trade-offs?",
		"Write a SQL query to find the second highest salary from an employees table. Explain your approach.",
		"What is the difference between concurrency and parallelism? Give examples in Go.",
		"Explain how TLS/SSL handshake works step by step.",
	}

	// Concurrency levels to test
	concurrencyLevels := []int{1, 2, 5, 8, 10}

	// Summary data structure
	type summary struct {
		concurrency int
		succeeded   int
		failed      int
		wallTime    time.Duration
		avgPerReq   time.Duration
		minPerReq   time.Duration
		maxPerReq   time.Duration
		errors      []string
	}
	var summaries []summary

	t.Logf("\n=== Starting GLM-4.5 Concurrency Test ===\n")

	for _, n := range concurrencyLevels {
		t.Logf("\n--- Testing concurrency level: %d ---", n)

		// Record wall-clock start time
		wallStart := time.Now()

		// WaitGroup to synchronize goroutines
		var wg sync.WaitGroup
		wg.Add(n)

		// Buffered channel to collect results
		resultsChan := make(chan concurrencyResult, n)

		// Launch n goroutines
		for i := 0; i < n; i++ {
			go func(idx int) {
				defer wg.Done()

				// Pick prompt based on index (distribute across all prompts)
				prompt := prompts[idx%len(prompts)]

				// Record individual request start time
				reqStart := time.Now()

				// Make the request
				result, err := client.Chat(context.Background(), ChatOptions{
					Messages: []Message{
						{Role: "user", Content: prompt},
					},
					Model:          ModelStandard,
					MaxTokens:      500,
					EnableThinking: false,
				})

				reqElapsed := time.Since(reqStart)

				// Determine success
				success := err == nil && result != nil && result.Content != ""

				resultsChan <- concurrencyResult{
					index:   idx,
					success: success,
					err:     err,
					elapsed: reqElapsed,
				}
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()
		close(resultsChan)

		// Calculate wall-clock time
		wallElapsed := time.Since(wallStart)

		// Collect results
		var results []concurrencyResult
		for result := range resultsChan {
			results = append(results, result)
		}

		// Analyze results
		var succeeded, failed int
		var totalElapsed time.Duration
		minElapsed := time.Duration(math.MaxInt64)
		maxElapsed := time.Duration(0)
		var errorTexts []string
		var rateLimitHits int

		for _, result := range results {
			if result.success {
				succeeded++
				totalElapsed += result.elapsed
				if result.elapsed < minElapsed {
					minElapsed = result.elapsed
				}
				if result.elapsed > maxElapsed {
					maxElapsed = result.elapsed
				}
			} else {
				failed++
				errMsg := fmt.Sprintf("Request %d: %v", result.index, result.err)
				errorTexts = append(errorTexts, errMsg)

				// Check for rate limiting
				if result.err != nil {
					errStr := strings.ToLower(result.err.Error())
					if strings.Contains(errStr, "429") ||
						strings.Contains(errStr, "rate") ||
						strings.Contains(errStr, "limit") ||
						strings.Contains(errStr, "concurrent") {
						rateLimitHits++
					}
				}
			}
		}

		// Calculate average
		var avgElapsed time.Duration
		if succeeded > 0 {
			avgElapsed = totalElapsed / time.Duration(succeeded)
		}

		// Handle case where no requests succeeded
		if succeeded == 0 {
			minElapsed = 0
			maxElapsed = 0
		}

		// Log detailed results for this concurrency level
		t.Logf("Concurrency %d Results:", n)
		t.Logf("  Succeeded: %d", succeeded)
		t.Logf("  Failed: %d", failed)
		t.Logf("  Wall Time: %v", wallElapsed)
		t.Logf("  Avg/Request: %v", avgElapsed)
		t.Logf("  Min: %v", minElapsed)
		t.Logf("  Max: %v", maxElapsed)

		if rateLimitHits > 0 {
			t.Logf("  RATE LIMIT HITS: %d", rateLimitHits)
		}

		if len(errorTexts) > 0 {
			t.Logf("  Errors:")
			for _, errText := range errorTexts {
				t.Logf("    %s", errText)
			}
		}

		// Store summary
		summaries = append(summaries, summary{
			concurrency: n,
			succeeded:   succeeded,
			failed:      failed,
			wallTime:    wallElapsed,
			avgPerReq:   avgElapsed,
			minPerReq:   minElapsed,
			maxPerReq:   maxElapsed,
			errors:      errorTexts,
		})

		// Sleep between tests to avoid spillover effects
		if n < concurrencyLevels[len(concurrencyLevels)-1] {
			t.Logf("  (sleeping 500ms before next test)")
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Print summary table
	t.Logf("\n=== Concurrency Test Summary ===")
	t.Logf("%-12s | %-10s | %-7s | %-10s | %-10s | %-7s | %-7s",
		"Concurrency", "Succeeded", "Failed", "Wall Time", "Avg/Req", "Min", "Max")
	t.Logf("-------------|------------|---------|------------|------------|---------|--------")

	for _, s := range summaries {
		t.Logf("%-12d | %-10d | %-7d | %-10v | %-10v | %-7v | %-7v",
			s.concurrency,
			s.succeeded,
			s.failed,
			s.wallTime.Round(100*time.Millisecond),
			s.avgPerReq.Round(100*time.Millisecond),
			s.minPerReq.Round(100*time.Millisecond),
			s.maxPerReq.Round(100*time.Millisecond),
		)
	}

	// Determine maximum successful concurrency
	maxSuccessful := 0
	for _, s := range summaries {
		if s.succeeded == s.concurrency {
			maxSuccessful = s.concurrency
		}
	}

	t.Logf("\nMaximum observed concurrency: %d (all %d requests succeeded)", maxSuccessful, maxSuccessful)

	// Additional analysis: check if response times scale linearly or if there's batching
	t.Logf("\n=== Performance Analysis ===")

	// Sort summaries by concurrency for sequential analysis
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].concurrency < summaries[j].concurrency
	})

	// Compare wall time vs expected linear scaling
	if len(summaries) > 0 && summaries[0].succeeded > 0 {
		baselineWall := summaries[0].wallTime
		t.Logf("Baseline (concurrency=1) wall time: %v", baselineWall)

		for i := 1; i < len(summaries); i++ {
			s := summaries[i]
			if s.succeeded == s.concurrency {
				// Expected wall time if requests were sequential
				expectedSequential := baselineWall * time.Duration(s.concurrency)
				// Actual wall time
				actualWall := s.wallTime
				// Speedup factor
				speedup := float64(expectedSequential) / float64(actualWall)

				t.Logf("Concurrency %d: Wall=%v, Expected(sequential)=%v, Speedup=%.2fx",
					s.concurrency,
					actualWall.Round(100*time.Millisecond),
					expectedSequential.Round(100*time.Millisecond),
					speedup,
				)
			}
		}
	}

	// Write dated report to output/
	var reportBuilder strings.Builder
	reportBuilder.WriteString(fmt.Sprintf("GLM-4.5 Concurrency Test Report\nTimestamp: %s\n\n", testTimestamp()))
	reportBuilder.WriteString(fmt.Sprintf("%-12s | %-10s | %-7s | %-10s | %-10s | %-7s | %-7s\n",
		"Concurrency", "Succeeded", "Failed", "Wall Time", "Avg/Req", "Min", "Max"))
	reportBuilder.WriteString("-------------|------------|---------|------------|------------|---------|--------\n")
	for _, s := range summaries {
		reportBuilder.WriteString(fmt.Sprintf("%-12d | %-10d | %-7d | %-10v | %-10v | %-7v | %-7v\n",
			s.concurrency, s.succeeded, s.failed,
			s.wallTime.Round(100*time.Millisecond),
			s.avgPerReq.Round(100*time.Millisecond),
			s.minPerReq.Round(100*time.Millisecond),
			s.maxPerReq.Round(100*time.Millisecond)))
	}
	reportBuilder.WriteString(fmt.Sprintf("\nMaximum observed concurrency: %d\n", maxSuccessful))
	writeTestReport(t, "glm45_concurrency.txt", reportBuilder.String())

	t.Logf("\n=== Test Complete ===\n")
}
