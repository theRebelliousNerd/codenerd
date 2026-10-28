package core

import (
	"strings"
	"testing"
)

// A critical-path warning exists so a person decides: the approval of that
// exact request lifts it. A hard reason (go.mod) is never lifted, and an
// approval of another request lifts nothing.
func TestDreamGate_ApprovalLiftsOnlyLiftableReasons(t *testing.T) {
	vs := gateStore(t)
	grant := func(id, target string) {
		t.Helper()
		if err := vs.kernel.Assert(Fact{Predicate: "human_approval_granted", Args: []any{id, MangleAtom("/delete_file"), target, "{}", "sha256:test"}}); err != nil {
			t.Fatal(err)
		}
	}
	del := func(id, path string) error {
		return boundedPreflight(vs, id, "delete_file", map[string]any{"path": path})
	}

	if err := del("del-1", ".nerd/scratch.txt"); err == nil || !strings.Contains(err.Error(), "critical_path_missing") {
		t.Fatalf("an unapproved delete under a critical path passed: %v", err)
	}
	grant("del-other", ".nerd/scratch.txt")
	if err := del("del-1", ".nerd/scratch.txt"); err == nil {
		t.Fatal("an approval of a different request lifted the block")
	}
	grant("del-1", ".nerd/scratch.txt")
	if err := del("del-1", ".nerd/scratch.txt"); err != nil {
		t.Fatalf("the approval of this exact request did not lift a critical-path warning: %v", err)
	}
	grant("del-2", "go.mod")
	if err := del("del-2", "go.mod"); err == nil {
		t.Fatal("an approval lifted the hard go.mod block")
	}
}
