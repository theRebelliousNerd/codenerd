package build

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
)

// =============================================================================
// Key normalization (no duplicate keys anywhere in the merge pipeline)
// =============================================================================

func envKeyCounts(env []string) map[string]int {
	counts := map[string]int{}
	for _, e := range env {
		if key, _, ok := strings.Cut(e, "="); ok {
			counts[key]++
		}
	}
	return counts
}

func TestGetBuildEnv_WhenWhitelistRepeatsEssentialVar_ShouldNotDuplicateKey(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOPATH", filepath.Join(root, "gopath"))
	t.Setenv("GOFLAGS", "-mod=mod")

	userCfg := &config.UserConfig{
		Execution: &config.ExecutionConfig{
			// PATH/GOPATH/GOFLAGS are already emitted by getBaseGoEnv; a whitelist
			// that names them again used to append a second entry for each.
			AllowedEnvVars: []string{"PATH", "GOPATH", "GOFLAGS"},
		},
	}

	env := GetBuildEnv(userCfg, root)
	for key, n := range envKeyCounts(env) {
		if n != 1 {
			t.Errorf("env key %q appears %d times, want 1: %v", key, n, env)
		}
	}
}

func TestGetBuildEnv_WhenConfigOverridesEssentialVar_ShouldNotDuplicateKey(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLAGS", "-mod=mod")

	userCfg := &config.UserConfig{
		Build: &config.BuildConfig{
			EnvVars: map[string]string{
				"GOFLAGS":    "-tags=custom",
				"CGO_CFLAGS": "-Icustom",
			},
		},
	}

	env := GetBuildEnv(userCfg, root)
	counts := envKeyCounts(env)
	if counts["GOFLAGS"] != 1 {
		t.Errorf("GOFLAGS appears %d times, want 1: %v", counts["GOFLAGS"], env)
	}
	if got := envValue(env, "GOFLAGS"); got != "-tags=custom" {
		t.Errorf("config GOFLAGS should win over ambient: got %q", got)
	}
}

func TestGetBuildEnv_WhenCalledTwice_ShouldBeDeterministic(t *testing.T) {
	root := t.TempDir()
	userCfg := &config.UserConfig{
		Build: &config.BuildConfig{
			EnvVars: map[string]string{"CC": "clang", "CXX": "clang++", "CGO_LDFLAGS": "-lm", "CGO_CFLAGS": "-Ix"},
		},
	}

	first := strings.Join(GetBuildEnv(userCfg, root), "\x00")
	for i := range 8 {
		if got := strings.Join(GetBuildEnv(userCfg, root), "\x00"); got != first {
			t.Fatalf("GetBuildEnv ordering not deterministic on iteration %d:\n%q\n%q", i, first, got)
		}
	}
}

// =============================================================================
// Detection root vs module dir
// =============================================================================

func TestDetectionRootFor_WhenNestedModule_ShouldFindMonorepoHeaders(t *testing.T) {
	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".git"))
	mustMkdir(t, filepath.Join(repo, "sqlite_headers"))
	module := filepath.Join(repo, "services", "indexer")
	mustMkdir(t, module)

	if got := DetectionRootFor(module); got != repo {
		t.Errorf("DetectionRootFor(%q) = %q, want %q", module, got, repo)
	}

	// And the resulting env must carry the repo-root headers, which is the
	// whole point: cmd.Dir stays the submodule, CGO_CFLAGS comes from the repo.
	env := GetBuildEnvForModule(nil, module)
	if got := envValue(env, "CGO_CFLAGS"); !strings.Contains(got, filepath.Join(repo, "sqlite_headers")) {
		t.Errorf("CGO_CFLAGS = %q, want the repo-root sqlite_headers", got)
	}
	// GetBuildEnv on the module dir alone finds nothing — the bug this replaces.
	if hasEnvKey(GetBuildEnv(nil, module), "CGO_CFLAGS") {
		t.Error("module dir alone should not yield CGO_CFLAGS; test premise is wrong")
	}
}

func TestDetectionRootFor_WhenNoHeaders_ShouldStopAtRepoBoundary(t *testing.T) {
	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".git"))
	module := filepath.Join(repo, "a", "b")
	mustMkdir(t, module)

	if got := DetectionRootFor(module); got != repo {
		t.Errorf("DetectionRootFor(%q) = %q, want the repo boundary %q", module, got, repo)
	}
}

