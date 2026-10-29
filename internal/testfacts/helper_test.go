package testfacts

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// TestMain gives child `go` invocations a fresh, writable, package-shared
// build cache: the tests shell out to `go test -json`, and sharing one
// GOCACHE across the package's throwaway modules keeps that fast while
// staying hermetic (no reliance on the ambient cache's state).
func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "testfacts-gocache-")
	if err != nil {
		os.Exit(1)
	}
	os.Setenv("GOCACHE", dir)
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

// toolchainRe matches released Go versions ("go1.26.4"); anything else
// (devel builds) falls back to the ambient toolchain selection.
var toolchainRe = regexp.MustCompile(`^go\d+\.\d+`)

// writeModule writes files (path -> content) into a fresh throwaway module
// and returns its directory. Keys sort so multi-file modules lay out in a
// fixed order.
func writeModule(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(files[name]), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	goMod := "module example.com/mod\n\ngo 1.26.0\n"
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runGoTestJSON runs `go test -json -count=1` over args in dir and returns
// its stdout. A failing test run still returns its stream: the exit code
// is the subject under test, not a test failure. Only a failure to run
// the toolchain at all fails the test.
func runGoTestJSON(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmdArgs := append([]string{"test", "-json", "-count=1"}, args...)
	cmd := exec.Command("go", cmdArgs...)
	cmd.Dir = dir
	cmd.Env = childEnv()
	var stdout strings.Builder
	cmd.Stdout = &stdout
	// Stderr carries toolchain warnings, not events; the mixed-stderr
	// case is covered synthetically in raw_test.go instead.
	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); !ok {
			t.Fatalf("go test -json: %v", err)
		}
	}
	return stdout.String()
}

// childEnv pins the child toolchain to the one running the tests (derived
// from runtime.Version, never hardcoded) and takes the network off the
// table: throwaway modules have no dependencies to fetch.
func childEnv() []string {
	env := os.Environ()
	if v := runtime.Version(); toolchainRe.MatchString(v) {
		env = append(env, "GOTOOLCHAIN="+v)
	}
	return append(env, "GOPROXY=off")
}

// parseString parses s as a `go test -json` stream whose command ran in dir.
// dir is the workspace the files are canonicalised against; "" leaves a
// relative path cleaned but unanchored. A read error fails the test
// (strings do not produce any; the check keeps the helper honest).
func parseString(t *testing.T, dir, s string) *Result {
	t.Helper()
	res, err := Parse(dir, strings.NewReader(s))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return res
}

// findTest locates a package's test by name, failing the test when absent.
func findTest(t *testing.T, p *Package, name string) *Test {
	t.Helper()
	for _, ct := range p.Tests {
		if ct.Name == name {
			return ct
		}
	}
	t.Fatalf("package %s: no test %q", p.Name, name)
	return nil
}
