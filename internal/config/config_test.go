package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)


// =============================================================================
// USER CONFIG TESTS (Legacy)
// =============================================================================

func TestFindWorkspaceRoot_PrefersNerdDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir .nerd: %v", err)
	}
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	t.Chdir(nested)

	got, err := FindWorkspaceRoot()
	if err != nil {
		t.Fatalf("FindWorkspaceRoot: %v", err)
	}
	if got != root {
		t.Fatalf("FindWorkspaceRoot=%q, want %q", got, root)
	}
}

// TestFindWorkspaceRoot_BypassesStrayNestedNerd is the regression guard for
// the nested-.nerd trap: a stray .nerd/ in a subpackage must NOT capture
// workspace discovery away from the real project root marked by go.mod.
// Without this fix, every run from inside the subpackage would write state
// (config.json, knowledge.db, session.json, sessions/) into the stray dir
// and compound the pollution over time.
func TestFindWorkspaceRoot_BypassesStrayNestedNerd(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir root/.nerd: %v", err)
	}
	subPkg := filepath.Join(root, "cmd", "nerd", "chat")
	if err := os.MkdirAll(filepath.Join(subPkg, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir stray subPkg/.nerd: %v", err)
	}

	t.Chdir(subPkg)

	got, err := FindWorkspaceRoot()
	if err != nil {
		t.Fatalf("FindWorkspaceRoot: %v", err)
	}
	if got != root {
		t.Fatalf("FindWorkspaceRoot=%q, want %q (stray nested .nerd must not trap discovery)", got, root)
	}
}

func TestFindWorkspaceRoot_FallsBackToGoMod(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	nested := filepath.Join(root, "subdir")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	t.Chdir(nested)

	got, err := FindWorkspaceRoot()
	if err != nil {
		t.Fatalf("FindWorkspaceRoot: %v", err)
	}
	if got != root {
		t.Fatalf("FindWorkspaceRoot=%q, want %q", got, root)
	}
}

func TestDefaultUserConfigPath_UsesWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir .nerd: %v", err)
	}
	nested := filepath.Join(root, "x", "y")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	t.Chdir(nested)

	got := DefaultUserConfigPath()
	want := filepath.Join(root, ".nerd", "config.json")
	if got != want {
		t.Fatalf("DefaultUserConfigPath=%q, want %q", got, want)
	}
}

func TestUserConfig_GetActiveProvider_PriorityAndLegacy(t *testing.T) {
	cfg := &UserConfig{
		Provider:        "openai",
		OpenAIAPIKey:    "k-openai",
		AnthropicAPIKey: "k-anthropic",
	}
	provider, key := cfg.GetActiveProvider()
	if provider != "openai" || key != "k-openai" {
		t.Fatalf("GetActiveProvider=%q/%q, want openai/k-openai", provider, key)
	}

	legacy := &UserConfig{APIKey: "k-legacy"}
	provider, key = legacy.GetActiveProvider()
	if provider != "zai" || key != "k-legacy" {
		t.Fatalf("GetActiveProvider legacy=%q/%q, want zai/k-legacy", provider, key)
	}
}

func TestUserConfig_SetEngine_Validates(t *testing.T) {
	cfg := &UserConfig{}
	if err := cfg.SetEngine("not-a-real-engine"); err == nil {
		t.Fatalf("expected invalid engine to error")
	}
	if err := cfg.SetEngine("codex-cli"); err != nil {
		t.Fatalf("SetEngine(codex-cli) error: %v", err)
	}
	if got := cfg.GetEngine(); got != "codex-cli" {
		t.Fatalf("GetEngine=%q, want codex-cli", got)
	}
}

func TestUserConfig_GetCodexCLIConfig_Defaults(t *testing.T) {
	cfg := &UserConfig{}
	codexCfg := cfg.GetCodexCLIConfig()

	if codexCfg.SkillEnabled == nil || !*codexCfg.SkillEnabled {
		t.Fatalf("expected SkillEnabled default true, got %#v", codexCfg.SkillEnabled)
	}
	if codexCfg.SkillName != DefaultCodexExecSkillName {
		t.Fatalf("SkillName=%q, want %q", codexCfg.SkillName, DefaultCodexExecSkillName)
	}
	if codexCfg.MaxConcurrentCalls != DefaultCodexMaxConcurrentCalls {
		t.Fatalf("MaxConcurrentCalls=%d, want %d", codexCfg.MaxConcurrentCalls, DefaultCodexMaxConcurrentCalls)
	}
}

