package system

import (
	"context"
	"fmt"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// stubLivenessManager is a minimal shardLivenessChecker double. A nil running
// map means dormant. Returning (nil, true) for a running shard is sufficient
// because the gate only checks the boolean.
type stubLivenessManager struct {
	running map[string]bool
}

func (m *stubLivenessManager) GetRunningShardByConfigName(name string) (types.ShardAgent, bool) {
	if m != nil && m.running[name] {
		return nil, true
	}
	return nil, false
}

func routerRunningManager() *stubLivenessManager {
	return &stubLivenessManager{running: map[string]bool{"tactile_router": true}}
}

func routerDormantManager() *stubLivenessManager {
	return &stubLivenessManager{running: map[string]bool{}}
}

func assertPendingAction(t *testing.T, kernel *core.RealKernel, actionID string) {
	t.Helper()
	if err := kernel.Assert(core.Fact{
		Predicate: "pending_action",
		// Payload is asserted the way production does (encodeActionPayload →
		// string): RetractExactFact compares stored args with the queried
		// fact's args, and a raw Go map never round-trips equal.
		Args:      []any{actionID, "/read_file", "hello.txt", encodeActionPayload(map[string]any{}), time.Now().Unix()},
	}); err != nil {
		t.Fatalf("assert pending_action %q: %v", actionID, err)
	}
}

func queryIDs(t *testing.T, kernel *core.RealKernel, predicate string) map[string]bool {
	t.Helper()
	facts, err := kernel.Query(predicate)
	if err != nil {
		t.Fatalf("Query(%s): %v", predicate, err)
	}
	ids := make(map[string]bool, len(facts))
	for _, f := range facts {
		if len(f.Args) == 0 {
			continue
		}
		ids[types.ExtractString(f.Args[0])] = true
	}
	return ids
}

func TestOwnsPendingAction(t *testing.T) {
	cases := []struct {
		id   string
		want bool
	}{
		{"action-123", true},
		{"action-test", true},
		{"delegate-456", true},
		{"exec-call_1", false},
		{"exec-action-123", false},
		{"call_1", false},
		{"toolu_abc", false},
		{"c1", false},
		{"", false},
		{"action_123", false},
		{"Action-123", false},
	}
	for _, tc := range cases {
		if got := ownsPendingAction(tc.id); got != tc.want {
			t.Errorf("ownsPendingAction(%q) = %v, want %v", tc.id, got, tc.want)
		}
	}
}

func TestConstitutionGateSkipsExecutorPendingAction(t *testing.T) {
	ctx := context.Background()
	kernel := newTestKernel(t)
	shard := NewConstitutionGateShard()
	shard.Kernel = kernel
	shard.SetShardManager(routerRunningManager())

	execID := "exec-call_1"
	assertPendingAction(t, kernel, execID)

	if err := shard.processPendingActions(ctx); err != nil {
		t.Fatalf("processPendingActions: %v", err)
	}

	pending := queryIDs(t, kernel, "pending_action")
	if !pending[execID] {
		t.Errorf("executor pending_action %q was retracted, want it to survive", execID)
	}
	if permitted := queryIDs(t, kernel, "permitted_action"); permitted[execID] {
		t.Errorf("permitted_action emitted for executor ID %q, want none", execID)
	}
	if results := queryIDs(t, kernel, "permission_check_result"); results[execID] {
		t.Errorf("permission_check_result emitted for skipped executor ID %q, want none", execID)
	}
}

func TestConstitutionGateProcessesExecutivePendingAction(t *testing.T) {
	ctx := context.Background()
	kernel := newTestKernel(t)
	shard := NewConstitutionGateShard()
	shard.Kernel = kernel
	shard.SetShardManager(routerRunningManager())

	execID := "action-ownership-b"
	assertPendingAction(t, kernel, execID)

	if err := shard.processPendingActions(ctx); err != nil {
		t.Fatalf("processPendingActions: %v", err)
	}

	if pending := queryIDs(t, kernel, "pending_action"); pending[execID] {
		t.Errorf("executive pending_action %q survived, want it retracted", execID)
	}
	if permitted := queryIDs(t, kernel, "permitted_action"); !permitted[execID] {
		t.Errorf("permitted_action missing for executive ID %q, want it emitted", execID)
	}
	foundPermit := false
	results, err := kernel.Query("permission_check_result")
	if err != nil {
		t.Fatalf("Query(permission_check_result): %v", err)
	}
	for _, f := range results {
		if len(f.Args) < 2 {
			continue
		}
		if types.ExtractString(f.Args[0]) != execID {
			continue
		}
		foundPermit = true
		if status := fmt.Sprintf("%v", f.Args[1]); status != "/permit" {
			t.Fatalf("permission_check_result status = %v, want /permit", f.Args[1])
		}
	}
	if !foundPermit {
		t.Errorf("permission_check_result missing for executive ID %q", execID)
	}
}

func TestConstitutionGateDormantRouterEmitsNoPermittedAction(t *testing.T) {
	ctx := context.Background()
	kernel := newTestKernel(t)
	shard := NewConstitutionGateShard()
	shard.Kernel = kernel
	shard.SetShardManager(routerDormantManager())

	const n = 5
	ids := make([]string, 0, n)
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("action-dormant-%d", i)
		ids = append(ids, id)
		assertPendingAction(t, kernel, id)
	}

	if err := shard.processPendingActions(ctx); err != nil {
		t.Fatalf("processPendingActions: %v", err)
	}

	permitted, err := kernel.Query("permitted_action")
	if err != nil {
		t.Fatalf("Query(permitted_action): %v", err)
	}
	if len(permitted) != 0 {
		t.Errorf("permitted_action count = %d with dormant router, want 0", len(permitted))
	}

	results := queryIDs(t, kernel, "permission_check_result")
	for _, id := range ids {
		if !results[id] {
			t.Errorf("permission_check_result missing for %q (observability must persist)", id)
		}
	}
	pending, err := kernel.Query("pending_action")
	if err != nil {
		t.Fatalf("Query(pending_action): %v", err)
	}
	for _, f := range pending {
		if len(f.Args) == 0 {
			continue
		}
		id := types.ExtractString(f.Args[0])
		if ownsPendingAction(id) {
			t.Errorf("owned pending_action %q survived, want it retracted", id)
		}
	}
}

func TestConstitutionGatePermittedActionEmittedAndPruned(t *testing.T) {
	ctx := context.Background()
	kernel := newTestKernel(t)
	shard := NewConstitutionGateShard()
	shard.Kernel = kernel
	shard.SetShardManager(routerRunningManager())

	staleTS := time.Now().Add(-16 * time.Minute).Unix()
	staleID := "action-stale-prune"
	if err := kernel.Assert(core.Fact{
		Predicate: "permitted_action",
		Args:      []any{staleID, "/read_file", "hello.txt", "{}", staleTS},
	}); err != nil {
		t.Fatalf("assert stale permitted_action: %v", err)
	}

	freshID := "action-fresh-prune"
	assertPendingAction(t, kernel, freshID)

	if err := shard.processPendingActions(ctx); err != nil {
		t.Fatalf("processPendingActions: %v", err)
	}

	ids := queryIDs(t, kernel, "permitted_action")
	if ids[staleID] {
		t.Errorf("stale permitted_action %q survived prune, want it retracted", staleID)
	}
	if !ids[freshID] {
		t.Errorf("fresh permitted_action %q missing with router running, want it emitted", freshID)
	}
}
