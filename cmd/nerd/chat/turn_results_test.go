package chat

import (
	"context"
	"strings"
	"testing"
	"time"

	"codenerd/internal/core"
)

func execRow(id string) core.Fact {
	return core.Fact{Predicate: "execution_result", Args: []any{id, "/read_file", "main.go", "/true", "ok " + id, int64(1)}}
}

func routeRow(id, status, detail string) core.Fact {
	return core.Fact{Predicate: "routing_result", Args: []any{id, status, detail, int64(1)}}
}

func assertAll(t *testing.T, m Model, facts ...core.Fact) {
	t.Helper()
	for _, f := range facts {
		if err := m.kernel.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
}

func resultIDs(facts []core.Fact) string {
	var ids []string
	for _, f := range facts {
		ids = append(ids, f.Args[0].(string))
	}
	return strings.Join(ids, ",")
}

// A turn's results are the rows that were not standing, not rows[count:]. The
// previous turn leaves one row; this turn records its baseline, the chat clears
// the predicate after perception, and two new results arrive. The old count
// baseline (1) would have returned nothing for one new row and dropped one of
// two; identity returns every row the turn produced.
func TestSystemResults_ClearThenNewRowsAreAllVisible(t *testing.T) {
	t.Parallel()
	m := newRoundtripModel(t)
	assertAll(t, m, execRow("prev-1"))

	m.clearStaleKernelFacts() // chat clears after perception
	base := m.snapshotSystemResults()
	if got := m.newSystemResults("execution_result", base); len(got) != 0 {
		t.Fatalf("a cleared kernel has no new rows, got %s", resultIDs(got))
	}

	assertAll(t, m, execRow("a-1"))
	if got := resultIDs(m.newSystemResults("execution_result", base)); got != "a-1" {
		t.Fatalf("one new result after a clear = %q, want a-1 (a count baseline of 1 hid it)", got)
	}
	assertAll(t, m, execRow("a-2"))
	got := m.newSystemResults("execution_result", base)
	if len(got) != 2 {
		t.Fatalf("two new results = %q, want both (a count baseline of 1 dropped one)", resultIDs(got))
	}
}

// A row that stood when the turn began is never the turn's, however the kernel
// orders or prunes the rest.
func TestSystemResults_StandingRowsAreExcludedByIdentity(t *testing.T) {
	t.Parallel()
	m := newRoundtripModel(t)
	assertAll(t, m, execRow("old-1"), execRow("old-2"), routeRow("old-1", "/success", "x"))
	base := m.snapshotSystemResults()

	if err := m.kernel.RetractFact(execRow("old-1")); err != nil { // pruned
		t.Fatalf("retract: %v", err)
	}
	assertAll(t, m, execRow("new-1"))
	if got := resultIDs(m.newSystemResults("execution_result", base)); got != "new-1" {
		t.Fatalf("new results = %q, want only new-1 (a count would have returned old-2 or nothing)", got)
	}
	if got := m.newSystemResults("routing_result", base); len(got) != 0 {
		t.Fatalf("the standing routing row was reported as new: %v", got)
	}
}

// waitForSystemResults returns on the first new row of either predicate and
// reports an empty timeout as nothing observed.
func TestWaitForSystemResults_ReturnsNewRowsAndTimesOutEmpty(t *testing.T) {
	t.Parallel()
	m := newRoundtripModel(t)
	base := m.snapshotSystemResults()

	routing, exec := m.waitForSystemResults(context.Background(), base, 150*time.Millisecond)
	if len(routing)+len(exec) != 0 {
		t.Fatalf("nothing happened, observed %v %v", routing, exec)
	}

	assertAll(t, m, execRow("z-1"))
	routing, exec = m.waitForSystemResults(context.Background(), base, 2*time.Second)
	if len(routing) != 0 || resultIDs(exec) != "z-1" {
		t.Fatalf("observed routing=%v exec=%s, want exec z-1", routing, resultIDs(exec))
	}
}

// A routing failure is a typed outcome: the action never ran. One that also has
// an execution row is a result, not a blocker.
func TestUnroutedActions_NamesFailuresWithoutExecution(t *testing.T) {
	routing := []core.Fact{
		routeRow("a-1", "/failure", "no_handler"),
		routeRow("a-2", "/success", "done"),
		routeRow("a-3", "/failure", "rate_limit_exceeded"),
		routeRow("a-4", "/failure", "tool error"),
	}
	exec := []core.Fact{execRow("a-4")}
	notes := unroutedActions(routing, exec)
	if len(notes) != 2 {
		t.Fatalf("notes = %v, want a-1 and a-3 only", notes)
	}
	if !strings.Contains(notes[0], "a-1") || !strings.Contains(notes[0], "no_handler") ||
		!strings.Contains(notes[1], "a-3") || !strings.Contains(notes[1], "rate_limit_exceeded") {
		t.Fatalf("notes do not name the action and reason: %v", notes)
	}
}