func TestUserConfig_GetEffectiveMaxConcurrentAPICalls(t *testing.T) {
	cfg := &UserConfig{
		Engine: "codex-cli",
		CoreLimits: &CoreLimits{
			MaxConcurrentAPICalls: 5,
		},
		CodexCLI: &CodexCLIConfig{
			MaxConcurrentCalls: 2,
		},
	}

	if got := cfg.GetEffectiveMaxConcurrentAPICalls(); got != 2 {
		t.Fatalf("GetEffectiveMaxConcurrentAPICalls=%d, want 2", got)
	}

	cfg.CodexCLI.MaxConcurrentCalls = 9
	if got := cfg.GetEffectiveMaxConcurrentAPICalls(); got != 5 {
		t.Fatalf("GetEffectiveMaxConcurrentAPICalls=%d, want global ceiling 5", got)
	}

	cfg.Engine = "api"
	if got := cfg.GetEffectiveMaxConcurrentAPICalls(); got != 5 {
		t.Fatalf("GetEffectiveMaxConcurrentAPICalls=%d, want 5 in api mode", got)
	}
}

func TestUserConfig_GetEffectiveAPISchedulerPolicy(t *testing.T) {
	// SuperGrok defaults: spacing + adaptive on
	oauth := &UserConfig{
		Engine: "xai-oauth",
		CoreLimits: &CoreLimits{
			MaxConcurrentAPICalls: 5,
		},
		XAIOAuth: &XAIOAuthConfig{
			MaxConcurrentCalls: 2,
		},
	}
	pol := oauth.GetEffectiveAPISchedulerPolicy()
	if pol.MaxConcurrentAPICalls != 2 {
		t.Fatalf("max=%d want 2", pol.MaxConcurrentAPICalls)
	}
	if pol.MinCallSpacing != 150*time.Millisecond {
		t.Fatalf("spacing=%v want 150ms", pol.MinCallSpacing)
	}
	if !pol.AdaptiveConcurrency {
		t.Fatal("expected adaptive concurrency for xai-oauth")
	}
	if pol.AdaptiveFloor != 1 {
		t.Fatalf("floor=%d want 1", pol.AdaptiveFloor)
	}

	// Explicit config.json overrides win
	off := false
	zero := 0
	floor := 2
	oauth.APIScheduler = &APISchedulerPolicy{
		MinCallSpacingMs:        &zero,
		AdaptiveConcurrency:     &off,
		AdaptiveFloor:           &floor,
		AdaptiveRecoverAfterSec: intPtr(60),
		SlotAcquireTimeoutSec:   intPtr(120),
	}
	pol = oauth.GetEffectiveAPISchedulerPolicy()
	if pol.MinCallSpacing != 0 {
		t.Fatalf("spacing override=%v want 0", pol.MinCallSpacing)
	}
	if pol.AdaptiveConcurrency {
		t.Fatal("adaptive should be off via config")
	}
	if pol.AdaptiveFloor != 2 {
		t.Fatalf("floor=%d want 2", pol.AdaptiveFloor)
	}
	if pol.AdaptiveRecoverAfter != 60*time.Second {
		t.Fatalf("recover=%v want 60s", pol.AdaptiveRecoverAfter)
	}
	if pol.SlotAcquireTimeout != 120*time.Second {
		t.Fatalf("slot timeout=%v want 120s", pol.SlotAcquireTimeout)
	}

	// API engine defaults: no spacing, no adaptive
	api := &UserConfig{Engine: "api", CoreLimits: &CoreLimits{MaxConcurrentAPICalls: 5}}
	pol = api.GetEffectiveAPISchedulerPolicy()
	if pol.MinCallSpacing != 0 || pol.AdaptiveConcurrency {
		t.Fatalf("api engine should be aggressive: spacing=%v adaptive=%v", pol.MinCallSpacing, pol.AdaptiveConcurrency)
	}
}

func intPtr(v int) *int { return &v }

func TestUserConfig_GetContext7APIKey_EnvOverridesConfig(t *testing.T) {
	t.Setenv("CONTEXT7_API_KEY", "env-key")
	cfg := &UserConfig{Context7APIKey: "file-key"}
	if got := cfg.GetContext7APIKey(); got != "env-key" {
		t.Fatalf("GetContext7APIKey=%q, want env-key", got)
	}
}

func TestLoadUserConfig_SaveRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".nerd", "config.json")

	cfg := &UserConfig{
		Provider:       "zai",
		Model:          "glm-4.6",
		ZAIAPIKey:      "k-zai",
		Theme:          "dark",
		Context7APIKey: "ctx7",
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	if loaded.Provider != cfg.Provider || loaded.Model != cfg.Model || loaded.ZAIAPIKey != cfg.ZAIAPIKey || loaded.Theme != cfg.Theme {
		t.Fatalf("round-trip mismatch: got=%+v want=%+v", loaded, cfg)
	}
}
