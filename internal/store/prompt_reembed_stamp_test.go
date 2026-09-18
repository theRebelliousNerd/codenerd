package store

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

// The vector searcher accepts an atom only when prompt_atoms.embedding_model
// equals the engine name; NULL counts as unstamped and is skipped. Measured
// 2026-09-17: /embedding reembed rewrote all 914 vectors in
// .nerd/prompts/corpus.db and left every stamp NULL, so the very turn meant to
// validate the selector had nothing to rank. A re-embed must stamp what it
// writes.
func TestReembedAllPromptAtomsForce_StampsTheEngineOnEveryAtom(t *testing.T) {
	s := openLocalTestStore(t)
	for _, id := range []string{"go/concurrency/race_conditions", "identity/coder/mission"} {
		if err := s.StorePromptAtom(&PromptAtom{AtomID: id, Version: 1, Content: "content for " + id, Category: "test"}); err != nil {
			t.Fatalf("StorePromptAtom(%s): %v", id, err)
		}
	}
	s.SetEmbeddingEngine(&MockEmbeddingEngine{NameFunc: func() string { return "ollama/embeddinggemma:300m" }})

	n, err := s.ReembedAllPromptAtomsForce(context.Background())
	if err != nil {
		t.Fatalf("ReembedAllPromptAtomsForce: %v", err)
	}
	if n != 2 {
		t.Fatalf("re-embedded %d atoms, want 2", n)
	}

	rows, err := s.db.Query("SELECT atom_id, embedding_model FROM prompt_atoms")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()
	seen := 0
	for rows.Next() {
		var id string
		var stamp sql.NullString
		if err := rows.Scan(&id, &stamp); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen++
		if !stamp.Valid || stamp.String != "ollama/embeddinggemma:300m" {
			t.Errorf("%s: embedding_model = %#v after re-embed, want the engine name; the searcher will skip this atom as unstamped", id, stamp)
		}
	}
	if seen != 2 {
		t.Errorf("saw %d prompt_atoms rows, want 2", seen)
	}
}

// A database created before the column existed (the shipped seed corpora and
// .nerd/knowledge.db have no embedding_model column) must get the column and
// the stamp rather than a failed UPDATE.
func TestReembedAllPromptAtomsForce_AddsTheStampColumnToALegacyDB(t *testing.T) {
	path := filepath.Join(t.TempDir(), "legacy.db")
	raw, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	stmts := []string{
		"CREATE TABLE prompt_atoms (id INTEGER PRIMARY KEY AUTOINCREMENT, atom_id TEXT NOT NULL UNIQUE, version INTEGER, content TEXT, description TEXT, embedding BLOB, embedding_task TEXT)",
		"INSERT INTO prompt_atoms (atom_id, version, content) VALUES ('legacy/atom', 1, 'old corpus row')",
	}
	for _, q := range stmts {
		if _, err := raw.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if err := raw.Close(); err != nil {
		t.Fatalf("close raw: %v", err)
	}

	s, err := NewLocalStore(path)
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer func() { _ = s.Close() }()
	s.SetEmbeddingEngine(&MockEmbeddingEngine{NameFunc: func() string { return "ollama/embeddinggemma:300m" }})

	if _, err := s.ReembedAllPromptAtomsForce(context.Background()); err != nil {
		t.Fatalf("ReembedAllPromptAtomsForce on a legacy DB: %v", err)
	}
	var stamp sql.NullString
	if err := s.db.QueryRow("SELECT embedding_model FROM prompt_atoms WHERE atom_id = 'legacy/atom'").Scan(&stamp); err != nil {
		t.Fatalf("the legacy DB still has no usable embedding_model column: %v", err)
	}
	if !stamp.Valid || stamp.String != "ollama/embeddinggemma:300m" {
		t.Errorf("legacy atom stamp = %#v, want the engine name", stamp)
	}
}
