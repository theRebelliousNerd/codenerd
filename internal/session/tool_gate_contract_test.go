package session

import (
	"testing"

	"codenerd/internal/core"
)

// TestIntentRequiresToolCall_AdapterVerbs pins the executor's live hollow gate
// against the delegation policy for verbs the understanding adapter emits.
// A verb the policy gates must return true here (prose-only turns retry),
// and a prose-terminal verb must return false (direct answers pass through).
func TestIntentRequiresToolCall_AdapterVerbs(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	executor := &Executor{kernel: kernel}
	cases := map[string]bool{
		// New mappings: execution verbs the adapter emits.
		"/deploy":    true,
		"/configure": true,
		"/assault":   true,
		// Pre-existing behavior anchors.
		"/create":   true,
		"/migrate":  true,
		"/test":     true,
		"/document": true,
		// Prose terminals: must never force tool calls.
		"/converse": false,
		"/remember": false,
		"/forget":   false,
		"/explain":  false,
		"/review":   false,
		// Malformed input fails closed to false, never panics or queries.
		"":             false,
		"not-a-verb":   false,
		"/two/slashes": false,
		"with space":   false,
	}
	for verb, want := range cases {
		if got := executor.intentRequiresToolCall(verb); got != want {
			t.Errorf("intentRequiresToolCall(%q)=%v, want %v", verb, got, want)
		}
	}
}

// TestIntentRequiresToolCall_NilKernelFailsOpen documents the degraded mode:
// without a kernel the gate cannot consult policy, so it returns false and
// the turn proceeds rather than blocking every final answer on missing
// policy. Fail-open here is deliberate (see the method comment).
func TestIntentRequiresToolCall_NilKernelFailsOpen(t *testing.T) {
	executor := &Executor{}
	if executor.intentRequiresToolCall("/deploy") {
		t.Error("nil kernel: intentRequiresToolCall(/deploy)=true, want false (degraded mode)")
	}
}