func TestDetectionRootFor_WhenNoRepoMarkers_ShouldReturnModuleDir(t *testing.T) {
	// No .git, no go.work, no sqlite_headers anywhere up the chain: the walk
	// must not adopt /tmp or / as a detection root.
	module := filepath.Join(t.TempDir(), "loose")
	mustMkdir(t, module)

	if got := DetectionRootFor(module); got != module {
		t.Errorf("DetectionRootFor(%q) = %q, want the module dir itself", module, got)
	}
}

func mustMkdir(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q): %v", dir, err)
	}
}

// =============================================================================
// GetBuildEnvForTest specialization
// =============================================================================

func TestGetBuildEnvForTest_WhenDefault_ShouldSetGotracebackAndCountOne(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOTRACEBACK", "")

	env := GetBuildEnvForTest(nil, root)

	if got := envValue(env, "GOTRACEBACK"); got != "all" {
		t.Errorf("GOTRACEBACK = %q, want \"all\"", got)
	}
	if got := envValue(env, "GOFLAGS"); !strings.Contains(got, "-count=1") {
		t.Errorf("GOFLAGS = %q, want it to contain -count=1", got)
	}
}

func TestGetBuildEnvForTest_WhenSqliteTagsPresent_ShouldPreserveExistingGoflags(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sqlite_headers"), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	t.Setenv("GOFLAGS", "")

	env := GetBuildEnvForTest(nil, root)

	got := envValue(env, "GOFLAGS")
	if !strings.Contains(got, "-tags=sqlite_vec") {
		t.Errorf("GOFLAGS = %q, want it to keep -tags=sqlite_vec", got)
	}
	if !strings.Contains(got, "-count=1") {
		t.Errorf("GOFLAGS = %q, want it to add -count=1", got)
	}
	if n := envKeyCounts(env)["GOFLAGS"]; n != 1 {
		t.Errorf("GOFLAGS appears %d times, want 1", n)
	}
}

func TestGetBuildEnvForTest_WhenCallerPinnedCount_ShouldNotOverride(t *testing.T) {
	root := t.TempDir()
	t.Setenv("GOFLAGS", "")

	userCfg := &config.UserConfig{
		Build: &config.BuildConfig{EnvVars: map[string]string{"GOFLAGS": "-count=5"}},
	}

	env := GetBuildEnvForTest(userCfg, root)
	if got := envValue(env, "GOFLAGS"); got != "-count=5" {
		t.Errorf("GOFLAGS = %q, want the caller's -count=5 untouched", got)
	}
}

func TestGetBuildEnvForTest_WhenCISet_ShouldPropagate(t *testing.T) {
	root := t.TempDir()
	t.Setenv("CI", "true")
	t.Setenv("GORACE", "halt_on_error=1")

	env := GetBuildEnvForTest(nil, root)

	if got := envValue(env, "CI"); got != "true" {
		t.Errorf("CI = %q, want \"true\"", got)
	}
	if got := envValue(env, "GORACE"); got != "halt_on_error=1" {
		t.Errorf("GORACE = %q, want propagated value", got)
	}
	// GetBuildEnv must not leak these into plain build environments.
	if hasEnvKey(GetBuildEnv(nil, root), "GORACE") {
		t.Error("GetBuildEnv should not carry GORACE; it is test-only specialization")
	}
}

func TestGetBuildEnvForTest_ShouldBeSupersetOfBuildEnvKeys(t *testing.T) {
	root := t.TempDir()

	buildEnv := GetBuildEnv(nil, root)
	testEnv := GetBuildEnvForTest(nil, root)

	for key := range envKeyCounts(buildEnv) {
		if !hasEnvKey(testEnv, key) {
			t.Errorf("test env dropped build env key %q", key)
		}
	}
}

// =============================================================================
// AppendGoFlags
// =============================================================================

func TestAppendGoFlags_WhenConfigured_ShouldInsertAfterSubcommand(t *testing.T) {
	root := t.TempDir()
	userCfg := &config.UserConfig{
		Build: &config.BuildConfig{GoFlags: []string{"-mod=mod", "-v"}},
	}

	got := AppendGoFlags(userCfg, root, []string{"test", "-count=1", "./..."})
	want := []string{"test", "-mod=mod", "-v", "-count=1", "./..."}

	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("AppendGoFlags = %v, want %v", got, want)
	}
}

