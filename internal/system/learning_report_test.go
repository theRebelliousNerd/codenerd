package system

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

func costFact(session string, turn int, prompt, completion, tools int64, outcome string) types.Fact {
	return types.Fact{Predicate: "turn_cost", Args: []any{
		session, int64(turn), prompt, completion, tools, types.MangleAtom(outcome),
	}}
}

func TestLearningReportFrom_PartitionsByKernelVerdict(t *testing.T) {
	r := LearningReportFrom([]types.Fact{
		costFact("s", 1, 1000, 200, 3, "/done"),
		costFact("s", 2, 2000, 400, 5, "/hollow"),
		costFact("s", 3, 500, 100, 1, "/failed"),
		costFact("s", 4, 300, 50, 0, "/unverified"),
	})

	if r.Turns != 4 {
		t.Fatalf("Turns = %d, want 4", r.Turns)
	}
	if r.Verified != 1 || r.Hollow != 1 || r.Failed != 1 || r.Unverified != 1 {
		t.Errorf("partition = verified:%d hollow:%d failed:%d unverified:%d, want one each",
			r.Verified, r.Hollow, r.Failed, r.Unverified)
	}
	if r.PromptTokens != 3800 || r.CompletionTokens != 750 || r.ToolCalls != 9 {
		t.Errorf("spend = %d prompt / %d completion / %d tools",
			r.PromptTokens, r.CompletionTokens, r.ToolCalls)
	}
}

// TestLearningReport_HollowCountsAgainstYou: the whole point of the number is
// that a turn which claimed success without evidence must not be able to
// improve the cost-per-verified-work figure.
func TestLearningReport_HollowCountsAgainstYou(t *testing.T) {
	honest := LearningReportFrom([]types.Fact{
		costFact("s", 1, 1000, 0, 1, "/done"),
	})
	withHollow := LearningReportFrom([]types.Fact{
		costFact("s", 1, 1000, 0, 1, "/done"),
		costFact("s", 2, 1000, 0, 1, "/hollow"),
	})

	if withHollow.TokensPerVerifiedTurn() <= honest.TokensPerVerifiedTurn() {
		t.Fatalf("a hollow turn made the cost look better or equal (%d vs %d); "+
			"claiming success would then be cheaper than achieving it",
			withHollow.TokensPerVerifiedTurn(), honest.TokensPerVerifiedTurn())
	}
	if withHollow.HollowRate() != 0.5 {
		t.Errorf("HollowRate = %.2f, want 0.50", withHollow.HollowRate())
	}
}

// TestLearningReport_NothingVerifiedHasNoCostPerUnit: a session that spent a
// million tokens and verified nothing produced no units of work, so there is
// no cost per unit — reporting zero is more honest than an infinity, and it
// must not panic.
func TestLearningReport_NothingVerifiedHasNoCostPerUnit(t *testing.T) {
	r := LearningReportFrom([]types.Fact{
		costFact("s", 1, 1000000, 0, 40, "/hollow"),
	})
	if got := r.TokensPerVerifiedTurn(); got != 0 {
		t.Errorf("TokensPerVerifiedTurn = %d, want 0", got)
	}
	if !strings.Contains(r.String(), "no verified work yet") {
		t.Errorf("report hides that nothing was verified:\n%s", r.String())
	}
}

func TestLearningReportFrom_SkipsMalformedFacts(t *testing.T) {
	// A miscounted denominator is worse than a missing one, so a fact with the
	// wrong arity is dropped rather than guessed at.
	r := LearningReportFrom([]types.Fact{
		{Predicate: "turn_cost", Args: []any{"s", int64(1)}},
		costFact("s", 2, 100, 10, 1, "/done"),
	})
	if r.Turns != 1 {
		t.Fatalf("Turns = %d, want 1 (the malformed fact must be skipped)", r.Turns)
	}
}

func TestLearningReportFrom_AcceptsGoNumericSpellings(t *testing.T) {
	// Facts reach the kernel from Go call sites that build them with int or
	// float64, not only as Mangle's int64.
	r := LearningReportFrom([]types.Fact{
		{Predicate: "turn_cost", Args: []any{"s", 1, 100, 10.0, 2, "/done"}},
	})
	if r.PromptTokens != 100 || r.CompletionTokens != 10 || r.ToolCalls != 2 {
		t.Errorf("numeric coercion dropped values: %+v", r)
	}
	if r.Verified != 1 {
		t.Error("a bare-string outcome was not recognised")
	}
}

func TestLearningReport_EmptyIsNotAnError(t *testing.T) {
	var r LearningReport
	if got := r.String(); got != "no turns recorded yet" {
		t.Errorf("empty report renders as %q", got)
	}
	if r.HollowRate() != 0 {
		t.Error("HollowRate on an empty report must not divide by zero")
	}
}

func TestCortexLearningReport_NilIsSafe(t *testing.T) {
	var c *Cortex
	if c.LearningReport().Turns != 0 {
		t.Error("nil Cortex should yield an empty report")
	}
	if (&Cortex{}).LearningReport().Turns != 0 {
		t.Error("a kernel-less Cortex should yield an empty report")
	}
}
