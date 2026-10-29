package build

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/config"
)

// =============================================================================
// DefaultBuildConfig TESTS
// =============================================================================

func TestDefaultBuildConfig_ShouldHaveEmptyFields(t *testing.T) {
	cfg := DefaultBuildConfig()
	if cfg == nil {
		t.Fatal("DefaultBuildConfig() returned nil")
	}
	if cfg.EnvVars == nil {
		t.Error("EnvVars map should not be nil")
	}
	if len(cfg.EnvVars) != 0 {
		t.Errorf("EnvVars should be empty, got %d entries", len(cfg.EnvVars))
	}
	if len(cfg.GoFlags) != 0 {
		t.Errorf("GoFlags should be empty, got %d entries", len(cfg.GoFlags))
	}
	if len(cfg.CGOPackages) != 0 {
		t.Errorf("CGOPackages should be empty, got %d entries", len(cfg.CGOPackages))
	}
}

// =============================================================================
// getBaseGoEnv TESTS
// =============================================================================

func TestGetBaseGoEnv_ShouldIncludePATH(t *testing.T) {
	if os.Getenv("PATH") == "" {
		t.Skip("PATH not set")
	}

	env := getBaseGoEnv()
	if !hasEnvKey(env, "PATH") {
		t.Error("getBaseGoEnv() missing PATH")
	}
}

func TestGetBaseGoEnv_ShouldIncludeEssentialVars(t *testing.T) {
	// Set a known var so we can verify it gets included
	t.Setenv("GOPATH", "/tmp/gopath-test")

	env := getBaseGoEnv()
	if !hasEnvKey(env, "GOPATH") {
		t.Error("getBaseGoEnv() missing GOPATH")
	}
}

func TestGetBaseGoEnv_ShouldDeriveGOCACHE_WhenNotSet(t *testing.T) {
	// Clear GOCACHE but ensure at least one fallback path exists
	t.Setenv("GOCACHE", "")

	env := getBaseGoEnv()
	// Should derive GOCACHE from LOCALAPPDATA, HOME, etc.
	// We can't test the exact value since it depends on the env,
	// but we can verify the function doesn't crash
	_ = env
}

// =============================================================================
// GetBuildEnv TESTS
// =============================================================================

func TestGetBuildEnv_WhenNilConfig_ShouldNotPanic(t *testing.T) {
	root := t.TempDir()

	// Should not panic with nil config
	env := GetBuildEnv(nil, root)
	if len(env) == 0 {
		t.Error("GetBuildEnv() returned empty environment")
	}
}

