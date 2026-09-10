package chat

import (
	"context"
	"testing"
)

const failingGoOutput = `=== RUN   TestAlpha
--- PASS: TestAlpha (0.00s)
=== RUN   TestBeta
    beta_test.go:12: want 5, got 0
--- FAIL: TestBeta (0.01s)
FAIL`

const passingGoOutput = `=== RUN   TestAlpha
--- PASS: TestAlpha (0.00s)
ok  	codenerd/internal/example	0.01s`

// The whole chain, end to end, because every link in it was broken and each
// break was invisible on its own.
//
// A failing suite has to reach the prompt as two separate signals: TestState
// puts the compile into /tdd_repair, and the count of failing tests arms the
// failing_tests world state. The three TDD atoms in methodology/tdd.yaml are
// gated on BOTH, so either one missing is the same as neither -- the guidance
// for repairing a failing suite silently does not load at the one moment
// anybody wants it.
func TestFailingTesterOutputReachesTDDRepairMode(t *testing.T) {
	m, _ := SetupLiveModel(t)
	m.shardResultHistory = append(m.shardResultHistory, &ShardResult{
		ShardType: "tester",
		RawOutput: failingGoOutput,
	})

	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx == nil {
		t.Fatal("buildSessionContext returned nil")
	}

	if sessionCtx.TestState != "/failing" {
		t.Errorf("TestState = %q, want \"/failing\"", sessionCtx.TestState)
	}
	if len(sessionCtx.FailingTests) != 1 || sessionCtx.FailingTests[0] != "TestBeta" {
		t.Errorf("FailingTests = %v, want [TestBeta]", sessionCtx.FailingTests)
	}

	// The other half of the chain -- that these two fields actually produce
	// /tdd_repair and the failing_tests world state -- is asserted in
	// internal/articulation, where the compile context is built. Neither half
	// is worth much alone: this one would pass just as happily if the compile
	// stopped reading these fields.
}

// A passing suite must not put the session into repair mode. /tdd_repair
// displaces /active, so a false positive here does not merely add atoms -- it
// changes which operational mode the whole compile selects for.
func TestPassingTesterOutputLeavesModeAlone(t *testing.T) {
	m, _ := SetupLiveModel(t)
	m.shardResultHistory = append(m.shardResultHistory, &ShardResult{
		ShardType: "tester",
		RawOutput: passingGoOutput,
	})

	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx.TestState != "/passing" {
		t.Errorf("TestState = %q, want \"/passing\"", sessionCtx.TestState)
	}
	if len(sessionCtx.FailingTests) != 0 {
		t.Errorf("FailingTests = %v, want none", sessionCtx.FailingTests)
	}
}

// Output this parser cannot read is not a verdict. Treating it as failing would
// put the model into repair mode over a runner nobody taught it to read;
// treating it as passing would hide a real failure. Saying nothing is the only
// honest answer, and it is what the previous implementation accidentally did
// for every input.
func TestUnreadableTesterOutputAssertsNothing(t *testing.T) {
	m, _ := SetupLiveModel(t)
	m.shardResultHistory = append(m.shardResultHistory, &ShardResult{
		ShardType: "tester",
		RawOutput: "Segmentation fault (core dumped)",
	})

	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx.TestState != "" {
		t.Errorf("TestState = %q, want empty: this output states no verdict", sessionCtx.TestState)
	}
}

// A tester result from three turns ago describes a tree that has since been
// edited. Reporting its failures sends the model at tests that may already
// pass, which is worse than saying nothing because it looks current.
func TestMostRecentTesterResultWins(t *testing.T) {
	m, _ := SetupLiveModel(t)
	m.shardResultHistory = append(m.shardResultHistory,
		&ShardResult{ShardType: "tester", RawOutput: failingGoOutput},
		&ShardResult{ShardType: "coder", RawOutput: "wrote some code"},
		&ShardResult{ShardType: "tester", RawOutput: passingGoOutput},
	)

	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx.TestState != "/passing" {
		t.Errorf("TestState = %q, want \"/passing\": the newer tester run said the suite passes",
			sessionCtx.TestState)
	}
}
