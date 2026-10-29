package prompt

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core/defaults"

	_ "github.com/mattn/go-sqlite3"
)

// TestPromptCorpusSeedMatchesEmbeddedYAML pins the shipped first-boot seed
// internal/core/defaults/prompt_corpus.db to the embedded YAML corpus: every
// YAML atom is in the seed with identical content, and the seed holds no
// removed atom. It had drifted to 878 atoms against 917 (2026-09-28), nine of
// them deleted atoms, and nothing noticed. Rebuild with
// `go run ./cmd/tools/prompt_builder -skip-embeddings -output <tmp>` and carry
// embedding/embedding_task over for atoms whose embedding input is unchanged
// (the reconciler's retention rule); a vectorless seed degrades first-boot
// vector search to lexical. The predicate corpus equivalent is
// `predicate_corpus_builder -check`.
func TestPromptCorpusSeedMatchesEmbeddedYAML(t *testing.T) {
	if !defaults.PromptCorpusAvailable() {
		t.Skip("no embedded prompt corpus seed")
	}
	data, err := defaults.PromptCorpusDB.ReadFile("prompt_corpus.db")
	if err != nil {
		t.Fatalf("read embedded seed: %v", err)
	}
	dbPath := filepath.Join(t.TempDir(), "prompt_corpus.db")
	if err := os.WriteFile(dbPath, data, 0o644); err != nil {
		t.Fatalf("materialize seed: %v", err)
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatalf("open seed: %v", err)
	}
	defer db.Close()

	seed := make(map[string]string)
	rows, err := db.Query(`SELECT atom_id, content FROM prompt_atoms`)
	if err != nil {
		t.Fatalf("read seed atoms: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		var id, content string
		if err := rows.Scan(&id, &content); err != nil {
			t.Fatalf("scan seed atom: %v", err)
		}
		seed[id] = content
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate seed atoms: %v", err)
	}

	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	for _, a := range corpus.All() {
		got, ok := seed[a.ID]
		if !ok {
			t.Errorf("seed lacks YAML atom %s", a.ID)
			continue
		}
		if got != a.Content {
			t.Errorf("seed content for %s differs from YAML", a.ID)
		}
		delete(seed, a.ID)
	}
	for id := range seed {
		t.Errorf("seed holds removed atom %s", id)
	}
}