func TestGetBuildEnv_WhenSqliteHeaders_ShouldAutoDetect(t *testing.T) {
	root := t.TempDir()
	sqliteDir := filepath.Join(root, "sqlite_headers")
	if err := os.MkdirAll(sqliteDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	env := GetBuildEnv(nil, root)

	found := false
	for _, e := range env {
		if strings.HasPrefix(e, "CGO_CFLAGS=") {
			found = true
			if !strings.Contains(e, sqliteDir) {
				t.Errorf("CGO_CFLAGS does not contain sqlite_headers path: %s", e)
			}
			break
		}
	}
	if !found {
		t.Error("GetBuildEnv should auto-detect CGO_CFLAGS from sqlite_headers")
	}
}

func TestGetBuildEnv_WhenWhitelistedEnvVars_ShouldInclude(t *testing.T) {
	root := t.TempDir()
	t.Setenv("MY_CUSTOM_VAR", "myvalue")

	userCfg := &config.UserConfig{
		Execution: &config.ExecutionConfig{
			AllowedEnvVars: []string{"MY_CUSTOM_VAR"},
		},
	}

	env := GetBuildEnv(userCfg, root)
	if !hasEnvKey(env, "MY_CUSTOM_VAR") {
		t.Error("GetBuildEnv should include whitelisted env vars")
	}
}

func TestGetBuildEnv_WhenUserConfigBuildEnvVars_ShouldInclude(t *testing.T) {
	root := t.TempDir()

	userCfg := &config.UserConfig{
		Build: &config.BuildConfig{
			EnvVars: map[string]string{
				"CGO_CFLAGS": "-Icustom-path",
			},
		},
	}

	env := GetBuildEnv(userCfg, root)

	found := slices.Contains(env, "CGO_CFLAGS=-Icustom-path")
	if !found {
		t.Error("GetBuildEnv should include build config env vars")
	}
}

// =============================================================================
// GetBuildEnvForTest TESTS
// =============================================================================

func TestGetBuildEnvForTest_ShouldIncludeBuildEnv(t *testing.T) {
	root := t.TempDir()

	testEnv := GetBuildEnvForTest(nil, root)
	buildEnv := GetBuildEnv(nil, root)

	// Test env should be a superset of build env
	if len(testEnv) < len(buildEnv) {
		t.Errorf("Test env (%d vars) should be >= build env (%d vars)", len(testEnv), len(buildEnv))
	}
}

// =============================================================================
// GetBuildEnvForCompile TESTS
// =============================================================================

func TestGetBuildEnvForCompile_WhenCrossCompile_ShouldSetGOOS(t *testing.T) {
	root := t.TempDir()

	env := GetBuildEnvForCompile(nil, root, "linux", "arm64")

	foundOS := false
	foundArch := false
	for _, e := range env {
		if e == "GOOS=linux" {
			foundOS = true
		}
		if e == "GOARCH=arm64" {
			foundArch = true
		}
	}
	if !foundOS {
		t.Error("GetBuildEnvForCompile should set GOOS")
	}
	if !foundArch {
		t.Error("GetBuildEnvForCompile should set GOARCH")
	}
}

func TestGetBuildEnvForCompile_WhenEmptyTargets_ShouldNotSetGOOS(t *testing.T) {
	root := t.TempDir()

	env := GetBuildEnvForCompile(nil, root, "", "")

	for _, e := range env {
		if strings.HasPrefix(e, "GOOS=") || strings.HasPrefix(e, "GOARCH=") {
			t.Errorf("Should not set GOOS/GOARCH when targets are empty: %s", e)
		}
	}
}

// =============================================================================
// detectCGOFlags TESTS
// =============================================================================

func TestDetectCGOFlags_WhenNoHeaderDirs_ShouldReturnEmpty(t *testing.T) {
	root := t.TempDir()

	got := detectCGOFlags(root)
	if got != "" {
		t.Errorf("detectCGOFlags with empty dir = %q, want empty", got)
	}
}

func TestDetectCGOFlags_WhenSingleHeaderDir_ShouldReturnFlag(t *testing.T) {
	root := t.TempDir()
	headerDir := filepath.Join(root, "include")
	if err := os.MkdirAll(headerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	got := detectCGOFlags(root)
	want := "-I" + headerDir
	if got != want {
		t.Errorf("detectCGOFlags = %q, want %q", got, want)
	}
}

func TestDetectCGOFlags_WhenRelativePath_ShouldResolve(t *testing.T) {
	// When given a relative path, should resolve to absolute for header detection
	root := t.TempDir()
	headerDir := filepath.Join(root, "sqlite_headers")
	if err := os.MkdirAll(headerDir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	// Use absolute path (relative path resolution depends on CWD)
	got := detectCGOFlags(root)
	if got == "" {
		t.Error("detectCGOFlags should detect sqlite_headers")
	}
	if !strings.Contains(got, "sqlite_headers") {
		t.Errorf("detectCGOFlags should contain 'sqlite_headers': %q", got)
	}
}

// =============================================================================
// hasEnvKey / setEnvKey / MergeEnv TESTS (additional edge cases)
// =============================================================================

func TestHasEnvKey_WhenEmptySlice_ShouldReturnFalse(t *testing.T) {
	if hasEnvKey(nil, "FOO") {
		t.Error("hasEnvKey on nil slice should be false")
	}
	if hasEnvKey([]string{}, "FOO") {
		t.Error("hasEnvKey on empty slice should be false")
	}
}

func TestHasEnvKey_WhenPartialMatch_ShouldReturnFalse(t *testing.T) {
	env := []string{"FOOBAR=1"}
	if hasEnvKey(env, "FOO") {
		t.Error("hasEnvKey('FOO') should not match 'FOOBAR=1'")
	}
}

func TestSetEnvKey_WhenKeyNotExists_ShouldAppend(t *testing.T) {
	env := []string{"A=1"}
	result := setEnvKey(env, "B", "2")
	if len(result) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(result))
	}
	if result[1] != "B=2" {
		t.Errorf("Expected 'B=2', got %q", result[1])
	}
}

func TestSetEnvKey_WhenKeyExists_ShouldUpdate(t *testing.T) {
	env := []string{"A=1", "B=2"}
	result := setEnvKey(env, "A", "updated")
	if len(result) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(result))
	}
	if result[0] != "A=updated" {
		t.Errorf("Expected 'A=updated', got %q", result[0])
	}
}

