package sync

import (
	"context"
	"path/filepath"
	"testing"
)

// An agent KB's boot heal sent a zero-byte ecosystem paragraph to the embedder
// on every boot. A row with no text is never embedded; rows with text are.
func TestBackfill_ARowWithNoTextIsNeverEmbedded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "goexpert_knowledge.db")
	db := openSeededTestDB(t, path)
	insertKnowledgeAtom(t, db, "ecosystem paragraph 888/1141 bytes=0", "", 0.95)
	insertKnowledgeAtom(t, db, "ecosystem paragraph 889/1141", "  \n", 0.95)
	insertKnowledgeAtom(t, db, "guide/errors", "Wrap errors with %w so callers can match them.", 0.9)
	eng := &fakeBackfillEngine{}
	if err := backfillKnowledgeVectors(context.Background(), db, path, eng); err != nil {
		t.Fatal(err)
	}
	if eng.calls != 1 || vectorCount(t, db) != 1 {
		t.Fatalf("embedder called %d time(s), %d vector(s); want 1 and 1", eng.calls, vectorCount(t, db))
	}
	if knowledgeCount(t, db) != 3 {
		t.Fatal("a blank row was dropped; it is kept, only not embedded")
	}
}
