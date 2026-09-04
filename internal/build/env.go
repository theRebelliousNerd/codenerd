// Package build provides unified build environment configuration.
// This addresses the wiring issue where components spawned `go build` / `go test`
// through raw exec.Command without the environment the monorepo needs — most
// visibly CGO_CFLAGS pointing at sqlite_headers, and GOCACHE, whose absence makes
// the toolchain refuse to build at all.
//
// All components that run go build/test should use GetBuildEnv() to ensure consistent
// environment configuration across the codebase.
//
// Real importers as of this revision (kept honest by
// TestGoInvocations_WhenSpawningGo_ShouldUseBuildEnvOrBeExempt in this package,
// which fails when a new unmarked `go` invocation appears anywhere in the repo):
//
//	internal/autopoiesis — tool_compiler.go, thunderdome.go
//	internal/session     — build_verify.go, test_verify.go, coverage_profile.go, lsp_diagnostics.go
//	internal/core        — virtual_store_actions.go
//	internal/system      — factory_execution.go (tactile ExecutorConfig.BaseEnvironment)
//
// The historical list in this comment named preflight, attack_runner and tester,
// none of which ever imported the package; that fiction is what the inventory
// test now prevents.
package build

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/logging"
)

// BuildConfig holds project-specific build configuration, loaded from
// .nerd/config.json under the "build" key.
//
// This is an alias, not a second struct. The package used to declare its own
// copy of the same three fields; the two drifted (config carried yaml tags, this
// one did not) and every change had to be made twice. The persisted shape in
// internal/config is the single definition, and this package reads it.
type BuildConfig = config.BuildConfig

// DefaultBuildConfig returns sensible defaults with non-nil maps and slices.
func DefaultBuildConfig() *BuildConfig {
	cfg := config.DefaultBuildConfig()
	return &cfg
}

// goFlagSubcommands are the `go` subcommands that accept build flags, and so
// the only ones AppendGoFlags will inject into. `go mod tidy` and friends take a
// sub-subcommand in argv[1], where an injected flag would be read as the verb.
var goFlagSubcommands = map[string]bool{
	"build":    true,
	"generate": true,
	"install":  true,
	"list":     true,
	"run":      true,
	"test":     true,
	"vet":      true,
}

// nonSecretEnvKeys are keys whose values are safe to write to the debug log.
// Everything else — and config env_vars is a free-form map, so it routinely
// carries API keys, registry tokens and proxy credentials — is redacted.
var nonSecretEnvKeys = map[string]bool{
	"CC":            true,
	"CGO_CFLAGS":    true,
	"CGO_CXXFLAGS":  true,
	"CGO_ENABLED":   true,
	"CGO_LDFLAGS":   true,
	"CI":            true,
	"CXX":           true,
	"GOARCH":        true,
	"GOCACHE":       true,
	"GOFLAGS":       true,
	"GOMAXPROCS":    true,
	"GOMODCACHE":    true,
	"GOOS":          true,
	"GOPATH":        true,
	"GORACE":        true,
	"GOROOT":        true,
	"GOTMPDIR":      true,
	"GOTRACEBACK":   true,
	"HOME":          true,
	"LOCALAPPDATA":  true,
	"PATH":          true,
	"TEMP":          true,
	"TMP":           true,
	"TMPDIR":        true,
	"USERPROFILE":   true,
	"GO111MODULE":   true,
	"GOTOOLCHAIN":   true,
	"GOWORK":        true,
	"GOEXPERIMENT":  true,
	"GODEBUG":       true,
	"GOPRIVATE":     true,
	"GONOSUMDB":     true,
	"GOINSECURE":    true,
	"GOTESTTIMEOUT": true,
}

// secretishFragments mark a key as sensitive even if it is otherwise allowlisted,
// so a key like GOFLAGS_TOKEN cannot slip a credential into the log by prefix.
var secretishFragments = []string{"TOKEN", "SECRET", "PASSWORD", "PASSWD", "CREDENTIAL", "APIKEY", "API_KEY", "_KEY", "AUTH", "SESSION", "COOKIE", "PRIVATE_KEY"}

// buildWarn is a seam over logging.BuildWarn. The logging package writes to
// files behind a category filter, so the only way to prove the GOCACHE warning
// actually fires is to substitute it in a test.
var buildWarn = logging.BuildWarn

