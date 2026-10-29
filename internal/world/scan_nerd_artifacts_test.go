package world

import (
	"context"
	"strings"
	"testing"
)

// TestScanner_WhenNerdDirIsNested_ShouldNotIngestCrashDumps
//
// A failed kernel evaluation dumps the combined Mangle program to
// <cwd>/.nerd/debug/debug_program_ERROR.mg. Running the core package's tests
// makes that cwd a package directory, so the repository grew a stray
// internal/core/.nerd/debug/debug_program_ERROR.mg — a 700 KB crash artifact
// sitting inside the scanned source tree. When such a file is ingested it
// asserts symbol facts for every predicate it contains, which duplicates real
// declarations in the knowledge graph and makes a crash dump look like source.
//
// The stray file has been removed. This pins the property that keeps it from
// mattering: .nerd is skipped at ANY depth, not just at the workspace root.
func TestScanner_WhenNerdDirIsNested_ShouldNotIngestCrashDumps(t *testing.T) {
	root := t.TempDir()
	writeWorkspaceFile(t, root, "internal/core/kernel.go", "package core\n\nfunc Run() {}\n")
	writeWorkspaceFile(t, root, "internal/core/.nerd/debug/debug_program_ERROR.mg",
		"Decl panic_state(A, B) bound [/string, /string].\npanic_state(\"x\", \"y\").\n")
	writeWorkspaceFile(t, root, ".nerd/debug/debug_program_ERROR.mg",
		"Decl panic_state(A, B) bound [/string, /string].\n")

	facts, err := NewScanner().ScanWorkspaceCtx(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range facts {
		for _, a := range f.Args {
			s, ok := a.(string)
			if !ok {
				continue
			}
			if strings.Contains(s, ".nerd/") || strings.Contains(s, "debug_program_ERROR") {
				t.Errorf("%s fact references a .nerd artifact (%q); crash dumps must never enter the world model", f.Predicate, s)
			}
		}
	}
}
