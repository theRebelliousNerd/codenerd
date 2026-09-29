package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func restoreInstalledTimeouts(t *testing.T) {
	t.Helper()
	llm := GetLLMTimeouts()
	emb := EmbeddingRequestTimeout()
	t.Cleanup(func() {
		SetLLMTimeouts(llm)
		SetEmbeddingRequestTimeout(emb)
	})
}

func TestEmbeddingRequestTimeout_DefaultIs60s(t *testing.T) {
	d, err := DefaultEmbeddingConfig().ResolvedRequestTimeout()
	if err != nil {
		t.Fatal(err)
	}
	if d != 60*time.Second {
		t.Fatalf("default embedding.request_timeout = %s, want 60s", d)
	}
	if got, err := (EmbeddingConfig{}).ResolvedRequestTimeout(); err != nil || got != 60*time.Second {
		t.Fatalf("empty RequestTimeout = %s, %v; want 60s", got, err)
	}
	for _, raw := range []string{"0s", "-1s", "soon"} {
		if _, err := (EmbeddingConfig{RequestTimeout: raw}).ResolvedRequestTimeout(); err == nil {
			t.Fatalf("RequestTimeout %q parsed", raw)
		}
	}
}

func TestGetEmbeddingConfig_FillsRequestTimeout(t *testing.T) {
	if got := (*UserConfig)(nil).GetEmbeddingConfig().RequestTimeout; got != "60s" {
		t.Fatalf("nil config request_timeout = %q, want 60s", got)
	}
	partial := (&UserConfig{Embedding: &EmbeddingConfig{Provider: "ollama"}}).GetEmbeddingConfig()
	if partial.RequestTimeout != "60s" {
		t.Fatalf("absent request_timeout = %q, want 60s", partial.RequestTimeout)
	}
	kept := (&UserConfig{Embedding: &EmbeddingConfig{RequestTimeout: "2s"}}).GetEmbeddingConfig()
	if kept.RequestTimeout != "2s" {
		t.Fatalf("explicit request_timeout = %q, want 2s", kept.RequestTimeout)
	}
}

func TestLoadUserConfig_InstallsEmbeddingRequestTimeout(t *testing.T) {
	restoreInstalledTimeouts(t)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"embedding":{"request_timeout":"1500ms"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := EmbeddingRequestTimeout(); got != 1500*time.Millisecond {
		t.Fatalf("installed embedding request timeout = %s, want 1500ms", got)
	}
	resolved, err := cfg.GetEmbeddingConfig().ResolvedRequestTimeout()
	if err != nil || resolved != 1500*time.Millisecond {
		t.Fatalf("resolved = %s, %v; want 1500ms", resolved, err)
	}
}

func TestLoadUserConfig_AbsentEmbeddingRequestTimeoutInstallsDefault(t *testing.T) {
	restoreInstalledTimeouts(t)
	SetEmbeddingRequestTimeout(2 * time.Second)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"embedding":{"provider":"ollama"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUserConfig(path); err != nil {
		t.Fatal(err)
	}
	if got := EmbeddingRequestTimeout(); got != 60*time.Second {
		t.Fatalf("installed embedding request timeout = %s, want 60s", got)
	}
}

func TestLoadUserConfig_RefusesBadEmbeddingRequestTimeout(t *testing.T) {
	restoreInstalledTimeouts(t)
	before := EmbeddingRequestTimeout()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"embedding":{"request_timeout":"soon"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUserConfig(path)
	if err == nil || !strings.Contains(err.Error(), "embedding.request_timeout") {
		t.Fatalf("LoadUserConfig error = %v, want embedding.request_timeout", err)
	}
	if got := EmbeddingRequestTimeout(); got != before {
		t.Fatalf("refused file changed the installed timeout from %s to %s", before, got)
	}

	if err := os.WriteFile(path, []byte(`{"embedding":{"request_timeout":"0s"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = LoadUserConfig(path)
	if err == nil || !strings.Contains(err.Error(), "embedding.request_timeout") {
		t.Fatalf("LoadUserConfig error = %v, want a non-positive request_timeout refused", err)
	}
}

func TestFullJSON_EmbeddingRequestTimeoutPresent(t *testing.T) {
	full, err := FullJSON([]byte(`{"embedding":{"request_timeout":"1500ms"}}`))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(full, &doc); err != nil {
		t.Fatal(err)
	}
	emb, _ := doc["embedding"].(map[string]any)
	if emb["request_timeout"] != "1500ms" {
		t.Fatalf("full document request_timeout = %v, want 1500ms", emb["request_timeout"])
	}
	if implicit := ImplicitFields(full); len(implicit) != 0 {
		t.Fatalf("full document leaves fields implicit: %v", implicit)
	}
}
