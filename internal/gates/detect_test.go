package gates

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// installed stubs lookPath so only the named programs are on PATH.
func installed(t *testing.T, progs ...string) {
	t.Helper()
	prev := lookPath
	lookPath = func(p string) (string, error) {
		if slices.Contains(progs, p) {
			return "/usr/bin/" + p, nil
		}
		return "", errors.New("not found")
	}
	t.Cleanup(func() { lookPath = prev })
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func ids(gs []Gate) []string {
	out := make([]string, len(gs))
	for i, g := range gs {
		out[i] = g.ID
	}
	return out
}

func unavailableIDs(us []Unavailable) []string {
	out := make([]string, len(us))
	for i, u := range us {
		out[i] = u.Gate.ID
	}
	return out
}

func find(t *testing.T, s Set, id string) Gate {
	t.Helper()
	for _, g := range s.Gates {
		if g.ID == id {
			return g
		}
	}
	t.Fatalf("no runnable gate %q in %v (unavailable %v)", id, ids(s.Gates), unavailableIDs(s.Unavailable))
	return Gate{}
}

func mustDetect(t *testing.T, root string) Set {
	t.Helper()
	s, err := Detect(root)
	if err != nil {
		t.Fatalf("Detect: %v", err)
	}
	return s
}

func TestDetect_GoModuleGetsBuildVetTest(t *testing.T) {
	installed(t, "go")
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"go.mod": "module m\n"})
	s := mustDetect(t, root)
	if got, want := ids(s.Gates), []string{"go:build", "go:test", "go:vet"}; !slices.Equal(got, want) {
		t.Fatalf("gates = %v, want %v", got, want)
	}
	if got := find(t, s, "go:test").ForNode("internal/store"); !slices.Equal(got, []string{"go", "test", "-count=1", "./internal/store"}) {
		t.Fatalf("go:test for a node = %v", got)
	}
	if got := find(t, s, "go:vet").ForNode("."); !slices.Equal(got, []string{"go", "vet", "."}) {
		t.Fatalf("go:vet for the root = %v", got)
	}
}

func TestDetect_PythonPrefersPython3AndTreatsNoTestsAsPass(t *testing.T) {
	installed(t, "python3")
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"pyproject.toml": "[project]\nname='x'\n"})
	s := mustDetect(t, root)
	pytest := find(t, s, "python:pytest")
	if got := pytest.ForNode("src/shop"); !slices.Equal(got, []string{"python3", "-m", "pytest", "-q", "src/shop"}) {
		t.Fatalf("pytest argv = %v", got)
	}
	if !pytest.Passed(0) || !pytest.Passed(5) || pytest.Passed(1) {
		t.Fatalf("pytest: exit 0 and 5 pass, 1 fails")
	}
	find(t, s, "python:compile")
}

// A package.json's own scripts are the gates, through the package manager its
// lockfile names; npm's placeholder test script is not a test gate.
func TestDetect_PackageJSONScriptsThroughTheLockfilesManager(t *testing.T) {
	installed(t, "pnpm", "npx")
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"package.json":   `{"scripts":{"build":"tsc","test":"vitest run","lint":"eslint ."}}`,
		"pnpm-lock.yaml": "",
	})
	s := mustDetect(t, root)
	if got, want := ids(s.Gates), []string{"js:build", "js:lint", "js:test"}; !slices.Equal(got, want) {
		t.Fatalf("gates = %v, want %v", got, want)
	}
	if got := find(t, s, "js:test").Argv; !slices.Equal(got, []string{"pnpm", "run", "test"}) {
		t.Fatalf("js:test argv = %v", got)
	}

	root2 := t.TempDir()
	writeFiles(t, root2, map[string]string{
		"package.json":  `{"scripts":{"test":"echo \"Error: no test specified\" && exit 1"}}`,
		"tsconfig.json": "{}",
	})
	installed(t, "npm", "npx")
	s2 := mustDetect(t, root2)
	if got, want := ids(s2.Gates), []string{"js:typecheck"}; !slices.Equal(got, want) {
		t.Fatalf("a TS project without a build script gets a type check and no placeholder test: %v", got)
	}
}

