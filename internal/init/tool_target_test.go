package init

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"codenerd/internal/config"
)

// TestCreateDefaultConfig_ShouldTargetTheHost is the regression test for a bug
// that was "fixed" twice and stayed live both times.
//
// Ouroboros compiles a generated tool and then EXECUTES the binary itself, so a
// build target the host cannot run means every generated tool compiles cleanly
// and dies with "exec format error" the first time an agent calls it.
//
// The first fix set autopoiesis.DefaultConfig's default to runtime.GOOS. That
// function also honours an explicit user setting, so the second fix corrected
// config.DefaultToolGenerationConfig, on the theory that `nerd auth` persisted
// it. Adversarial review disproved that mechanism — LoadUserConfig returns an
// empty config for a MISSING file, so `nerd auth` never reaches the defaults
// and writes no target at all — and then found the path that really does it:
// createDefaultConfig here, which hardcoded windows/amd64 into every fresh
// workspace's .nerd/config.json. Measured on the branch AFTER both earlier
// fixes, `nerd init` still produced {windows amd64} and autopoiesis then read
// it back as a deliberate user choice.
//
// Three call sites, one defect. That is the recurring shape, so this test does
// not just check the one that was found: internal/repoguard scans the whole
// tree for a third (TestNoHardcodedToolTarget_AnywhereInTheRepo).
func TestCreateDefaultConfig_ShouldTargetTheHost(t *testing.T) {
	ws := t.TempDir()
	path := filepath.Join(ws, "config.json")
	ini := &Initializer{config: InitConfig{Workspace: ws}}
	if err := ini.createDefaultConfig(path); err != nil {
		t.Fatalf("createDefaultConfig: %v", err)
	}

	// Read it back off disk, because the file is what autopoiesis later treats
	// as the operator's explicit choice.
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config.json: %v", err)
	}
	var cfg config.UserConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse config.json: %v", err)
	}
	if cfg.ToolGeneration == nil {
		t.Fatal("a freshly initialised workspace records no ToolGeneration section")
	}
	if got := cfg.ToolGeneration.TargetOS; got != runtime.GOOS {
		t.Errorf("TargetOS = %q, want the host's %q. `nerd init` writes this to "+
			"config.json, and autopoiesis.DefaultConfig reads the raw section as an "+
			"explicit user choice — so a foreign target here is indistinguishable "+
			"from one the operator asked for, and every generated tool is built for "+
			"a machine that will not run it.", got, runtime.GOOS)
	}
	if got := cfg.ToolGeneration.TargetArch; got != runtime.GOARCH {
		t.Errorf("TargetArch = %q, want the host's %q", got, runtime.GOARCH)
	}
	if cfg.ToolGeneration.AllowToolExec {
		t.Error("AllowToolExec defaults to true in a freshly initialised workspace; " +
			"generated tools must not get os/exec unless the operator opts in")
	}
}
