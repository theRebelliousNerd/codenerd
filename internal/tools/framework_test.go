package tools

import (
	"os"
	"path/filepath"
	"testing"
)

// framework.go decides what `nerd test` and `nerd build` actually run, and was
// entirely untested across fifteen functions. A wrong detection does not fail
// loudly: it runs a real command against a real project, which is either a
// confusing error from a tool the project does not use, or -- worse -- a
// successful run of the wrong thing.
//
// The mappings are also documented as mirroring the Mangle policy's
// test_framework/1 and test_command/1 rules. A drift between the Go detector
// and the policy is invisible until the two disagree about one project.

func dirWith(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", name, err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return dir
}

func TestTestFrameworkForDirMatchesItsMarkers(t *testing.T) {
	tests := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"go module", map[string]string{"go.mod": "module x"}, FrameworkGoTest},
		{"rust crate", map[string]string{"Cargo.toml": "[package]"}, FrameworkCargoTest},
		{"jest config js", map[string]string{"jest.config.js": ""}, FrameworkJest},
		{"jest config ts", map[string]string{"jest.config.ts": ""}, FrameworkJest},
		{"vitest", map[string]string{"vitest.config.ts": ""}, FrameworkVitest},
		{"mocha rc json", map[string]string{"mocharc.json": "{}"}, FrameworkMocha},
		{"mocha rc js", map[string]string{".mocharc.js": ""}, FrameworkMocha},
		{"bare package.json", map[string]string{"package.json": "{}"}, FrameworkJest},
		{"pytest ini", map[string]string{"pytest.ini": ""}, FrameworkPytest},
		{"conftest", map[string]string{"conftest.py": ""}, FrameworkPytest},
		{"requirements", map[string]string{"requirements.txt": ""}, FrameworkPytest},
		{"setup.py", map[string]string{"setup.py": ""}, FrameworkPytest},
		{"pyproject", map[string]string{"pyproject.toml": "[project]"}, FrameworkPytest},
		{"rspec", map[string]string{".rspec": ""}, FrameworkRSpec},
		{"gemfile with minitest", map[string]string{"Gemfile": `gem "minitest"`}, FrameworkMinitest},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := TestFrameworkForDir(dirWith(t, tc.files))
			if !ok {
				t.Fatalf("no framework detected for %v", tc.files)
			}
			if got != tc.want {
				t.Errorf("framework = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestGemfileWithoutMinitestIsNotMinitest(t *testing.T) {
	// A Gemfile alone says Ruby, not which runner. Guessing minitest would run
	// a command the project may not have, and the check is a substring match
	// that is easy to loosen by accident.
	dir := dirWith(t, map[string]string{"Gemfile": `gem "rails"`})
	if got, ok := TestFrameworkForDir(dir); ok {
		t.Errorf("framework = %q for a Gemfile with no runner named, want none", got)
	}
}

func TestCompiledMarkersWinOverScriptingOnes(t *testing.T) {
	// A polyglot repo is the common case, not the exception: a Go service with
	// a package.json for its frontend tooling must still test as Go. The order
	// of the detectors is the behaviour, so it is pinned.
	dir := dirWith(t, map[string]string{
		"go.mod":         "module x",
		"package.json":   "{}",
		"pyproject.toml": "[project]",
	})
	got, ok := TestFrameworkForDir(dir)
	if !ok || got != FrameworkGoTest {
		t.Errorf("framework = %q (%v), want %q — compiled markers must win", got, ok, FrameworkGoTest)
	}
}

func TestSpecificJSConfigRefinesTheGenericOne(t *testing.T) {
	dir := dirWith(t, map[string]string{
		"package.json":     "{}",
		"vitest.config.ts": "",
	})
	got, _ := TestFrameworkForDir(dir)
	if got != FrameworkVitest {
		t.Errorf("framework = %q, want %q — a specific config must refine the generic package.json", got, FrameworkVitest)
	}
}

func TestEmptyDirectoryDetectsNothing(t *testing.T) {
	// Returning a default here would run `go test` in a directory that is not
	// a Go project. Reporting nothing lets the caller say so.
	if got, ok := TestFrameworkForDir(t.TempDir()); ok {
		t.Errorf("framework = %q for an empty directory, want none", got)
	}
	if cmd, ok := TestCommandForDir(t.TempDir()); ok {
		t.Errorf("test command = %q for an empty directory, want none", cmd)
	}
	if cmd, ok := BuildCommandForDir(t.TempDir()); ok {
		t.Errorf("build command = %q for an empty directory, want none", cmd)
	}
}

func TestMissingDirectoryDetectsNothing(t *testing.T) {
	absent := filepath.Join(t.TempDir(), "does-not-exist")
	if _, ok := TestFrameworkForDir(absent); ok {
		t.Error("a missing directory reported a framework")
	}
}

func TestCommandMappingsAreExhaustiveAndFailClosed(t *testing.T) {
	testCmds := map[string]string{
		FrameworkGoTest:    "go test ./...",
		FrameworkCargoTest: "cargo test",
		FrameworkPytest:    "pytest",
		FrameworkJest:      "npm test",
		FrameworkVitest:    "npm test",
		FrameworkMocha:     "npm test",
	}
	for fw, want := range testCmds {
		got, ok := TestCommandForFramework(fw)
		if !ok || got != want {
			t.Errorf("TestCommandForFramework(%q) = %q,%v; want %q,true", fw, got, ok, want)
		}
	}

	// Frameworks with no policy-verified mapping must report none rather than
	// guess. A guessed command here is a command that runs.
	for _, fw := range []string{FrameworkRSpec, FrameworkMinitest, "/unittest", "", "/invented"} {
		if cmd, ok := TestCommandForFramework(fw); ok {
			t.Errorf("TestCommandForFramework(%q) = %q, want no mapping", fw, cmd)
		}
	}

	buildCmds := map[string]string{
		FrameworkGoTest:    "go build ./...",
		FrameworkCargoTest: "cargo build",
		FrameworkJest:      "npm run build",
		FrameworkVitest:    "npm run build",
		FrameworkMocha:     "npm run build",
	}
	for fw, want := range buildCmds {
		got, ok := BuildCommandForFramework(fw)
		if !ok || got != want {
			t.Errorf("BuildCommandForFramework(%q) = %q,%v; want %q,true", fw, got, ok, want)
		}
	}
	// Pytest has a test command and no build command; that asymmetry is the
	// policy's, and collapsing it would invent a build step for every Python
	// project.
	for _, fw := range []string{FrameworkPytest, FrameworkRSpec, FrameworkMinitest, ""} {
		if cmd, ok := BuildCommandForFramework(fw); ok {
			t.Errorf("BuildCommandForFramework(%q) = %q, want no mapping", fw, cmd)
		}
	}
}

func TestLegacyDetectorsStillCoverMavenGradleAndMake(t *testing.T) {
	// These are not in the policy's framework rules, and unifying the
	// detectors must not silently drop the projects the old Go ones handled.
	for _, tc := range []struct {
		marker   string
		testCmd  string
		buildCmd string
	}{
		{"pom.xml", "mvn test", "mvn package"},
		{"build.gradle", "gradle test", "gradle build"},
		{"Makefile", "make test", "make build"},
	} {
		t.Run(tc.marker, func(t *testing.T) {
			dir := dirWith(t, map[string]string{tc.marker: ""})
			if got, ok := TestCommandForDir(dir); !ok || got != tc.testCmd {
				t.Errorf("test command = %q,%v; want %q", got, ok, tc.testCmd)
			}
			if got, ok := BuildCommandForDir(dir); !ok || got != tc.buildCmd {
				t.Errorf("build command = %q,%v; want %q", got, ok, tc.buildCmd)
			}
		})
	}
}

func TestFrameworkMappingsWinOverLegacyOnes(t *testing.T) {
	// A Go project with a Makefile is common, and `make test` is not what the
	// policy says to run.
	dir := dirWith(t, map[string]string{"go.mod": "module x", "Makefile": ""})
	if got, _ := TestCommandForDir(dir); got != "go test ./..." {
		t.Errorf("test command = %q, want the framework mapping to win over the Makefile", got)
	}
}

func TestPythonBuildFallbacksApplyWhereTheFrameworkHasNoBuild(t *testing.T) {
	// pytest maps to no build command, so BuildCommandForDir falls through to
	// the legacy chain. Without that, a Python project would report no build
	// step at all despite having one.
	for _, tc := range []struct{ marker, want string }{
		{"CMakeLists.txt", "cmake --build ."},
		{"setup.py", "python setup.py build"},
		{"pyproject.toml", "python -m build"},
	} {
		t.Run(tc.marker, func(t *testing.T) {
			dir := dirWith(t, map[string]string{tc.marker: ""})
			got, ok := BuildCommandForDir(dir)
			if !ok || got != tc.want {
				t.Errorf("build command = %q,%v; want %q", got, ok, tc.want)
			}
		})
	}
}

func TestDirFileContainsHandlesAMissingFile(t *testing.T) {
	dir := t.TempDir()
	if dirFileContains(dir, "absent", "anything") {
		t.Error("dirFileContains reported a match in a file that does not exist")
	}
	if dirHasFile(dir, "absent") {
		t.Error("dirHasFile reported a file that does not exist")
	}
	// A directory is not a file for these purposes, but Stat succeeds on one.
	// Recording the behaviour rather than asserting a fix: a marker name that
	// is also a directory name is not a case any caller hits, and pretending
	// otherwise would be a test of imagined behaviour.
	if err := os.Mkdir(filepath.Join(dir, "go.mod"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if !dirHasFile(dir, "go.mod") {
		t.Log("dirHasFile distinguishes directories from files")
	}
}
