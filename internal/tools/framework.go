package tools

import (
	"os"
	"path/filepath"
	"strings"
)

// =============================================================================
// TEST FRAMEWORK / COMMAND DETECTION — canonical single source
// =============================================================================
//
// Test-framework detection used to live in four places that drifted apart:
// CheckpointRunner.detectTestCommand/detectBuildCommand in
// internal/campaign/checkpoint.go and detectTestCommand/detectBuildCommand in
// internal/tools/shell/execute.go. Each kept its own ordered file_exists
// table, so adding a framework to one left the other stale.
//
// This file is the single canonical projection. Both Go call sites consume
// these helpers instead of keeping private detectors. The tables below mirror
// the test_framework/1 facts and test_command/1 + build_command/1 projections
// in internal/core/defaults/policy/intent_routing_rules.mg (Section 3); only
// mappings verified there are treated as framework mappings. A small set of
// legacy extras (Maven/Gradle/Make/CMake, Python build Auge) preserves the
// union of what the old Go detectors recognized so unifying them does not
// regress projects those detectors already handled.
//
// This lives in the leaf tools package (no codenerd imports) on purpose:
// internal/core imports internal/tools/shell for tool hydration, so the
// detectors cannot live in core — shell's run_tests/run_build tools would
// then import core and close an import cycle (core -> shell -> core) that
// breaks `go build ./...` for every package that transitively touches the
// campaign checkpoints. Callers in core import this package; callers in
// shell already do. Callers with a kernel should prefer querying the
// canonical test_framework/1, test_command/1 and build_command/1 facts.
//
// Fallback policy differs by caller and stays with the caller: the campaign
// CheckpointRunner falls back to the Go defaults on unknown workspaces
// (DefaultTestCommand/DefaultBuildCommand), while the shell run_tests/run_build
// tools treat unknown as undetected ("", false) and report "could not detect"
// so the agent must pass an explicit command. The helpers therefore return
// (command, ok) instead of baking in a default.

// Framework atoms. The slash-prefixed spelling matches the policy
// test_framework/1 facts.
const (
	FrameworkGoTest    = "/go_test"
	FrameworkJest      = "/jest"
	FrameworkVitest    = "/vitest"
	FrameworkMocha     = "/mocha"
	FrameworkPytest    = "/pytest"
	FrameworkCargoTest = "/cargo_test"
	FrameworkRSpec     = "/rspec"
	FrameworkMinitest  = "/minitest"
)

// Go-default fallbacks, preserved from the old CheckpointRunner detectors.
// An empty workspace resolves to these.
const (
	DefaultTestCommand  = "go test ./..."
	DefaultBuildCommand = "go build ./..."
)

// TestFrameworkForDir derives the test framework atom for dir from marker
// files. It mirrors the policy test_framework/1 rules. The unittest case is
// intentionally absent: policy derives it from the file_topology scanner
// (IsTestFile), which a bare directory scan cannot reproduce.
//
// Returns ("", false) when no marker matches.
func TestFrameworkForDir(dir string) (string, bool) {
	if fw, ok := compiledFrameworkForDir(dir); ok {
		return fw, true
	}
	if fw, ok := jsFrameworkForDir(dir); ok {
		return fw, true
	}
	if fw, ok := pythonFrameworkForDir(dir); ok {
		return fw, true
	}
	if fw, ok := rubyFrameworkForDir(dir); ok {
		return fw, true
	}
	return "", false
}

func compiledFrameworkForDir(dir string) (string, bool) {
	if dirHasFile(dir, "go.mod") {
		return FrameworkGoTest, true
	}
	if dirHasFile(dir, "Cargo.toml") {
		return FrameworkCargoTest, true
	}
	return "", false
}

func jsFrameworkForDir(dir string) (string, bool) {
	if dirHasFile(dir, "jest.config.js") || dirHasFile(dir, "jest.config.ts") {
		return FrameworkJest, true
	}
	if dirHasFile(dir, "vitest.config.js") || dirHasFile(dir, "vitest.config.ts") {
		return FrameworkVitest, true
	}
	if dirHasFile(dir, "mocharc.json") || dirHasFile(dir, ".mocharc.js") {
		return FrameworkMocha, true
	}
	// Generic npm runner. The specific configs above refine the atom, but
	// all three map to the same test/build commands.
	if dirHasFile(dir, "package.json") {
		return FrameworkJest, true
	}
	return "", false
}

