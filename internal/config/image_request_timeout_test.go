package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func restoreImageRequestTimeout(t *testing.T) {
	t.Helper()
	before := ImageRequestTimeout()
	t.Cleanup(func() { SetImageRequestTimeout(before) })
}

// The image request bound defaults to 120s: one generation is one request to
// the image model, and the shard's old 2-minute literal moved here.
func TestDefaultImageLLMConfig_TimeoutIs120s(t *testing.T) {
	def := DefaultImageLLMConfig()
	if def.Provider != "gemini" {
		t.Fatalf("provider = %q, want gemini", def.Provider)
	}
	if def.Model != "" {
		t.Fatalf("model = %q, want unset (image.model is required)", def.Model)
	}
	if def.Timeout != 120 {
		t.Fatalf("timeout = %d, want 120", def.Timeout)
	}
	if got := ImageRequestTimeout(); got != 120*time.Second {
		t.Fatalf("installed default = %s, want 120s", got)
	}
}

func TestGetImageLLMConfig_FillsTimeout(t *testing.T) {
	if got := (*UserConfig)(nil).GetImageLLMConfig().Timeout; got != 120 {
		t.Fatalf("nil config timeout = %d, want 120", got)
	}
	partial := (&UserConfig{Image: &ImageLLMConfig{Model: "gemini-3.1-flash-image"}}).GetImageLLMConfig()
	if partial.Timeout != 120 {
		t.Fatalf("absent timeout = %d, want 120", partial.Timeout)
	}
	for _, nonPositive := range []int{0, -5} {
		filled := (&UserConfig{Image: &ImageLLMConfig{Timeout: nonPositive}}).GetImageLLMConfig()
		if filled.Timeout != 120 {
			t.Fatalf("timeout %d filled to %d, want 120", nonPositive, filled.Timeout)
		}
	}
	kept := (&UserConfig{Image: &ImageLLMConfig{Timeout: 45}}).GetImageLLMConfig()
	if kept.Timeout != 45 {
		t.Fatalf("explicit timeout = %d, want 45", kept.Timeout)
	}
}

func TestLoadUserConfig_InstallsImageRequestTimeout(t *testing.T) {
	restoreImageRequestTimeout(t)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"image":{"timeout":45}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := ImageRequestTimeout(); got != 45*time.Second {
		t.Fatalf("installed image request timeout = %s, want 45s", got)
	}
	if got := cfg.GetImageLLMConfig().Timeout; got != 45 {
		t.Fatalf("resolved timeout = %d, want 45", got)
	}
}

func TestLoadUserConfig_AbsentImageTimeoutInstallsDefault(t *testing.T) {
	restoreImageRequestTimeout(t)
	SetImageRequestTimeout(2 * time.Second)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"image":{"provider":"gemini"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUserConfig(path); err != nil {
		t.Fatal(err)
	}
	if got := ImageRequestTimeout(); got != 120*time.Second {
		t.Fatalf("installed image request timeout = %s, want 120s", got)
	}
}