// GetBuildEnv returns the proper environment for go build/test commands.
// It merges:
// 1. Current process environment (filtered)
// 2. Whitelisted env vars from config
// 3. Project-specific build config (CGO_CFLAGS, etc.)
//
// This is the single source of truth for build environment.
// All components should use this instead of raw os.Environ().
//
// workspaceRoot is the *detection* root: the directory searched for
// sqlite_headers and other include dirs. It is deliberately separate from the
// command's working directory, so a monorepo caller can compile inside a
// submodule (cmd.Dir) while still picking up the repo-root headers.
func GetBuildEnv(userCfg *config.UserConfig, workspaceRoot string) []string {
	logging.BuildDebug("Building environment for workspace: %s", workspaceRoot)

	// Start with essential Go environment
	env := getBaseGoEnv()

	// Add whitelisted vars from execution config.
	// setEnvKey rather than append: a whitelist that repeats an essential var
	// (PATH, GOPATH, ...) used to emit the key twice, and while exec resolves
	// duplicates last-wins, the duplicates confused every env diff we logged.
	if userCfg != nil {
		execCfg := userCfg.GetExecution()
		for _, key := range execCfg.AllowedEnvVars {
			if val := os.Getenv(key); val != "" {
				env = setEnvKey(env, key, val)
				logging.BuildDebug("Added whitelisted env: %s", key)
			}
		}
	}

	// Add project-specific build config. Sorted so the resulting slice is
	// deterministic; map order made env slices differ run to run.
	buildCfg := loadBuildConfig(userCfg, workspaceRoot)
	for _, key := range slices.Sorted(maps.Keys(buildCfg.EnvVars)) {
		val := buildCfg.EnvVars[key]
		env = setEnvKey(env, key, val)
		logging.BuildDebug("Added build config env: %s=%s", key, redactEnvValue(key, val))
	}

	// Auto-detect CGO requirements if not explicitly set
	if !hasEnvKey(env, "CGO_CFLAGS") {
		if cgoFlags := detectCGOFlags(workspaceRoot); cgoFlags != "" {
			env = setEnvKey(env, "CGO_CFLAGS", cgoFlags)
			logging.BuildDebug("Auto-detected CGO_CFLAGS: %s", cgoFlags)
		}
	}

	logging.BuildDebug("Final build environment (%d vars): %s", len(env), SummarizeEnv(env))
	return env
}

// GetBuildEnvForModule builds the environment for a command whose working
// directory is moduleDir, resolving the header-detection root separately.
//
// The two are not the same thing and conflating them is the monorepo CGO bug:
// a caller sets cmd.Dir to a nested module and passes that same path as the
// detection root, so the repo-root sqlite_headers is never found and the
// compile dies on a missing sqlite3.h. Use this when cmd.Dir is a submodule;
// use GetBuildEnv directly when you already hold the workspace root.
func GetBuildEnvForModule(userCfg *config.UserConfig, moduleDir string) []string {
	return GetBuildEnv(userCfg, DetectionRootFor(moduleDir))
}

// DetectionRootFor walks up from moduleDir to the directory whose headers a
// build started there should see.
//
// The walk stops at the first sqlite_headers it finds, or at the repository
// boundary (.git / go.work), whichever comes first — it never escapes above the
// repo into /usr or /, so a stray system include directory cannot be adopted as
// a project's detection root. Only sqlite_headers is used as the walk-up marker;
// the broader list in detectCGOFlags stays scoped to the resolved root.
func DetectionRootFor(moduleDir string) string {
	abs := moduleDir
	if !filepath.IsAbs(abs) {
		if resolved, err := filepath.Abs(abs); err == nil {
			abs = resolved
		}
	}

	dir := abs
	for {
		if info, err := os.Stat(filepath.Join(dir, "sqlite_headers")); err == nil && info.IsDir() {
			return dir
		}
		if isRepoBoundary(dir) {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return abs
		}
		dir = parent
	}
}

// isRepoBoundary reports whether dir is the outermost directory a detection-root
// walk may consider.
func isRepoBoundary(dir string) bool {
	for _, marker := range []string{".git", "go.work"} {
		if _, err := os.Stat(filepath.Join(dir, marker)); err == nil {
			return true
		}
	}
	return false
}

