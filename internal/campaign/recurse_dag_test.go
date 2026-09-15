package campaign

import (
	"strings"
	"testing"
)

func TestTopoOrder_RespectsDependencies(t *testing.T) {
	ordered, err := TopoOrder(RecurseDAG())
	if err != nil {
		t.Fatalf("TopoOrder: %v", err)
	}
	if len(ordered) != len(RecurseDAG()) {
		t.Fatalf("ordered %d nodes, want %d", len(ordered), len(RecurseDAG()))
	}
	position := make(map[string]int, len(ordered))
	for i, n := range ordered {
		position[n.ID] = i
	}
	for _, n := range ordered {
		for _, dep := range n.DependsOn {
			if position[dep] >= position[n.ID] {
				t.Errorf("dependency %q sweeps at %d, after dependent %q at %d",
					dep, position[dep], n.ID, position[n.ID])
			}
		}
	}
	if ordered[0].ID != "mangle" {
		t.Errorf("first node = %q, want mangle (the base primitive)", ordered[0].ID)
	}
	last := ordered[len(ordered)-1].ID
	if last != "bench" {
		t.Errorf("last node = %q, want bench (prove the gains last)", last)
	}
	// The wave closes across, then review, then bench — in that order.
	if !(position["wiring"] < position["review"] && position["review"] < position["bench"]) {
		t.Errorf("closing order broken: wiring=%d review=%d bench=%d",
			position["wiring"], position["review"], position["bench"])
	}
}

func TestTopoOrder_RejectsBadGraphs(t *testing.T) {
	good := SubsystemNode{ID: "a", Title: "a"}
	cases := []struct {
		name  string
		nodes []SubsystemNode
		want  string
	}{
		{"duplicate", []SubsystemNode{good, good}, "duplicate"},
		{"unknown dep", []SubsystemNode{{ID: "a", DependsOn: []string{"ghost"}}}, "unknown"},
		{"self dep", []SubsystemNode{{ID: "a", DependsOn: []string{"a"}}}, "itself"},
		{"cycle", []SubsystemNode{{ID: "a", DependsOn: []string{"b"}}, {ID: "b", DependsOn: []string{"a"}}}, "cycle"},
		{"empty id", []SubsystemNode{{Title: "nameless"}}, "empty ID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := TopoOrder(tc.nodes); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

func TestTopoOrder_IsStableUnderShuffle(t *testing.T) {
	dag := RecurseDAG()
	// Reverse the input: the lexicographic tie-break must still yield the
	// same order, or wave plans would wobble run to run.
	for i, j := 0, len(dag)-1; i < j; i, j = i+1, j-1 {
		dag[i], dag[j] = dag[j], dag[i]
	}
	a, err := TopoOrder(RecurseDAG())
	if err != nil {
		t.Fatal(err)
	}
	b, err := TopoOrder(dag)
	if err != nil {
		t.Fatal(err)
	}
	for i := range a {
		if a[i].ID != b[i].ID {
			t.Fatalf("position %d: %q vs %q — order depends on input shuffle", i, a[i].ID, b[i].ID)
		}
	}
}

func TestFilterDAG_PullsDependencies(t *testing.T) {
	filtered, err := FilterDAG(RecurseDAG(), []string{"session"})
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, n := range filtered {
		ids[n.ID] = true
	}
	for _, want := range []string{"mangle", "kernel", "store", "context", "perception", "prompt", "tools", "session"} {
		if !ids[want] {
			t.Errorf("focused sweep on session is missing premise %q", want)
		}
	}
	for _, excluded := range []string{"cli", "campaign", "bench"} {
		if ids[excluded] {
			t.Errorf("focused sweep on session wrongly includes %q", excluded)
		}
	}
	if _, err := TopoOrder(filtered); err != nil {
		t.Fatalf("filtered DAG does not order: %v", err)
	}
}

func TestFilterDAG_UnknownFails(t *testing.T) {
	if _, err := FilterDAG(RecurseDAG(), []string{"sauron"}); err == nil {
		t.Fatal("unknown subsystem must fail closed")
	}
	if _, err := FilterDAG(RecurseDAG(), nil); err != nil {
		t.Fatalf("empty filter must keep everything: %v", err)
	}
}
