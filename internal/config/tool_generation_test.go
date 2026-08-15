package config

import (
	"encoding/json"
	"runtime"
	"testing"
)

// TestDefaultToolGenerationConfig_ShouldTargetTheHost is the regression test
// for a cross-compile that came back after it was fixed.
//
// Ouroboros compiles a generated tool and then EXECUTES the binary itself, so a
// target the host cannot run means every generated tool compiles cleanly and
// dies with "exec format error" on first call. autopoiesis.DefaultConfig was
// changed to default to runtime.GOOS/GOARCH — but it also honours an explicit
// user setting, and this function is where the "explicit" setting came from:
// DefaultUserConfig embeds it, and `nerd auth` on a machine with no config.json
// writes the whole struct to disk. One ordinary onboarding command was enough to
// turn a default into a persisted explicit choice of windows/amd64 and undo the
// fix permanently for that user.
//
// The rule this encodes: a default that can be persisted as an explicit choice
// has to be correct on its own, not merely overridden downstream.
func TestDefaultToolGenerationConfig_ShouldTargetTheHost(t *testing.T) {
	cfg := DefaultToolGenerationConfig()
	if cfg.TargetOS != runtime.GOOS {
		t.Errorf("TargetOS = %q, want the host's %q — a generated tool built for %q "+
			"cannot be executed on this machine", cfg.TargetOS, runtime.GOOS, cfg.TargetOS)
	}
	if cfg.TargetArch != runtime.GOARCH {
		t.Errorf("TargetArch = %q, want the host's %q", cfg.TargetArch, runtime.GOARCH)
	}
	if cfg.AllowToolExec {
		t.Error("AllowToolExec defaults to true; generated tools must not get os/exec " +
			"unless a workspace opts in")
	}
}

// TestDefaultUserConfig_ShouldNotPersistAForeignBuildTarget closes the actual
// path the bug travelled: not the default itself, but the default written to
// disk. This is what `nerd auth login` leaves behind on a fresh machine.
func TestDefaultUserConfig_ShouldNotPersistAForeignBuildTarget(t *testing.T) {
	c := DefaultUserConfig()
	if c.ToolGeneration == nil {
		t.Fatal("DefaultUserConfig has no ToolGeneration section")
	}
	if c.ToolGeneration.TargetOS != runtime.GOOS || c.ToolGeneration.TargetArch != runtime.GOARCH {
		t.Errorf("a config saved by `nerd auth` on this host would record %s/%s; "+
			"autopoiesis.DefaultConfig reads that raw section as an explicit user choice, "+
			"so every generated tool would be cross-compiled and then executed locally",
			c.ToolGeneration.TargetOS, c.ToolGeneration.TargetArch)
	}
}

// TestToolGenerationConfig_AllowToolExec_ShouldRoundTripThroughJSON proves the
// documented per-workspace opt-in is actually reachable.
//
// Docs/architecture/autopoiesis/09-SAFETY-AND-INVARIANTS.md §10 and the package
// README both told operators to grant exec via Config.AllowToolExec.
// autopoiesis read that field, but nothing ever set it and no config key
// existed to set it from — so exec was off no matter what anyone wrote, and the
// documentation described a control that did not exist. Off-by-default is
// correct; unreachable-by-design is a different claim.
func TestToolGenerationConfig_AllowToolExec_ShouldRoundTripThroughJSON(t *testing.T) {
	var c UserConfig
	if err := json.Unmarshal([]byte(`{"tool_generation":{"allow_tool_exec":true}}`), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.ToolGeneration == nil || !c.ToolGeneration.AllowToolExec {
		t.Fatalf(`{"tool_generation":{"allow_tool_exec":true}} did not reach the struct: %+v`,
			c.ToolGeneration)
	}
	if got := c.GetToolGenerationConfig(); !got.AllowToolExec {
		t.Error("GetToolGenerationConfig dropped the grant on its way through the defaults merge")
	}

	// And the absence of the key must not read as a grant.
	var empty UserConfig
	if err := json.Unmarshal([]byte(`{"tool_generation":{}}`), &empty); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if empty.GetToolGenerationConfig().AllowToolExec {
		t.Error("an unset allow_tool_exec granted os/exec to generated tools")
	}
}

// TestToolGenerationConfig_AllowToolExecFalse_ShouldSurviveTheMerge covers the
// direction a zero-value check would break: an operator who writes `false`
// explicitly, over a default that was somehow true, must get false.
func TestToolGenerationConfig_AllowToolExecFalse_ShouldSurviveTheMerge(t *testing.T) {
	c := UserConfig{ToolGeneration: &ToolGenerationConfig{AllowToolExec: false}}
	if c.GetToolGenerationConfig().AllowToolExec {
		t.Error("an explicit false was overridden by the default")
	}
}
