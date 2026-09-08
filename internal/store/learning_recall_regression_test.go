package store

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

// Regression: production Save succeeds but fresh-process lexical recall misses
// because Save leaves semantic_handle NULL while recall searches only
// semantic_handle. These tests use actually saved content with no embedding
// worker and a fresh reopen to simulate a new process.

func TestLearningRecall_FreshReopenReturnsSavedContent(t *testing.T) {
	dir := t.TempDir()

	ls, err := NewLearningStore(dir)
	if err != nil {
		t.Fatalf("NewLearningStore failed: %v", err)
	}
	args := []any{"kernel indexing measured observation", "measured capsule alpha"}
	if err := ls.Save("symbolicefficiencyexpert", "learned_pattern", args, "campaign"); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if err := ls.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Fresh process: reopen same dir, no embedding worker configured.
	ls2, err := NewLearningStore(dir)
	if err != nil {
		t.Fatalf("reopen NewLearningStore failed: %v", err)
	}
	defer ls2.Close()

	hits, err := ls2.RecallLearningsLexical("kernel indexing measured observation", 5)
	if err != nil {
		t.Fatalf("RecallLearningsLexical failed: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected lexical recall to return saved learning after fresh reopen, got 0 hits")
	}
	found := false
	for _, h := range hits {
		if h.Predicate == "learned_pattern" {
			found = true
			if h.SourceCampaign != "campaign" || h.Confidence != 1 || h.LearnedAt.IsZero() {
				t.Errorf("learning provenance was lost: %+v", h)
			}
			combined := strings.ToLower(h.Summary + " " + h.Predicate)
			if !strings.Contains(combined, "measured capsule alpha") {
				t.Errorf("hit summary does not contain saved content: %q", h.Summary)
			}
		}
	}
	if !found {
		t.Fatalf("expected predicate learned_pattern in hits, got %+v", hits)
	}
}

func TestLearningRecall_BatchPathFreshReopen(t *testing.T) {
	dir := t.TempDir()

	ls, err := NewLearningStore(dir)
	if err != nil {
		t.Fatalf("NewLearningStore failed: %v", err)
	}
	batch := []types.ShardLearning{
		{FactPredicate: "learned_pattern", FactArgs: []any{"kernel indexing batch observation", "measured capsule beta"}},
	}
	if err := ls.SaveBatch("symbolicefficiencyexpert", batch, "campaign"); err != nil {
		t.Fatalf("SaveBatch failed: %v", err)
	}
	if err := ls.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	ls2, err := NewLearningStore(dir)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer ls2.Close()

	hits, err := ls2.RecallLearningsLexical("kernel indexing batch observation", 5)
	if err != nil {
		t.Fatalf("RecallLearningsLexical failed: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected batch-saved learning to be lexically recallable after reopen, got 0 hits")
	}
	if !strings.Contains(hits[0].Summary, "measured capsule beta") {
		t.Fatalf("saved batch content missing: %+v", hits)
	}
}

func TestLearningRecall_LegacyNullHandleRemainsRecallable(t *testing.T) {
	dir := t.TempDir()

	ls, err := NewLearningStore(dir)
	if err != nil {
		t.Fatalf("NewLearningStore failed: %v", err)
	}
	// Simulate an older row written before handles existed: semantic_handle NULL.
	db, err := ls.getDB("symbolicefficiencyexpert")
	if err != nil {
		t.Fatalf("getDB failed: %v", err)
	}
	legacyArgs := `["kernel indexing legacy observation","measured capsule legacy"]`
	if _, err := db.Exec(
		`INSERT INTO learnings (shard_type, fact_predicate, fact_args, source_campaign, confidence) VALUES (?, ?, ?, ?, 1.0)`,
		"symbolicefficiencyexpert", "learned_pattern", legacyArgs, "campaign",
	); err != nil {
		t.Fatalf("legacy insert failed: %v", err)
	}
	// Also cover empty-string handle variant.
	if _, err := db.Exec(
		`INSERT INTO learnings (shard_type, fact_predicate, fact_args, source_campaign, confidence, semantic_handle, handle_version, handle_hash) VALUES (?, ?, ?, ?, 1.0, '', 0, '')`,
		"symbolicefficiencyexpert", "learned_pattern", `["kernel indexing emptyhandle observation","capsule"]`, "campaign",
	); err != nil {
		t.Fatalf("empty-handle insert failed: %v", err)
	}
	if err := ls.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	ls2, err := NewLearningStore(dir)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer ls2.Close()

	hits, err := ls2.RecallLearningsLexical("kernel indexing legacy observation", 5)
	if err != nil {
		t.Fatalf("RecallLearningsLexical failed: %v", err)
	}
	if len(hits) == 0 {
		t.Fatalf("expected legacy NULL-handle row to remain recallable, got 0 hits")
	}
	foundLegacy := false
	for _, hit := range hits {
		foundLegacy = foundLegacy || strings.Contains(hit.Summary, "measured capsule legacy")
	}
	if !foundLegacy {
		t.Fatalf("legacy content missing: %+v", hits)
	}

	// Empty-handle row must also be recallable via its own args.
	hitsEmpty, err := ls2.RecallLearningsLexical("kernel indexing emptyhandle observation", 5)
	if err != nil {
		t.Fatalf("RecallLearningsLexical (empty handle) failed: %v", err)
	}
	if len(hitsEmpty) == 0 {
		t.Fatalf("expected empty-handle row to remain recallable, got 0 hits")
	}
	foundEmpty := false
	for _, hit := range hitsEmpty {
		foundEmpty = foundEmpty || strings.Contains(hit.Summary, "emptyhandle observation")
	}
	if !foundEmpty {
		t.Fatalf("empty-handle content missing: %+v", hitsEmpty)
	}
}

