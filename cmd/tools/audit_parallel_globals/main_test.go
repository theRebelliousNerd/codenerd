package main

import (
	"errors"
	"flag"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func writeFixture(t *testing.T, root, name, source string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestScan_SeparatesInternalAndExternalTestHelpers(t *testing.T) {
	for _, unsafePackage := range []string{"sample", "sample_test"} {
		t.Run(unsafePackage, func(t *testing.T) {
			root := t.TempDir()
			for _, pkg := range []string{"sample", "sample_test"} {
				helper := "func observe() {}"
				imports := `import "testing"`
				if pkg == unsafePackage {
					imports = "import (\"testing\"; \"runtime\")"
					helper = "func observe() { _ = runtime.NumGoroutine() }"
				}
				writeFixture(t, root, pkg+"_test.go", "package "+pkg+"\n"+imports+"\n"+helper+"\nfunc TestProbe(t *testing.T) { t.Parallel(); observe() }")
			}
			found, err := scan(root, "example.test/sample")
			if err != nil {
				t.Fatal(err)
			}
			if len(found) != 1 {
				t.Fatalf("findings = %+v; want only the helper in package %s", found, unsafePackage)
			}
			if f := found[0]; f.key() != "example.test/sample\tTestProbe\tobserve()" || !strings.HasSuffix(f.pos, "/"+unsafePackage+"_test.go:4") || !strings.Contains(f.reason, "reached through observe") {
				t.Fatalf("finding = %+v; want the call in package %s", f, unsafePackage)
			}
		})
	}
}

func TestScan_ParallelScopes(t *testing.T) {
	cases := []struct {
		name string
		body string
		want int
	}{
		{"serial", `_ = runtime.NumGoroutine()`, 0},
		{"parallel", `t.Parallel(); _ = runtime.NumGoroutine()`, 1},
		{"parallel sibling leaves serial scopes alone", `
			_ = runtime.NumGoroutine()
			t.Run("parallel", func(t *testing.T) { t.Parallel(); _ = runtime.NumGoroutine() })
			t.Run("serial", func(t *testing.T) { _ = runtime.NumGoroutine() })`, 1},
		{"parallel parent propagates to descendants", `
			t.Parallel()
			t.Run("child", func(t *testing.T) {
				_ = runtime.NumGoroutine()
				t.Run("grandchild", func(t *testing.T) { _ = runtime.NumGoroutine() })
			})`, 2},
		{"nested parallel child leaves parent serial", `
			t.Run("child", func(t *testing.T) {
				_ = runtime.NumGoroutine()
				t.Run("grandchild", func(t *testing.T) { t.Parallel(); _ = runtime.NumGoroutine() })
			})`, 1},
		{"ordinary closure stays in scope", `t.Parallel(); func() { _ = runtime.NumGoroutine() }()`, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFixture(t, root, "scopes_test.go", "package sample\nimport (\"testing\"; \"runtime\")\nfunc TestScope(t *testing.T) {"+tc.body+"}")
			found, err := scan(root, "example.test/sample")
			if err != nil {
				t.Fatal(err)
			}
			if len(found) != tc.want {
				t.Fatalf("findings = %+v; want %d", found, tc.want)
			}
			for _, f := range found {
				if f.key() != "example.test/sample\tTestScope\truntime.NumGoroutine" || f.reason == "" || !strings.Contains(f.pos, "scopes_test.go:") {
					t.Errorf("incomplete finding: %+v", f)
				}
			}
		})
	}
}

func TestScan_WatchedCallsAndImportAliases(t *testing.T) {
	cases := []struct {
		name    string
		imports string
		call    string
		want    string
	}{
		{"goroutines", `"runtime"`, `runtime.NumGoroutine()`, "runtime.NumGoroutine"},
		{"memory", `rt "runtime"`, `rt.ReadMemStats(nil)`, "rt.ReadMemStats"},
		{"scheduler write", `"runtime"`, `runtime.GOMAXPROCS(2)`, "runtime.GOMAXPROCS"},
		{"scheduler dynamic", `"runtime"`, `runtime.GOMAXPROCS(n)`, "runtime.GOMAXPROCS"},
		{"scheduler read", `"runtime"`, `runtime.GOMAXPROCS(0)`, ""},
		{"set environment", `env "os"`, `env.Setenv("A", "B")`, "env.Setenv"},
		{"unset environment", `"os"`, `os.Unsetenv("A")`, "os.Unsetenv"},
		{"clear environment", `"os"`, `os.Clearenv()`, "os.Clearenv"},
		{"working directory", `"os"`, `os.Chdir(".")`, "os.Chdir"},
		{"GC percent", `"runtime/debug"`, `debug.SetGCPercent(100)`, "debug.SetGCPercent"},
		{"memory limit", `dbg "runtime/debug"`, `dbg.SetMemoryLimit(1024)`, "dbg.SetMemoryLimit"},
		{"stack limit", `"runtime/debug"`, `debug.SetMaxStack(1024)`, "debug.SetMaxStack"},
		{"unwatched call", `"runtime"`, `runtime.GC()`, ""},
		{"unrelated import named runtime", `runtime "example.test/other"`, `runtime.NumGoroutine()`, ""},
		{"blank import", `_ "runtime"`, `ordinary()`, ""},
		{"dot import is not followed", `. "runtime"`, `NumGoroutine()`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			// These snippets are parsed, not executed: no test changes global state.
			writeFixture(t, root, "calls_test.go", "package sample\nimport (\"testing\"; "+tc.imports+")\nfunc TestCall(t *testing.T) { t.Parallel(); "+tc.call+" }")
			found, err := scan(root, "example.test/sample")
			if err != nil {
				t.Fatal(err)
			}
			if tc.want == "" {
				if len(found) != 0 {
					t.Fatalf("unexpected findings: %+v", found)
				}
				return
			}
			if len(found) != 1 || found[0].call != tc.want || found[0].reason == "" {
				t.Fatalf("findings = %+v; want %s with a reason", found, tc.want)
			}
		})
	}
}

