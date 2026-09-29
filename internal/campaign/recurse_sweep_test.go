package campaign

import (
	"context"
	"slices"
	"strings"
	"testing"
)

func sweepIDs(nodes []SubsystemNode) []string {
	ids := make([]string, len(nodes))
	for i, n := range nodes {
		ids[i] = n.ID
	}
	return ids
}

// The order the kernel derives for the fixture is the order TopoOrder
// produces, including its lexicographic tie-break, and it does not depend on
// the slice the graph was handed in.
func TestPolicySweepOrder_MatchesTopoOrderOnTheFixture(t *testing.T) {
	want, err := TopoOrder(fixtureDAG())
	if err != nil {
		t.Fatal(err)
	}
	k := newRecursePolicy(t).k
	got, err := policySweepOrder(k, fixtureDAG())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sweepIDs(got), sweepIDs(want)) {
		t.Fatalf("policy %v, topo %v", sweepIDs(got), sweepIDs(want))
	}

	shuffled := fixtureDAG()
	for i, j := 0, len(shuffled)-1; i < j; i, j = i+1, j-1 {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}
	again, err := policySweepOrder(k, shuffled)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sweepIDs(again), sweepIDs(want)) {
		t.Fatalf("shuffled input changed the order: %v, want %v", sweepIDs(again), sweepIDs(want))
	}
}

// RecurseSweepOrder, the plan printer, is that same order, and a focused
// sweep is the focused subgraph's order.
func TestRecurseSweepOrder_MatchesTopoOrderOnTheFixture(t *testing.T) {
	useFixtureDAG(t)
	want, err := TopoOrder(fixtureDAG())
	if err != nil {
		t.Fatal(err)
	}
	got, err := RecurseSweepOrder(context.Background(), t.TempDir(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sweepIDs(got), sweepIDs(want)) {
		t.Fatalf("plan order %v, topo %v", sweepIDs(got), sweepIDs(want))
	}
	focused, err := FilterDAG(fixtureDAG(), []string{"session"})
	if err != nil {
		t.Fatal(err)
	}
	wantFocused, err := TopoOrder(focused)
	if err != nil {
		t.Fatal(err)
	}
	gotFocused, err := RecurseSweepOrder(context.Background(), t.TempDir(), []string{"session"})
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sweepIDs(gotFocused), sweepIDs(wantFocused)) {
		t.Fatalf("focused plan %v, topo %v", sweepIDs(gotFocused), sweepIDs(wantFocused))
	}
}

// A pass is complete exactly when every node has been visited, and a node
// already visited is not next. That is the resume skip.
func TestSweepPass_CompletesWhenEveryNodeWasVisitedAndSkipsTheRest(t *testing.T) {
	k := newRecursePolicy(t).k
	nodes := fixtureDAG()
	want, err := TopoOrder(nodes)
	if err != nil {
		t.Fatal(err)
	}
	if err := assertSweepGraph(k, nodes); err != nil {
		t.Fatal(err)
	}
	const pass = 4
	if err := assertSweepPass(k, pass); err != nil {
		t.Fatal(err)
	}
	complete, err := sweepPassComplete(k, pass)
	if err != nil || complete {
		t.Fatalf("complete = %v, %v; an unvisited graph is not a finished pass", complete, err)
	}
	// The first three are already done, as a resumed pass would assert.
	for _, n := range want[:3] {
		if err := assertNodeVisited(k, pass, n.ID); err != nil {
			t.Fatal(err)
		}
	}
	id, err := sweepNextNode(k)
	if err != nil {
		t.Fatal(err)
	}
	if id != want[3].ID {
		t.Fatalf("next = %q, want %q (the first three are already visited)", id, want[3].ID)
	}
	for _, n := range want[3:] {
		got, err := sweepNextNode(k)
		if err != nil {
			t.Fatal(err)
		}
		if got != n.ID {
			t.Fatalf("next = %q, want %q", got, n.ID)
		}
		if err := assertNodeVisited(k, pass, got); err != nil {
			t.Fatal(err)
		}
		complete, err = sweepPassComplete(k, pass)
		if err != nil {
			t.Fatal(err)
		}
		last := n.ID == want[len(want)-1].ID
		if complete != last {
			t.Fatalf("after %q, complete = %v, want %v", n.ID, complete, last)
		}
	}
	id, err = sweepNextNode(k)
	if err != nil || id != "" {
		t.Fatalf("a finished pass's next node = %q, %v", id, err)
	}
	// Another pass has visited nothing.
	if err := assertSweepPass(k, pass+1); err != nil {
		t.Fatal(err)
	}
	complete, err = sweepPassComplete(k, pass+1)
	if err != nil || complete {
		t.Fatalf("pass %d complete = %v, %v", pass+1, complete, err)
	}
}

