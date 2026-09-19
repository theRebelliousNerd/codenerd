package campaign

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// External audit F4: a campaign task's turn that writes outside its declared
// write set takes the path's lease for the task at the moment of the write,
// holds it until the attempt ends, and is refused a path another task holds.
func TestWriteGuard_HoldsAnUndeclaredPathForTheAttemptAndRefusesAHeldOne(t *testing.T) {
	ws := t.TempDir()
	ctx := context.Background()
	o := &Orchestrator{writeSetLocks: newWriteSetLockManager(ws)}
	other, err := o.writeSetLocks.acquire(ctx, "/task_other", []string{"internal/held.go"}, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	defer other.release()

	task := &Task{ID: "/task_me"}
	o.beginAttempt(task)
	guard := o.writeGuard(task)

	if gerr := guard(ctx, []string{filepath.Join(ws, "internal", "held.go")}); gerr == nil || !strings.Contains(gerr.Error(), "/task_other") {
		t.Fatalf("guard(internal/held.go) = %v, want it refused naming /task_other", gerr)
	}
	free := filepath.Join(ws, "cmd", "free.go")
	if gerr := guard(ctx, []string{free}); gerr != nil {
		t.Fatalf("guard(cmd/free.go) = %v, want the write let through", gerr)
	}
	if _, holder, _ := o.writeSetLocks.tryAcquire("/task_third", []string{free}); holder != "/task_me" {
		t.Fatalf("while the attempt is open cmd/free.go is held by %q, want /task_me", holder)
	}

	o.endAttempt(task).release()
	lease, holder, _ := o.writeSetLocks.tryAcquire("/task_third", []string{free})
	if holder != "" || lease == nil {
		t.Fatalf("after the attempt ended cmd/free.go is held by %q", holder)
	}
	lease.release()

	// With no attempt open there is nothing to hold the lease for: the write
	// is checked and the lease goes back at once.
	if gerr := guard(ctx, []string{free}); gerr != nil {
		t.Fatalf("guard with no attempt open = %v", gerr)
	}
	if _, holder, _ := o.writeSetLocks.tryAcquire("/task_third", []string{free}); holder != "" {
		t.Fatalf("a checked write with no attempt open left cmd/free.go held by %q", holder)
	}
}
