package prompt

import (
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

func TestVectorSearch_LoaderStampsModelAndIsSearchable(t *testing.T) {
	queryVec := []float32{1, 0, 0}
	engine := &modelFakeEngine{name: "fake:current", vec: queryVec}
	ctx := t.Context()

	// StoreAtom path: stamp equals the engine name and the atom is searchable.
	db, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "store.db"))
	if err != nil {
		t.Fatal(err)
	}
	loader := NewAtomLoader(engine)
	if err := loader.EnsureSchema(ctx, db); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if err := loader.StoreAtom(ctx, db, NewPromptAtom("test/stamped", AtomCategory("test"), "match content")); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	var storedModel sql.NullString
	if err := db.QueryRow(`SELECT embedding_model FROM prompt_atoms WHERE atom_id = ?`, "test/stamped").Scan(&storedModel); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	if !storedModel.Valid || storedModel.String != engine.Name() {
		_ = db.Close()
		t.Fatalf("StoreAtom embedding_model = %q, want %q", storedModel.String, engine.Name())
	}

	search := NewCompilerVectorSearcher(engine)
	compiler, err := NewJITPromptCompiler(WithProjectDB(db), WithVectorSearcher(search))
	if err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	search.SetCompiler(compiler)

	results, err := search.Search(ctx, "refactoring query", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results) != 1 || results[0].AtomID != "test/stamped" {
		t.Fatalf("Search returned %v, want exactly [test/stamped]", results)
	}
	if skipped := search.LastSkipped(); len(skipped) != 0 {
		t.Fatalf("LastSkipped() = %v, want no skipped atoms", skipped)
	}

	// Re-storing the same ID under a different engine replaces the stamp.
	updated := &modelFakeEngine{name: "fake:updated", vec: queryVec}
	if err := NewAtomLoader(updated).StoreAtom(ctx, db, NewPromptAtom("test/stamped", AtomCategory("test"), "match content")); err != nil {
		t.Fatal(err)
	}
	var restamped sql.NullString
	if err := db.QueryRow(`SELECT embedding_model FROM prompt_atoms WHERE atom_id = ?`, "test/stamped").Scan(&restamped); err != nil {
		t.Fatal(err)
	}
	if !restamped.Valid || restamped.String != updated.Name() {
		t.Fatalf("re-stored embedding_model = %q, want %q", restamped.String, updated.Name())
	}

	// ReplaceAtoms path: every row is stamped and searchable with nothing skipped.
	db2, err := sql.Open("sqlite3", filepath.Join(t.TempDir(), "replace.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := NewAtomLoader(engine).EnsureSchema(ctx, db2); err != nil {
		_ = db2.Close()
		t.Fatal(err)
	}
	replaceAtoms := []*PromptAtom{
		NewPromptAtom("test/one", AtomCategory("test"), "first content"),
		NewPromptAtom("test/two", AtomCategory("test"), "second content"),
	}
	if err := NewAtomLoader(engine).ReplaceAtoms(ctx, db2, replaceAtoms); err != nil {
		_ = db2.Close()
		t.Fatal(err)
	}
	rows, err := db2.Query(`SELECT embedding_model FROM prompt_atoms`)
	if err != nil {
		_ = db2.Close()
		t.Fatal(err)
	}
	stamped := 0
	for rows.Next() {
		var m sql.NullString
		if err := rows.Scan(&m); err != nil {
			rows.Close()
			_ = db2.Close()
			t.Fatal(err)
		}
		if !m.Valid || m.String != engine.Name() {
			rows.Close()
			_ = db2.Close()
			t.Fatalf("ReplaceAtoms embedding_model = %q, want %q", m.String, engine.Name())
		}
		stamped++
	}
	rows.Close()
	if stamped != 2 {
		_ = db2.Close()
		t.Fatalf("ReplaceAtoms wrote %d atoms, want 2", stamped)
	}

	search2 := NewCompilerVectorSearcher(engine)
	compiler2, err := NewJITPromptCompiler(WithProjectDB(db2), WithVectorSearcher(search2))
	if err != nil {
		_ = db2.Close()
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = compiler2.Close() })
	search2.SetCompiler(compiler2)

	results2, err := search2.Search(ctx, "refactoring query", 10)
	if err != nil {
		t.Fatalf("Search returned error: %v", err)
	}
	if len(results2) != 2 {
		t.Fatalf("Search returned %d results, want 2: %v", len(results2), results2)
	}
	if skipped := search2.LastSkipped(); len(skipped) != 0 {
		t.Fatalf("LastSkipped() = %v, want no skipped atoms", skipped)
	}
}

// The skipped-vector warning fires when the skipped set appears or changes,
// not on every search over the same unstamped atoms.
func TestWarnSkipped_OnlyWhenTheSkippedSetChanges(t *testing.T) {
	stale := map[string]int{"unstamped": 3}
	if !warnSkipped(nil, stale) {
		t.Fatal("the first skipped set must warn")
	}
	if warnSkipped(map[string]int{"unstamped": 3}, stale) {
		t.Fatal("the same skipped set warned again")
	}
	if !warnSkipped(stale, map[string]int{"unstamped": 4}) {
		t.Fatal("a changed skipped set must warn")
	}
	if warnSkipped(stale, nil) {
		t.Fatal("nothing skipped must not warn")
	}
}
