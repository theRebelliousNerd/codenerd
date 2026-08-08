package session

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// F-VERIFY-2: the repo compiled but had no test gate, so a turn could write
// Go source with a logic error and still be reported as success. These tests
// pin the test gate that stops that.

// TestPackagesForPaths verifies the written-paths → package-pattern mapping.
func TestPackagesForPaths(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  []string
	}{
		{"nothing written", nil, nil},
		{"empty slice", []string{}, nil},
		{"markdown only", []string{"Docs/architecture/README.md", ".nerd/notes.txt"}, nil},
		{"one go file", []string{"internal/session/executor.go"}, []string{"./internal/session"}},
		{"uppercase extension", []string{"cmd/nerd/Main.GO"}, []string{"./cmd/nerd"}},
		{"whitespace padded", []string{"  internal/core/kernel.go  "}, []string{"./internal/core"}},
		{"go in middle of name", []string{"internal/gopher/notes.md"}, nil},
		{"non-go ignored among go", []string{"README.md", "internal/foo/bar.go", "assets/style.css"}, []string{"./internal/foo"}},
		{"root file becomes dot", []string{"main.go"}, []string{"."}},
		{"root file with dot-slash", []string{"./main.go"}, []string{"."}},
		{"deduplicates same package", []string{"internal/foo/a.go", "internal/foo/b.go", "internal/foo/c_test.go"}, []string{"./internal/foo"}},
		{"multiple packages sorted", []string{"internal/b/b.go", "internal/a/a.go", "internal/b/c.go"}, []string{"./internal/a", "./internal/b"}},
		{"test file maps to its package", []string{"internal/foo/bar_test.go"}, []string{"./internal/foo"}},
		{"mixed root and subpackage", []string{"main.go", "internal/foo/bar.go"}, []string{".", "./internal/foo"}},
		{"skips whitespace-only entries", []string{"   ", "\t", "internal/foo/bar.go"}, []string{"./internal/foo"}},
		{"already dot-prefixed", []string{"./internal/session/foo.go"}, []string{"./internal/session"}},
		{"duplicate across prefixed and bare", []string{"internal/foo/a.go", "./internal/foo/b.go"}, []string{"./internal/foo"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := packagesForPaths(tc.paths)
			// Normalise nil vs empty for comparison.
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Fatalf("packagesForPaths(%v) = %v; want %v", tc.paths, got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Errorf("packagesForPaths(%v)[%d] = %q; want %q (full got=%v want=%v)", tc.paths, i, got[i], want[i], got, want)
				}
			}
		})
	}
}

// TestUntestedGoFiles verifies the "written Go files without a matching
// _test.go written in the same turn" check.
func TestUntestedGoFiles(t *testing.T) {
	cases := []struct {
		name  string
		paths []string
		want  []string
	}{
		{"nothing written", nil, nil},
		{"non-go only", []string{"README.md", "docs/guide.md"}, nil},
		{"test file alone not returned", []string{"internal/foo/bar_test.go"}, nil},
		{"go file without test is untested", []string{"internal/foo/bar.go"}, []string{"internal/foo/bar.go"}},
		{"go file with matching test is tested", []string{"internal/foo/bar.go", "internal/foo/bar_test.go"}, nil},
		{"go file with test in different dir is untested", []string{"internal/foo/bar.go", "internal/other/bar_test.go"}, []string{"internal/foo/bar.go"}},
		{"different base name not a match", []string{"internal/foo/bar.go", "internal/foo/other_test.go"}, []string{"internal/foo/bar.go"}},
		{"multiple files mixed", []string{
			"internal/foo/a.go", "internal/foo/a_test.go",
			"internal/foo/b.go",
			"internal/bar/c.go", "internal/bar/c_test.go",
			"internal/bar/d.go",
		}, []string{"internal/bar/d.go", "internal/foo/b.go"}},
		{"non-go files never returned", []string{"internal/foo/bar.go", "internal/foo/readme.md"}, []string{"internal/foo/bar.go"}},
		{"test files themselves never returned", []string{"internal/foo/bar.go", "internal/foo/bar_test.go", "internal/foo/baz_test.go"}, nil},
		{"deduplicated", []string{"internal/foo/bar.go", "internal/foo/bar.go"}, []string{"internal/foo/bar.go"}},
		{"whitespace trimmed", []string{"  internal/foo/bar.go  ", "  internal/foo/bar_test.go  "}, nil},
		{"whitespace untested", []string{"  internal/foo/bar.go  "}, []string{"internal/foo/bar.go"}},
		{"uppercase GO with matching test", []string{"internal/foo/Bar.GO", "internal/foo/Bar_test.go"}, nil},
		{"root file without test", []string{"foo.go"}, []string{"foo.go"}},
		{"root file with matching test", []string{"foo.go", "foo_test.go"}, nil},
		{"dot-prefixed pair is tested", []string{"./internal/foo/bar.go", "./internal/foo/bar_test.go"}, nil},
		{"dot-prefixed go without dot-prefixed test is untested", []string{"./internal/foo/bar.go", "internal/foo/bar_test.go"}, nil},
		{"test written but source not written not returned", []string{"internal/foo/bar_test.go", "internal/foo/other.go"}, []string{"internal/foo/other.go"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := untestedGoFiles(tc.paths)
			if len(got) == 0 && len(tc.want) == 0 {
				return
			}
			sort.Strings(got)
			want := append([]string(nil), tc.want...)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Fatalf("untestedGoFiles(%v) = %v; want %v", tc.paths, got, want)
			}
			for i := range got {
				if got[i] != want[i] {
					t.Errorf("untestedGoFiles(%v)[%d] = %q; want %q (full got=%v want=%v)", tc.paths, i, got[i], want[i], got, want)
				}
			}
		})
	}
}