func TestScan_FollowsHelpersAcrossFilesAndCycles(t *testing.T) {
	root := t.TempDir()
	writeFixture(t, root, "sub/a_test.go", `package sample
import "testing"
func first() { second() }
func second() { first(); probe() }
func harmlessA() { harmlessB() }
func harmlessB() { harmlessA() }
func TestProbe(t *testing.T) { t.Parallel(); first(); harmlessA() }
func TestSerial(t *testing.T) { first() }
func BenchmarkProbe(b *testing.B) { first() }
func FuzzProbe(f *testing.F) { f.Fuzz(func(t *testing.T, input string) { t.Parallel(); first() }) }
`)
	writeFixture(t, root, "sub/z_test.go", `package sample
import "runtime"
func probe() { _ = runtime.NumGoroutine() }
`)
	found, err := scan(root, "example.test/module")
	if err != nil {
		t.Fatal(err)
	}
	var keys []string
	for _, f := range found {
		keys = append(keys, f.key())
		if !strings.Contains(f.reason, "counts goroutines") || !strings.Contains(f.reason, "reached through first") {
			t.Errorf("helper lost its reason: %+v", f)
		}
	}
	sort.Strings(keys)
	want := []string{
		"example.test/module/sub\tFuzzProbe\tfirst()",
		"example.test/module/sub\tTestProbe\tfirst()",
	}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys = %q; want %q", keys, want)
	}
}

func TestScan_SkipsExcludedAndMalformedFiles(t *testing.T) {
	root := t.TempDir()
	source := "package sample\nimport (\"testing\"; \"runtime\")\nfunc TestProbe(t *testing.T) { t.Parallel(); _ = runtime.NumGoroutine() }"
	for _, name := range []string{".hidden/a_test.go", "vendor/a_test.go", "node_modules/a_test.go", "production.go"} {
		writeFixture(t, root, name, source)
	}
	writeFixture(t, root, "broken_test.go", "package sample\nfunc broken(")
	writeFixture(t, root, "visible/a_test.go", source)
	found, err := scan(root, "example.test/module")
	if err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0].pkg != "example.test/module/visible" {
		t.Fatalf("findings = %+v; want only the visible test file", found)
	}
}

func TestScan_MissingRootReturnsError(t *testing.T) {
	found, err := scan(filepath.Join(t.TempDir(), "missing"), "example.test/module")
	if !errors.Is(err, fs.ErrNotExist) || len(found) != 0 {
		t.Fatalf("scan = %+v, %v; want no findings and a missing-root error", found, err)
	}
}

