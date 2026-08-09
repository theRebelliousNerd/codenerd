package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	nerdconfig "codenerd/internal/config"
	"codenerd/internal/core"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

func TestInitCmd(t *testing.T) {
	// Initialize global logger
	logger = zap.NewNop()

	// Setup temp workspace
	ws := t.TempDir()
	workspace = ws // Set global workspace flag
	defer func() { workspace = "" }()

	// Mock args
	cmd := &cobra.Command{}

	// Execute runInit
	err := runInitWithLLMConfigurer(cmd, []string{}, nil)
	if err != nil {
		t.Fatalf("runInit failed: %v", err)
	}

	// Verify .nerd directory exists
	if _, err := os.Stat(filepath.Join(ws, ".nerd")); os.IsNotExist(err) {
		t.Error(".nerd directory was not created")
	}

	// Test idempotency (running it again should warn but pass)
	err = runInitWithLLMConfigurer(cmd, []string{}, nil)
	if err != nil {
		t.Errorf("runInit second run failed: %v", err)
	}
}

func TestInitCmdHonorsCancelledCommandContext(t *testing.T) {
	logger = zap.NewNop()
	ws := t.TempDir()
	workspace = ws
	defer func() { workspace = "" }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)

	err := runInitWithLLMConfigurer(cmd, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("runInitWithLLMConfigurer() error = %v, want context.Canceled", err)
	}
}

func TestScanCmd(t *testing.T) {
	logger = zap.NewNop()
	// Setup temp workspace
	ws := t.TempDir()
	workspace = ws
	defer func() { workspace = "" }()

	// Mock args
	cmd := &cobra.Command{}

	// 1. Run scan before init (must fail for reliable automation)
	err := runScan(cmd, []string{})
	if err == nil || !strings.Contains(err.Error(), "not initialized") {
		t.Fatalf("runScan error = %v, want not-initialized failure", err)
	}

	// 2. Init
	if err := runInitWithLLMConfigurer(cmd, []string{}, nil); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	// 3. Create some files to scan
	if err := os.WriteFile(filepath.Join(ws, "main.go"), []byte("package main\nfunc main() {}"), 0644); err != nil {
		t.Fatal(err)
	}

	// 4. Run scan
	err = runScan(cmd, []string{})
	if err != nil {
		t.Fatalf("runScan failed: %v", err)
	}

	// Verify facts persisted
	factsPath := filepath.Join(ws, ".nerd", "mangle", "scan.mg")
	if _, err := os.Stat(factsPath); os.IsNotExist(err) {
		t.Error("scan.mg was not created")
	}

}

func TestScanCmdHonorsCancelledCommandContext(t *testing.T) {
	logger = zap.NewNop()
	ws := t.TempDir()
	workspace = ws
	defer func() { workspace = "" }()

	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "profile.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cmd := &cobra.Command{}
	cmd.SetContext(ctx)
	if err := runScan(cmd, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("runScan error = %v, want context.Canceled", err)
	}
}

func TestApplyInitAPIKeyOverrideUsesSelectedProvider(t *testing.T) {
	cfg := &nerdconfig.UserConfig{
		Provider:     "openai",
		APIKey:       "legacy-key",
		OpenAIAPIKey: "old-openai-key",
	}
	if err := applyInitAPIKeyOverride(cfg, "flag-key"); err != nil {
		t.Fatal(err)
	}
	if cfg.OpenAIAPIKey != "flag-key" {
		t.Fatalf("OpenAIAPIKey = %q, want flag-key", cfg.OpenAIAPIKey)
	}
	if cfg.APIKey != "legacy-key" {
		t.Fatalf("legacy APIKey changed to %q", cfg.APIKey)
	}
}

func TestInitContext7APIKeyUsesTargetWorkspace(t *testing.T) {
	t.Setenv("CONTEXT7_API_KEY", "")
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	if err := os.MkdirAll(nerdDir, 0755); err != nil {
		t.Fatal(err)
	}
	configJSON := []byte(`{"context7_api_key":"workspace-key"}`)
	if err := os.WriteFile(filepath.Join(nerdDir, "config.json"), configJSON, 0600); err != nil {
		t.Fatal(err)
	}
	if got := loadCampaignConfig(nerdDir).GetContext7APIKey(); got != "workspace-key" {
		t.Fatalf("workspace Context7 key = %q, want workspace-key", got)
	}
}

