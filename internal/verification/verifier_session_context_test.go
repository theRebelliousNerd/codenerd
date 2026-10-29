package verification

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// Every attempt of a verified delegation runs with the delegation's session
// context -- the blackboard an unverified delegation of the same request is
// handed. Attempts went through ExecuteObserved, which takes none, so the
// delegations the kernel routes through verification (mutations) were exactly
// the ones that never saw the prior shard results.
func TestVerifyWithRetry_EveryAttemptRunsWithTheDelegationsSessionContext(t *testing.T) {
	client, _ := judgeClient(judgeSays)
	exec := &stubTaskExecutor{result: "done", outcomes: []string{"/unverified", "/done"}, missing: []string{"/tests_not_written"}}
	v := newDelegationVerifier(t, client, nil, exec)

	blackboard := &types.SessionContext{}
	d := delegation("create internal/a/a.go", 2)
	d.SessionContext = blackboard
	d.Priority = types.PriorityHigh

	if _, _, err := v.VerifyWithRetry(context.Background(), d); err != nil {
		t.Fatalf("VerifyWithRetry: %v", err)
	}
	if len(exec.sessionCtxs) != 2 {
		t.Fatalf("%d attempt(s) reached the context-carrying entry point, want both", len(exec.sessionCtxs))
	}
	for i, got := range exec.sessionCtxs {
		if got != blackboard {
			t.Errorf("attempt %d ran with session context %p, want the delegation's %p", i+1, got, blackboard)
		}
		if exec.priorities[i] != types.PriorityHigh {
			t.Errorf("attempt %d ran at priority %v, want the delegation's %v", i+1, exec.priorities[i], types.PriorityHigh)
		}
	}
}