func TestModuleName(t *testing.T) {
	for _, tc := range []struct {
		name    string
		content string
		want    string
	}{
		{"module directive", "go 1.27\n  module example.test/module  \n", "example.test/module"},
		{"missing directive", "go 1.27\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeFixture(t, root, "go.mod", tc.content)
			got, err := moduleName(root)
			if got != tc.want || (err != nil) != (tc.want == "") {
				t.Fatalf("moduleName = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
	t.Run("missing file", func(t *testing.T) {
		_, err := moduleName(t.TempDir())
		if !errors.Is(err, fs.ErrNotExist) {
			t.Fatalf("error = %v; want missing go.mod", err)
		}
	})
}

func TestBaseline_RoundTripAndComments(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "nested", "baseline.txt")
	found := []finding{
		{pkg: "example.test/module", fn: "TestProbe", call: "runtime.NumGoroutine", pos: "ignored:1", reason: "ignored"},
		{pkg: "example.test/module/sub", fn: "TestEnv", call: "os.Setenv"},
	}
	if err := writeBaseline(path, found); err != nil {
		t.Fatal(err)
	}
	got, err := readBaseline(path)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{
		"example.test/module\tTestProbe\truntime.NumGoroutine": true,
		"example.test/module/sub\tTestEnv\tos.Setenv":          true,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("baseline = %v; want %v", got, want)
	}
	writeFixture(t, root, "comments.txt", "# comment\n\n   \nexample.test/module\tTestProbe\tos.Chdir\nexample.test/module\tTestProbe\tos.Chdir\n")
	got, err = readBaseline(filepath.Join(root, "comments.txt"))
	if err != nil || !reflect.DeepEqual(got, map[string]bool{"example.test/module\tTestProbe\tos.Chdir": true}) {
		t.Fatalf("comments and duplicates: baseline = %v, error = %v", got, err)
	}
}

func TestBaseline_Errors(t *testing.T) {
	root := t.TempDir()
	t.Run("missing baseline is empty", func(t *testing.T) {
		got, err := readBaseline(filepath.Join(root, "missing.txt"))
		if err != nil || got == nil || len(got) != 0 {
			t.Fatalf("baseline = %v, %v; want empty baseline", got, err)
		}
	})
	t.Run("scanner failure", func(t *testing.T) {
		writeFixture(t, root, "long.txt", strings.Repeat("x", 1024*1024+1))
		if _, err := readBaseline(filepath.Join(root, "long.txt")); err == nil {
			t.Fatal("oversized baseline entry was accepted")
		}
	})
	t.Run("parent is a file", func(t *testing.T) {
		writeFixture(t, root, "file", "not a directory")
		if err := writeBaseline(filepath.Join(root, "file", "baseline.txt"), nil); err == nil {
			t.Fatal("writing beneath a file succeeded")
		}
	})
	t.Run("destination is a directory", func(t *testing.T) {
		if err := writeBaseline(root, nil); err == nil {
			t.Fatal("writing a baseline over a directory succeeded")
		}
	})
}

func TestAuditParallelGlobals_Command(t *testing.T) {
	// Re-exec the real main in an isolated process: it owns flags and calls os.Exit.
	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = append([]string{"audit_parallel_globals"}, os.Args[i+1:]...)
			flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ExitOnError)
			main()
			return
		}
	}
	for _, tc := range []struct {
		name     string
		source   string
		external string
		baseline string
		update   bool
		wantExit int
		wantText string
	}{
		{name: "empty", wantText: "No parallel test reads or writes process-global state"},
		{name: "addition", source: `package sample; import ("testing"; "runtime"); func TestProbe(t *testing.T) { t.Parallel(); _ = runtime.NumGoroutine() }`, wantExit: 1, wantText: "not in the baseline"},
		{name: "unchanged", source: `package sample; import ("testing"; "runtime"); func TestProbe(t *testing.T) { t.Parallel(); _ = runtime.NumGoroutine() }`, baseline: "example.test/module\tTestProbe\truntime.NumGoroutine\n", wantText: "Parallel-globals budget holds: 1 entry(ies), no drift"},
		{
			name: "separate test packages",
			source: `package sample; import ("testing"; "runtime")
func observe() { _ = runtime.NumGoroutine() }
func TestProbe(t *testing.T) { t.Parallel(); observe() }`,
			external: `package sample_test; import "testing"
func observe() {}
func TestSafe(t *testing.T) { t.Parallel(); observe() }`,
			baseline: "example.test/module\tTestProbe\tobserve()\n",
			wantText: "Parallel-globals budget holds: 1 entry(ies), no drift",
		},
		{name: "removal", baseline: "example.test/module\tTestProbe\truntime.NumGoroutine\n", wantExit: 1, wantText: "Entries that are no longer present (1)"},
		{name: "update", source: `package sample; import ("testing"; "runtime"); func TestProbe(t *testing.T) { t.Parallel(); _ = runtime.NumGoroutine() }`, update: true, wantText: "Baseline rewritten: 1 parallel test(s)"},
		{name: "missing module", wantExit: 2, wantText: "read go.mod"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			if tc.name != "missing module" {
				writeFixture(t, root, "go.mod", "module example.test/module\n")
			}
			if tc.source != "" {
				writeFixture(t, root, "probe_test.go", tc.source)
			}
			if tc.external != "" {
				writeFixture(t, root, "external_test.go", tc.external)
			}
			if tc.baseline != "" {
				writeFixture(t, root, baselinePath, tc.baseline)
			}
			args := []string{"-test.run=^TestAuditParallelGlobals_Command$", "--"}
			if tc.update {
				args = append(args, "-update")
			}
			args = append(args, root)
			cmd := exec.CommandContext(t.Context(), os.Args[0], args...)
			output, err := cmd.CombinedOutput()
			var exitErr *exec.ExitError
			exit := 0
			if err != nil {
				if !errors.As(err, &exitErr) {
					t.Fatalf("run audit command: %v", err)
				}
				exit = exitErr.ExitCode()
			}
			if exit != tc.wantExit || !strings.Contains(string(output), tc.wantText) {
				t.Fatalf("exit = %d, output = %s; want exit %d and %q", exit, output, tc.wantExit, tc.wantText)
			}
			if tc.update {
				got, err := readBaseline(filepath.Join(root, baselinePath))
				if err != nil || !reflect.DeepEqual(got, map[string]bool{"example.test/module\tTestProbe\truntime.NumGoroutine": true}) {
					t.Fatalf("updated baseline = %v, %v", got, err)
				}
			}
		})
	}
}
