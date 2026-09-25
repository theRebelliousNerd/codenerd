package context

import (
	"context"
	"testing"
	"time"

	"codenerd/internal/store"
)

// A turn whose compressed state cannot be stored still completes, but the
// failure is counted and reported. ProcessTurn discarded both write errors
// (WIRING-AND-NOT-BUILT: "persistence is assumed durable but coded
// best-effort"), so a session that could never be rehydrated said nothing.
func TestProcessTurn_WhenTheStoreRefusesTheState_ShouldCountTheFailure(t *testing.T) {
	comp := newKernelBackedCompressor(t)
	healthy := func() int { return comp.GetMetrics()["persist_failures"].(int) }

	if _, err := comp.ProcessTurn(context.Background(), Turn{Number: 1, Role: "user", UserInput: "hello", Timestamp: time.Now()}); err != nil {
		t.Fatalf("ProcessTurn: %v", err)
	}
	if n := healthy(); n != 0 {
		t.Fatalf("persist_failures = %d with a working store, want 0", n)
	}

	// A closed store refuses every write.
	closed, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	if err := closed.Close(); err != nil {
		t.Fatal(err)
	}
	comp.mu.Lock()
	comp.store = closed
	comp.mu.Unlock()

	if _, err := comp.ProcessTurn(context.Background(), Turn{Number: 2, Role: "user", UserInput: "again", Timestamp: time.Now()}); err != nil {
		t.Fatalf("a failed persistence write failed the turn: %v", err)
	}
	if n := healthy(); n == 0 {
		t.Fatal("the store refused the compressed state and persist_failures stayed 0: the failure was silent")
	}
}
