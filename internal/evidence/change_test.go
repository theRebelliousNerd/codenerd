package evidence

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func fixture(t *testing.T) (string, Contract) {
	t.Helper()
	root := t.TempDir()
	write := func(name, s string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, name), []byte(s), 0600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module fixture\n\ngo 1.23\n")
	write("add.go", "package fixture\nfunc Add(a,b int) int { return a-b }\n")
	write("add_test.go", "package fixture\nimport \"testing\"\nfunc TestAdd(t *testing.T) { if Add(2,3)!=5 { t.Fatal(\"wrong sum\") } }\nfunc TestZero(t *testing.T) { if Add(0,0)!=0 { t.Fatal(\"zero\") } }\n")
	return root, Contract{Task: "Fix addition", Authority: "user regression contract", Obligations: []Obligation{
		{ID: "sum", Description: "add distinct positive values", Package: ".", Test: "TestAdd", TestFile: "add_test.go", Reproducer: true},
		{ID: "zero", Description: "preserve zero", Package: ".", Test: "TestZero", TestFile: "add_test.go"},
	}}
}

func TestChangeCarriesExecutedRevisionBoundEvidence(t *testing.T) {
	root, c := fixture(t)
	ctx := context.Background()
	tx, err := Begin(ctx, root, c)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "add.go"), []byte("package fixture\nfunc Add(a,b int) int { return a+b }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	// Production boot changes this process-global hint. The transaction must
	// still execute both baselines and witnesses with its pinned environment.
	t.Setenv("CODENERD_WORKSPACE_ROOT", filepath.Join(root, "another-workspace"))
	r := tx.Verify(ctx)
	if r.Status != "verified" || r.Before == r.After || len(r.Witnesses) != 2 || r.Baseline[0].Status != "failed" {
		t.Fatalf("missing evidence: %+v", r)
	}
	for _, w := range r.Witnesses {
		if w.Snapshot != r.After || w.Toolchain == "" || w.OutputHash == "" {
			t.Fatalf("incomplete witness: %+v", w)
		}
	}
	if _, err := Persist(root, r); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bypass_test.go"), []byte("package fixture\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if added := tx.Verify(ctx); added.Status == "verified" {
		t.Fatal("unreviewed test addition accepted")
	}
	if err := os.Remove(filepath.Join(root, "bypass_test.go")); err != nil {
		t.Fatal(err)
	}
	// A later defect cannot inherit the earlier green witnesses.
	if err := os.WriteFile(filepath.Join(root, "add.go"), []byte("package fixture\nfunc Add(a,b int) int { return a-b }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if stale := tx.Verify(ctx); stale.Status == "verified" {
		t.Fatal("stale green evidence survived an edit")
	}
	// Weakening the caller's test cannot close the contract either.
	if err := os.WriteFile(filepath.Join(root, "add_test.go"), []byte("package fixture\nimport \"testing\"\nfunc TestAdd(t *testing.T) {}\nfunc TestZero(t *testing.T) {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if weakened := tx.Verify(ctx); weakened.Status == "verified" {
		t.Fatal("weakened acceptance accepted")
	}
}

func TestMissingAndSkippedTestsAreNotWitnesses(t *testing.T) {
	root, c := fixture(t)
	c.Obligations[0].Test = "TestDoesNotExist"
	if _, err := Begin(t.Context(), root, c); err == nil {
		t.Fatal("absent reproducer accepted")
	}
	c.Obligations[0].Test = "TestAdd"
	if err := os.WriteFile(filepath.Join(root, "add_test.go"), []byte("package fixture\nimport \"testing\"\nfunc TestAdd(t *testing.T) { t.Skip(\"not evidence\") }\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Begin(t.Context(), root, c); err == nil {
		t.Fatal("skipped reproducer accepted")
	}
}

func TestSnapshotTracksDirtyUntrackedAndConfiguration(t *testing.T) {
	root, _ := fixture(t)
	before, err := Snapshot(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "untracked.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	after, err := Snapshot(t.Context(), root)
	if err != nil || before == after {
		t.Fatal("untracked file omitted")
	}
	if err := os.Mkdir(filepath.Join(root, ".nerd"), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".nerd", "config.json"), []byte(`{"changed":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	configured, err := Snapshot(t.Context(), root)
	if err != nil || configured == after {
		t.Fatal("configuration omitted")
	}
}
