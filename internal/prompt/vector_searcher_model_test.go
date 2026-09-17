package prompt

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"testing"
)

type modelFakeEngine struct {
	name       string
	vec        []float32
	batchCalls int
}

func (f *modelFakeEngine) Name() string { return f.name }

func (f *modelFakeEngine) Dimensions() int { return len(f.vec) }

func (f *modelFakeEngine) Embed(_ context.Context, _ string) ([]float32, error) {
	return append([]float32(nil), f.vec...), nil
}

func (f *modelFakeEngine) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	f.batchCalls++
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = append([]float32(nil), f.vec...)
	}
	return out, nil
}

func TestVectorSearch_IgnoresVectorsFromAnotherModel(t *testing.T) {
	queryVec := []float32{1, 0, 0}
	engine := &modelFakeEngine{name: "fake:current", vec: queryVec}

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "corpus.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := NewAtomLoader(engine).EnsureSchema(t.Context(), db); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}

	blob := encodeFloat32Slice(queryVec)
	if _, err := db.Exec(
		`INSERT INTO prompt_atoms (atom_id, content, token_count, content_hash, category, embedding, embedding_model) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"test/match", "match content", 1, "hash-match", "test", blob, engine.Name(),
	); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if _, err := db.Exec(
		`INSERT INTO prompt_atoms (atom_id, content, token_count, content_hash, category, embedding, embedding_model) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"test/other-model", "other content", 1, "hash-other", "test", blob, "ollama:other-model",
	); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}

	search := NewCompilerVectorSearcher(engine)
	compiler, err := NewJITPromptCompiler(WithProjectDB(db), WithVectorSearcher(search))
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	search.SetCompiler(compiler)

	results, err := search.Search(t.Context(), "refactoring query", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("Search returned %d results, want exactly 1: %v", len(results), results)
	}
	if results[0].AtomID != "test/match" {
		t.Fatalf("Search returned atom %q, want %q", results[0].AtomID, "test/match")
	}

	skipped := search.LastSkipped()
	if len(skipped) != 1 || skipped["ollama:other-model"] != 1 {
		t.Fatalf("LastSkipped() = %v, want map[ollama:other-model:1]", skipped)
	}
}

func TestVectorSearch_UnstampedVectorsAreSkipped(t *testing.T) {
	queryVec := []float32{1, 0, 0}
	engine := &modelFakeEngine{name: "fake:current", vec: queryVec}

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "corpus.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := NewAtomLoader(engine).EnsureSchema(t.Context(), db); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}

	blob := encodeFloat32Slice(queryVec)
	if _, err := db.Exec(
		`INSERT INTO prompt_atoms (atom_id, content, token_count, content_hash, category, embedding, embedding_model) VALUES (?, ?, ?, ?, ?, ?, NULL)`,
		"test/unstamped", "unstamped content", 1, "hash-unstamped", "test", blob,
	); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}

	search := NewCompilerVectorSearcher(engine)
	compiler, err := NewJITPromptCompiler(WithProjectDB(db), WithVectorSearcher(search))
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	search.SetCompiler(compiler)

	results, err := search.Search(t.Context(), "refactoring query", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Search returned %d results, want 0: %v", len(results), results)
	}

	skipped := search.LastSkipped()
	if len(skipped) != 1 || skipped["unstamped"] != 1 {
		t.Fatalf("LastSkipped() = %v, want map[unstamped:1]", skipped)
	}
}

