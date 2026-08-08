package store

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *LocalStore {
	t.Helper()
	s, err := NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// The defect these guard (F-KB-1, observed live): `nerd knowledge` printed
// "No knowledge entries found." on a workspace where `nerd knowledge search`
// answered every query from the same database file.
//
// The dual store has two halves that diverge. StoreKnowledgeAtom writes the
// knowledge_atoms table; StoreKnowledgeAtomWithEmbedding also writes vectors.
// In this workspace .nerd/knowledge.db held 0 rows in knowledge_atoms and
// 1,417 in vectors — every one content_type=knowledge_atom — because the
// unembedded atoms had been written to per-shard databases instead. The lister
// read knowledge_atoms, and additionally filtered on a "session/" concept
// prefix that no atom the system writes ever uses.

func seedVector(t *testing.T, s *LocalStore, content string, meta map[string]any, created time.Time) {
	t.Helper()
	raw, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if _, err := s.db.Exec(
		`INSERT INTO vectors (content, embedding, metadata, created_at) VALUES (?, ?, ?, ?)`,
		content, []byte{}, string(raw), created,
	); err != nil {
		t.Fatalf("seed vector: %v", err)
	}
}

func TestRecentKnowledgeAtoms_ReadsTheVectorStore(t *testing.T) {
	s := newTestStore(t)

	base := time.Now().Add(-time.Hour)
	seedVector(t, s, "predicate mg_decl/2 — Mangle declaration",
		map[string]any{"content_type": "knowledge_atom", "concept": "mg_decl", "confidence": 0.9}, base)

	atoms, err := s.RecentKnowledgeAtoms(10)
	if err != nil {
		t.Fatalf("RecentKnowledgeAtoms: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("got %d atoms, want 1 — the lister must read the half of the dual store the live path uses", len(atoms))
	}
	if atoms[0].Concept != "mg_decl" {
		t.Errorf("concept = %q, want mg_decl", atoms[0].Concept)
	}
	if atoms[0].Confidence != 0.9 {
		t.Errorf("confidence = %v, want 0.9 from metadata", atoms[0].Confidence)
	}
}

// The vectors table holds more than knowledge atoms. Listing everything would
// mix file chunks and reasoning traces into the knowledge report.
func TestRecentKnowledgeAtoms_ExcludesOtherContentTypes(t *testing.T) {
	s := newTestStore(t)

	now := time.Now()
	seedVector(t, s, "a knowledge atom", map[string]any{"content_type": "knowledge_atom"}, now)
	seedVector(t, s, "a file chunk", map[string]any{"content_type": "file_chunk"}, now)
	seedVector(t, s, "untyped entry", map[string]any{}, now)

	atoms, err := s.RecentKnowledgeAtoms(10)
	if err != nil {
		t.Fatalf("RecentKnowledgeAtoms: %v", err)
	}
	if len(atoms) != 1 {
		t.Fatalf("got %d atoms, want only the knowledge_atom entry: %+v", len(atoms), atoms)
	}
}

// Newest first, and the limit is a limit on atoms returned — not on rows
// scanned, or a burst of non-atom vectors would starve the listing.
func TestRecentKnowledgeAtoms_NewestFirstWithinLimit(t *testing.T) {
	s := newTestStore(t)

	base := time.Now().Add(-24 * time.Hour)
	for i := range 6 {
		seedVector(t, s, "atom "+string(rune('a'+i)),
			map[string]any{"content_type": "knowledge_atom", "concept": string(rune('a' + i))},
			base.Add(time.Duration(i)*time.Minute))
	}
	// Interleaved noise that must not consume the limit.
	for i := range 20 {
		seedVector(t, s, "noise", map[string]any{"content_type": "file_chunk"},
			base.Add(time.Duration(i)*time.Second))
	}

	atoms, err := s.RecentKnowledgeAtoms(3)
	if err != nil {
		t.Fatalf("RecentKnowledgeAtoms: %v", err)
	}
	if len(atoms) != 3 {
		t.Fatalf("got %d atoms, want 3", len(atoms))
	}
	if atoms[0].Concept != "f" {
		t.Errorf("first atom concept = %q, want the newest (f)", atoms[0].Concept)
	}
}

func TestRecentKnowledgeAtoms_EmptyStoreReturnsNothing(t *testing.T) {
	s := newTestStore(t)

	atoms, err := s.RecentKnowledgeAtoms(10)
	if err != nil {
		t.Fatalf("an empty store must not be an error: %v", err)
	}
	if len(atoms) != 0 {
		t.Errorf("got %d atoms from an empty store", len(atoms))
	}
}
