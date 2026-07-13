package system

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
)

func TestExecutionLayerConfigsProjectUserPolicy(t *testing.T) {
	workspace := t.TempDir()
	appCfg := &config.UserConfig{Execution: &config.ExecutionConfig{
		AllowedBinaries:  []string{"go", "git"},
		AllowedEnvVars:   []string{"PATH", "GOCACHE"},
		DefaultTimeout:   "2m",
		WorkingDirectory: "src",
	}}

	executorCfg, virtualStoreCfg, err := executionLayerConfigs(appCfg, workspace)
	if err != nil {
		t.Fatal(err)
	}
	wantDir := filepath.Join(workspace, "src")
	if executorCfg.DefaultWorkingDir != wantDir || virtualStoreCfg.WorkingDir != wantDir {
		t.Fatalf("working directory projection = %q / %q, want %q", executorCfg.DefaultWorkingDir, virtualStoreCfg.WorkingDir, wantDir)
	}
	if executorCfg.DefaultTimeout != 2*time.Minute {
		t.Fatalf("executor timeout = %s, want 2m", executorCfg.DefaultTimeout)
	}
	if executorCfg.DefaultLimits == nil || executorCfg.DefaultLimits.TimeoutMs != (2*time.Minute).Milliseconds() {
		t.Fatalf("executor default limits did not inherit timeout: %#v", executorCfg.DefaultLimits)
	}
	if !reflect.DeepEqual(executorCfg.AllowedEnvironment, appCfg.Execution.AllowedEnvVars) ||
		!reflect.DeepEqual(virtualStoreCfg.AllowedEnvVars, appCfg.Execution.AllowedEnvVars) ||
		!reflect.DeepEqual(virtualStoreCfg.AllowedBinaries, appCfg.Execution.AllowedBinaries) {
		t.Fatalf("execution allowlists were not projected: executor=%v virtualStore=%+v", executorCfg.AllowedEnvironment, virtualStoreCfg)
	}
}

func TestExecutionLayerConfigsRejectInvalidPolicy(t *testing.T) {
	workspace := t.TempDir()
	tests := map[string]*config.ExecutionConfig{
		"invalid timeout": {DefaultTimeout: "eventually"},
		"path escape":     {DefaultTimeout: "30s", WorkingDirectory: "../outside"},
	}
	for name, execution := range tests {
		t.Run(name, func(t *testing.T) {
			_, _, err := executionLayerConfigs(&config.UserConfig{Execution: execution}, workspace)
			if err == nil {
				t.Fatal("invalid execution policy was accepted")
			}
		})
	}
}

func TestInitCoreComponentsRejectsPresentInvalidConfig(t *testing.T) {
	workspace := t.TempDir()
	nerdDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nerdDir, "config.json"), []byte(`{"provider":"openai","typo_key":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XAI_API_KEY", "must-not-select-ambient-provider")

	bctx := &bootContext{cfg: BootConfig{Workspace: workspace}}
	err := initCoreComponents(bctx)
	if err == nil {
		t.Fatal("present-invalid config did not fail boot")
	}
	if !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("boot error = %v, want strict config parse failure", err)
	}
}
