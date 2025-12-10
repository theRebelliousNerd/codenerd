// Package build provides unified build environment configuration.
// This addresses the wiring issue where multiple components (preflight, thunderdome,
// ouroboros, attack_runner, tester) bypass the tactile layer and use raw exec.Command
// without proper environment variables like CGO_CFLAGS.
//
// All components that run go build/test should use GetBuildEnv() to ensure consistent
// environment configuration across the codebase.
package build

import (
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/config"
	"codenerd/internal/logging"
)

// BuildConfig holds project-specific build configuration.
// This is loaded from .nerd/config.json under the "build" key.
type BuildConfig struct {
	// EnvVars are additional environment variables for builds.
	// Key examples: CGO_CFLAGS, CGO_LDFLAGS, CGO_ENABLED, CC, CXX
	EnvVars map[string]string `json:"env_vars,omitempty"`

	// GoFlags are additional flags for go build/test commands.
	GoFlags []string `json:"go_flags,omitempty"`

	// CGOPackages lists packages that require CGO (for documentation/detection).
	CGOPackages []string `json:"cgo_packages,omitempty"`
}

// DefaultBuildConfig returns sensible defaults.
func DefaultBuildConfig() *BuildConfig {
	return &BuildConfig{
		EnvVars:     make(map[string]string),
		GoFlags:     []string{},
		CGOPackages: []string{},
	}
}

// GetBuildEnv returns the proper environment for go build/test commands.
// It merges:
// 1. Current process environment (filtered)
// 2. Whitelisted env vars from config
// 3. Project-specific build config (CGO_CFLAGS, etc.)
//
// This is the single source of truth for build environment.
// All components should use this instead of raw os.Environ().
func GetBuildEnv(userCfg *config.UserConfig, workspaceRoot string) []string {
	logging.BuildDebug("Building environment for workspace: %s", workspaceRoot)

	// Start with essential Go environment
	env := getBaseGoEnv()

	// Add whitelisted vars from execution config
	if userCfg != nil {
		execCfg := userCfg.GetExecution()
		for _, key := range execCfg.AllowedEnvVars {
			if val := os.Getenv(key); val != "" {
				env = append(env, key+"="+val)
				logging.BuildDebug("Added whitelisted env: %s", key)
			}
		}
	}

	// Add project-specific build config
	buildCfg := loadBuildConfig(userCfg, workspaceRoot)
	for key, val := range buildCfg.EnvVars {
		env = append(env, key+"="+val)
		logging.BuildDebug("Added build config env: %s=%s", key, val)
	}

	// Auto-detect CGO requirements if not explicitly set
	if !hasEnvKey(env, "CGO_CFLAGS") {
		if cgoFlags := detectCGOFlags(workspaceRoot); cgoFlags != "" {
			env = append(env, "CGO_CFLAGS="+cgoFlags)
			logging.BuildDebug("Auto-detected CGO_CFLAGS: %s", cgoFlags)
		}
	}

	logging.BuildDebug("Final build environment has %d vars", len(env))
	return env
}

// GetBuildEnvForTest returns environment for go test commands.
// Includes everything from GetBuildEnv plus test-specific settings.
func GetBuildEnvForTest(userCfg *config.UserConfig, workspaceRoot string) []string {
	env := GetBuildEnv(userCfg, workspaceRoot)

	// Enable race detector by default if not in CI
	if !hasEnvKey(env, "GOFLAGS") && os.Getenv("CI") == "" {
		// Don't force race detector as it's slower
		// Let callers add -race flag explicitly if needed
	}

	return env
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
		"HOME",        // Required on Unix
		"USERPROFILE", // Required on Windows
		"LOCALAPPDATA", // Required for GOCACHE default on Windows
		"TEMP",        // Required for go build temp files
		"TMP",
		"TMPDIR",
	}

	for _, key := range essentialVars {
		if val := os.Getenv(key); val != "" {
			env = append(env, key+"="+val)
		}
	}

	// Ensure GOCACHE is set - Go requires this for builds
	// If not set in environment, provide a sensible default
	if !hasEnvKey(env, "GOCACHE") {
		gocache := deriveGOCACHE()
		if gocache != "" {
			env = append(env, "GOCACHE="+gocache)
			logging.BuildDebug("Derived GOCACHE: %s", gocache)
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

	// TODO: Once BuildConfig is added to UserConfig, load from there
	// For now, use heuristics based on project structure

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
		// Found sqlite_headers - add CGO_CFLAGS with absolute path
		cfg.EnvVars["CGO_CFLAGS"] = "-I" + sqliteHeaders
		cfg.CGOPackages = append(cfg.CGOPackages, "sqlite-vec")
		logging.BuildDebug("Detected sqlite_headers at: %s", sqliteHeaders)
	}

	return cfg
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
		parts := strings.SplitN(add, "=", 2)
		if len(parts) == 2 {
			result = setEnvKey(result, parts[0], parts[1])
		}
	}

	return result
}
