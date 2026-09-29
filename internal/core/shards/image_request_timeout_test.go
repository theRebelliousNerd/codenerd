package shards

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
)

func restoreImageRequestTimeout(t *testing.T) {
	t.Helper()
	before := config.ImageRequestTimeout()
	t.Cleanup(func() { config.SetImageRequestTimeout(before) })
}

// The image profile carries the configured request bound (image.timeout),
// not a literal: one generation is one request to the image model.
func TestDefaultImageGeneratorConfig_TimeoutFromConfig(t *testing.T) {
	restoreImageRequestTimeout(t)
	config.SetImageRequestTimeout(42 * time.Second)
	if got := DefaultImageGeneratorConfig("image_generator").Timeout; got != 42*time.Second {
		t.Fatalf("profile timeout = %s, want the configured 42s", got)
	}
}

// A profile without a timeout still gets the configured bound instead of an
// unbounded call: a slow model hits it.
func TestImageGeneratorAgent_NonPositiveProfileTimeoutUsesConfiguredBound(t *testing.T) {
	restoreImageRequestTimeout(t)
	config.SetImageRequestTimeout(50 * time.Millisecond)
	for _, profileTimeout := range []time.Duration{0, -time.Second} {
		cfg := DefaultImageGeneratorConfig("image_generator")
		cfg.Timeout = profileTimeout
		agent := NewImageGeneratorAgent("img-bound", cfg)
		agent.SetLLMClient(&stubImageLLM{delay: 300 * time.Millisecond, response: "late"})
		_, err := agent.Execute(context.Background(), "draw a square")
		if err == nil || !strings.Contains(err.Error(), "timed out") {
			t.Errorf("profile timeout %s: err = %v, want the configured-bound timeout", profileTimeout, err)
		}
	}
}

// ...while a fast call under the configured bound succeeds.
func TestImageGeneratorAgent_ConfiguredBoundAllowsFastCall(t *testing.T) {
	restoreImageRequestTimeout(t)
	config.SetImageRequestTimeout(5 * time.Second)
	cfg := DefaultImageGeneratorConfig("image_generator")
	cfg.Timeout = 0
	agent := NewImageGeneratorAgent("img-fast", cfg)
	agent.SetLLMClient(&stubImageLLM{delay: 10 * time.Millisecond, response: "image-ok"})
	res, err := agent.Execute(context.Background(), "tiny green checkmark")
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res != "image-ok" {
		t.Fatalf("got %q", res)
	}
}
