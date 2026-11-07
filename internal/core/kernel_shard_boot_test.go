package core

import (
	"fmt"
	"testing"
)

// Shards of one workspace share a single kernel boot, yet each owns its kernel:
// a fact written through one is invisible to its siblings and to a shard of a
// different boot key. Order follows the configs, and a config without a domain
// is refused before anything boots.
func TestNewKernelShards_SiblingsShareABootAndOwnTheirKernels(t *testing.T) {
	t.Parallel()
	ws, other := t.TempDir(), t.TempDir()
	configs := []KernelShardConfig{
		{Domain: "alpha", WorkspaceRoot: ws, OwnedPredicates: []string{"a_pred"}},
		{Domain: "beta", WorkspaceRoot: ws, OwnedPredicates: []string{"b_pred"}},
		{Domain: "gamma", WorkspaceRoot: other},
		{Domain: "delta", WorkspaceRoot: ws},
	}
	shards, err := NewKernelShards(configs)
	if err != nil {
		t.Fatalf("NewKernelShards: %v", err)
	}
	if len(shards) != len(configs) {
		t.Fatalf("got %d shards for %d configs", len(shards), len(configs))
	}
	for i, s := range shards {
		if s.Domain() != configs[i].Domain {
			t.Fatalf("shard %d is %q, want %q", i, s.Domain(), configs[i].Domain)
		}
	}
	if !shards[0].OwnsPredicate("a_pred") || shards[0].OwnsPredicate("b_pred") || !shards[1].OwnsPredicate("b_pred") {
		t.Fatal("shards lost their own predicate ownership")
	}
	if shards[0].Kernel() == shards[1].Kernel() || shards[1].Kernel() == shards[3].Kernel() {
		t.Fatal("sibling shards share one kernel object")
	}
	want := shards[0].Kernel().FactCount()
	for i, s := range shards {
		// A shard stays reloadable (ReloadConstitution): the constitution/extension
		// split of the boot survives the clone.
		if k := s.Kernel(); k.constSchemas == "" || k.constPolicy == "" ||
			k.constSchemas != shards[0].Kernel().constSchemas || k.constPolicy != shards[0].Kernel().constPolicy {
			t.Fatalf("shard %d lost the constitution split its boot recorded", i)
		}
		if got := s.Kernel().FactCount(); got != want {
			t.Fatalf("shard %d boots with %d facts, shard 0 with %d", i, got, want)
		}
		if got := s.Kernel().GetWorkspace(); got != configs[i].WorkspaceRoot {
			t.Fatalf("shard %d workspace = %q, want %q", i, got, configs[i].WorkspaceRoot)
		}
	}

	for i, s := range shards {
		fact := Fact{Predicate: "user_intent", Args: []any{fmt.Sprintf("/shard_%d", i), "/query", "/explain", "target", ""}}
		if err := s.Kernel().Assert(fact); err != nil {
			t.Fatalf("assert on shard %d: %v", i, err)
		}
	}
	for i, s := range shards {
		rows, err := s.Kernel().Query("user_intent")
		if err != nil {
			t.Fatal(err)
		}
		if len(rows) != 1 || fmt.Sprint(rows[0].Args[0]) != fmt.Sprintf("/shard_%d", i) {
			t.Fatalf("shard %d sees %d user_intent rows %v, want only its own", i, len(rows), rows)
		}
	}

	if _, err := NewKernelShards([]KernelShardConfig{{Domain: "ok", WorkspaceRoot: ws}, {WorkspaceRoot: ws}}); err == nil {
		t.Fatal("a config without a domain was accepted")
	}
}
