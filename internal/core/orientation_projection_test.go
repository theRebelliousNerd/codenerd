package core

import (
	"os"
	"path/filepath"
	"testing"
)

// The orientation projection is refreshed at runtime by exact retracts, so its
// rows must boot as EDB facts. Compiled into the program they were permanent:
// a refreshed HEAD stacked next to the stale one. The boot-to-refresh path is
// covered end to end by cmd/nerd/chat TestRunScanAppliesOrientationWhenWorldIsUnchanged.
func TestOrientationProjectionSplitsDeclsFromRows(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, ".nerd", "orientation")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	projection := "# Generated orientation projection.\n" +
		"Decl oriented_head(Head) bound [/string].\n" +
		"oriented_head(\"old\").\n"
	if err := os.WriteFile(filepath.Join(dir, "orientation.mg"), []byte(projection), 0o644); err != nil {
		t.Fatal(err)
	}

	decls, facts, err := loadOrientationProjection(filepath.Join(dir, "orientation.mg"))
	if err != nil {
		t.Fatal(err)
	}
	if decls != "Decl oriented_head(Head) bound [/string].\n" || len(facts) != 1 || facts[0].Predicate != "oriented_head" {
		t.Fatalf("decls=%q facts=%v", decls, facts)
	}

}

func TestOrientationProjectionRejectsNonFactRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "orientation.mg")
	body := "Decl oriented_head(Head) bound [/string].\noriented_head(H) :- current_repo_head(H).\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadOrientationProjection(path); err == nil {
		t.Fatal("a rule in the projection must be refused, not dropped")
	}
}
