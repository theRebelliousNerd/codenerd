package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAddEdit_PendingMutationStoresContentIdentity(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	oldBody := []byte(strings.Repeat("A", 250) + "OLD")
	newBody := []byte(strings.Repeat("A", 250) + "NEW")
	if err := os.WriteFile(path, oldBody, 0644); err != nil {
		t.Fatal(err)
	}

	kernel, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	tm := NewTransactionManager(kernel, dir)
	if _, err := tm.Begin(context.Background(), "identity"); err != nil {
		t.Fatal(err)
	}
	if err := tm.AddEdit(context.Background(), FileEdit{
		FilePath: path,
		Content:  newBody,
		EditType: EditTypeModify,
	}); err != nil {
		t.Fatal(err)
	}

	facts, err := kernel.Query("pending_mutation")
	if err != nil {
		t.Fatal(err)
	}
	if len(facts) != 1 {
		t.Fatalf("pending_mutation facts = %d, want 1", len(facts))
	}
	if len(facts[0].Args) != 4 {
		t.Fatalf("arity = %d, want 4", len(facts[0].Args))
	}
	gotOld, _ := facts[0].Args[2].(string)
	gotNew, _ := facts[0].Args[3].(string)
	wantOld := factContentIdentity(string(oldBody))
	wantNew := factContentIdentity(string(newBody))
	if gotOld != wantOld || gotNew != wantNew {
		t.Fatalf("old=%q new=%q, want old=%q new=%q", gotOld, gotNew, wantOld, wantNew)
	}
	if gotOld == gotNew {
		t.Fatal("old and new bodies that share a 250-byte prefix collapsed")
	}
	if strings.Contains(gotOld, "...") || strings.HasPrefix(gotOld, "AAA") {
		t.Fatalf("old content is a prefix: %q", gotOld)
	}
}
