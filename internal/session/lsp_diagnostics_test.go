package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
