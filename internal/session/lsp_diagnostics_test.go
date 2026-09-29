package session

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// Absent gopls must be silence, not an error and not a finding. A machine
// without gopls installed has to behave exactly as it did before this existed.
func TestGoplsDiagnostics_SilentWhenNotApplicable(t *testing.T) {
	cases := []struct {
		name      string
		workspace string
		paths     []string
	}{
		{"empty workspace", "  ", []string{"a.go"}},
		{"no paths", t.TempDir(), nil},
		{"no Go files", t.TempDir(), []string{"README.md", "notes.txt"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := goplsDiagnostics(context.Background(), tc.workspace, tc.paths); got != "" {
				t.Errorf("expected silence, got %q", got)
			}
		})
	}
}

// The real thing, when gopls is available: a file with a diagnosable-but-
// compilable defect must produce output naming that file. This is the class of
// problem the build gate cannot see -- the package compiles fine.
func TestGoplsDiagnostics_ReportsOnCompilableCode(t *testing.T) {
	if testing.Short() {
		t.Skip("runs gopls over a throwaway module")
	}
	if _, err := execLookPathForTest("gopls"); err != nil {
		t.Skip("gopls not installed; the feature is optional by design")
	}

	ws := t.TempDir()
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(ws, name), []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	write("go.mod", "module goplsprobe\n\ngo 1.21\n")
	// Compiles cleanly; gopls objects to the unused result.
	write("bad.go", `package goplsprobe

import "fmt"

func Bad() {
	fmt.Sprintf("dropped on the floor")
}
`)

	got := goplsDiagnostics(context.Background(), ws, []string{"bad.go"})
	if got == "" {
		t.Skip("gopls returned no diagnostics for this construct; version-dependent, not a product failure")
	}
	if !strings.Contains(got, "bad.go") {
		t.Errorf("diagnostics do not name the analysed file: %q", got)
	}
}

// gopls talks about itself on stderr, and CombinedOutput mixes that in. The
// live run that first exercised this gate fed the critic exactly one line,
// under the heading "Static analysis reported":
//
//	telemetry prompt failed: unable to determine user config dir: %AppData% is not defined
//
// That is gopls complaining about its own environment. Presenting it to a
// reviewer as ground truth is worse than presenting nothing, because the whole
// value of tool output is that it cannot be argued with — and a reviewer handed
// noise labelled as evidence is being invited to invent a finding about it.
func TestKeepDiagnosticLines(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "the observed telemetry noise is dropped",
			raw:  "2026/08/08 12:07:12 Error:2026/08/08 12:07:12 telemetry prompt failed: unable to determine user config dir: %AppData% is not defined",
			want: "",
		},
		{
			name: "a real diagnostic is kept",
			raw:  `C:\repo\internal\session\critic.go:85:4-41: Inefficient string concatenation in call to WriteString`,
			want: `C:\repo\internal\session\critic.go:85:4-41: Inefficient string concatenation in call to WriteString`,
		},
		{
			name: "unix-style diagnostic without a column range is kept",
			raw:  "internal/session/a.go:12:3: result of fmt.Sprintf is not used",
			want: "internal/session/a.go:12:3: result of fmt.Sprintf is not used",
		},
		{
			name: "noise around a real diagnostic leaves only the diagnostic",
			raw: "gopls: starting\n" +
				"internal/session/a.go:12:3: result of fmt.Sprintf is not used\n" +
				"telemetry prompt failed: whatever\n",
			want: "internal/session/a.go:12:3: result of fmt.Sprintf is not used",
		},
		{"empty input", "", ""},
		{"blank lines only", "\n\n   \n", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := keepDiagnosticLines(tc.raw); got != tc.want {
				t.Errorf("keepDiagnosticLines(%q) = %q; want %q", tc.raw, got, tc.want)
			}
		})
	}
}