// GetBuildEnvForTest returns the environment for `go test` subprocesses.
//
// A test binary is project code, not just the toolchain, so it needs more than
// GetBuildEnv gives a compiler. The specialization is deliberately small and
// each part earns its place:
//
//  1. GOTRACEBACK=all — a panic in a background goroutine otherwise prints only
//     the panicking goroutine, and the verification parsers cannot attribute
//     the failure to a package.
//  2. -count=1 folded into GOFLAGS — the toolchain will happily replay a cached
//     PASS for a package whose source the agent just rewrote, which reports
//     green for code that was never run. Unknown flags in GOFLAGS are ignored
//     by subcommands that do not define them, so this stays safe if a caller
//     reuses the slice for a non-test command.
//  3. Test-relevant ambient vars that the build filter drops. Suites branch on
//     CI, GORACE configures the race detector, GOMAXPROCS bounds parallelism.
//
// Callers that already pin one of these keep their value: nothing here
// overwrites an explicit setting from config or the whitelist.
func GetBuildEnvForTest(userCfg *config.UserConfig, workspaceRoot string) []string {
	env := GetBuildEnv(userCfg, workspaceRoot)

	if !hasEnvKey(env, "GOTRACEBACK") {
		env = setEnvKey(env, "GOTRACEBACK", "all")
	}

	// Propagate test-relevant ambient vars the base filter intentionally omits.
	for _, key := range []string{"CI", "GORACE", "GOMAXPROCS", "GOTMPDIR"} {
		if hasEnvKey(env, key) {
			continue
		}
		if val := os.Getenv(key); val != "" {
			env = setEnvKey(env, key, val)
		}
	}

	env = setEnvKey(env, "GOFLAGS", withCountOne(envValue(env, "GOFLAGS")))

	return env
}

// withCountOne appends -count=1 to a GOFLAGS value unless the caller already
// chose a -count, so an explicit -count=5 benchmark run is not silently reset.
func withCountOne(goflags string) string {
	if strings.Contains(goflags, "-count=") {
		return goflags
	}
	if goflags == "" {
		return "-count=1"
	}
	return goflags + " -count=1"
}

// GetBuildEnvForCompile returns environment for compiling tools (Ouroboros).
// Includes cross-compilation settings from ToolGenerationConfig.
func GetBuildEnvForCompile(userCfg *config.UserConfig, workspaceRoot string, targetOS, targetArch string) []string {
	env := GetBuildEnv(userCfg, workspaceRoot)

	// Add cross-compilation settings
	if targetOS != "" {
		env = setEnvKey(env, "GOOS", targetOS)
	}
	if targetArch != "" {
		env = setEnvKey(env, "GOARCH", targetArch)
	}

	return env
}

// AppendGoFlags injects the configured BuildConfig.GoFlags into a `go` argv.
//
// GoFlags was previously stored and never read: config could ask for -v or
// -mod=mod and nothing applied it. Flags go immediately after the subcommand
// (`go test -mod=mod ./...`) because package patterns must stay last, and a
// flag whose name the caller already passed is skipped so an explicit argv
// always wins over config.
//
// args is the argv after the "go" binary itself, e.g. ["test", "-count=1", "./..."].
func AppendGoFlags(userCfg *config.UserConfig, workspaceRoot string, args []string) []string {
	if len(args) == 0 {
		return args
	}
	if !goFlagSubcommands[args[0]] {
		return args
	}

	cfg := loadBuildConfig(userCfg, workspaceRoot)
	if len(cfg.GoFlags) == 0 {
		return args
	}

	existing := make(map[string]bool, len(args))
	for _, a := range args[1:] {
		if name := goFlagName(a); name != "" {
			existing[name] = true
		}
	}

	var inject []string
	for _, flag := range cfg.GoFlags {
		name := goFlagName(flag)
		if name == "" || existing[name] {
			continue
		}
		existing[name] = true
		inject = append(inject, flag)
	}
	if len(inject) == 0 {
		return args
	}

	out := make([]string, 0, len(args)+len(inject))
	out = append(out, args[0])
	out = append(out, inject...)
	out = append(out, args[1:]...)
	return out
}

