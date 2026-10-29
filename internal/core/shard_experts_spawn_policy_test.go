package core

import (
	"testing"

	"codenerd/internal/types"
)

func spawnsFor(t *testing.T, k *RealKernel, task string) map[string]string {
	t.Helper()
	rows, err := k.Query("expert_spawn")
	if err != nil {
		t.Fatalf("query expert_spawn: %v", err)
	}
	out := map[string]string{}
	for _, f := range rows {
		if types.ExtractString(f.Args[0]) == task {
			out[types.ExtractString(f.Args[1])] = types.ExtractString(f.Args[2])
		}
	}
	return out
}

// Spawn second: an executor routed to execute is consulted to lead; strong
// evidence on an agent with no executor classification is consulted to
// advise; a language match spawns no one; and a task that already runs as a
// shard agent (a consultation) spawns no one, so consultations never recurse.
func TestShardExpertPolicyDerivesExpertSpawn(t *testing.T) {
	k := expertPolicyKernel(t)
	facts := []types.Fact{
		// An unclassified shard agent whose domain is the Bubble Tea import.
		{Predicate: "shard_agent", Args: []any{types.MangleAtom("/tuireviewer")}},
		{Predicate: "shard_agent_domain", Args: []any{types.MangleAtom("/tuireviewer"), types.MangleAtom("/bubbletea"), types.MangleAtom("/knowledge_base")}},
		{Predicate: "expert_task", Args: []any{"t-tui", types.MangleAtom("/fix")}},
		{Predicate: "expert_task_target", Args: []any{"t-tui", "cmd/nerd/chat/view.go"}},
		{Predicate: "expert_task", Args: []any{"t-mg", types.MangleAtom("/fix")}},
		{Predicate: "expert_task_target", Args: []any{"t-mg", "a.mg"}},
		{Predicate: "expert_task_extension", Args: []any{"t-mg", "a.mg", ".mg"}},
		{Predicate: "expert_task", Args: []any{"t-consult", types.MangleAtom("/consult/bubbleteaexpert")}},
		{Predicate: "expert_task_target", Args: []any{"t-consult", "cmd/nerd/chat/view.go"}},
		{Predicate: "expert_task_speaker", Args: []any{"t-consult", types.MangleAtom("/bubbleteaexpert")}},
	}
	if err := k.AssertBatch(facts); err != nil {
		t.Fatalf("assert: %v", err)
	}
	if got := spawnsFor(t, k, "t-tui"); got["/bubbleteaexpert"] != "/lead" || got["/tuireviewer"] != "/advise" || len(got) != 2 {
		t.Errorf("expert_spawn for the Bubble Tea task = %v, want bubbleteaexpert /lead and tuireviewer /advise", got)
	}
	if got := spawnsFor(t, k, "t-mg"); len(got) != 0 {
		t.Errorf("expert_spawn for a language-only match = %v, want none", got)
	}
	if got := spawnsFor(t, k, "t-consult"); len(got) != 0 {
		t.Errorf("a consultation spawned %v; it must spawn no one", got)
	}
	if got := derivedFor(t, k, "expert_needed", "t-consult", 1); len(got) == 0 {
		t.Errorf("a consultation still carries its experts' knowledge; expert_needed = %v", got)
	}
}

// A roster agent with no recorded domains has every derived domain queued for
// recording; a recorded agent has none queued.
func TestShardExpertPolicyQueuesUnrecordedDomains(t *testing.T) {
	k := expertPolicyKernel(t)
	facts := []types.Fact{
		{Predicate: "shard_agent_recorded", Args: []any{types.MangleAtom("/mangleexpert")}},
		{Predicate: "shard_agent", Args: []any{types.MangleAtom("/goexpert")}},
		{Predicate: "shard_agent_domain", Args: []any{types.MangleAtom("/goexpert"), types.MangleAtom("/go"), types.MangleAtom("/catalog")}},
	}
	if err := k.AssertBatch(facts); err != nil {
		t.Fatalf("assert: %v", err)
	}
	rows, err := k.Query("shard_agent_domain_to_record")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	got := map[string]string{}
	for _, f := range rows {
		got[types.ExtractString(f.Args[0])] = types.ExtractString(f.Args[1])
	}
	if got["/goexpert"] != "/go" || got["/bubbleteaexpert"] != "/bubbletea" {
		t.Errorf("queued = %v, want goexpert /go and bubbleteaexpert /bubbletea", got)
	}
	if _, ok := got["/mangleexpert"]; ok {
		t.Errorf("a recorded agent was queued: %v", got)
	}
}