// A verification that did not run must never be reported as a pass. Ran=false
// means "unknown" — not "ok". Every early-exit path in verifyTests must honour
// this, and callers must check Ran before OK.
func TestTestVerification_DidNotRunIsNotAPass(t *testing.T) {
	cases := []struct {
		name      string
		workspace string
		packages  []string
	}{
		{"empty workspace", "  ", []string{"./internal/session"}},
		{"empty packages", t.TempDir(), nil},
		{"nil packages", t.TempDir(), []string{}},
		{"whitespace packages", t.TempDir(), []string{"  ", "\t"}},
		{"empty workspace and empty packages", "", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := verifyTests(context.Background(), tc.workspace, tc.packages)
			if v.Ran {
				t.Errorf("verifyTests(%q, %v) claimed to run; want Ran=false", tc.workspace, tc.packages)
			}
			if v.OK {
				t.Error("a verification that did not run must not report OK (Ran=false must never be a pass)")
			}
			if v.Output != "" {
				t.Errorf("a not-run verification should have empty output, got %q", v.Output)
			}
			// The gate predicate is Ran && OK. When Ran is false the predicate
			// must be false regardless of OK.
			if v.Ran && v.OK {
				t.Error("Ran && OK must be false when Ran is false")
			}
		})
	}
}

// Struct-level pin: even a hand-constructed TestVerification with Ran=false
// must not be treated as a pass by any predicate callers use.
func TestTestVerification_StructDidNotRunIsNotAPass(t *testing.T) {
	cases := []struct {
		name string
		v    TestVerification
	}{
		{"zero value", TestVerification{}},
		{"ran false ok false", TestVerification{Ran: false, OK: false}},
		{"ran false ok true is still not a pass", TestVerification{Ran: false, OK: true}},
		{"ran true ok false is failure not pass", TestVerification{Ran: true, OK: false}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			isPass := tc.v.Ran && tc.v.OK
			if !tc.v.Ran && isPass {
				t.Error("Ran=false must never satisfy the pass predicate (Ran && OK)")
			}
			if tc.name == "ran false ok true is still not a pass" && isPass {
				t.Error("Ran=false with OK=true must still not be a pass; Ran is the gate")
			}
		})
	}
}

