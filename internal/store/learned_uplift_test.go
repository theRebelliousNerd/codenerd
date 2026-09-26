package store

import (
	"context"
	"path/filepath"
	"testing"
)

// openLearnedTestStore builds a file-backed learned corpus with the mock
// 4-dimensional engine.
func openLearnedTestStore(t *testing.T, dbPath string) *LearnedCorpusStore {
	t.Helper()
	s, err := NewLearnedCorpusStore(dbPath, &MockEmbeddingEngine{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// A successful write must be recallable after reopening the store. The ANN
// index used to be dropped on every open and never backfilled, so every
// previously learned pattern went unsearchable until re-added.
func TestLearnedStore_ReopenRecallsPatterns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "learned.db")
	s := openLearnedTestStore(t, dbPath)
	ctx := context.Background()
	if err := s.AddPattern(ctx, "create app", "create", "app", "", 0.9); err != nil {
		t.Fatal(err)
	}
	if err := s.AddPattern(ctx, "fix bug", "fix", "bug", "", 0.8); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	reopened := openLearnedTestStore(t, dbPath)
	matches, err := reopened.Search([]float32{0.1, 0.2, 0.3, 0.4}, 5)
	if err != nil {
		t.Fatalf("Search after reopen: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("matches after reopen = %d, want 2", len(matches))
	}
}

// Without an ANN index (extension missing, engine deferred), search falls
// back to brute force instead of failing every query.
func TestLearnedStore_SearchWithoutVecFallsBack(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "learned.db")
	s := openLearnedTestStore(t, dbPath)
	ctx := context.Background()
	if err := s.AddPattern(ctx, "create app", "create", "app", "", 0.9); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.Exec("DROP TABLE vec_learned"); err != nil {
		t.Fatal(err)
	}

	matches, err := s.Search([]float32{0.1, 0.2, 0.3, 0.4}, 5)
	if err != nil {
		t.Fatalf("Search without vec index: %v", err)
	}
	if len(matches) != 1 || matches[0].TextContent != "create app" {
		t.Fatalf("matches = %+v, want the learned pattern", matches)
	}
	if matches[0].Rank != 1 || matches[0].Predicate != "learned_intent" {
		t.Errorf("match = %+v, want rank 1 with the learned contract", matches[0])
	}
	if matches[0].Similarity <= 0 || matches[0].Similarity > 1 {
		t.Errorf("similarity = %f, want (0, 1]", matches[0].Similarity)
	}
}

// Learnings with big-int args must round-trip exactly: plain JSON decoding
// turned every number into a float64, corrupting int64s past 2^53 and —
// worse — forking the lexical handle, since Save hashes the caller's args
// while the worker rehashes decoded args.
func TestLearningStore_Int64ArgsRoundTrip(t *testing.T) {
	ls, err := NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ls.Close()

	const big = int64(1786773933859876776)
	if err := ls.Save("coder", "event_id", []any{big, "x"}, "camp"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	loaded, err := ls.Load("coder")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("loaded = %d learnings, want 1", len(loaded))
	}
	got, ok := loaded[0].FactArgs[0].(int64)
	if !ok || got != big {
		t.Fatalf("FactArgs[0] = %#v, want int64(%d)", loaded[0].FactArgs[0], big)
	}

	// Re-saving the loaded learning must reinforce the same row (UNIQUE on
	// predicate+args), not fork a float64-spelled duplicate.
	if err := ls.Save("coder", loaded[0].FactPredicate, loaded[0].FactArgs, "camp"); err != nil {
		t.Fatalf("re-Save: %v", err)
	}
	db, err := ls.getDB("coder")
	if err != nil {
		t.Fatal(err)
	}
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM learnings").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("rows after re-save = %d, want 1 (reinforced, not duplicated)", n)
	}
}

// The worker's handle for a numeric learning must equal the handle Save
// stored — otherwise the hash never converges and the row re-embeds forever.
func TestLearningStore_NumericHandleConverges(t *testing.T) {
	ls, err := NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ls.Close()

	if err := ls.Save("coder", "retry_budget", []any{int64(5)}, "camp"); err != nil {
		t.Fatalf("Save: %v", err)
	}
	db, err := ls.getDB("coder")
	if err != nil {
		t.Fatal(err)
	}
	var storedHandle, storedHash, argsJSON string
	if err := db.QueryRow(
		"SELECT semantic_handle, handle_hash, fact_args FROM learnings LIMIT 1",
	).Scan(&storedHandle, &storedHash, &argsJSON); err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeLearningArgs(argsJSON)
	if err != nil {
		t.Fatalf("decodeLearningArgs: %v", err)
	}
	rebuilt := buildLearningHandle("coder", "retry_budget", decoded)
	if rebuilt != storedHandle {
		t.Errorf("rebuilt handle = %q, stored = %q", rebuilt, storedHandle)
	}
	if computeDescriptorHash(rebuilt) != storedHash {
		t.Error("rebuilt handle hash differs from the stored hash: worker would never converge")
	}
}

// The shared decoder: integers come back int64, decimals float64, nesting
// preserved, garbage rejected.
func TestDecodeLearningArgs_Types(t *testing.T) {
	args, err := decodeLearningArgs(`[5, 5.5, "s", true, null, {"k": 7}, [8]]`)
	if err != nil {
		t.Fatalf("decodeLearningArgs: %v", err)
	}
	if v, ok := args[0].(int64); !ok || v != 5 {
		t.Errorf("args[0] = %#v, want int64(5)", args[0])
	}
	if v, ok := args[1].(float64); !ok || v != 5.5 {
		t.Errorf("args[1] = %#v, want float64(5.5)", args[1])
	}
	if v, ok := args[5].(map[string]any)["k"].(int64); !ok || v != 7 {
		t.Errorf("nested = %#v, want int64(7)", args[5])
	}
	if v, ok := args[6].([]any)[0].(int64); !ok || v != 8 {
		t.Errorf("nested slice = %#v, want int64(8)", args[6])
	}
	if _, err := decodeLearningArgs(`[broken`); err == nil {
		t.Error("expected an error for malformed JSON")
	}
	if args, err := decodeLearningArgs(``); err != nil || args != nil {
		t.Errorf("empty input = %#v, %v; want nil, nil", args, err)
	}
}
