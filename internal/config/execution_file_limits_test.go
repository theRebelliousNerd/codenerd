package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func restoreProcessLimits(t *testing.T) {
	t.Helper()
	prevRead := ResolvedMaxReadFileBytes()
	prevSearch := ResolvedMaxSearchFileBytes()
	prevTimeouts := GetLLMTimeouts()
	t.Cleanup(func() {
		SetExecutionFileLimits(ExecutionConfig{
			MaxReadFileBytes:   prevRead,
			MaxSearchFileBytes: prevSearch,
		})
		SetLLMTimeouts(prevTimeouts)
	})
}

func TestLoadUserConfig_InstallsFileCeilings(t *testing.T) {
	restoreProcessLimits(t)
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"execution":{"max_read_file_bytes":1234,"max_search_file_bytes":5678}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	got := cfg.GetExecution()
	if got.MaxReadFileBytes != 1234 || got.MaxSearchFileBytes != 5678 {
		t.Fatalf("execution ceilings = %d/%d, want 1234/5678", got.MaxReadFileBytes, got.MaxSearchFileBytes)
	}
	if ResolvedMaxReadFileBytes() != 1234 || ResolvedMaxSearchFileBytes() != 5678 {
		t.Fatalf("process ceilings = %d/%d, want 1234/5678", ResolvedMaxReadFileBytes(), ResolvedMaxSearchFileBytes())
	}
}

func TestLoadUserConfig_AbsentFileCeilingsUseDefaults(t *testing.T) {
	restoreProcessLimits(t)
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"provider":"ollama"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	got := cfg.GetExecution()
	if got.MaxReadFileBytes != 0 {
		t.Fatalf("MaxReadFileBytes = %d, want 0 (read the file whole)", got.MaxReadFileBytes)
	}
	if got.MaxSearchFileBytes != 1<<20 {
		t.Fatalf("MaxSearchFileBytes = %d, want 1MiB", got.MaxSearchFileBytes)
	}
	if ResolvedMaxReadFileBytes() != 0 || ResolvedMaxSearchFileBytes() != 1<<20 {
		t.Fatalf("process ceilings = %d/%d, want 0/%d", ResolvedMaxReadFileBytes(), ResolvedMaxSearchFileBytes(), 1<<20)
	}
}

func TestLoadUserConfig_ZeroSearchCeilingTakesTheDefault(t *testing.T) {
	restoreProcessLimits(t)
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"execution":{"max_read_file_bytes":0,"max_search_file_bytes":0}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	got := cfg.GetExecution()
	if got.MaxReadFileBytes != 0 {
		t.Fatalf("MaxReadFileBytes = %d, want 0", got.MaxReadFileBytes)
	}
	if got.MaxSearchFileBytes != 1<<20 {
		t.Fatalf("MaxSearchFileBytes = %d, want the default 1MiB when the key is 0", got.MaxSearchFileBytes)
	}
}

func TestLoadUserConfig_RejectsNegativeFileCeilings(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	body := `{"execution":{"max_read_file_bytes":-1,"max_search_file_bytes":-5}}`
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadUserConfig(path)
	if err == nil {
		t.Fatal("expected a negative ceiling to refuse the file")
	}
	msg := err.Error()
	if !strings.Contains(msg, "max_read_file_bytes") || !strings.Contains(msg, "max_search_file_bytes") {
		t.Fatalf("error = %v", err)
	}
}
