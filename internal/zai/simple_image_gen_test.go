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

// imageGenResult captures the outcome of a single concurrent image generation request.
type imageGenResult struct {
	index     int
	success   bool
	err       error
	elapsed   time.Duration
	localPath string
}

// TestCogViewConcurrencyLimits tests the documented concurrency limit of 5
// for CogView-4-250304 image generation by sending 1, 2, 3, and 5 simultaneous
// image generation requests and measuring performance.
func TestCogViewConcurrencyLimits(t *testing.T) {
	client := testSetup(t)

	// Create output directory for generated images
	tmpDir := testOutputDir(t)

	// Simple image generation prompts suited for profile-image-like tasks
	prompts := []string{
		"A simple blue circle avatar on white background",
		"A minimalist red geometric pattern for a profile picture",
		"A green leaf icon on a light gray background",
		"A purple abstract gradient suitable for a user avatar",
		"An orange sun icon with simple rays on white background",
	}

	// First, test if image generation is available with a single request
	t.Logf("\n=== Pre-check: Testing if image generation is available ===\n")
	preCheckCtx, preCheckCancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer preCheckCancel()

	preCheckResult, preCheckErr := client.GenerateImage(preCheckCtx, GenerateImageOptions{
		Prompt:    prompts[0],
		OutputDir: tmpDir,
		Filename:  testTimestamp() + "_precheck.png",
	})

	if preCheckErr != nil {
		if isQuotaError(preCheckErr) {
			t.Skipf("Skipping all concurrency tests (requires payment): %v", preCheckErr)
		}
		t.Fatalf("Pre-check image generation failed: %v", preCheckErr)
	}

	if preCheckResult != nil && preCheckResult.LocalPath != "" {
		t.Logf("Pre-check passed. Generated image: %s", preCheckResult.LocalPath)
	}

	// Concurrency levels to test (documented limit is 5)
	concurrencyLevels := []int{1, 2, 3, 5}

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

	t.Logf("\n=== Starting CogView-4-250304 Concurrency Test ===\n")

	for _, n := range concurrencyLevels {
		t.Logf("\n--- Testing concurrency level: %d ---", n)

		// Record wall-clock start time
		wallStart := time.Now()

		// WaitGroup to synchronize goroutines
		var wg sync.WaitGroup
		wg.Add(n)

		// Buffered channel to collect results
		resultsChan := make(chan imageGenResult, n)

		// Launch n goroutines
		for i := 0; i < n; i++ {
			go func(idx int) {
				defer wg.Done()

				// Pick prompt based on index (distribute across all prompts)
				prompt := prompts[idx%len(prompts)]

				// Record individual request start time
				reqStart := time.Now()

				// Create context with timeout for individual request
				ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
				defer cancel()

				// Make the request
				result, err := client.GenerateImage(ctx, GenerateImageOptions{
					Prompt:    prompt,
					OutputDir: tmpDir,
					Filename:  fmt.Sprintf("%s_image_%d_%d.png", testTimestamp(), n, idx),
				})

				reqElapsed := time.Since(reqStart)

				// Determine success
				success := err == nil && result != nil && result.LocalPath != ""

				var localPath string
				if result != nil {
					localPath = result.LocalPath
				}

				resultsChan <- imageGenResult{
					index:     idx,
					success:   success,
					err:       err,
					elapsed:   reqElapsed,
					localPath: localPath,
				}
			}(i)
		}

		// Wait for all goroutines to complete
		wg.Wait()
		close(resultsChan)

		// Calculate wall-clock time
		wallElapsed := time.Since(wallStart)

		// Collect results
		var results []imageGenResult
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

		// Sleep between concurrency levels (image generation is heavier than chat)
		if n < concurrencyLevels[len(concurrencyLevels)-1] {
			t.Logf("  (sleeping 1s before next test)")
			time.Sleep(1 * time.Second)
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
	reportBuilder.WriteString(fmt.Sprintf("CogView Concurrency Test Report\nTimestamp: %s\n\n", testTimestamp()))
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
	writeTestReport(t, "cogview_concurrency.txt", reportBuilder.String())

	t.Logf("\n=== Test Complete (generated files in %s) ===\n", tmpDir)
}