func pythonFrameworkForDir(dir string) (string, bool) {
	if dirHasFile(dir, "pytest.ini") || dirHasFile(dir, "conftest.py") {
		return FrameworkPytest, true
	}
	if dirHasFile(dir, "requirements.txt") || dirHasFile(dir, "setup.py") {
		return FrameworkPytest, true
	}
	// Policy only fires when pyproject.toml mentions pytest; any
	// pyproject.toml is still Python, so pytest is the runner either way.
	if dirHasFile(dir, "pyproject.toml") {
		return FrameworkPytest, true
	}
	return "", false
}

func rubyFrameworkForDir(dir string) (string, bool) {
	if dirHasFile(dir, ".rspec") {
		return FrameworkRSpec, true
	}
	if dirHasFile(dir, "Gemfile") && dirFileContains(dir, "Gemfile", "minitest") {
		return FrameworkMinitest, true
	}
	return "", false
}

// TestCommandForFramework maps a framework atom to its test command. It
// mirrors the policy test_command/1 projection; only mappings verified there
// are present. Frameworks without a policy test mapping (/unittest, /rspec,
// /minitest) return ("", false).
func TestCommandForFramework(framework string) (string, bool) {
	switch framework {
	case FrameworkGoTest:
		return "go test ./...", true
	case FrameworkCargoTest:
		return "cargo test", true
	case FrameworkPytest:
		return "pytest", true
	case FrameworkJest, FrameworkVitest, FrameworkMocha:
		return "npm test", true
	default:
		return "", false
	}
}

// BuildCommandForFramework maps a framework atom to its build command. It
// mirrors the policy build_command/1 projection. Frameworks without a policy
// build mapping (pytest and the Ruby/unittest cases) return ("", false).
func BuildCommandForFramework(framework string) (string, bool) {
	switch framework {
	case FrameworkGoTest:
		return "go build ./...", true
	case FrameworkCargoTest:
		return "cargo build", true
	case FrameworkJest, FrameworkVitest, FrameworkMocha:
		return "npm run build", true
	default:
		return "", false
	}
}

// TestCommandForDir derives the test command for dir. Policy-verified
// framework mappings win; legacy extras the old Go detectors recognized
// (Maven/Gradle/Make) follow so unifying the detectors does not regress
// those projects. Returns ("", false) when nothing matches.
func TestCommandForDir(dir string) (string, bool) {
	if fw, ok := TestFrameworkForDir(dir); ok {
		if cmd, ok := TestCommandForFramework(fw); ok {
			return cmd, true
		}
	}
	return legacyTestCommandForDir(dir)
}

func legacyTestCommandForDir(dir string) (string, bool) {
	if dirHasFile(dir, "pom.xml") {
		return "mvn test", true
	}
	if dirHasFile(dir, "build.gradle") {
		return "gradle test", true
	}
	if dirHasFile(dir, "Makefile") {
		return "make test", true
	}
	return "", false
}

// BuildCommandForDir derives the build command for dir. Policy-verified
// framework mappings win; legacy extras the old Go detectors recognized
// follow. Returns ("", false) when nothing matches.
func BuildCommandForDir(dir string) (string, bool) {
	if fw, ok := TestFrameworkForDir(dir); ok {
		if cmd, ok := BuildCommandForFramework(fw); ok {
			return cmd, true
		}
	}
	return legacyBuildCommandForDir(dir)
}

func legacyBuildCommandForDir(dir string) (string, bool) {
	if dirHasFile(dir, "pom.xml") {
		return "mvn package", true
	}
	if dirHasFile(dir, "build.gradle") {
		return "gradle build", true
	}
	if dirHasFile(dir, "Makefile") {
		return "make build", true
	}
	return pythonBuildCommandForDir(dir)
}

func pythonBuildCommandForDir(dir string) (string, bool) {
	if dirHasFile(dir, "CMakeLists.txt") {
		return "cmake --build .", true
	}
	if dirHasFile(dir, "setup.py") {
		return "python setup.py build", true
	}
	if dirHasFile(dir, "pyproject.toml") {
		return "python -m build", true
	}
	return "", false
}

func dirHasFile(dir, name string) bool {
	if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
		return true
	}
	return false
}

func dirFileContains(dir, name, substr string) bool {
	data, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		return false
	}
	return strings.Contains(string(data), substr)
}
