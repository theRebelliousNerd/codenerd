package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
)

func TestExecutionLayerConfigsPopulatesBaseEnvironment(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, "sqlite_headers"), 0o755); err != nil {
		t.Fatalf("MkdirAll sqlite_headers: %v", err)
	}
	t.Setenv("GOFLAGS", "")

	appCfg := &config.UserConfig{Execution: &config.ExecutionConfig{
		AllowedBinaries: []string{"go"},
		AllowedEnvVars:  []string{"PATH"},
		DefaultTimeout:  "30s",
	}}

	executorCfg, _, err := executionLayerConfigs(appCfg, workspace)
	if err != nil {
		t.Fatal(err)
	}

	var cflags, goflags string
	foundCFlags, foundGoflags := false, false
	for _, e := range executorCfg.BaseEnvironment {
		if v, ok := strings.CutPrefix(e, "CGO_CFLAGS="); ok {
			foundCFlags = true
			cflags = v
			if !strings.HasPrefix(e, "CGO_CFLAGS=-I") || !strings.HasSuffix(v, "sqlite_headers") {
				t.Errorf("CGO_CFLAGS entry = %q, want -I<...>sqlite_headers", e)
			}
		}
		if v, ok := strings.CutPrefix(e, "GOFLAGS="); ok {
			foundGoflags = true
			goflags = v
		}
	}
	if !foundCFlags {
		t.Errorf("BaseEnvironment missing CGO_CFLAGS entry: %v", executorCfg.BaseEnvironment)
	} else if !strings.Contains(cflags, filepath.Join(workspace, "sqlite_headers")) {
		t.Errorf("CGO_CFLAGS = %q, want workspace sqlite_headers %q", cflags, filepath.Join(workspace, "sqlite_headers"))
	}
	if !foundGoflags {
		t.Errorf("BaseEnvironment missing GOFLAGS entry: %v", executorCfg.BaseEnvironment)
	} else if !strings.Contains(goflags, "sqlite_vec") {
		t.Errorf("GOFLAGS = %q, want it to contain sqlite_vec", goflags)
	}
}