// An empty sweep is a finished pass: there is no node to visit.
func TestSweepPass_AnEmptyGraphIsComplete(t *testing.T) {
	k := newRecursePolicy(t).k
	got, err := policySweepOrder(k, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("order = %v", sweepIDs(got))
	}
	complete, err := sweepPassComplete(k, 0)
	if err != nil || !complete {
		t.Fatalf("complete = %v, %v", complete, err)
	}
}

// The run stops when the budget is spent, and not while it is unbounded.
func TestSweepRun_StopsWhenTheBudgetIsSpent(t *testing.T) {
	k := newRecursePolicy(t).k
	stop, err := sweepRunStop(k)
	if err != nil || stop {
		t.Fatalf("no budget asserted: stop = %v, %v", stop, err)
	}
	cases := []struct {
		budget, finished int
		want             bool
	}{
		{0, 0, false},
		{0, 9, false},
		{3, 0, false},
		{3, 2, false},
		{3, 3, true},
		{3, 4, true},
		{1, 1, true},
	}
	for _, tc := range cases {
		if err := assertSweepBudget(k, tc.budget, tc.finished); err != nil {
			t.Fatal(err)
		}
		stop, err := sweepRunStop(k)
		if err != nil {
			t.Fatal(err)
		}
		if stop != tc.want {
			t.Fatalf("budget %d finished %d: stop = %v, want %v", tc.budget, tc.finished, stop, tc.want)
		}
	}
}

// A cycle is not given an order. The soundness walk reports it and leaves no
// probe visits behind for the real pass to skip.
func TestSweepGraph_ACycleIsNotAnOrder(t *testing.T) {
	k := newRecursePolicy(t).k
	cycle := []SubsystemNode{
		{ID: "a", DependsOn: []string{"b"}},
		{ID: "b", DependsOn: []string{"a"}},
	}
	if _, err := policySweepOrder(k, cycle); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("policySweepOrder err = %v, want a cycle", err)
	}
	if err := sweepGraphSound(k, fixtureDAG()); err != nil {
		t.Fatal(err)
	}
	rows, err := k.Query("recurse_node_visited")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 0 {
		t.Fatalf("probe visits left behind: %+v", rows)
	}
	if err := sweepGraphSound(k, cycle); err == nil || !strings.Contains(err.Error(), "cycle") {
		t.Fatalf("sweepGraphSound err = %v, want a cycle", err)
	}
}

func TestSweepGraph_RejectsAMalformedMeasurement(t *testing.T) {
	k := newRecursePolicy(t).k
	cases := []struct {
		name  string
		nodes []SubsystemNode
		want  string
	}{
		{"duplicate", []SubsystemNode{{ID: "a", Title: "a"}, {ID: "a", Title: "a"}}, "duplicate"},
		{"unknown dep", []SubsystemNode{{ID: "a", DependsOn: []string{"ghost"}}}, "unknown"},
		{"self dep", []SubsystemNode{{ID: "a", DependsOn: []string{"a"}}}, "itself"},
		{"empty id", []SubsystemNode{{Title: "nameless"}}, "empty ID"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := policySweepOrder(k, tc.nodes); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want containing %q", err, tc.want)
			}
		})
	}
}

// The two roots are ordered by identity, not by the slice they arrived in.
func TestPolicySweepOrder_TieBreakIsLexicographic(t *testing.T) {
	k := newRecursePolicy(t).k
	nodes := []SubsystemNode{
		{ID: "z", Title: "z"},
		{ID: "m", Title: "m", DependsOn: []string{"z", "a"}},
		{ID: "a", Title: "a"},
	}
	got, err := policySweepOrder(k, nodes)
	if err != nil {
		t.Fatal(err)
	}
	want, err := TopoOrder(nodes)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sweepIDs(got), sweepIDs(want)) {
		t.Fatalf("policy %v, topo %v", sweepIDs(got), sweepIDs(want))
	}
	if !slices.Equal(sweepIDs(got), []string{"a", "z", "m"}) {
		t.Fatalf("order = %v, want a then z then m", sweepIDs(got))
	}
}
