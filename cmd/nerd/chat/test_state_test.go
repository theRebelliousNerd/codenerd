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

// TDDRetryCount is the third field of the TEST STATE block and had the same
// defect as the other two: two production readers — prompt_assembler.go and
// shards/agents.go, both rendering "TDD Retry: N (fix root cause, not
// symptoms)" into the prompt — and nothing in the repository writing it. Two
// tests set it by hand, which is exactly how a field survives with no
// producer: the readers are exercised, so the feature looks covered.
func TestRepeatedFailuresArmTheStopTreatingSymptomsSignal(t *testing.T) {
	tests := []struct {
		name    string
		history []*ShardResult
		want    int
	}{
		{
			name:    "the first failure is not a retry",
			history: []*ShardResult{tester(failingGoOutput)},
			// Zero, and the readers print nothing at zero. A "TDD Retry: 1" on
			// a suite that has only just gone red tells a model that has not
			// attempted anything yet to stop treating symptoms.
			want: 0,
		},
		{
			name:    "failing twice in a row means one attempt has already been made",
			history: []*ShardResult{tester(failingGoOutput), tester(failingGoOutput)},
			want:    1,
		},
		{
			name: "a passing run ends the loop",
			history: []*ShardResult{
				tester(failingGoOutput),
				tester(failingGoOutput),
				tester(passingGoOutput),
				tester(failingGoOutput),
			},
			// This is the depth of the CURRENT repair loop, not a tally of
			// everything that has ever gone red. Green in between means what
			// came after is a new problem, and carrying the old count into it
			// would tell the model it is three attempts deep into something it
			// has not started.
			want: 0,
		},
		{
			name: "other shards between two tester runs are not the loop",
			history: []*ShardResult{
				tester(failingGoOutput),
				{ShardType: "coder", RawOutput: "wrote the fix"},
				{ShardType: "reviewer", RawOutput: "looks fine"},
				tester(failingGoOutput),
			},
			// A coder run between two failures is the retry, not a break in it.
			want: 1,
		},
		{
			name: "a runner the parser cannot read is neither a retry nor a fix",
			history: []*ShardResult{
				tester(failingGoOutput),
				tester("pytest exited 1; no summary line this parser knows"),
				tester(failingGoOutput),
			},
			// Skipped rather than counted or treated as green: unreadable
			// output is not evidence of a repair and not evidence of one
			// landing.
			want: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, _ := SetupLiveModel(t)
			m.shardResultHistory = append(m.shardResultHistory, tt.history...)

			sessionCtx := m.buildSessionContext(context.Background())
			if sessionCtx == nil {
				t.Fatal("buildSessionContext returned nil")
			}
			if sessionCtx.TDDRetryCount != tt.want {
				t.Errorf("TDDRetryCount = %d, want %d", sessionCtx.TDDRetryCount, tt.want)
			}
		})
	}
}

// And a suite that has gone green must not carry a retry count into the next
// turn. The count is only meaningful alongside /failing; a nonzero one on a
// passing suite would put "fix root cause, not symptoms" in the prompt of a
// turn with nothing to fix.
func TestAPassingSuiteReportsNoRetryCount(t *testing.T) {
	m, _ := SetupLiveModel(t)
	m.shardResultHistory = append(m.shardResultHistory,
		tester(failingGoOutput), tester(failingGoOutput), tester(passingGoOutput))

	sessionCtx := m.buildSessionContext(context.Background())
	if sessionCtx == nil {
		t.Fatal("buildSessionContext returned nil")
	}
	if sessionCtx.TestState != "/passing" {
		t.Fatalf("TestState = %q, want \"/passing\"", sessionCtx.TestState)
	}
	if sessionCtx.TDDRetryCount != 0 {
		t.Errorf("TDDRetryCount = %d on a passing suite, want 0", sessionCtx.TDDRetryCount)
	}
}

func tester(out string) *ShardResult {
	return &ShardResult{ShardType: "tester", RawOutput: out}
}
