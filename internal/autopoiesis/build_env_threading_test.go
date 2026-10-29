package autopoiesis

import (
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/build"
	"codenerd/internal/config"
)

// Both compile sites (tool_compiler.go and thunderdome.go) used to pass nil
// for *config.UserConfig, so generated tools and Thunderdome arenas were built
// with a different toolchain environment than internal/session verification —
// no operator CGO flags, no allowlisted env vars, no configured GOFLAGS. A
// tool that needed any of those failed to compile for a reason that had
// nothing to do with the code the model wrote.

func userConfigWithBuildEnv(key, value string) *config.UserConfig {
	return &config.UserConfig{
		Build: &config.BuildConfig{
			EnvVars: map[string]string{key: value},
		},
	}
}

func TestToolCompiler_WhenUserConfigProvided_ShouldReachTheCompileEnvironment(t *testing.T) {
	userCfg := userConfigWithBuildEnv("OUROBOROS_ENV_PROBE", "compiler")

	cfg := DefaultOuroborosConfig(t.TempDir())
	cfg.UserConfig = userCfg
	tc := NewToolCompiler(cfg)

	env := build.GetBuildEnvForCompile(tc.config.UserConfig, tc.buildEnvRoot(t.TempDir()), cfg.TargetOS, cfg.TargetArch)
	if !containsEnv(env, "OUROBOROS_ENV_PROBE=compiler") {
		t.Errorf("compile environment does not carry the operator's build config: %v", summarize(env))
	}
}

// The detection root must be the workspace, not the throwaway temp module: a
// generated tool that imports codenerd through the `replace` directive needs
// the repo's headers and build tags, and none of those live in the temp dir.
func TestToolCompiler_WhenWorkspaceRootSet_ShouldDetectBuildEnvFromWorkspace(t *testing.T) {
	workspace := t.TempDir()
	cfg := DefaultOuroborosConfig(workspace)
	cfg.WorkspaceRoot = workspace
	tc := NewToolCompiler(cfg)

	tmp := filepath.Join(t.TempDir(), "ouroboros-build-xyz")
	if got := tc.buildEnvRoot(tmp); got != workspace {
		t.Errorf("buildEnvRoot = %q, want the workspace root %q", got, workspace)
	}

	cfg.WorkspaceRoot = ""
	tc = NewToolCompiler(cfg)
	if got := tc.buildEnvRoot(tmp); got != tmp {
		t.Errorf("with no workspace root, buildEnvRoot = %q, want the module dir %q", got, tmp)
	}
}

func TestNewOuroborosLoop_WhenUserConfigProvided_ShouldReachTheThunderdomeArena(t *testing.T) {
	userCfg := userConfigWithBuildEnv("OUROBOROS_ENV_PROBE", "arena")

	cfg := DefaultOuroborosConfig(t.TempDir())
	cfg.UserConfig = userCfg
	cfg.ThunderdomeConfig.WorkDir = t.TempDir()

	loop := NewOuroborosLoop(&MockLLMClient{}, cfg)
	if loop.thunderdome == nil {
		t.Fatal("Thunderdome is enabled by default and should have been constructed")
	}
	if loop.thunderdome.config.UserConfig != userCfg {
		t.Fatal("the arena did not inherit the operator's UserConfig; it would compile with a different environment than the tool itself")
	}

	env := build.GetBuildEnv(loop.thunderdome.config.UserConfig, t.TempDir())
	if !containsEnv(env, "OUROBOROS_ENV_PROBE=arena") {
		t.Errorf("arena environment does not carry the operator's build config: %v", summarize(env))
	}
}

// An explicitly configured arena environment must not be clobbered by the
// outer config.
func TestNewOuroborosLoop_WhenArenaConfigHasOwnUserConfig_ShouldKeepIt(t *testing.T) {
	outer := userConfigWithBuildEnv("PROBE", "outer")
	inner := userConfigWithBuildEnv("PROBE", "inner")

	cfg := DefaultOuroborosConfig(t.TempDir())
	cfg.UserConfig = outer
	cfg.ThunderdomeConfig.WorkDir = t.TempDir()
	cfg.ThunderdomeConfig.UserConfig = inner

	loop := NewOuroborosLoop(&MockLLMClient{}, cfg)
	if loop.thunderdome.config.UserConfig != inner {
		t.Error("an explicitly configured arena UserConfig was overwritten by the outer config")
	}
}

func containsEnv(env []string, kv string) bool {
	for _, e := range env {
		if e == kv {
			return true
		}
	}
	return false
}

func summarize(env []string) string {
	keys := make([]string, 0, len(env))
	for _, e := range env {
		if i := strings.IndexByte(e, '='); i > 0 {
			keys = append(keys, e[:i])
		}
	}
	return strings.Join(keys, ",")
}
