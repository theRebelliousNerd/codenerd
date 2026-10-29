package testfacts

import (
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// Windows-shaped streams must parse to the same slash-form Files as their
// Unix twins: the compiler reports `.\bf_test.go` while panic frames on
// that same host arrive `C:/...`, so without normalization one file gets
// two spellings and its facts never join in the kernel. WSL cannot produce
// Windows compiler output, so these streams are synthetic; each case
// asserts the slash-form File in the Result, the Facts, and the Summary.
func TestWindowsPathsSlashForm(t *testing.T) {
	// dir is the workspace the command ran in. The panic frame below is that
	// directory's own file, spelled the way a Windows traceback prints it
	// (backslashes). CanonicalPath has to land on the same identity as the
	// t.Errorf basename.
	dir := writeModule(t, map[string]string{"x_test.go": "package mod\n"})
	frame := strings.ReplaceAll(filepath.Join(dir, "x_test.go"), "/", `\`)
	buildOut := func(importPath, text string) string {
		return `{"Action":"build-output","ImportPath":"` + importPath +
			`","Output":` + quoteForStream(text) + "}\n"
	}
	buildFail := func(importPath string) string {
		return `{"Action":"build-fail","ImportPath":"` + importPath + `"}` + "\n"
	}
	run := func(pkg, test string) string {
		return `{"Action":"run","Package":"` + pkg + `","Test":"` + test + `"}` + "\n"
	}
	out := func(pkg, test, text string) string {
		return `{"Action":"output","Package":"` + pkg + `","Test":"` + test +
			`","Output":` + quoteForStream(text) + "}\n"
	}
	verdict := func(action, pkg, test string) string {
		return `{"Action":"` + action + `","Package":"` + pkg + `","Test":"` + test + `"}` + "\n"
	}
	cases := []struct {
		name   string
		stream string
		check  func(t *testing.T, res *Result)
	}{
		{
			name: "backslash build diagnostic",
			stream: buildOut("example.com/mod", "# example.com/mod\n.\\bf_test.go:5:33: undefined: x\n") +
				buildFail("example.com/mod") +
				`{"Action":"fail","Package":"example.com/mod","FailedBuild":"example.com/mod"}` + "\n",
			check: func(t *testing.T, res *Result) {
				if len(res.BuildFailures) != 1 {
					t.Fatalf("BuildFailures = %+v, want 1", res.BuildFailures)
				}
				bf := res.BuildFailures[0]
				if bf.File != "bf_test.go" || bf.Line != 5 || bf.Column != 33 {
					t.Errorf("BuildFailure = %+v, want bf_test.go:5:33", bf)
				}
				assertFact(t, findFact(t, res.Facts(), PredTestBuildFailure), PredTestBuildFailure, []any{
					types.MangleString("example.com/mod"),
					types.MangleString("bf_test.go"),
					int64(5),
					types.MangleString("undefined: x"),
				})
				summary := res.Summary()
				if !strings.Contains(summary, "build-failed example.com/mod bf_test.go:5:33: undefined: x") {
					t.Errorf("summary missing slash-form diagnostic:\n%s", summary)
				}
				assertNoBackslashFile(t, res)
			},
		},
		{
			name: "subtest error with backslash panic frame",
			stream: run("example.com/mod", "TestOuter/inner") +
				out("example.com/mod", "TestOuter/inner", "    x_test.go:12: boom\n") +
				out("example.com/mod", "TestOuter/inner", "panic: kaboom\n") +
				out("example.com/mod", "TestOuter/inner", "\t"+frame+":12 +0x1d\n") +
				verdict("fail", "example.com/mod", "TestOuter/inner") +
				verdict("fail", "example.com/mod", ""),
			check: func(t *testing.T, res *Result) {
				if len(res.Failures) != 2 {
					t.Fatalf("Failures = %+v, want 2", res.Failures)
				}
				if res.Failures[0].File != "x_test.go" || res.Failures[0].Line != 12 {
					t.Errorf("first = %+v, want x_test.go:12", res.Failures[0])
				}
				if res.Failures[1].File != "x_test.go" || res.Failures[1].Line != 12 || res.Failures[1].Message != "kaboom" {
					t.Errorf("panic = %+v, want x_test.go:12 kaboom", res.Failures[1])
				}
				var files []string
				for _, f := range res.Facts() {
					if f.Predicate != PredTestFailureAt {
						continue
					}
					file, _ := f.Args[2].(types.MangleString)
					files = append(files, string(file))
				}
				if len(files) != 2 || files[0] != "x_test.go" || files[1] != "x_test.go" {
					t.Errorf("test_failure_at files = %q, want both x_test.go", files)
				}
				summary := res.Summary()
				if !strings.Contains(summary, "FAIL example.com/mod TestOuter/inner x_test.go:12: boom") {
					t.Errorf("summary missing slash-form FAIL line:\n%s", summary)
				}
				assertNoBackslashFile(t, res)
			},
		},
		{
			// A frame on another drive is not under dir. It keeps the absolute
			// slash form: rewriting it relative would give it a workspace
			// identity the tree does not have.
			name: "panic frame outside the workspace stays absolute",
			stream: run("example.com/mod", "TestOut") +
				out("example.com/mod", "TestOut", "panic: elsewhere\n") +
				out("example.com/mod", "TestOut", "\tD:\\other\\place\\x_test.go:4 +0x1d\n") +
				verdict("fail", "example.com/mod", "TestOut") +
				verdict("fail", "example.com/mod", ""),
			check: func(t *testing.T, res *Result) {
				if len(res.Failures) != 1 {
					t.Fatalf("Failures = %+v, want 1", res.Failures)
				}
				if res.Failures[0].File != "D:/other/place/x_test.go" || res.Failures[0].Line != 4 {
					t.Errorf("panic = %+v, want D:/other/place/x_test.go:4", res.Failures[0])
				}
				assertNoBackslashFile(t, res)
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.check(t, parseString(t, dir, tc.stream))
		})
	}
}

// findFact returns the first fact with the given predicate, failing the
// test when the stream produced none.
func findFact(t *testing.T, facts []types.Fact, pred string) types.Fact {
	t.Helper()
	for _, f := range facts {
		if f.Predicate == pred {
			return f
		}
	}
	t.Fatalf("no %s fact in %v", pred, facts)
	return types.Fact{}
}

// assertNoBackslashFile enforces the one-spelling invariant: no File the
// package produces -- in the Result, the File args of the Facts, or the
// File-bearing Summary lines -- may carry a backslash separator. Repeat
// lines are excluded by design: they quote raw output verbatim for recall.
func assertNoBackslashFile(t *testing.T, res *Result) {
	t.Helper()
	for _, f := range res.Failures {
		if strings.Contains(f.File, "\\") {
			t.Errorf("Failure.File = %q, want slash form", f.File)
		}
	}
	for _, b := range res.BuildFailures {
		if strings.Contains(b.File, "\\") {
			t.Errorf("BuildFailure.File = %q, want slash form", b.File)
		}
	}
	for _, f := range res.Facts() {
		var file any
		switch f.Predicate {
		case PredTestFailureAt:
			file = f.Args[2]
		case PredTestBuildFailure:
			file = f.Args[1]
		default:
			continue
		}
		if s, ok := file.(types.MangleString); ok && strings.Contains(string(s), "\\") {
			t.Errorf("fact %s File = %q, want slash form", f.Predicate, s)
		}
	}
	for _, line := range strings.Split(res.Summary(), "\n") {
		if !strings.HasPrefix(line, "build-failed ") && !strings.HasPrefix(line, "FAIL ") {
			continue
		}
		if strings.Contains(line, "\\") {
			t.Errorf("summary line = %q, want slash form", line)
		}
	}
}
