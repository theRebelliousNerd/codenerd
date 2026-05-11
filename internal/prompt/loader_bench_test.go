package prompt

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"testing"
	"math/rand"

	_ "github.com/mattn/go-sqlite3"
)

func BenchmarkStoreAtomsWithEmbeddings(b *testing.B) {
	// Setup a temporary database
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench.db")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		b.Fatalf("Failed to open db: %v", err)
	}
	defer db.Close()

	// Initialize schema
	_, err = db.Exec(`
		CREATE TABLE IF NOT EXISTS prompt_atoms (
			atom_id TEXT PRIMARY KEY,
			version INTEGER,
			content TEXT,
			token_count INTEGER,
			content_hash TEXT,
			description TEXT,
			content_concise TEXT,
			content_min TEXT,
			category TEXT,
			subcategory TEXT,
			priority INTEGER,
			is_mandatory BOOLEAN,
			is_exclusive TEXT,
			embedding BLOB,
			embedding_task TEXT,
			source_file TEXT
		);
		CREATE TABLE IF NOT EXISTS atom_context_tags (
			atom_id TEXT,
			dimension TEXT,
			tag TEXT,
			FOREIGN KEY(atom_id) REFERENCES prompt_atoms(atom_id) ON DELETE CASCADE
		);
	`)
	if err != nil {
		b.Fatalf("Failed to create schema: %v", err)
	}

	// Generate test data
	numAtoms := 1000
	atoms := make([]*PromptAtom, numAtoms)
	embeddings := make([][]float32, numAtoms)

	for i := 0; i < numAtoms; i++ {
		atoms[i] = &PromptAtom{
			ID:          fmt.Sprintf("atom_%d", i),
			Version:     1,
			Content:     fmt.Sprintf("This is the content for atom %d.", i),
			TokenCount:  rand.Intn(100),
			ContentHash: fmt.Sprintf("hash_%d", i),
			Category:    AtomCategory("test"),
			OperationalModes: []string{"active", "debugging"},
		}

		emb := make([]float32, 1536)
		for j := 0; j < 1536; j++ {
			emb[j] = rand.Float32()
		}
		embeddings[i] = emb
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := storeAtomsWithEmbeddings(ctx, db, atoms, embeddings, "retrieval_document")
		if err != nil {
			b.Fatalf("Failed to store atoms: %v", err)
		}
	}
}
