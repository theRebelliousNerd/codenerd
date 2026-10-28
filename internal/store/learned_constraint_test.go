package store

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

var learnedConstraintQuery = []float32{0.1, 0.2, 0.3, 0.4}

func constraintOf(t *testing.T, matches []SemanticMatch, pattern string) string {
	t.Helper()
	for _, m := range matches {
		if m.TextContent == pattern {
			return m.Constraint
		}
	}
	t.Fatalf("pattern %q is not among the matches %+v", pattern, matches)
	return ""
}

// A learned mapping's qualifier is part of what was learned. It must come back
// from the durable store after a reopen on both search paths: the ANN query
// used to scan the column into a local it dropped, and the brute-force
// fallback never selected it. A mapping with no qualifier must read back
// empty, so absence is not confused with loss.
func TestLearnedStore_ConstraintSurvivesReopenOnBothSearchPaths(t *testing.T) {
	const qualified, bare = "create app", "fix bug"
	const constraint = "only under internal/api, never the generated files"

	dbPath := filepath.Join(t.TempDir(), "learned.db")
	first := openLearnedTestStore(t, dbPath)
	ctx := context.Background()
	if err := first.AddPattern(ctx, qualified, "create", "app", constraint, 0.9); err != nil {
		t.Fatal(err)
	}
	if err := first.AddPattern(ctx, bare, "fix", "bug", "", 0.9); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	s := openLearnedTestStore(t, dbPath)
	check := func(t *testing.T, matches []SemanticMatch) {
		t.Helper()
		if len(matches) != 2 {
			t.Fatalf("matches = %d, want 2: %+v", len(matches), matches)
		}
		if got := constraintOf(t, matches, qualified); got != constraint {
			t.Errorf("qualified mapping Constraint = %q, want %q", got, constraint)
		}
		if got := constraintOf(t, matches, bare); got != "" {
			t.Errorf("unqualified mapping Constraint = %q, want empty", got)
		}
	}

	t.Run("ann", func(t *testing.T) {
		if !tableExists(s.db, "vec_learned") {
			t.Fatal("vec_learned is absent after reopen, so the ANN path is not under test")
		}
		matches, err := s.searchVec(encodeFloat32SliceToBlob(learnedConstraintQuery), 5)
		if err != nil {
			t.Fatalf("searchVec: %v", err)
		}
		check(t, matches)
		viaSearch, err := s.Search(learnedConstraintQuery, 5)
		if err != nil {
			t.Fatal(err)
		}
		check(t, viaSearch)
	})

	t.Run("brute force", func(t *testing.T) {
		if _, err := s.db.Exec("DROP TABLE vec_learned"); err != nil {
			t.Fatal(err)
		}
		matches, err := s.Search(learnedConstraintQuery, 5)
		if err != nil {
			t.Fatalf("Search without the ANN index: %v", err)
		}
		check(t, matches)
	})
}

// Close sets db to nil. A write that embedded before the close and takes the
// lock after it must be refused, not dereference a nil database.
func TestLearnedStore_AddPatternAfterCloseIsRefused(t *testing.T) {
	s := openLearnedTestStore(t, filepath.Join(t.TempDir(), "learned.db"))
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.AddPattern(context.Background(), "create app", "create", "app", "", 0.9); err == nil {
		t.Fatal("AddPattern on a closed store succeeded")
	}
}

// The close can land between the embedding and the lock: the engine below
// closes the store while it embeds, which is exactly that interleaving.
func TestLearnedStore_CloseDuringEmbeddingIsRefused(t *testing.T) {
	var s *LearnedCorpusStore
	engine := &MockEmbeddingEngine{EmbedFunc: func(ctx context.Context, _ string) ([]float32, error) {
		if err := s.Close(); err != nil {
			t.Errorf("Close during embedding: %v", err)
		}
		return learnedConstraintQuery, nil
	}}
	var err error
	s, err = NewLearnedCorpusStore(filepath.Join(t.TempDir(), "learned.db"), &MockEmbeddingEngine{})
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	s.embedEngine = engine
	s.mu.Unlock()

	if err := s.AddPattern(context.Background(), "create app", "create", "app", "", 0.9); err == nil {
		t.Fatal("AddPattern whose store was closed while it embedded succeeded")
	}
}

// A context that ended while the embedding ran or the lock was awaited writes
// nothing.
func TestLearnedStore_AddPatternObservesContextAfterTheLock(t *testing.T) {
	s := openLearnedTestStore(t, filepath.Join(t.TempDir(), "learned.db"))
	ctx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	s.embedEngine = &MockEmbeddingEngine{EmbedFunc: func(context.Context, string) ([]float32, error) {
		cancel() // ends while the embedding "ran"; the lock is free
		return learnedConstraintQuery, nil
	}}
	s.mu.Unlock()

	if err := s.AddPattern(ctx, "create app", "create", "app", "", 0.9); !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
	matches, err := s.Search(learnedConstraintQuery, 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("a cancelled AddPattern wrote %+v", matches)
	}
}

// A writer waiting on a contended lock gives up when its context ends, and the
// lock it was waiting for is not lost: once the holder releases, the store
// still takes writes. The lock is contended twice over: held when AddPattern
// starts (its engine snapshot waits), and taken while it embeds (its write
// lock waits).
func TestLearnedStore_AddPatternLockWaitsHonourContext(t *testing.T) {
	expire := func(t *testing.T, s *LearnedCorpusStore) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- s.AddPattern(ctx, "create app", "create", "app", "", 0.9) }()
		select {
		case err := <-done:
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("err = %v, want context.DeadlineExceeded", err)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("AddPattern kept waiting for the lock after its context ended")
		}
	}
	afterwards := func(t *testing.T, s *LearnedCorpusStore) {
		t.Helper()
		if err := s.AddPattern(context.Background(), "fix bug", "fix", "bug", "", 0.9); err != nil {
			t.Fatalf("the store did not take a write after the abandoned wait: %v", err)
		}
		matches, err := s.Search(learnedConstraintQuery, 5)
		if err != nil {
			t.Fatal(err)
		}
		if len(matches) != 1 || matches[0].TextContent != "fix bug" {
			t.Fatalf("matches = %+v, want only the later write", matches)
		}
	}

	t.Run("held at the start", func(t *testing.T) {
		s := openLearnedTestStore(t, filepath.Join(t.TempDir(), "learned.db"))
		s.mu.Lock()
		expire(t, s)
		s.mu.Unlock()
		afterwards(t, s)
	})

	t.Run("taken while embedding", func(t *testing.T) {
		s := openLearnedTestStore(t, filepath.Join(t.TempDir(), "learned.db"))
		var held atomic.Bool
		s.mu.Lock()
		s.embedEngine = &MockEmbeddingEngine{EmbedFunc: func(context.Context, string) ([]float32, error) {
			if held.CompareAndSwap(false, true) {
				s.mu.Lock() // another writer wins the lock the embedding was racing for
			}
			return learnedConstraintQuery, nil
		}}
		s.mu.Unlock()
		expire(t, s)
		s.mu.Unlock()
		s.mu.Lock()
		s.embedEngine = &MockEmbeddingEngine{}
		s.mu.Unlock()
		afterwards(t, s)
	})
}
