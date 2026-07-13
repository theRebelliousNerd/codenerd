package config

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLoadUserConfigRejectsUnknownAndTrailingJSON(t *testing.T) {
	tests := map[string]string{
		"unknown root field":   `{"provider":"ollama","typo_provider":true}`,
		"unknown jit field":    `{"jit":{"enabled":true,"typo_budget":12}}`,
		"unknown reflection":   `{"reflection":{"enabled":true,"typo_score":0.5}}`,
		"second JSON document": `{"provider":"ollama"}{"provider":"zai"}`,
	}
	for name, content := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := LoadUserConfig(path); err == nil {
				t.Fatalf("LoadUserConfig accepted invalid policy: %s", content)
			}
		})
	}
}

func TestPrivateAtomicWritePreservesOriginalOnReplaceFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("original"), 0o600); err != nil {
		t.Fatal(err)
	}

	wantErr := errors.New("injected replace failure")
	err := writePrivateFileAtomicallyWithReplace(path, []byte("replacement"), func(_, _ string) error {
		return wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("write error = %v, want injected replace failure", err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "original" {
		t.Fatalf("existing config changed after failed replace: %q", got)
	}
	leftovers, err := filepath.Glob(filepath.Join(dir, ".codenerd-config-*.tmp"))
	if err != nil {
		t.Fatal(err)
	}
	if len(leftovers) != 0 {
		t.Fatalf("temporary config files leaked: %v", leftovers)
	}
}

func TestUserConfigSaveIsPrivateAndRoundTrips(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".nerd", "config.json")
	want := &UserConfig{Provider: "ollama", Context7APIKey: "secret"}
	if err := want.Save(path); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Fatal("saved config is missing final newline")
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := info.Mode().Perm(); got != privateConfigMode {
			t.Fatalf("config permissions = %o, want %o", got, privateConfigMode)
		}
	}
	got, err := LoadUserConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.Provider != want.Provider || got.Context7APIKey != want.Context7APIKey {
		t.Fatalf("round trip = %#v, want provider/key preserved", got)
	}
}

func TestSensitiveTracingDefaultsOff(t *testing.T) {
	if DefaultUserConfig().GetLogging().TraceLLMIO {
		t.Fatal("default user config enables raw LLM I/O tracing")
	}
	if (&UserConfig{}).GetLogging().TraceLLMIO {
		t.Fatal("empty user config enables raw LLM I/O tracing")
	}
	if DefaultJITConfig().TraceLLMIO {
		t.Fatal("default JIT config enables raw LLM I/O tracing")
	}
}

func TestHasExplicitLLMSelection(t *testing.T) {
	if (&UserConfig{}).HasExplicitLLMSelection() {
		t.Fatal("empty config reported an explicit LLM selection")
	}
	if !(&UserConfig{Provider: "openai"}).HasExplicitLLMSelection() {
		t.Fatal("explicit provider was not detected")
	}
	if !(&UserConfig{CodexCLI: &CodexCLIConfig{}}).HasExplicitLLMSelection() {
		t.Fatal("explicit Codex CLI block was not detected")
	}
}