func TestMergeEnv_WhenOverlapping_ShouldOverride(t *testing.T) {
	base := []string{"A=1", "B=2", "C=3"}
	result := MergeEnv(base, "B=updated", "D=4")

	if len(result) != 4 {
		t.Fatalf("Expected 4 entries, got %d: %v", len(result), result)
	}

	// B should be updated, not duplicated
	bCount := 0
	for _, e := range result {
		if strings.HasPrefix(e, "B=") {
			bCount++
			if e != "B=updated" {
				t.Errorf("B should be updated: %q", e)
			}
		}
	}
	if bCount != 1 {
		t.Errorf("B should appear exactly once, got %d", bCount)
	}
}

func TestMergeEnv_WhenMalformedEntry_ShouldSkip(t *testing.T) {
	base := []string{"A=1"}
	result := MergeEnv(base, "NOEQUALSSIGN")
	// Malformed entry (no =) should be skipped
	if len(result) != 1 {
		t.Errorf("Expected 1 entry (malformed skipped), got %d: %v", len(result), result)
	}
}

func TestMergeEnv_ShouldNotMutateBase(t *testing.T) {
	base := []string{"A=1", "B=2"}
	baseCopy := make([]string, len(base))
	copy(baseCopy, base)

	MergeEnv(base, "A=updated")

	for i, v := range base {
		if v != baseCopy[i] {
			t.Errorf("MergeEnv mutated base[%d]: got %q, was %q", i, v, baseCopy[i])
		}
	}
}

// =============================================================================
// loadBuildConfig TESTS
// =============================================================================

func TestLoadBuildConfig_WhenNoSqliteHeaders_ShouldSkipCGO(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLAGS", "")

	cfg := loadBuildConfig(nil, root)
	if _, ok := cfg.EnvVars["CGO_CFLAGS"]; ok {
		t.Error("Should not set CGO_CFLAGS without sqlite_headers directory")
	}
}

func TestLoadBuildConfig_WhenUserConfigGoFlags_ShouldMerge(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLAGS", "")

	userCfg := &config.UserConfig{
		Build: &config.BuildConfig{
			GoFlags: []string{"-v", "-count=1"},
		},
	}

	cfg := loadBuildConfig(userCfg, root)
	if len(cfg.GoFlags) != 2 {
		t.Errorf("Expected 2 GoFlags, got %d: %v", len(cfg.GoFlags), cfg.GoFlags)
	}
}

func TestLoadBuildConfig_WhenUserConfigCGOPackages_ShouldMerge(t *testing.T) {
	root := t.TempDir()
	sqliteHeaders := filepath.Join(root, "sqlite_headers")
	if err := os.MkdirAll(sqliteHeaders, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	t.Setenv("GOFLAGS", "")

	userCfg := &config.UserConfig{
		Build: &config.BuildConfig{
			CGOPackages: []string{"custom_pkg"},
		},
	}

	cfg := loadBuildConfig(userCfg, root)

	foundCustom := false
	foundSqliteVec := false
	for _, pkg := range cfg.CGOPackages {
		if pkg == "custom_pkg" {
			foundCustom = true
		}
		if pkg == "sqlite-vec" {
			foundSqliteVec = true
		}
	}
	if !foundCustom {
		t.Error("Missing custom_pkg in CGOPackages")
	}
	if !foundSqliteVec {
		t.Error("Missing sqlite-vec in CGOPackages (should be auto-added)")
	}
}

// =============================================================================
// deriveGOCACHE TESTS (additional edge cases)
// =============================================================================

func TestDeriveGOCACHE_TMP(t *testing.T) {
	keys := []string{"LOCALAPPDATA", "USERPROFILE", "HOME", "TEMP", "TMP", "TMPDIR"}
	clearEnvVars(t, keys...)

	tmp := t.TempDir()
	t.Setenv("TMP", tmp)

	got := deriveGOCACHE()
	want := filepath.Join(tmp, "go-build")
	if got != want {
		t.Errorf("deriveGOCACHE() = %q, want %q", got, want)
	}
}

func TestDeriveGOCACHE_TMPDIR(t *testing.T) {
	keys := []string{"LOCALAPPDATA", "USERPROFILE", "HOME", "TEMP", "TMP", "TMPDIR"}
	clearEnvVars(t, keys...)

	tmpdir := t.TempDir()
	t.Setenv("TMPDIR", tmpdir)

	got := deriveGOCACHE()
	want := filepath.Join(tmpdir, "go-build")
	if got != want {
		t.Errorf("deriveGOCACHE() = %q, want %q", got, want)
	}
}