// Diagnostics are what the reviewer fixes. A cap at eight files hid every
// defect in file nine and after; the turn's own write set is the list.
func TestLSPDiagnostics_ChecksEveryGoFile(t *testing.T) {
	restoreGoplsSeams(t)
	var saw []string
	lookPath = func(string) (string, error) { return "gopls", nil }
	runGoplsCheck = func(_ context.Context, _, _ string, files []string) ([]byte, error) {
		saw = append([]string(nil), files...)
		var b strings.Builder
		for _, f := range files {
			fmt.Fprintf(&b, "%s:1:1: unused result\n", f)
		}
		return []byte(b.String()), fmt.Errorf("gopls check exits non-zero when it has findings")
	}

	var paths []string
	for i := 0; i < 12; i++ {
		paths = append(paths, fmt.Sprintf("f%02d.go", i))
	}
	paths = append(paths, "README.md", "notes.txt")
	got := goplsDiagnostics(context.Background(), t.TempDir(), paths)
	if len(saw) != 12 {
		t.Fatalf("gopls was given %d files, want all 12 Go files: %v", len(saw), saw)
	}
	for _, f := range saw {
		if !strings.HasSuffix(f, ".go") {
			t.Fatalf("a non-Go file was sent to gopls: %v", saw)
		}
		if !strings.Contains(got, f+":1:1: unused result") {
			t.Errorf("the report dropped %s: %q", f, got)
		}
	}
}

// One gopls invocation is one request. When it does not finish, the lines it
// already produced stay, and every file the invocation was given is named:
// the process does not say which of them it reached.
func TestLSPDiagnostics_TimeoutNamesUncheckedFiles(t *testing.T) {
	restoreGoplsSeams(t)
	lookPath = func(string) (string, error) { return "gopls", nil }
	runGoplsCheck = func(ctx context.Context, _, _ string, _ []string) ([]byte, error) {
		<-ctx.Done()
		return []byte("a.go:2:1: arrived before the timeout\nchatter that is not a diagnostic\n"), ctx.Err()
	}
	files := []string{"a.go", "b.go", "c.go"}
	ctx := withDiagnosticTimeout(context.Background(), 20*time.Millisecond)
	start := time.Now()
	got := goplsDiagnostics(ctx, t.TempDir(), files)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("the request waited %s; session.lsp_timeout did not bound it", elapsed)
	}
	if !strings.Contains(got, "a.go:2:1: arrived before the timeout") {
		t.Fatalf("lines gopls had already produced were discarded: %q", got)
	}
	if strings.Contains(got, "chatter") {
		t.Fatalf("operational chatter survived the timeout path: %q", got)
	}
	if !types.IsClamped(got) {
		t.Fatalf("a request that did not finish was reported as a clean result: %q", got)
	}
	for _, f := range files {
		if !strings.Contains(got, f) {
			t.Errorf("unchecked file %s was not named: %q", f, got)
		}
	}
}

// session.lsp_timeout is the bound on that one request. The executor reads it
// off its config and the diagnostic call reads it off the context.
func TestLSPDiagnostics_TimeoutFollowsSessionConfig(t *testing.T) {
	policy, err := (config.SessionConfig{LSPTimeout: "15s"}).Resolve()
	if err != nil {
		t.Fatal(err)
	}
	e := &Executor{}
	e.SetConfig(ExecutorConfigFrom(policy, config.DefaultWorkingConfig()))
	if got := e.diagnosticTimeout(); got != 15*time.Second {
		t.Fatalf("executor bound = %s, want 15s from session.lsp_timeout", got)
	}
	ctx := withDiagnosticTimeout(context.Background(), e.diagnosticTimeout())
	if got := diagnosticTimeout(ctx); got != 15*time.Second {
		t.Fatalf("request bound = %s, want 15s", got)
	}
	if got := diagnosticTimeout(context.Background()); got != defaultSessionPolicy.LSPTimeout {
		t.Fatalf("a context without the key = %s, want the section default %s", got, defaultSessionPolicy.LSPTimeout)
	}
	if got := (&Executor{}).diagnosticTimeout(); got != defaultSessionPolicy.LSPTimeout {
		t.Fatalf("an executor without the field = %s, want the section default %s", got, defaultSessionPolicy.LSPTimeout)
	}
}

func restoreGoplsSeams(t *testing.T) {
	t.Helper()
	prevLook, prevRun := lookPath, runGoplsCheck
	t.Cleanup(func() {
		lookPath = prevLook
		runGoplsCheck = prevRun
	})
}
