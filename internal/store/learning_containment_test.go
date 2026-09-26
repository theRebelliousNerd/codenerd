package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A shard type names a file under the learnings directory, so it must never
// leave it. Until 2026-09-26 "../escape" wrote escape_learnings.db one level
// up; shard types reach Save from Dream consultations, which a model shapes.
func TestLearningStoreShardPathContainment(t *testing.T) {
	root := t.TempDir()
	base := filepath.Join(root, "shards")
	store, err := NewLearningStore(base)
	if err != nil {
		t.Fatalf("NewLearningStore: %v", err)
	}
	defer store.Close()

	// Real names, agents included, are contained -- folded to lower case,
	// because on a case-insensitive filesystem the two spellings are one file.
	for name, file := range map[string]string{
		"coder":          "coder_learnings.db",
		"test_architect": "test_architect_learnings.db",
		"worker-7":       "worker-7_learnings.db",
		"MangleExpert":   "mangleexpert_learnings.db",
	} {
		if err := store.Save(name, "preference", []any{"value"}, "test"); err != nil {
			t.Fatalf("Save(%q): %v", name, err)
		}
		if _, err := os.Stat(filepath.Join(base, file)); err != nil {
			t.Fatalf("Save(%q) did not write %s: %v", name, file, err)
		}
	}
	// One name in two spellings is one database.
	if err := store.Save("mangleexpert", "preference", []any{"other"}, "test"); err != nil {
		t.Fatal(err)
	}
	rows, err := store.Load("MangleExpert")
	if err != nil || len(rows) != 2 {
		t.Fatalf("Load(MangleExpert) = %d rows, %v; want both spellings' saves", len(rows), err)
	}

	for _, name := range []string{
		"", ".", "..", "../escape", `..\escape`, "/absolute", `C:\escape`,
		"space name", "colon:name", "dot.name", "é", strings.Repeat("a", 65),
	} {
		t.Run(name, func(t *testing.T) {
			if err := store.Save(name, "preference", []any{"value"}, "test"); err == nil {
				t.Fatalf("Save(%q) was accepted", name)
			}
		})
	}
	if _, err := os.Stat(filepath.Join(root, "escape_learnings.db")); !os.IsNotExist(err) {
		t.Fatalf("a traversal wrote outside the learnings directory: %v", err)
	}
}

// Close is terminal: a Save after it used to reopen a database no one would
// close again.
func TestLearningStoreCloseIsTerminal(t *testing.T) {
	store, err := NewLearningStore(filepath.Join(t.TempDir(), "shards"))
	if err != nil {
		t.Fatalf("NewLearningStore: %v", err)
	}
	if err := store.Save("coder", "preference", nil, "test"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := store.Save("coder", "preference", nil, "test"); err == nil {
		t.Fatal("Save after Close reopened the store")
	}
	if err := store.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
}
