package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/embedding"
)

func TestEmbeddingPullTimeout_AbsentIs30m(t *testing.T) {
	d, err := DefaultEmbeddingConfig().ResolvedPullTimeout()
	if err != nil {
		t.Fatal(err)
	}
	if d != 30*time.Minute {
		t.Fatalf("default embedding.pull_timeout = %s, want 30m", d)
	}
	if got, err := (EmbeddingConfig{}).ResolvedPullTimeout(); err != nil || got != 30*time.Minute {
		t.Fatalf("empty PullTimeout = %s, %v; want 30m", got, err)
	}
	for _, raw := range []string{"0s", "-1s", "soon"} {
		if _, err := (EmbeddingConfig{PullTimeout: raw}).ResolvedPullTimeout(); err == nil {
			t.Fatalf("PullTimeout %q parsed", raw)
		}
	}
	if got := (*UserConfig)(nil).GetEmbeddingConfig().PullTimeout; got != "30m" {
		t.Fatalf("nil config pull_timeout = %q, want 30m", got)
	}
	partial := (&UserConfig{Embedding: &EmbeddingConfig{Provider: "ollama"}}).GetEmbeddingConfig()
	if partial.PullTimeout != "30m" {
		t.Fatalf("absent pull_timeout = %q, want 30m", partial.PullTimeout)
	}
	kept := (&UserConfig{Embedding: &EmbeddingConfig{PullTimeout: "45s"}}).GetEmbeddingConfig()
	if kept.PullTimeout != "45s" {
		t.Fatalf("explicit pull_timeout = %q, want 45s", kept.PullTimeout)
	}
	restoreInstalledTimeouts(t)
	installDefaultEmbeddingPullTimeout()
	if got := embedding.PullTimeout(); got != 30*time.Minute {
		t.Fatalf("default ollama pull bound = %s, want 30m published from embedding.pull_timeout", got)
	}
}

func TestLoadUserConfig_AbsentPullTimeoutInstalls30m(t *testing.T) {
	restoreInstalledTimeouts(t)
	SetEmbeddingPullTimeout(2 * time.Second)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"embedding":{"provider":"ollama"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadUserConfig(path); err != nil {
		t.Fatal(err)
	}
	if got := EmbeddingPullTimeout(); got != 30*time.Minute {
		t.Fatalf("installed pull timeout = %s, want 30m", got)
	}
	if got := embedding.PullTimeout(); got != 30*time.Minute {
		t.Fatalf("published pull timeout = %s, want 30m", got)
	}
}

func TestLoadUserConfig_MissingFileInstallsDefaultPullTimeout(t *testing.T) {
	restoreInstalledTimeouts(t)
	SetEmbeddingPullTimeout(2 * time.Second)
	if _, err := LoadUserConfig(filepath.Join(t.TempDir(), "missing.json")); err != nil {
		t.Fatal(err)
	}
	if got := EmbeddingPullTimeout(); got != 30*time.Minute {
		t.Fatalf("missing file installed pull timeout = %s, want 30m", got)
	}
	if got := embedding.PullTimeout(); got != 30*time.Minute {
		t.Fatalf("published pull timeout = %s, want 30m", got)
	}
}

func TestLoadUserConfig_InstallsPullTimeout(t *testing.T) {
	restoreInstalledTimeouts(t)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"embedding":{"pull_timeout":"45s"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := EmbeddingPullTimeout(); got != 45*time.Second {
		t.Fatalf("installed pull timeout = %s, want 45s", got)
	}
	if got := embedding.PullTimeout(); got != 45*time.Second {
		t.Fatalf("published pull timeout = %s, want 45s", got)
	}
	resolved, err := cfg.GetEmbeddingConfig().ResolvedPullTimeout()
	if err != nil || resolved != 45*time.Second {
		t.Fatalf("resolved = %s, %v; want 45s", resolved, err)
	}
}

func TestCheck_InvalidPullTimeout(t *testing.T) {
	for _, raw := range []string{"soon", "0s", "-5s"} {
		cfg := &UserConfig{Embedding: &EmbeddingConfig{PullTimeout: raw}}
		if p := problemAt(cfg.Check(nil), SeverityError, "embedding.pull_timeout"); p == nil {
			t.Fatalf("pull_timeout %q was accepted: %v", raw, cfg.Check(nil))
		}
	}
	absent := &UserConfig{Embedding: &EmbeddingConfig{}}
	if p := problemAt(absent.Check(nil), SeverityError, "embedding.pull_timeout"); p != nil {
		t.Fatalf("absent pull_timeout was an error: %+v", p)
	}

	restoreInstalledTimeouts(t)
	before := EmbeddingPullTimeout()
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"embedding":{"pull_timeout":"soon"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUserConfig(path)
	if err == nil || !strings.Contains(err.Error(), "embedding.pull_timeout") {
		t.Fatalf("LoadUserConfig error = %v, want embedding.pull_timeout", err)
	}
	if got := EmbeddingPullTimeout(); got != before {
		t.Fatalf("refused file changed the installed pull timeout from %s to %s", before, got)
	}
	if err := os.WriteFile(path, []byte(`{"embedding":{"pull_timeout":"0s"}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = LoadUserConfig(path)
	if err == nil || !strings.Contains(err.Error(), "embedding.pull_timeout") {
		t.Fatalf("LoadUserConfig error = %v, want a non-positive pull_timeout refused", err)
	}
}

func TestFullJSON_EmbeddingPullTimeoutPresent(t *testing.T) {
	full, err := FullJSON([]byte(`{"embedding":{"pull_timeout":"45s"}}`))
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(full, &doc); err != nil {
		t.Fatal(err)
	}
	emb, _ := doc["embedding"].(map[string]any)
	if emb["pull_timeout"] != "45s" {
		t.Fatalf("full document pull_timeout = %v, want 45s", emb["pull_timeout"])
	}
	if implicit := ImplicitFields(full); len(implicit) != 0 {
		t.Fatalf("full document leaves fields implicit: %v", implicit)
	}
}
