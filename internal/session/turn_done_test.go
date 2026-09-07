package session

import (
	"testing"

	"codenerd/internal/types"
)

// These tests exercise the loaded production completion rules. Executed tools
// and an absence of hollow success establish turn_executed. turn_done also
// needs a host-issued acceptance witness; a clean write alone is insufficient.
type turnCounts struct {
	tools  int
	writes int
	tests  int
}

func assertTurnEvidence(t *testing.T, e *Executor, verb string, c turnCounts) {
	t.Helper()
	fact := types.Fact{
		Predicate: "turn_evidence",
		Args: []any{
			types.MangleAtom(verb),
			c.tools,
			c.writes,
			c.tests,
			types.MangleAtom("/false"),
			types.MangleAtom("/false"),
		},
	}
	if err := e.kernel.Assert(fact); err != nil {
		t.Fatalf("assert turn_evidence(%q, %+v): %v", verb, c, err)
	}
}

func assertBuildFailing(t *testing.T, e *Executor) {
	t.Helper()
	if err := e.kernel.Assert(types.Fact{
		Predicate: "build_state",
		Args:      []any{types.MangleAtom("/failing")},
	}); err != nil {
		t.Fatalf("assert build_state(/failing): %v", err)
	}
}

func queryCount(t *testing.T, e *Executor, predicate string) int {
	t.Helper()
	facts, err := e.kernel.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	return len(facts)
}

// A /create with tool calls but no write-mutation tool is hollow, so done
// must not derive.
func TestTurnDone_NoWriteCannotDeriveDone(t *testing.T) {
	e := newObligationExec(t)
	assertTurnEvidence(t, e, "/create", turnCounts{tools: 1})

	if got := queryCount(t, e, "hollow_success"); got == 0 {
		t.Fatal("expected hollow_success for /create with no write tool; a zero-result query cannot pin the done gate")
	}
	if facts, err := e.kernel.Query("turn_done"); err != nil {
		t.Fatalf("query turn_done: %v", err)
	} else if len(facts) != 0 {
		t.Fatalf("turn_done must not derive for a no-write /create, got %v", facts)
	}
}

// A /create whose writes landed but whose build is red is not done either,
// even though no hollow_success fires.
func TestTurnDone_FailedBuildCannotDeriveDone(t *testing.T) {
	e := newObligationExec(t)
	assertBuildFailing(t, e)
	assertTurnEvidence(t, e, "/create", turnCounts{tools: 1, writes: 1})

	if facts, err := e.kernel.Query("turn_done"); err != nil {
		t.Fatalf("query turn_done: %v", err)
	} else if len(facts) != 0 {
		t.Fatalf("turn_done must not derive while build_state(/failing) holds, got %v", facts)
	}
}

// A clean /create with explicit acceptance derives done, proving the
// two gates above block for the right reason rather than turn_done never
// firing at all.
func TestTurnDone_CleanCreateDerivesDone(t *testing.T) {
	e := newObligationExec(t)
	if err := e.kernel.Assert(types.Fact{Predicate: "turn_acceptance", Args: []any{types.MangleAtom("/create"), "caller-contract", "current-snapshot"}}); err != nil {
		t.Fatal(err)
	}

	assertTurnEvidence(t, e, "/create", turnCounts{tools: 1, writes: 1})

	if got := queryCount(t, e, "hollow_success"); got != 0 {
		t.Fatalf("clean /create must not derive hollow_success, got %d facts", got)
	}
	facts, err := e.kernel.Query("turn_done")
	if err != nil {
		t.Fatalf("query turn_done: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("clean /create must derive exactly one turn_done, got %v", facts)
	}
}

func TestTurnDone_ExecutedIsNotVerified(t *testing.T) {
	e := newObligationExec(t)
	assertTurnEvidence(t, e, "/create", turnCounts{tools: 1, writes: 1})
	if queryCount(t, e, "turn_executed") != 1 {
		t.Fatal("expected executed action")
	}
	if queryCount(t, e, "turn_done") != 0 {
		t.Fatal("missing acceptance must not derive done")
	}
}