// goFlagName extracts the flag name from an argv token: "-count=1" -> "-count".
// Non-flag tokens (package patterns, file paths) return "".
func goFlagName(arg string) string {
	if !strings.HasPrefix(arg, "-") {
		return ""
	}
	if i := strings.IndexByte(arg, '='); i >= 0 {
		return arg[:i]
	}
	return arg
}

// SummarizeEnv renders an environment slice as a sorted, keys-only list.
// Values are never included: build envs carry whatever the operator whitelisted,
// which in practice includes API keys. Use this for logs and diffs.
func SummarizeEnv(env []string) string {
	keys := make([]string, 0, len(env))
	seen := make(map[string]bool, len(env))
	for _, e := range env {
		key, _, ok := strings.Cut(e, "=")
		if !ok || key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	slices.Sort(keys)
	return strings.Join(keys, ",")
}

// redactEnvValue returns a value safe to log for the given key.
func redactEnvValue(key, value string) string {
	upper := strings.ToUpper(key)
	for _, frag := range secretishFragments {
		if strings.Contains(upper, frag) {
			return redactedPlaceholder(value)
		}
	}
	if nonSecretEnvKeys[upper] {
		return value
	}
	return redactedPlaceholder(value)
}

func redactedPlaceholder(value string) string {
	if value == "" {
		return "<empty>"
	}
	return "<redacted>"
}

// envValue returns the value for key in an env slice, or "" when absent.
func envValue(env []string, key string) string {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return e[len(prefix):]
		}
	}
	return ""
}

// getBaseGoEnv returns essential Go environment variables.
func getBaseGoEnv() []string {
	env := []string{}

	// Always include PATH for finding go binary
	if path := os.Getenv("PATH"); path != "" {
		env = append(env, "PATH="+path)
	}

	// Go-specific essential vars
	essentialVars := []string{
		"GOPATH",
		"GOROOT",
		"GOCACHE",
		"GOMODCACHE",
		"GOFLAGS",      // Allow global build tags/flags to propagate
		"HOME",         // Required on Unix
		"USERPROFILE",  // Required on Windows
		"LOCALAPPDATA", // Required for GOCACHE default on Windows
		"TEMP",         // Required for go build temp files
		"TMP",
		"TMPDIR",
	}

	for _, key := range essentialVars {
		if val := os.Getenv(key); val != "" {
			env = setEnvKey(env, key, val)
		}
	}

	// Ensure GOCACHE is set - Go requires this for builds
	// If not set in environment, provide a sensible default
	if !hasEnvKey(env, "GOCACHE") {
		gocache := deriveGOCACHE()
		if gocache != "" {
			env = setEnvKey(env, "GOCACHE", gocache)
			logging.BuildDebug("Derived GOCACHE: %s", gocache)
		} else {
			// Every fallback (LOCALAPPDATA/USERPROFILE/HOME/TEMP/TMP/TMPDIR) was
			// empty. The subprocess will fail with "GOCACHE is not defined",
			// which reads as a mysterious toolchain error at the call site, so
			// name the real cause here.
			buildWarn("GOCACHE could not be derived (no LOCALAPPDATA/USERPROFILE/HOME/TEMP/TMP/TMPDIR); go subprocesses will fail with 'GOCACHE is not defined'")
		}
	}

	return env
}

// deriveGOCACHE determines a sensible GOCACHE path when not explicitly set.
// This prevents "GOCACHE is not defined" errors in subprocess builds.
func deriveGOCACHE() string {
	// Try standard locations in order of preference

	// 1. Check if LocalAppData is available (Windows standard)
	if localAppData := os.Getenv("LOCALAPPDATA"); localAppData != "" {
		return filepath.Join(localAppData, "go-build")
	}

	// 2. Check USERPROFILE (Windows fallback)
	if userProfile := os.Getenv("USERPROFILE"); userProfile != "" {
		return filepath.Join(userProfile, ".cache", "go-build")
	}

	// 3. Check HOME (Unix standard)
	if home := os.Getenv("HOME"); home != "" {
		return filepath.Join(home, ".cache", "go-build")
	}

	// 4. Use temp directory as last resort
	if tmp := os.Getenv("TEMP"); tmp != "" {
		return filepath.Join(tmp, "go-build")
	}
	if tmp := os.Getenv("TMP"); tmp != "" {
		return filepath.Join(tmp, "go-build")
	}
	if tmp := os.Getenv("TMPDIR"); tmp != "" {
		return filepath.Join(tmp, "go-build")
	}

	// Give up - Go will error but at least we tried
	return ""
}