func TestAppendGoFlags_WhenFlagAlreadyInArgv_ShouldNotDuplicate(t *testing.T) {
	root := t.TempDir()
	userCfg := &config.UserConfig{
		Build: &config.BuildConfig{GoFlags: []string{"-count=5", "-v"}},
	}

	got := AppendGoFlags(userCfg, root, []string{"test", "-count=1", "./..."})
	want := []string{"test", "-v", "-count=1", "./..."}

	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("AppendGoFlags = %v, want %v (explicit argv wins)", got, want)
	}
}

func TestAppendGoFlags_WhenSubcommandTakesNoBuildFlags_ShouldReturnArgsUnchanged(t *testing.T) {
	root := t.TempDir()
	userCfg := &config.UserConfig{
		Build: &config.BuildConfig{GoFlags: []string{"-v"}},
	}

	// `go mod tidy` reads argv[1] as the verb; injecting there breaks the command.
	got := AppendGoFlags(userCfg, root, []string{"mod", "tidy"})
	if strings.Join(got, " ") != "mod tidy" {
		t.Errorf("AppendGoFlags = %v, want [mod tidy] untouched", got)
	}
}

func TestAppendGoFlags_WhenNoConfig_ShouldReturnArgsUnchanged(t *testing.T) {
	root := t.TempDir()

	got := AppendGoFlags(nil, root, []string{"build", "./..."})
	if strings.Join(got, " ") != "build ./..." {
		t.Errorf("AppendGoFlags = %v, want unchanged", got)
	}
	if got := AppendGoFlags(nil, root, nil); got != nil {
		t.Errorf("AppendGoFlags(nil args) = %v, want nil", got)
	}
}

// =============================================================================
// GOCACHE derivation warning
// =============================================================================

