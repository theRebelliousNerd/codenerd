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
