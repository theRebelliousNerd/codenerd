package world

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/store"
)

func snapshotFacts(t testing.TB, root string, files int) []core.Fact {
	t.Helper()
	var facts []core.Fact
	for i := 0; i < files; i++ {
		path := filepath.Join(root, fmt.Sprintf("f%d.go", i))
		if err := os.WriteFile(path, []byte("package p\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		facts = append(facts,
			core.Fact{Predicate: "file_topology", Args: []any{path, fmt.Sprintf("hash%d", i), core.MangleAtom("/go"), int64(1), core.MangleAtom("/false")}},
			core.Fact{Predicate: "dependency_link", Args: []any{path, "core.go", "import"}},
		)
	}
	return facts
}

// A full snapshot is written whole: every file's facts land, and a store that
// cannot take the write is reported to the caller (it used to be logged per
// file and the snapshot reported success).
func TestPersistFastSnapshotToDB_WritesWholeSnapshotOrReports(t *testing.T) {
	root := t.TempDir()
	db, err := store.NewLocalStore(filepath.Join(t.TempDir(), "world.db"))
	if err != nil {
		t.Fatal(err)
	}
	facts := snapshotFacts(t, root, 300)
	if err := PersistFastSnapshotToDBInRoot(db, root, facts); err != nil {
		t.Fatal(err)
	}
	loaded, err := db.LoadAllWorldFacts("fast")
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != len(facts) {
		t.Fatalf("loaded %d world facts, wrote %d", len(loaded), len(facts))
	}

	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if err := PersistFastSnapshotToDBInRoot(db, root, facts); err == nil {
		t.Fatal("a snapshot the store could not write was reported as persisted")
	}
}

// BenchmarkPersistFastSnapshotToDB writes a 2,000-file snapshot.
func BenchmarkPersistFastSnapshotToDB(b *testing.B) {
	root := b.TempDir()
	facts := snapshotFacts(b, root, 2000)
	db, err := store.NewLocalStore(filepath.Join(b.TempDir(), "world.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer db.Close()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := PersistFastSnapshotToDBInRoot(db, root, facts); err != nil {
			b.Fatal(err)
		}
	}
}