func TestDetect_UnparsablePackageJSONIsReportedNotDropped(t *testing.T) {
	installed(t)
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"package.json": "{nope"})
	s := mustDetect(t, root)
	if len(s.Gates) != 0 || !slices.Contains(unavailableIDs(s.Unavailable), "js:package.json") {
		t.Fatalf("an unparsable package.json must be an unavailable gate: gates %v, unavailable %v", ids(s.Gates), unavailableIDs(s.Unavailable))
	}
}

func TestDetect_CargoGetsBuildTestClippy(t *testing.T) {
	installed(t, "cargo")
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"Cargo.toml": "[package]\nname='x'\n"})
	s := mustDetect(t, root)
	if got, want := ids(s.Gates), []string{"rust:build", "rust:clippy", "rust:test"}; !slices.Equal(got, want) {
		t.Fatalf("gates = %v, want %v", got, want)
	}
}

// A toolchain that is not installed makes its gates unavailable, with the
// reason: never silently absent, never a pass.
func TestDetect_MissingToolchainIsUnavailableNotDropped(t *testing.T) {
	installed(t)
	root := t.TempDir()
	writeFiles(t, root, map[string]string{"go.mod": "module m\n", "setup.py": ""})
	s := mustDetect(t, root)
	if len(s.Gates) != 0 {
		t.Fatalf("nothing is installed; no gate is runnable: %v", ids(s.Gates))
	}
	got := unavailableIDs(s.Unavailable)
	for _, want := range []string{"go:build", "go:test", "go:vet", "python:compile", "python:pytest"} {
		if !slices.Contains(got, want) {
			t.Fatalf("unavailable = %v, missing %s", got, want)
		}
	}
	for _, u := range s.Unavailable {
		if !strings.Contains(u.Reason, "not on PATH") {
			t.Fatalf("reason must say why: %+v", u)
		}
	}
}

// nerd.md commands replace the detected gates of their kind; kinds nerd.md
// does not name keep their detected gates; the gates: list adds audits.
func TestDetect_NerdMDCommandsReplaceDetectedGatesOfTheirKind(t *testing.T) {
	installed(t, "go", "make")
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod": "module m\n",
		"nerd.md": `---
schema: nerd/v1
commands:
  test: make test ARGS="-count=1 -race"
  env:
    CGO_ENABLED: "1"
gates:
  - id: deadcode
    kind: audit
    run: ./scripts/deadcode-budget.sh
  - id: node-lint
    kind: lint
    run: make lint PKG={pkg}
    scope: node
---
# project
`,
	})
	s := mustDetect(t, root)
	got := ids(s.Gates)
	if slices.Contains(got, "go:test") {
		t.Fatalf("nerd.md's test command replaces go:test: %v", got)
	}
	for _, want := range []string{"go:build", "go:vet", "nerd.md:test", "nerd.md:deadcode", "nerd.md:node-lint"} {
		if !slices.Contains(got, want) {
			t.Fatalf("gates = %v, missing %s", got, want)
		}
	}
	test := find(t, s, "nerd.md:test")
	if !slices.Equal(test.Argv, []string{"make", "test", "ARGS=-count=1 -race"}) || test.Env["CGO_ENABLED"] != "1" {
		t.Fatalf("nerd.md test gate = %+v", test)
	}
	if audit := find(t, s, "nerd.md:deadcode"); audit.Kind != Audit {
		t.Fatalf("a workspace-relative script is runnable without PATH, as its declared kind: %+v", audit)
	}
	if got := find(t, s, "nerd.md:node-lint").ForNode("pkg/a"); !slices.Equal(got, []string{"make", "lint", "PKG=./pkg/a"}) {
		t.Fatalf("node-scoped nerd.md gate = %v", got)
	}
}

func TestDetect_MalformedNerdMDIsAnError(t *testing.T) {
	installed(t, "go")
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"go.mod":  "module m\n",
		"nerd.md": "---\nschema: nerd/v1\ngates:\n  - id: x\n    kind: bogus\n    run: true\n---\n",
	})
	if _, err := Detect(root); err == nil {
		t.Fatal("a nerd.md gate with an unknown kind must fail detection, not be ignored")
	}
}

func TestDetect_EmptyWorkspaceHasNoGates(t *testing.T) {
	installed(t, "go")
	s := mustDetect(t, t.TempDir())
	if len(s.Gates) != 0 || len(s.Unavailable) != 0 {
		t.Fatalf("an empty workspace has no gates: %+v", s)
	}
}
