package shell

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/lintreport"
	"codenerd/internal/tools"
)

// lintModule is a temp Go module with a planted vet finding (a Printf verb
// that does not match its argument) in ./bad, and a clean package in ./good.
func lintModule(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := map[string]string{
		"go.mod":      "module lintprobe\n\ngo 1.22\n",
		"bad/bad.go":  "package bad\n\nimport \"fmt\"\n\n// Say prints n.\nfunc Say(n string) { fmt.Printf(\"%d\\n\", n) }\n",
		"good/ok.go":  "package good\n\n// Two is 2.\nfunc Two() int { return 2 }\n",
		"broken/b.go": "package broken\n\n// F is broken.\nfunc F() int { return missing }\n",
	}
	for name, content := range files {
		full := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func runLint(t *testing.T, root string, args map[string]any) (LintResult, error) {
	t.Helper()
	out, err := executeRunLint(shellWsCtx(root), args)
	var r LintResult
	if jerr := json.Unmarshal([]byte(out), &r); jerr != nil {
		t.Fatalf("run_lint returned no JSON result (err %v): %q", err, out)
	}
	return r, err
}

// The model's vet: real go vet over a temp module with a planted finding comes
// back as a typed finding -- file, line, column, analyzer, message -- not a log.
func TestRunLint_VetReportsThePlantedFindingTyped(t *testing.T) {
	t.Parallel()
	root := lintModule(t)
	r, err := runLint(t, root, map[string]any{"gate": "vet", "packages": []any{"./bad", "./good"}})
	if err != nil {
		t.Fatalf("run_lint vet: %v (%+v)", err, r)
	}
	if r.Status != LintFindings || len(r.Findings) != 1 {
		t.Fatalf("status %s with %d finding(s), want one finding: %+v", r.Status, len(r.Findings), r)
	}
	f := r.Findings[0]
	if f.File != "bad/bad.go" || f.Line != 6 || f.Column == 0 || f.Check != "printf" || !strings.Contains(f.Message, "wrong type") {
		t.Fatalf("finding = %+v, want bad/bad.go:6 printf wrong type", f)
	}
	if r.Argv[0] != "go" || r.Argv[1] != "vet" || r.Argv[2] != "-json" {
		t.Fatalf("argv = %v, want go vet -json", r.Argv)
	}

	clean, err := runLint(t, root, map[string]any{"gate": "vet", "packages": []any{"./good"}})
	if err != nil || clean.Status != LintClean || len(clean.Findings) != 0 {
		t.Fatalf("a clean package: %+v (err %v), want status clean", clean, err)
	}
}

// A package that does not compile is a compile finding at its position, not an
// opaque failure.
func TestRunLint_VetCompileErrorIsAFinding(t *testing.T) {
	t.Parallel()
	root := lintModule(t)
	r, err := runLint(t, root, map[string]any{"gate": "vet", "packages": []any{"./broken"}})
	if err != nil {
		t.Fatalf("run_lint vet: %v", err)
	}
	if r.Status != LintFindings || len(r.Findings) != 1 || r.Findings[0].Check != lintreport.CompileCheck ||
		r.Findings[0].File != "broken/b.go" || r.Findings[0].Line != 4 || !strings.Contains(r.Findings[0].Message, "undefined: missing") {
		t.Fatalf("result = %+v, want one compile finding at broken/b.go:4", r)
	}
}

// staticcheck and golangci-lint run when installed, each read from its own
// JSON report.
func TestRunLint_InstalledLintersReportTyped(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ gate, check string }{{"staticcheck", "SA5009"}, {"golangci-lint", "govet"}} {
		t.Run(tc.gate, func(t *testing.T) {
			t.Parallel()
			if _, err := exec.LookPath(tc.gate); err != nil {
				t.Skipf("%s is not installed", tc.gate)
			}
			root := lintModule(t)
			r, err := runLint(t, root, map[string]any{"gate": tc.gate, "packages": []any{"./bad"}})
			if err != nil {
				t.Fatalf("run_lint %s: %v (%+v)", tc.gate, err, r)
			}
			var hit bool
			for _, f := range r.Findings {
				hit = hit || (f.File == "bad/bad.go" && f.Line == 6 && f.Check == tc.check)
			}
			if r.Status != LintFindings || !hit {
				t.Fatalf("%s: %+v, want a %s finding at bad/bad.go:6", tc.gate, r, tc.check)
			}
		})
	}
}

// A gate whose binary is not installed is a typed answer, not an error, and
// nothing is downloaded or run.
func TestRunLint_MissingBinaryIsUnavailable(t *testing.T) {
	root := lintModule(t)
	t.Setenv("PATH", t.TempDir())
	for _, gate := range []string{"staticcheck", "golangci-lint"} {
		r, err := runLint(t, root, map[string]any{"gate": gate, "packages": []any{"./bad"}})
		if err != nil {
			t.Fatalf("%s: an uninstalled gate must not fail the call: %v", gate, err)
		}
		if r.Status != LintUnavailable || r.ExitCode != nil || len(r.Argv) != 0 || !strings.Contains(r.Reason, "not installed") {
			t.Fatalf("%s: %+v, want status unavailable, nothing run, the reason named", gate, r)
		}
	}
}

// With no packages the gate checks what the turn wrote; nothing written means
// nothing runs, in a result that cannot pass for a clean run.
func TestRunLint_DefaultsToTheWrittenPackages(t *testing.T) {
	t.Parallel()
	root := lintModule(t)
	ctx := tools.WithTestScope(shellWsCtx(root), func() []string { return []string{"./bad"} })
	out, err := executeRunLint(ctx, map[string]any{"gate": "vet"})
	if err != nil {
		t.Fatalf("run_lint: %v", err)
	}
	var r LintResult
	if err := json.Unmarshal([]byte(out), &r); err != nil {
		t.Fatal(err)
	}
	if r.Status != LintFindings || r.Scope != writtenScopeNote || r.Argv[len(r.Argv)-1] != "./bad" {
		t.Fatalf("%+v, want the written package ./bad vetted", r)
	}

	none := tools.WithTestScope(shellWsCtx(root), func() []string { return nil })
	out, err = executeRunLint(none, map[string]any{"gate": "vet"})
	if err != nil || !strings.Contains(out, `"status":"nothing_written"`) || strings.Contains(out, "exit_code") {
		t.Fatalf("nothing written: %s (err %v), want nothing_written with no exit code", out, err)
	}
}

// The model chooses a gate from a closed set and workspace-relative packages;
// anything else is refused before a process starts.
func TestRunLint_RefusesWhatIsNotAGateOrAPackage(t *testing.T) {
	t.Parallel()
	root := lintModule(t)
	for _, args := range []map[string]any{
		{"gate": "rm -rf"},
		{},
		{"gate": "vet", "packages": []any{"-vettool=/bin/sh"}},
		{"gate": "vet", "packages": []any{"../outside"}},
	} {
		if out, err := executeRunLint(shellWsCtx(root), args); err == nil {
			t.Errorf("args %v were accepted: %s", args, out)
		}
	}
}

// The gate runs under the workspace build env, and the result says which
// tags that was: on a tree with sqlite_headers (codeNERD's own), vet runs
// with -tags=sqlite_vec, reaches a file only that tag builds, and reports
// build_tags ["sqlite_vec"]. In ladder run R1-27 the model could not tell,
// called the tagged vet unverified, and ran an untagged one by hand.
func TestRunLint_ResultNamesTheBuildTagsItRanWith(t *testing.T) {
	t.Setenv("GOFLAGS", "")
	root := lintModule(t)
	for _, d := range []string{".git", "sqlite_headers"} {
		if err := os.MkdirAll(filepath.Join(root, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	tagged := "//go:build sqlite_vec\n\npackage tagged\n\nimport \"fmt\"\n\n// T prints n.\nfunc T(n string) { fmt.Printf(\"%d\\n\", n) }\n"
	if err := os.MkdirAll(filepath.Join(root, "tagged"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "tagged", "t.go"), []byte(tagged), 0o644); err != nil {
		t.Fatal(err)
	}
	r, err := runLint(t, root, map[string]any{"gate": "vet", "packages": []any{"./tagged"}})
	if err != nil {
		t.Fatalf("run_lint: %v (%+v)", err, r)
	}
	if len(r.BuildTags) != 1 || r.BuildTags[0] != "sqlite_vec" || !strings.Contains(r.GoFlags, "sqlite_vec") {
		t.Fatalf("build_tags = %v, goflags = %q; want the sqlite_vec tag the workspace env applies", r.BuildTags, r.GoFlags)
	}
	if r.Status != LintFindings || len(r.Findings) != 1 || r.Findings[0].File != "tagged/t.go" {
		t.Fatalf("the tagged file's finding was not reported: %+v", r)
	}
	out, _ := executeRunLint(shellWsCtx(root), map[string]any{"gate": "vet", "packages": []any{"./good"}})
	if !strings.Contains(out, `"build_tags":["sqlite_vec"]`) {
		t.Fatalf("the JSON result does not carry build_tags: %s", out)
	}
	if desc := RunLintTool().Description; !strings.Contains(desc, "build tags") || !strings.Contains(desc, "build_tags") {
		t.Fatalf("the tool's description does not say the workspace's build tags apply: %q", desc)
	}
}
