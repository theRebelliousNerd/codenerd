package prompt_evolution

import (
	"fmt"
	"sync"
	"testing"
)

func cacheTestStore(t *testing.T) *StrategyStore {
	t.Helper()
	store, err := NewStrategyStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close strategy store: %v", err)
		}
	})
	return store
}

func TestStrategyStoreConcurrentSelectionCache(t *testing.T) {
	for _, mode := range []string{"warm", "expiring"} {
		t.Run(mode, func(t *testing.T) {
			store := cacheTestStore(t)
			for _, shard := range []string{"coder", "tester"} {
				if err := store.CreateStrategy(&Strategy{ID: shard, ProblemType: ProblemDebugging, ShardType: shard, Content: shard, SuccessRate: 0.8}); err != nil {
					t.Fatal(err)
				}
				if _, err := store.SelectStrategies(ProblemDebugging, shard, 1); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "expiring" {
				// Force the real expiry/miss/populate path without sleeps or a
				// clock hook; the store is still owned by this single setup.
				store.cacheTTL = 0
			}
			start := make(chan struct{})
			errors := make(chan error, 12)
			var workers sync.WaitGroup
			for i := 0; i < 12; i++ {
				shard := []string{"coder", "tester"}[i%2]
				workers.Go(func() {
					<-start
					for range 32 {
						selected, err := store.SelectStrategies(ProblemDebugging, shard, 1)
						if err != nil || len(selected) != 1 || selected[0].ID != shard || selected[0].Content != shard {
							errors <- fmt.Errorf("selection for %s = %+v, %v", shard, selected, err)
							return
						}
					}
				})
			}
			close(start)
			workers.Wait()
			close(errors)
			for err := range errors {
				t.Error(err)
			}
		})
	}
}

func TestStrategyStoreCacheMutationInvalidation(t *testing.T) {
	store := cacheTestStore(t)
	if err := store.CreateStrategy(&Strategy{ID: "original", ProblemType: ProblemDebugging, ShardType: "coder", Content: "original", SuccessRate: 0.6}); err != nil {
		t.Fatal(err)
	}
	selectOne := func(id, content string, version, successes int) {
		t.Helper()
		selected, err := store.SelectStrategies(ProblemDebugging, "coder", 1)
		if err != nil || len(selected) != 1 {
			t.Fatalf("select strategy = %+v, %v", selected, err)
		}
		got := selected[0]
		if got.ID != id || got.Content != content || got.Version != version || got.SuccessCount != successes {
			t.Fatalf("stale strategy after mutation: %+v", got)
		}
	}
	selectOne("original", "original", 1, 0)
	if err := store.RefineStrategy("original", "refined"); err != nil {
		t.Fatal(err)
	}
	selectOne("original", "refined", 2, 0)
	if err := store.RecordOutcome("original", "cache-task", true); err != nil {
		t.Fatal(err)
	}
	selectOne("original", "refined", 2, 1)
	if err := store.CreateStrategy(&Strategy{ID: "better", ProblemType: ProblemDebugging, ShardType: "coder", Content: "better", SuccessRate: 1, SuccessCount: 10}); err != nil {
		t.Fatal(err)
	}
	selectOne("better", "better", 1, 10)
	if err := store.DeleteStrategy("better"); err != nil {
		t.Fatal(err)
	}
	selectOne("original", "refined", 2, 1)
}

func TestStrategyStoreCacheExpiryReloadsDatabase(t *testing.T) {
	store := cacheTestStore(t)
	if err := store.CreateStrategy(&Strategy{ID: "expiry", ProblemType: ProblemTesting, ShardType: "tester", Content: "cached"}); err != nil {
		t.Fatal(err)
	}
	check := func(want string) {
		t.Helper()
		selected, err := store.SelectStrategies(ProblemTesting, "tester", 1)
		if err != nil || len(selected) != 1 || selected[0].Content != want {
			t.Fatalf("strategy content want %q, got %+v, %v", want, selected, err)
		}
	}
	check("cached")
	if _, err := store.db.Exec("UPDATE strategies SET content = ? WHERE id = ?", "database", "expiry"); err != nil {
		t.Fatal(err)
	}
	check("cached") // Warm positive control: the cached value is actually used.
	store.cacheTTL = 0
	check("database")
}
