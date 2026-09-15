package core

import (
	"context"
	"strings"
	"testing"
)

// A denied action must tell the agent WHAT was refused and WHY in the error
// string itself: that string is the only denial feedback that reaches model
// context. A bare "action write_file not permitted by kernel policy" sent a
// live campaign agent in circles — it never learned it had aimed at a
// directory. The target, the rule identity, and a remediation hint must all
// survive into the message.
func TestRouteAction_WhenKernelDenies_ErrorNamesTargetAndRule(t *testing.T) {
	vs := NewVirtualStoreWithConfig(nil, DefaultVirtualStoreConfig())
	vs.DisableBootGuard()
	vs.SetKernel(&stubKernel{
		permitted: []Fact{{Predicate: "permitted", Args: []any{"/list_files"}}},
	})

	// read_file is not destructive, so this reaches the kernel permission
	// gate rather than the dreamer gate.
	_, err := vs.RouteAction(context.Background(), Fact{
		Predicate: "next_action",
		Args:      []any{"act_deny_target", "/read_file", "/repo/internal/mangle", map[string]any{"content": "x"}},
	})
	if err == nil {
		t.Fatal("expected the action to be denied")
	}
	msg := err.Error()
	// The classifier heuristics still match on this core phrase; keep it.
	if !strings.Contains(msg, "not permitted by kernel policy") {
		t.Fatalf("denial lost its stable core phrase: %q", msg)
	}
	if !strings.Contains(msg, "/repo/internal/mangle") {
		t.Fatalf("denial does not name the target: %q", msg)
	}
	if !strings.Contains(msg, "permitted/3") {
		t.Fatalf("denial does not name the rule identity: %q", msg)
	}
	if !strings.Contains(msg, "content") {
		t.Fatalf("denial does not list payload keys: %q", msg)
	}
	if !strings.Contains(strings.ToLower(msg), "directory") {
		t.Fatalf("denial gives no remediation hint: %q", msg)
	}
}