// Integration: verifyTests against real throwaway modules. Mirrors
// TestVerifyBuild_DetectsBrokenPackage but exercises `go test`.
func TestVerifyTests_DetectsPassingAndFailingPackages(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles throwaway packages")
	}
	ws := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		p := filepath.Join(ws, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("go.mod", "module verifyprobe\n\ngo 1.21\n")

	// Healthy package: one exported function and a passing test.
	write("calc.go", "package verifyprobe\n\nfunc Add(a, b int) int { return a + b }\n")
	write("calc_test.go", "package verifyprobe\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { if Add(2,3) != 5 { t.Fatal(\"bad\") } }\n")
	v := verifyTests(context.Background(), ws, []string{"."})
	if !v.Ran || !v.OK {
		t.Fatalf("healthy package reported Ran=%v OK=%v output=%q", v.Ran, v.OK, v.Output)
	}
	if v.Output != "" {
		t.Errorf("passing tests should have empty output, got %q", v.Output)
	}

	// Break the test: same file, assertion now fails.
	write("calc_test.go", "package verifyprobe\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { if Add(2,3) != 999 { t.Fatal(\"intentional failure\") } }\n")
	v = verifyTests(context.Background(), ws, []string{"."})
	if !v.Ran {
		t.Fatal("verification did not run against a failing package")
	}
	if v.OK {
		t.Fatal("verification passed a package whose tests fail")
	}
	if !strings.Contains(v.Output, "intentional failure") && !strings.Contains(v.Output, "FAIL") {
		t.Errorf("test output does not carry the failure, so a repair round has nothing to work from: %q", v.Output)
	}
}

// verifyTests must run `go test` on exactly the packages it is given, not on
// ./... and not on a default. A package outside the list whose tests fail must
// not cause the verification to fail.
func TestVerifyTests_RunsExactlyRequestedPackages(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles throwaway packages")
	}
	ws := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		p := filepath.Join(ws, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("go.mod", "module verifyprobe\n\ngo 1.21\n")

	write("good/good.go", "package good\n\nfunc Add(a, b int) int { return a + b }\n")
	write("good/good_test.go", "package good\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { if Add(1,2) != 3 { t.Fatal(\"bad\") } }\n")

	write("bad/bad.go", "package bad\n\nfunc Add(a, b int) int { return a + b }\n")
	write("bad/bad_test.go", "package bad\n\nimport \"testing\"\n\nfunc TestAdd(t *testing.T) { t.Fatal(\"always fails\") }\n")

	// Only the good package was touched: verification must pass.
	v := verifyTests(context.Background(), ws, []string{"./good"})
	if !v.Ran || !v.OK {
		t.Fatalf("verification of ./good alone reported Ran=%v OK=%v output=%q (must not be polluted by ./bad)", v.Ran, v.OK, v.Output)
	}

	// Now include the bad package: must fail.
	v = verifyTests(context.Background(), ws, []string{"./good", "./bad"})
	if !v.Ran {
		t.Fatal("verification did not run")
	}
	if v.OK {
		t.Fatal("verification passed when the bad package was included")
	}
}