func TestInitAPIKeyWorkerWarning(t *testing.T) {
	tests := []struct {
		name     string
		cfg      *nerdconfig.UserConfig
		override string
		want     bool
	}{
		{name: "different provider", cfg: &nerdconfig.UserConfig{Provider: "openai", OpenAIAPIKey: "key", Worker: &nerdconfig.WorkerLLMConfig{Provider: "meta"}}, override: "flag", want: true},
		{name: "same provider shares root key", cfg: &nerdconfig.UserConfig{Provider: "openai", OpenAIAPIKey: "key", Worker: &nerdconfig.WorkerLLMConfig{Provider: "openai"}}, override: "flag"},
		{name: "no override", cfg: &nerdconfig.UserConfig{Provider: "openai", Worker: &nerdconfig.WorkerLLMConfig{Provider: "meta"}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := initAPIKeyWorkerWarning(tt.cfg, tt.override)
			if (got != "") != tt.want {
				t.Fatalf("initAPIKeyWorkerWarning() = %q, want warning=%v", got, tt.want)
			}
		})
	}
}

type failingScanKernel struct{ err error }

func (k failingScanKernel) LoadFacts([]core.Fact) error    { return k.err }
func (k failingScanKernel) LoadFactsFromFile(string) error { return nil }

func TestScanValidationFailureDoesNotPersistArtifacts(t *testing.T) {
	logger = zap.NewNop()
	ws := t.TempDir()
	workspace = ws
	defer func() { workspace = "" }()
	if err := os.MkdirAll(filepath.Join(ws, ".nerd"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, ".nerd", "profile.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "main.go"), []byte("package main\n"), 0644); err != nil {
		t.Fatal(err)
	}
	wantErr := errors.New("rejected facts")
	err := runScanWithKernelFactory(&cobra.Command{}, nil, func(string) (scanKernel, error) {
		return failingScanKernel{err: wantErr}, nil
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("runScanWithKernelFactory() error = %v, want %v", err, wantErr)
	}
	for _, path := range []string{
		filepath.Join(ws, ".nerd", "knowledge.db"),
		filepath.Join(ws, ".nerd", "mangle", "scan.mg"),
	} {
		if _, statErr := os.Stat(path); !os.IsNotExist(statErr) {
			t.Fatalf("validation failure persisted %s (stat error %v)", path, statErr)
		}
	}
}

func TestResolveCommandWorkspaceAbsolutizesFlag(t *testing.T) {
	oldWorkspace := workspace
	workspace = filepath.Join("relative", "workspace")
	defer func() { workspace = oldWorkspace }()
	got, err := resolveCommandWorkspace()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("resolveCommandWorkspace() = %q, want absolute path", got)
	}
}

func TestSortedLanguageNames(t *testing.T) {
	got := sortedLanguageNames(map[string]int{"python": 1, "go": 2, "c": 3})
	want := []string{"c", "go", "python"}
	if !slices.Equal(got, want) {
		t.Fatalf("sortedLanguageNames() = %v, want %v", got, want)
	}
}

func TestSpawnWaitTimeoutUsesCommandBudget(t *testing.T) {
	if got := spawnWaitTimeout(10 * time.Millisecond); got != 10*time.Millisecond {
		t.Fatalf("spawnWaitTimeout() = %v, want 10ms command budget", got)
	}
}

func TestDefineAgentCmd_Validation(t *testing.T) {
	logger = zap.NewNop()
	ws := t.TempDir()
	workspace = ws
	defer func() { workspace = "" }()

	cmd := &cobra.Command{}
	// Mock flags
	cmd.Flags().String("name", "Invalid Name!", "help") // Space and ! are invalid
	cmd.Flags().String("topic", "Go", "help")

	err := defineAgent(cmd, []string{})
	if err == nil {
		t.Error("defineAgent should fail with invalid name")
	}
}

func TestDirectActions_Validation(t *testing.T) {
	if err := reviewCmd.Args(reviewCmd, nil); err == nil {
		t.Fatal("review command accepted an empty target")
	}
	if err := reviewCmd.Args(reviewCmd, []string{"internal/core/kernel.go"}); err != nil {
		t.Fatalf("review command rejected a valid target: %v", err)
	}
}

func TestQueryCmd(t *testing.T) {
	if err := queryCmd.Args(queryCmd, nil); err == nil {
		t.Fatal("query command accepted a missing predicate")
	}
	if err := queryCmd.Args(queryCmd, []string{"next_action"}); err != nil {
		t.Fatalf("query command rejected one predicate: %v", err)
	}
	if err := queryCmd.Args(queryCmd, []string{"next_action", "extra"}); err == nil {
		t.Fatal("query command accepted multiple predicates")
	}
}
