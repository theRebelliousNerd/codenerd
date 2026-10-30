package system

import (
	"context"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/experts"
	"codenerd/internal/types"
)

func hasDomain(t *testing.T, k types.Kernel, agent, tech, source string) bool {
	t.Helper()
	rows, err := k.Query("shard_agent_domain")
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if types.ExtractString(r.Args[0]) == agent && types.ExtractString(r.Args[1]) == tech && types.ExtractString(r.Args[2]) == source {
			return true
		}
	}
	return false
}

// A roster refresh only asserted, so a domain the agent's sources no longer
// support stayed active in the running kernel and kept routing tasks to it.
// A refresh now retires what its last publication asserted and this one did
// not, and leaves facts other producers asserted alone.
func TestRosterRefresh_RetiresADomainNoSourceSupports(t *testing.T) {
	ws := t.TempDir()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	other := types.Fact{Predicate: "shard_agent_domain", Args: []any{experts.AgentAtom("OtherAgent"), types.MangleAtom("/go"), types.MangleAtom("/registry")}}
	if err := k.Assert(other); err != nil {
		t.Fatal(err)
	}
	writeAgentsJSON(t, ws, []map[string]any{{"name": "AgentA", "type": "persistent", "status": "ready", "domains": []string{"bubbletea"}}})
	if err := refreshShardRoster(context.Background(), k, ws, nil); err != nil {
		t.Fatalf("first refresh: %v", err)
	}
	if !hasDomain(t, k, "/agenta", "/bubbletea", "/registry") {
		t.Fatal("fixture: the registry domain was not published")
	}

	writeAgentsJSON(t, ws, []map[string]any{{"name": "AgentA", "type": "persistent", "status": "ready", "domains": []string{}}})
	if err := refreshShardRoster(context.Background(), k, ws, nil); err != nil {
		t.Fatalf("second refresh: %v", err)
	}
	if hasDomain(t, k, "/agenta", "/bubbletea", "/registry") {
		t.Fatal("the withdrawn domain stayed active after the refresh")
	}
	if rows, _ := k.Query("shard_agent"); len(rows) == 0 {
		t.Fatal("the agent itself was retired with its domain")
	}
	if !hasDomain(t, k, "/otheragent", "/go", "/registry") {
		t.Fatal("a fact another producer asserted was retired")
	}
}

// A source that could not be read measured nothing: its last published
// support stands until it is read again.
func TestReconcileRoster_KeepsTheSupportOfAnUnreadSource(t *testing.T) {
	a := experts.AgentAtom("AgentA")
	kb := types.Fact{Predicate: "shard_agent_domain", Args: []any{a, types.MangleAtom("/bubbletea"), types.MangleAtom("/knowledge_base")}}
	cat := types.Fact{Predicate: "shard_agent_domain", Args: []any{a, types.MangleAtom("/go"), types.MangleAtom("/catalog")}}
	agent := types.Fact{Predicate: "shard_agent", Args: []any{a}}
	prev := []types.Fact{agent, kb, cat}

	next, obsolete := reconcileRoster(prev, shardRoster{facts: []types.Fact{agent}, kbUnread: map[types.MangleAtom]bool{a: true}, catalogUnread: true})
	if len(next) != 3 || len(obsolete) != 0 {
		t.Fatalf("unread sources: next=%v obsolete=%v; want their support kept", next, obsolete)
	}
	next, obsolete = reconcileRoster(prev, shardRoster{facts: []types.Fact{agent}, kbUnread: map[types.MangleAtom]bool{}})
	if len(next) != 1 || len(obsolete) != 2 {
		t.Fatalf("read sources with no support: next=%v obsolete=%v; want both retired", next, obsolete)
	}
}
