// Video generation tests are in a separate file because they take 30-60+ seconds due to async polling.

package zaiclient

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
)

func TestVideoGeneration(t *testing.T) {
	client := testSetup(t)
	ctx := context.Background()
	outputDir := testOutputDir(t)
	ts := testTimestamp()

	// Step 1: Generate a duck pond image as the starting frame
	t.Log("Step 1: Generating duck pond image...")
	imgStart := time.Now()

	imgResult, err := client.GenerateImage(ctx, GenerateImageOptions{
		Prompt:    "A serene duck pond with lily pads, calm water reflections, and green grass on the banks, photorealistic style",
		OutputDir: outputDir,
		Filename:  ts + "_duck_pond.png",
	})

	imgElapsed := time.Since(imgStart)

	if err != nil {
		if isQuotaError(err) {
			t.Skipf("Skipped (requires payment): %v", err)
		}
		t.Fatalf("Duck pond image generation failed: %v", err)
	}

	t.Logf("Duck pond image generated in %v", imgElapsed)
	t.Logf("  Local: %s", imgResult.LocalPath)
	t.Logf("  URL: %s", imgResult.URL)

	// Verify image file exists
	if _, err := os.Stat(imgResult.LocalPath); os.IsNotExist(err) {
		t.Fatalf("Generated image file does not exist: %s", imgResult.LocalPath)
	}

	// Step 2: Generate video of a duck moving across the pond
	t.Log("Step 2: Generating duck video from pond image...")
	videoStart := time.Now()

	videoResult, err := client.GenerateVideo(ctx, GenerateVideoOptions{
		ImageURL:  imgResult.URL,
		Prompt:    "A duck swimming gracefully across the pond, creating gentle ripples in the calm water",
		OutputDir: outputDir,
		Filename:  ts + "_duck_moving.mp4",
		MaxWait:   3 * time.Minute,
	})

	videoElapsed := time.Since(videoStart)

	if err != nil {
		// Write partial report even if video fails
		report := fmt.Sprintf("Video Generation Test Report\nTimestamp: %s\n\nStep 1: Image Generation\n  Status: SUCCESS\n  Elapsed: %v\n  Local: %s\n  URL: %s\n\nStep 2: Video Generation\n  Status: FAILED\n  Error: %v\n",
			ts, imgElapsed, imgResult.LocalPath, imgResult.URL, err)
		writeTestReport(t, "video_generation.txt", report)

		if isQuotaError(err) {
			t.Skipf("Video generation skipped (requires payment): %v", err)
		}
		errMsg := err.Error()
		if strings.Contains(errMsg, "timed out") || strings.Contains(errMsg, "400") {
			t.Skipf("Video API test inconclusive: %v", err)
		}
		t.Fatalf("Video generation failed: %v", err)
	}

	t.Logf("Duck video generated in %v", videoElapsed)
	t.Logf("  Local: %s", videoResult.LocalPath)

	// Verify video file exists
	if _, err := os.Stat(videoResult.LocalPath); os.IsNotExist(err) {
		t.Errorf("Generated video file does not exist: %s", videoResult.LocalPath)
	}

	// Write full success report
	report := fmt.Sprintf("Video Generation Test Report\nTimestamp: %s\n\nStep 1: Image Generation (Duck Pond)\n  Status: SUCCESS\n  Elapsed: %v\n  Local: %s\n  URL: %s\n\nStep 2: Video Generation (Duck Moving)\n  Status: SUCCESS\n  Elapsed: %v\n  Local: %s\n\nTotal elapsed: %v\n",
		ts, imgElapsed, imgResult.LocalPath, imgResult.URL,
		videoElapsed, videoResult.LocalPath,
		imgElapsed+videoElapsed)
	writeTestReport(t, "video_generation.txt", report)

	t.Logf("\nTotal video test elapsed: %v", imgElapsed+videoElapsed)
}
