package problemlog

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"codenerd/internal/types"
)

// storeKernel holds facts by predicate, like a kernel's EDB: Retract clears a
// predicate, AssertBatch adds. Each call yields, so unserialized replaces
// interleave.
type storeKernel struct {
	mu    sync.Mutex
	facts map[string][]types.Fact
}

func (k *storeKernel) Retract(p string) error {
	runtime.Gosched()
	k.mu.Lock()
	defer k.mu.Unlock()
	delete(k.facts, p)
	return nil
}

func (k *storeKernel) AssertBatch(fs []types.Fact) error {
	runtime.Gosched()
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, f := range fs {
		k.facts[f.Predicate] = append(k.facts[f.Predicate], f)
	}
	return nil
}

func (k *storeKernel) Query(p string) ([]types.Fact, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	return append([]types.Fact(nil), k.facts[p]...), nil
}

// The chat's Update thread, its self-repair look and the rebuild goroutine
// replace the same ledger's facts concurrently: whatever the interleaving, the
// kernel ends with one projection of the ledger and one clock.
func TestReplaceRepair_ConcurrentReplacesNeverInterleave(t *testing.T) {
	l, _ := newLedger(t)
	if err := l.Begin([]string{"sig"}, LaneRepair, "j", nil, time.Unix(5, 0)); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < 20; round++ {
		k := &storeKernel{facts: map[string][]types.Fact{}}
		var wg sync.WaitGroup
		for g := 0; g < 4; g++ {
			wg.Add(1)
			go func(g int) {
				defer wg.Done()
				for i := 0; i < 25; i++ {
					if err := ReplaceRepair(k, l, time.Unix(int64(100*g+i), 0)); err != nil {
						t.Error(err)
						return
					}
				}
			}(g)
		}
		wg.Wait()
		clocks, _ := k.Query(RepairClockPredicate)
		attempts, _ := k.Query(RepairAttemptPredicate)
		if len(clocks) != 1 || len(attempts) != 1 {
			t.Fatalf("round %d: %d clock and %d attempt facts after concurrent replaces, want one of each", round, len(clocks), len(attempts))
		}
	}
}