// Truncation: a failing test that emits more than testVerifyMaxOutput bytes
// must have its output capped so it can be fed back without blowing the
// context budget. Mirrors the buildVerifyMaxOutput truncation contract.
func TestVerifyTests_TruncatesLongOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("compiles throwaway package with large output")
	}
	ws := t.TempDir()
	write := func(name, content string) {
		t.Helper()
		p := filepath.Join(ws, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("go.mod", "module verifyprobe\n\ngo 1.21\n")

	// Print a payload larger than testVerifyMaxOutput (6000) then fail.
	write("big_test.go", "package verifyprobe\n\nimport (\n\"strings\"\n\"testing\"\n)\n\nfunc TestBig(t *testing.T) {\n t.Log(strings.Repeat(\"X\", 8000))\n t.Fatal(\"fail big\")\n}\n")
	v := verifyTests(context.Background(), ws, []string{"."})
	if !v.Ran {
		t.Fatal("verification did not run")
	}
	if v.OK {
		t.Fatal("verification passed a failing test")
	}
	if len(v.Output) > testVerifyMaxOutput+100 {
		t.Errorf("output not truncated: len=%d want <= %d", len(v.Output), testVerifyMaxOutput+100)
	}
	if !strings.Contains(v.Output, "test output truncated") {
		t.Errorf("truncated output should contain truncation marker, got %q", v.Output)
	}
}

// The repair prompt must forbid the cheapest way out. An agent told only "the
// tests fail" will often delete or weaken the assertion, which turns red green
// while destroying the thing that made the suite worth running.
func TestTestRepairPrompt_ForbidsWeakeningTheTest(t *testing.T) {
	out := "--- FAIL: TestAdd (0.00s)\n    calc_test.go:5: intentional failure"
	p := testRepairPrompt(out)

	if !strings.Contains(p, out) {
		t.Error("repair prompt drops the test output, which is the only thing that makes the round useful")
	}
	for _, want := range []string{"delete the failing test", "weaken its assertion", "skip it"} {
		if !strings.Contains(p, want) {
			t.Errorf("repair prompt does not forbid %q — the cheapest way to make a test pass", want)
		}
	}
	// It must still leave an honest escape hatch for a genuinely wrong test,
	// or the agent is forced to contort correct code to satisfy a bad spec.
	if !strings.Contains(p, "incorrect") {
		t.Error("repair prompt gives no path for a test that is genuinely wrong")
	}
}

// Gating: the gate must not fire on turns it has no business running for.
// A markdown-only turn that pays for `go test` is a tax on every doc edit.
func TestVerifyAndRepairTests_SkipsWhenNotApplicable(t *testing.T) {
	cases := []struct {
		name   string
		cfgOn  bool
		result *ExecutionResult
	}{
		{"disabled by config", false, &ExecutionResult{SuccessfulWriteTools: 1, WrittenPaths: []string{"a.go"}}},
		{"nil result", true, nil},
		{"no successful writes", true, &ExecutionResult{SuccessfulWriteTools: 0, WrittenPaths: []string{"a.go"}}},
		{"wrote no Go", true, &ExecutionResult{SuccessfulWriteTools: 2, WrittenPaths: []string{"README.md", "notes.txt"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultExecutorConfig()
			cfg.VerifyTestsAfterEdits = tc.cfgOn
			e := &Executor{config: cfg}

			// A nil ToolResultsProvider would make any real repair round fail
			// loudly, so reaching the end without error proves we short-circuited.
			resp, errs, err := e.verifyAndRepairTests(
				context.Background(), nil, "", nil, nil, nil, tc.result)
			if err != nil {
				t.Fatalf("gate should have skipped, got error: %v", err)
			}
			if resp != nil || errs != nil {
				t.Errorf("gate should have skipped, got resp=%v errs=%v", resp, errs)
			}
		})
	}
}

// The same-turn predicate is too eager to gate on. It flagged
// internal/session/test_verify.go on two consecutive live turns (2026-08-08
// 11:08 and 11:13) because neither turn rewrote test_verify_test.go — a file
// sitting right next to it with 40 passing subtests. A gate that cries wolf
// about tested code is one that gets ignored, then switched off.
func TestUntestedWithoutCoverageOnDisk(t *testing.T) {
	ws := t.TempDir()
	mk := func(rel string) {
		t.Helper()
		p := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(p, []byte("package p\n"), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	// covered/: source edited this turn, its own _test.go already on disk.
	mk("covered/thing.go")
	mk("covered/thing_test.go")
	// sibling/: source edited this turn, no thing_test.go but the package has
	// another test file. Go does not require one test file per source file.
	mk("sibling/widget.go")
	mk("sibling/other_test.go")
	// bare/: source edited this turn, package has no test file at all.
	mk("bare/lonely.go")

	cases := []struct {
		name    string
		written []string
		want    []string
	}{
		{"own test on disk is covered", []string{"covered/thing.go"}, nil},
		{"package test file counts as covered", []string{"sibling/widget.go"}, nil},
		{"no test anywhere is flagged", []string{"bare/lonely.go"}, []string{"bare/lonely.go"}},
		{"test written this turn is covered", []string{"bare/lonely.go", "bare/lonely_test.go"}, nil},
		{"non-go never flagged", []string{"bare/README.md"}, nil},
		{
			"mixed reports only the bare one",
			[]string{"covered/thing.go", "sibling/widget.go", "bare/lonely.go"},
			[]string{"bare/lonely.go"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := untestedWithoutCoverageOnDisk(ws, tc.written)
			if len(got) != len(tc.want) {
				t.Fatalf("untestedWithoutCoverageOnDisk(%v) = %v; want %v", tc.written, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Errorf("index %d = %q; want %q", i, got[i], tc.want[i])
				}
			}
		})
	}
}

// An unreadable directory is not evidence that tests are missing. The gate
// fails toward silence: a false "untested" claim is worse than a missed one.
func TestPackageHasTestFile_UnreadableDirIsNotEvidence(t *testing.T) {
	if !packageHasTestFile(filepath.Join(t.TempDir(), "does-not-exist")) {
		t.Error("an unreadable directory was treated as proof of missing tests")
	}
}