func TestVectorSearch_LegacySchemaWithoutModelColumn(t *testing.T) {
	queryVec := []float32{1, 0, 0}
	engine := &modelFakeEngine{name: "fake:current", vec: queryVec}

	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "corpus.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE prompt_atoms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		atom_id TEXT NOT NULL UNIQUE,
		content TEXT NOT NULL,
		token_count INTEGER NOT NULL,
		content_hash TEXT NOT NULL,
		category TEXT NOT NULL,
		embedding BLOB
	)`); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}

	blob := encodeFloat32Slice(queryVec)
	if _, err := db.Exec(
		`INSERT INTO prompt_atoms (atom_id, content, token_count, content_hash, category, embedding) VALUES (?, ?, ?, ?, ?, ?)`,
		"test/legacy", "legacy content", 1, "hash-legacy", "test", blob,
	); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}

	search := NewCompilerVectorSearcher(engine)
	compiler, err := NewJITPromptCompiler(WithProjectDB(db), WithVectorSearcher(search))
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	search.SetCompiler(compiler)

	results, err := search.Search(t.Context(), "refactoring query", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 0 {
		t.Fatalf("Search returned %d results, want 0: %v", len(results), results)
	}

	skipped := search.LastSkipped()
	if len(skipped) != 1 || skipped["unstamped"] != 1 {
		t.Fatalf("LastSkipped() = %v, want map[unstamped:1]", skipped)
	}
}

func TestSyncEmbeddedToSQLite_ReembedsWhenEngineChanges(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "reembed.db")

	vecA := []float32{1, 0, 0}
	vecB := []float32{0, 1, 0}

	engineA := &modelFakeEngine{name: "fake:a", vec: vecA}
	if err := SyncEmbeddedToSQLite(ctx, dbPath, engineA); err != nil {
		t.Fatalf("first sync failed: %v", err)
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var total, stampedA int
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms").Scan(&total); err != nil {
		_ = db.Close()
		t.Fatalf("count atoms after first sync: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms WHERE embedding_model = ?", "fake:a").Scan(&stampedA); err != nil {
		_ = db.Close()
		t.Fatalf("count stamped rows after first sync: %v", err)
	}
	_ = db.Close()
	if total == 0 {
		t.Fatal("first sync stored no atoms")
	}
	if stampedA != total {
		t.Fatalf("after first sync %d/%d rows stamped fake:a", stampedA, total)
	}

	engineB := &modelFakeEngine{name: "fake:b", vec: vecB}
	if err := SyncEmbeddedToSQLite(ctx, dbPath, engineB); err != nil {
		t.Fatalf("second sync failed: %v", err)
	}

	db, err = sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	var stampedB int
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms").Scan(&total); err != nil {
		_ = db.Close()
		t.Fatalf("count atoms after second sync: %v", err)
	}
	if err := db.QueryRow("SELECT COUNT(*) FROM prompt_atoms WHERE embedding_model = ?", "fake:b").Scan(&stampedB); err != nil {
		_ = db.Close()
		t.Fatalf("count stamped rows after second sync: %v", err)
	}
	if stampedB != total {
		_ = db.Close()
		t.Fatalf("after second sync %d/%d rows stamped fake:b", stampedB, total)
	}
	want := encodeFloat32Slice(vecB)
	rows, err := db.Query("SELECT embedding FROM prompt_atoms")
	if err != nil {
		_ = db.Close()
		t.Fatalf("query embeddings after second sync: %v", err)
	}
	checked := 0
	for rows.Next() {
		var blob []byte
		if err := rows.Scan(&blob); err != nil {
			_ = rows.Close()
			_ = db.Close()
			t.Fatalf("scan embedding: %v", err)
		}
		if len(blob) == 0 {
			_ = rows.Close()
			_ = db.Close()
			t.Fatal("found row with NULL/empty embedding after re-embed")
		}
		if !bytes.Equal(blob, want) {
			_ = rows.Close()
			_ = db.Close()
			t.Fatal("stored vector is not engine b's vector after re-embed")
		}
		checked++
	}
	_ = rows.Close()
	if err := rows.Err(); err != nil {
		_ = db.Close()
		t.Fatalf("row iteration error: %v", err)
	}
	_ = db.Close()
	if checked != total {
		t.Fatalf("checked %d embeddings, want %d", checked, total)
	}

	before := engineB.batchCalls
	if err := SyncEmbeddedToSQLite(ctx, dbPath, engineB); err != nil {
		t.Fatalf("third sync failed: %v", err)
	}
	if engineB.batchCalls != before {
		t.Fatalf("third sync embedded: batchCalls %d -> %d, want no new embeddings", before, engineB.batchCalls)
	}
}
