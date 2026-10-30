package session

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"codenerd/internal/gates"
)

// Gate ownership follows the change that remains (turn_write_reverted), and
// so does the workspace test plan: a Rust file the turn edited and restored
// byte for byte owes no Rust test. The planner used to read the raw write
// history, so a restored file put an unavailable language-qualified check in
// the plan beside a passing Python one.
func TestWorkspaceTestPlan_ARestoredFileOwesNoTestOfItsLanguage(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	files := map[string]string{"svc/app.py": "def f():\n    return 2\n", "core/lib.rs": "pub fn f() {}\n"}
	for rel, body := range files {
		p := filepath.Join(ws, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	result := &ExecutionResult{
		WrittenPaths: []string{"svc/app.py", "core/lib.rs"},
		PreWriteContents: map[string]PreImage{
			"svc/app.py":  {Existed: true, Content: "def f():\n    return 1\n"}, // changed
			"core/lib.rs": {Existed: true, Content: "pub fn f() {}\n"},          // restored
		},
	}
	if got := netWrittenPaths(ws, result); !slices.Equal(got, []string{"svc/app.py"}) {
		t.Fatalf("net written paths = %v, want only the changed Python file", got)
	}
	if !slices.Equal(result.WrittenPaths, []string{"svc/app.py", "core/lib.rs"}) {
		t.Fatal("the write history was rewritten")
	}

	py := gates.Gate{ID: "python:pytest", Kind: gates.Test, Argv: []string{"pytest", gates.NodeToken}, Scope: gates.ScopeNode, Language: "python"}
	rust := gates.Gate{ID: "rust:test", Kind: gates.Test, Argv: []string{"cargo", "test"}, Scope: gates.ScopeAll, Language: "rust"}
	set := gates.Set{Gates: []gates.Gate{py}, Unavailable: []gates.Unavailable{{Gate: rust, Reason: "cargo is not on PATH"}}}

	langs, dirs := writtenNonGo(ws, netWrittenPaths(ws, result))
	plan := buildTestPlan(set, langs, dirs)
	for _, id := range planIDs(plan) {
		if id == "rust:test@." || id == "uncovered:rust" {
			t.Fatalf("plan %v owes a Rust check for a file restored to its preimage", planIDs(plan))
		}
	}
	// Control: a Rust change that remains keeps its unavailable check.
	if err := os.WriteFile(filepath.Join(ws, "core", "lib.rs"), []byte("pub fn g() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	langs, dirs = writtenNonGo(ws, netWrittenPaths(ws, result))
	if ids := planIDs(buildTestPlan(set, langs, dirs)); !slices.Contains(ids, "rust:test@.") {
		t.Fatalf("plan %v dropped the unavailable Rust check of a remaining Rust change", ids)
	}
}
