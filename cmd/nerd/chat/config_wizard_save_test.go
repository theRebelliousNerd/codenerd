package chat

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	internalconfig "codenerd/internal/config"
)

func TestSaveConfigWizardPreservesUnownedSettings(t *testing.T) {
	// The model carries its workspace, so the wizard saves into an explicit
	// path; nothing here depends on the process working directory or on
	// whatever DefaultUserConfigPath() resolves to.
	workspace := t.TempDir()

	base := &internalconfig.UserConfig{
		Provider: "openai",
		Execution: &internalconfig.ExecutionConfig{
			AllowedBinaries:  []string{"go", "git"},
			AllowedEnvVars:   []string{"PATH", "GOCACHE"},
			DefaultTimeout:   "2m",
			WorkingDirectory: "src",
		},
		Logging: &internalconfig.LoggingConfig{
			DebugMode:           true,
			TraceLLMIO:          false,
			Categories:          map[string]bool{"kernel": true},
			PerformanceSampling: 0.25,
		},
		Integrations: &internalconfig.IntegrationsConfig{
			Servers: map[string]internalconfig.MCPServerIntegration{
				"preserve-me": {Enabled: true, Protocol: "http", BaseURL: "http://127.0.0.1:9000"},
			},
		},
	}
	configPath := filepath.Join(workspace, ".nerd", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := base.Save(configPath); err != nil {
		t.Fatal(err)
	}

	wizard := NewConfigWizard()
	wizard.Engine = "codex-cli"
	wizard.CodexCLIModel = "gpt-5.4"
	wizard.CodexCLISandbox = "workspace-write" // legacy state must still save safely
	wizard.EmbeddingProvider = "ollama"

	model := Model{configWizard: wizard, workspace: workspace}
	if err := model.saveConfigWizard(); err != nil {
		t.Fatal(err)
	}
	got, err := internalconfig.LoadUserConfig(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got.Execution, base.Execution) {
		t.Fatalf("execution config was overwritten: got %#v want %#v", got.Execution, base.Execution)
	}
	if !reflect.DeepEqual(got.Logging, base.Logging) {
		t.Fatalf("logging config was overwritten: got %#v want %#v", got.Logging, base.Logging)
	}
	if !reflect.DeepEqual(got.Integrations, base.Integrations) {
		t.Fatalf("integration config was overwritten: got %#v want %#v", got.Integrations, base.Integrations)
	}
	if got.CodexCLI == nil || got.CodexCLI.Sandbox != "read-only" {
		t.Fatalf("wizard saved unsafe Codex sandbox: %#v", got.CodexCLI)
	}
}