// loadBuildConfig loads project-specific build configuration.
func loadBuildConfig(userCfg *config.UserConfig, workspaceRoot string) *BuildConfig {
	cfg := DefaultBuildConfig()

	// Load explicit build config from user config if present.
	if userCfg != nil && userCfg.Build != nil {
		if userCfg.Build.EnvVars != nil {
			maps.Copy(cfg.EnvVars, userCfg.Build.EnvVars)
		}
		cfg.GoFlags = append(cfg.GoFlags, userCfg.Build.GoFlags...)
		cfg.CGOPackages = append(cfg.CGOPackages, userCfg.Build.CGOPackages...)
		logging.BuildDebug("Loaded BuildConfig from user config (%d env keys: %s)",
			len(cfg.EnvVars), SummarizeEnv(envSliceFromMap(cfg.EnvVars)))
	}

	// Resolve workspaceRoot to absolute path for reliable detection
	absRoot := workspaceRoot
	if !filepath.IsAbs(workspaceRoot) {
		if abs, err := filepath.Abs(workspaceRoot); err == nil {
			absRoot = abs
		}
	}

	// Check for sqlite_headers directory (codeNERD-specific)
	sqliteHeaders := filepath.Join(absRoot, "sqlite_headers")
	if _, err := os.Stat(sqliteHeaders); err == nil {
		// Found sqlite_headers - add CGO_CFLAGS with absolute path if not already set
		if cfg.EnvVars["CGO_CFLAGS"] == "" {
			cfg.EnvVars["CGO_CFLAGS"] = "-I" + sqliteHeaders
		}
		// Enable sqlite-vec build tag by default for internal builds when headers are present,
		// unless the user already provided GOFLAGS via env or config.
		if cfg.EnvVars["GOFLAGS"] == "" && os.Getenv("GOFLAGS") == "" {
			cfg.EnvVars["GOFLAGS"] = "-tags=sqlite_vec"
		}
		// Add sqlite-vec to CGO packages if missing
		already := slices.Contains(cfg.CGOPackages, "sqlite-vec")
		if !already {
			cfg.CGOPackages = append(cfg.CGOPackages, "sqlite-vec")
		}
		logging.BuildDebug("Detected sqlite_headers at: %s", sqliteHeaders)
	}

	return cfg
}

// envSliceFromMap renders a config env map as KEY=VALUE entries so SummarizeEnv
// can reduce it to keys for logging.
func envSliceFromMap(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k, v := range m {
		out = append(out, k+"="+v)
	}
	return out
}

// detectCGOFlags attempts to auto-detect required CGO_CFLAGS.
// This is a fallback when no explicit config is provided.
func detectCGOFlags(workspaceRoot string) string {
	var flags []string

	// Resolve to absolute path for reliable detection
	absRoot := workspaceRoot
	if !filepath.IsAbs(workspaceRoot) {
		if abs, err := filepath.Abs(workspaceRoot); err == nil {
			absRoot = abs
		}
	}

	// Check common header locations
	headerDirs := []string{
		"sqlite_headers",
		"include",
		"vendor/include",
		"third_party/include",
	}

	for _, dir := range headerDirs {
		fullPath := filepath.Join(absRoot, dir)
		if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
			flags = append(flags, "-I"+fullPath)
		}
	}

	if len(flags) > 0 {
		return strings.Join(flags, " ")
	}
	return ""
}

// hasEnvKey checks if an environment key is already set.
func hasEnvKey(env []string, key string) bool {
	prefix := key + "="
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			return true
		}
	}
	return false
}

// setEnvKey sets or updates an environment variable.
func setEnvKey(env []string, key, value string) []string {
	prefix := key + "="
	for i, e := range env {
		if strings.HasPrefix(e, prefix) {
			env[i] = key + "=" + value
			return env
		}
	}
	return append(env, key+"="+value)
}

// MergeEnv merges additional environment variables into base env.
// Later values override earlier ones.
func MergeEnv(base []string, additional ...string) []string {
	result := make([]string, len(base))
	copy(result, base)

	for _, add := range additional {
		key, val, ok := strings.Cut(add, "=")
		if ok {
			result = setEnvKey(result, key, val)
		}
	}

	return result
}