func TestGetBaseGoEnv_WhenGOCACHEUnderivable_ShouldWarn(t *testing.T) {
	for _, key := range []string{"GOCACHE", "LOCALAPPDATA", "USERPROFILE", "HOME", "TEMP", "TMP", "TMPDIR"} {
		t.Setenv(key, "")
	}

	var warnings []string
	orig := buildWarn
	buildWarn = func(format string, args ...any) { warnings = append(warnings, format) }
	t.Cleanup(func() { buildWarn = orig })

	env := getBaseGoEnv()

	if hasEnvKey(env, "GOCACHE") {
		t.Fatalf("GOCACHE should be absent when nothing can be derived: %v", env)
	}
	if len(warnings) != 1 {
		t.Fatalf("expected exactly 1 warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "GOCACHE") {
		t.Errorf("warning does not mention GOCACHE: %q", warnings[0])
	}
}

func TestGetBaseGoEnv_WhenGOCACHEDerivable_ShouldNotWarn(t *testing.T) {
	for _, key := range []string{"GOCACHE", "LOCALAPPDATA", "USERPROFILE", "TEMP", "TMP", "TMPDIR"} {
		t.Setenv(key, "")
	}
	t.Setenv("HOME", t.TempDir())

	var warnings []string
	orig := buildWarn
	buildWarn = func(format string, args ...any) { warnings = append(warnings, format) }
	t.Cleanup(func() { buildWarn = orig })

	if env := getBaseGoEnv(); !hasEnvKey(env, "GOCACHE") {
		t.Fatalf("GOCACHE should be derived from HOME: %v", env)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
}

// =============================================================================
// SummarizeEnv / redaction
// =============================================================================

func TestSummarizeEnv_ShouldReturnSortedKeysOnly(t *testing.T) {
	got := SummarizeEnv([]string{"ZED=secret", "ALPHA=1", "ALPHA=2", "malformed", "BETA="})
	want := "ALPHA,BETA,ZED"
	if got != want {
		t.Errorf("SummarizeEnv = %q, want %q", got, want)
	}
	if strings.Contains(got, "secret") {
		t.Errorf("SummarizeEnv leaked a value: %q", got)
	}
}

func TestRedactEnvValue_WhenSecretProneKey_ShouldRedact(t *testing.T) {
	cases := []struct {
		key, val, want string
	}{
		{"CGO_CFLAGS", "-I/x", "-I/x"},
		{"GOFLAGS", "-tags=sqlite_vec", "-tags=sqlite_vec"},
		{"ANTHROPIC_API_KEY", "sk-ant-123", "<redacted>"},
		{"GITHUB_TOKEN", "ghp_123", "<redacted>"},
		{"MY_PASSWORD", "hunter2", "<redacted>"},
		{"NPM_AUTH", "abc", "<redacted>"},
		// Not on the allowlist: unknown keys are redacted by default rather than
		// logged, because build.env_vars is a free-form operator-supplied map.
		{"SOME_RANDOM_VAR", "value", "<redacted>"},
		{"SOME_RANDOM_VAR", "", "<empty>"},
	}
	for _, c := range cases {
		if got := redactEnvValue(c.key, c.val); got != c.want {
			t.Errorf("redactEnvValue(%q, %q) = %q, want %q", c.key, c.val, got, c.want)
		}
	}
}

func TestGetBuildEnv_WhenSecretInConfigEnvVars_ShouldStillBePassedToSubprocess(t *testing.T) {
	// Redaction is a logging concern only: the value must still reach the
	// toolchain, otherwise a private GOPROXY token would break builds.
	root := t.TempDir()
	userCfg := &config.UserConfig{
		Build: &config.BuildConfig{EnvVars: map[string]string{"NETRC_TOKEN": "s3cret"}},
	}

	if got := envValue(GetBuildEnv(userCfg, root), "NETRC_TOKEN"); got != "s3cret" {
		t.Errorf("NETRC_TOKEN = %q, want the real value in the subprocess env", got)
	}
}

// =============================================================================
// Integration: env against the real workspace, verified by the go toolchain
// =============================================================================

func TestGetBuildEnv_WhenRealWorkspace_ShouldSurviveGoEnv(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test spawns the go toolchain")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not on PATH: %v", err)
	}

	root := repoRoot(t)
	headers := filepath.Join(root, "sqlite_headers")
	if _, err := os.Stat(headers); err != nil {
		t.Skipf("workspace has no sqlite_headers: %v", err)
	}
	t.Setenv("GOFLAGS", "")

	env := GetBuildEnv(nil, root)

	cmd := exec.Command(goBin, "env", "CGO_CFLAGS", "GOFLAGS", "GOCACHE")
	cmd.Dir = root
	cmd.Env = env
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go env failed with the constructed environment: %v\nenv keys: %s\noutput: %s",
			err, SummarizeEnv(env), out)
	}

	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(string(out)), "\r\n", "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("go env returned %d lines, want 3: %q", len(lines), out)
	}
	gotCFlags, gotGoflags, gotCache := lines[0], lines[1], lines[2]

	if !strings.Contains(gotCFlags, headers) {
		t.Errorf("go env CGO_CFLAGS = %q, want it to contain %q", gotCFlags, headers)
	}
	if !strings.Contains(gotGoflags, "sqlite_vec") {
		t.Errorf("go env GOFLAGS = %q, want the sqlite_vec build tag", gotGoflags)
	}
	if strings.TrimSpace(gotCache) == "" || strings.TrimSpace(gotCache) == "off" {
		t.Errorf("go env GOCACHE = %q, want a usable cache directory", gotCache)
	}
}

func TestGetBuildEnvForTest_WhenRealWorkspace_ShouldCompileAndRunATest(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test spawns the go toolchain")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skipf("go toolchain not on PATH: %v", err)
	}

	// A throwaway module, so the test proves the env works end to end without
	// depending on the monorepo compiling cleanly while other work is in flight.
	mod := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(mod, name), []byte(body), 0o644); err != nil {
			t.Fatalf("WriteFile(%s): %v", name, err)
		}
	}
	write("go.mod", "module buildenvprobe\n\ngo 1.22\n")
	write("probe_test.go", "package probe\n\nimport (\n\t\"os\"\n\t\"testing\"\n)\n\nfunc TestProbe(t *testing.T) {\n\tif os.Getenv(\"GOTRACEBACK\") != \"all\" {\n\t\tt.Fatalf(\"GOTRACEBACK=%q\", os.Getenv(\"GOTRACEBACK\"))\n\t}\n}\n")

	args := AppendGoFlags(nil, mod, []string{"test", "./..."})
	cmd := exec.Command(goBin, args...)
	cmd.Dir = mod
	cmd.Env = GetBuildEnvForTest(nil, mod)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go %s failed: %v\nenv keys: %s\noutput: %s",
			strings.Join(args, " "), err, SummarizeEnv(cmd.Env), out)
	}
}
