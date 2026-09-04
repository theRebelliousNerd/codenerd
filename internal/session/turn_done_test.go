package session

import (
	"testing"

	"codenerd/internal/types"
)

// These tests pin the turn_done completion signal for create-file turns:
//
//	turn_done(Verb) :- turn_evidence(Verb, _, _, _, _, _),
//	    !has_hollow_success(), !build_state(/failing).
//
// declared in internal/core/defaults/policy/coder_safety.mg, against a real
// kernel loading the real policy corpus, so what fires here is what ships.
//
// A /create turn with no recognized write-mutation tool derives
// hollow_success and must NOT derive turn_done. A /create turn whose writes
// landed but whose build is red (build_state(/failing)) must NOT derive
// turn_done either. Deriving done in either case is hollow success with a
// policy stamp on it. A clean /create (writes landed, build not red) DOES
// derive turn_done, proving the negative tests fail for the right reason and
// not because turn_done never fires at all.
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

// A clean /create (write landed, build not red) derives done, proving the
// two gates above block for the right reason rather than turn_done never
// firing at all.
func TestTurnDone_CleanCreateDerivesDone(t *testing.T) {
	e := newObligationExec(t)
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
