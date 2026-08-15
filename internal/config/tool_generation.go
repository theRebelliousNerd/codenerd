package config

import "runtime"

// ToolGenerationConfig configures the Ouroboros tool generation settings.
type ToolGenerationConfig struct {
	TargetOS   string `yaml:"target_os" json:"target_os"`     // e.g., "windows", "linux", "darwin"
	TargetArch string `yaml:"target_arch" json:"target_arch"` // e.g., "amd64", "arm64"

	// AllowToolExec grants generated tools the os/exec package.
	//
	// Default false, and deliberately so: Ouroboros compiles LLM-authored Go
	// and runs the resulting binary with the user's workspace as its working
	// directory, so granting os/exec turns every generated tool into an
	// unrestricted shell. go_safety.mg has no call-level rule that narrows what
	// such a tool may spawn — the import allowlist is the entire gate.
	//
	// This field is the per-workspace opt-in that
	// Docs/architecture/autopoiesis/09-SAFETY-AND-INVARIANTS.md §10 and the
	// package README have documented for some time. Until now it did not
	// exist: autopoiesis.Config.AllowToolExec was read at
	// autopoiesis_orchestrator.go but nothing ever set it, and there was no
	// config key to set it from, so the documented grant was unreachable and
	// exec was permanently off no matter what an operator wrote in their
	// config. Off-by-default is the right behavior; being unable to turn it on
	// is a different thing, and it made the docs wrong.
	AllowToolExec bool `yaml:"allow_tool_exec" json:"allow_tool_exec,omitempty"`
}

// DefaultToolGenerationConfig returns default tool generation targets.
//
// The host, not windows/amd64.
//
// This defaulted to windows/amd64, which reads like a harmless preference and
// is not one: Ouroboros compiles a tool and then EXECUTES the binary itself
// (RuntimeTool.Execute), so a default the host cannot run means every generated
// tool compiles cleanly and dies with "exec format error" the first time an
// agent calls it.
//
// autopoiesis.DefaultConfig was fixed to default to runtime.GOOS/GOARCH, but
// that fix alone was not enough, because it also honours an explicit user
// setting — and this function is where the "explicit" setting came from.
// DefaultUserConfig embeds DefaultToolGenerationConfig(), and `nerd auth` on a
// machine with no config.json writes that whole struct to disk. From then on
// the user has a config.json that explicitly says windows/amd64, and the
// cross-compile is back for good. A default that becomes a persisted, explicit
// choice has to be correct on its own.
//
// Cross-compiling remains available; it just has to be asked for.
func DefaultToolGenerationConfig() ToolGenerationConfig {
	return ToolGenerationConfig{
		TargetOS:   runtime.GOOS,
		TargetArch: runtime.GOARCH,
	}
}