func TestLearningRecall_ReinforcementPreservesEmbeddingIdentity(t *testing.T) {
	ls, err := NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ls.Close()
	args := []any{"measured capsule"}
	if err := ls.Save("expert", "learned_pattern", args, "first"); err != nil {
		t.Fatal(err)
	}
	db, err := ls.getDB("expert")
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`UPDATE learnings SET semantic_handle='curated descriptor', handle_version=99, handle_hash='existing', embedding=x'01020304', confidence=0.4`)
	if err != nil {
		t.Fatal(err)
	}
	for _, save := range []func() error{
		func() error { return ls.Save("expert", "learned_pattern", args, "second") },
		func() error {
			return ls.SaveBatch("expert", []types.ShardLearning{{FactPredicate: "learned_pattern", FactArgs: args}}, "third")
		},
	} {
		if err := save(); err != nil {
			t.Fatal(err)
		}
		var handle, hash, embedding string
		var version int
		if err := db.QueryRow(`SELECT semantic_handle, handle_version, handle_hash, hex(embedding) FROM learnings`).Scan(&handle, &version, &hash, &embedding); err != nil {
			t.Fatal(err)
		}
		if handle != "curated descriptor" || version != 99 || hash != "existing" || embedding != "01020304" {
			t.Fatalf("reinforcement invalidated descriptor identity: %q %d %q %q", handle, version, hash, embedding)
		}
	}
}

func TestLearningRecall_ContextCancellation(t *testing.T) {
	ls, err := NewLearningStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ls.Close()
	if err := ls.Save("expert", "learned_pattern", []any{"kernel observation"}, "source"); err != nil {
		t.Fatal(err)
	}
	db, err := ls.getDB("expert")
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancel()
	hits, err := ls.RecallLearningsLexicalContext(ctx, "kernel", 5)
	if !errors.Is(err, context.DeadlineExceeded) || len(hits) != 0 {
		t.Fatalf("query ignored deadline: %+v %v", hits, err)
	}
	vectorCtx, cancelVector := context.WithTimeout(t.Context(), 40*time.Millisecond)
	defer cancelVector()
	hits, err = ls.RecallLearningsByEmbeddingContext(vectorCtx, []float32{1}, 5)
	if !errors.Is(err, context.DeadlineExceeded) || len(hits) != 0 {
		t.Fatalf("embedding recall ignored cancellation: %+v %v", hits, err)
	}
}

func TestLearningRecall_UnrelatedQueryReturnsNone(t *testing.T) {
	dir := t.TempDir()

	ls, err := NewLearningStore(dir)
	if err != nil {
		t.Fatalf("NewLearningStore failed: %v", err)
	}
	args := []any{"kernel indexing measured observation", "measured capsule alpha"}
	if err := ls.Save("symbolicefficiencyexpert", "learned_pattern", args, "campaign"); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if err := ls.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	ls2, err := NewLearningStore(dir)
	if err != nil {
		t.Fatalf("reopen failed: %v", err)
	}
	defer ls2.Close()

	hits, err := ls2.RecallLearningsLexical("zebra xylophone quantum violet", 5)
	if err != nil {
		t.Fatalf("RecallLearningsLexical failed: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("expected 0 hits for unrelated query, got %d: %+v", len(hits), hits)
	}
}
