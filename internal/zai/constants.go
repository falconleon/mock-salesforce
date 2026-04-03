package zaiclient

import "time"

// API Endpoints
const (
	EndpointChat        = "https://api.z.ai/api/coding/paas/v4/chat/completions"
	EndpointImage       = "https://api.z.ai/api/paas/v4/images/generations"
	EndpointVideo       = "https://api.z.ai/api/paas/v4/videos/generations"
	EndpointVideoStatus = "https://api.z.ai/api/paas/v4/async-result"
)

// Language Models (available on current plan)
const (
	ModelAdvanced = "GLM-4.7"       // Most advanced reasoning (recommended)
	ModelFlagship = "GLM-4.6"       // Previous flagship
	ModelStandard = "GLM-4.5"       // Standard, high concurrency
	ModelAir      = "GLM-4.5-Air"   // Lightweight
	ModelFlash    = "GLM-4.5-Flash" // Ultra-fast
)

// Vision Models (available on current plan)
const (
	ModelVision         = "GLM-4.6V"       // Vision flagship (recommended)
	ModelVisionStandard = "GLM-4.5V"       // Vision standard
	ModelVisionFlash    = "GLM-4.6V-Flash" // Vision fast
)

// Image Generation Models
const (
	ModelCogView = "cogView-4-250304" // Text-to-image ($0.01/image)
)

// Video Generation Models
const (
	ModelVidu2Image     = "vidu2-image"     // Image-to-video ($0.2/video)
	ModelVidu2StartEnd  = "vidu2-start-end" // Start/end keyframes ($0.2/video)
	ModelVidu2Reference = "vidu2-reference" // Reference-based ($0.4/video)
)

// Defaults
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
