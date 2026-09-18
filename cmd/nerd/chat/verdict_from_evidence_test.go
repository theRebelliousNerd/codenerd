package chat

import (
	"errors"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/observation"
	"codenerd/internal/types"
)

// passed builds an observed verification with a pass verdict.
func passedVerification(kind string) *observation.Verification {
	return &observation.Verification{
		Kind:    kind,
		Source:  observation.SourceObserved,
		Ran:     true,
		OK:      true,
		Outcome: "passed",
	}
}

// TestShardResultStatus_DerivedFromOutcomeNotProse pins the seam: the Status
// argument of shard_result/5 is a function of the producer's verdict and the
// observed evidence, never of the words in the result.
//
// Before this change injectShardResultFacts read the prose: "TODO" or "FIXME"
// anywhere in the output made the step /incomplete, and a coder result that
// happened not to contain the word "test" became /code_generated and always
// owed a follow-up test.
func TestShardResultStatus_DerivedFromOutcomeNotProse(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ret  observation.Return
		err  error
		want string
	}{
		{
			// The headline case: the word TODO in a verified turn's output is
			// not a verdict about the turn.
			name: "proseSaysTODOButOutcomeIsDone",
			ret: observation.Return{
				Agent:   "coder",
				Output:  "Added the handler. TODO in the tracker: document the new flag.",
				Outcome: "/done",
			},
			want: "/complete",
		},
		{
			name: "proseSaysFIXMEButOutcomeIsDone",
			ret: observation.Return{
				Agent:   "coder",
				Output:  "Done. Left a FIXME comment where the vendor API is flaky.",
				Outcome: "/done",
			},
			want: "/complete",
		},
		{
			// /unverified must never reach /complete, however confident the
			// prose sounds. That Go source was written is evidence that raises
			// pending_test; it is not a status word.
			name: "unverifiedIsNeverComplete",
			ret: observation.Return{
				Agent:   "coder",
				Output:  "All done — everything works perfectly and is fully tested.",
				Outcome: "/unverified",
				Changed: []string{"internal/core/kernel.go"},
			},
			want: "/unverified",
		},
		{
			name: "unverifiedWithNoChangeIsUnverified",
			ret: observation.Return{
				Agent:   "coder",
				Output:  "Reviewed the package; it looks fine.",
				Outcome: "/unverified",
			},
			want: "/unverified",
		},
		{
			name: "hollowOwesTheWorkToTheSameShard",
			ret: observation.Return{
				Agent:   "coder",
				Output:  "Here is the plan for the change.",
				Outcome: "/hollow",
			},
			want: "/incomplete",
		},
		{
			name: "failedOutcome",
			ret:  observation.Return{Agent: "coder", Output: "partial", Outcome: "/failed"},
			want: "/failed",
		},
		{
			name: "errorBeatsEverything",
			ret:  observation.Return{Agent: "coder", Output: "looks great", Outcome: "/done"},
			err:  errors.New("shard execution failed"),
			want: "/failed",
		},
		{
			// A producer with no structure has observed no verdict. That is
			// not completion.
			name: "absentVerdictIsUnverifiedNotComplete",
			ret:  observation.Return{Agent: "coder", Output: "Finished successfully."},
			want: "/unverified",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := shardResultStatus(tc.ret, tc.err); got != tc.want {
				t.Errorf("shardResultStatus = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestPendingTestOwed_DerivedFromEvidenceNotVocabulary covers the other half
// of the old heuristic: a coder turn used to owe a test whenever its output
// lacked the word "test", and never to owe one when the word appeared.
func TestPendingTestOwed_DerivedFromEvidenceNotVocabulary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		ret  observation.Return
		want bool
	}{
		{
			// The brief's case: the word "test" is nowhere in the output, but
			// a test run was observed and it passed.
			name: "noTestWordButTestsRanAndPassed",
			ret: observation.Return{
				Agent:   "coder",
				Output:  "Reworked the parser so the offsets line up.",
				Changed: []string{"internal/parser/parser.go"},
				Tests:   passedVerification("tests"),
			},
			want: false,
		},
		{
			name: "goSourceChangedAndNothingRanTheTests",
			ret: observation.Return{
				Agent:   "coder",
				Output:  "Reworked the parser; the tests should still pass.",
				Changed: []string{"internal/parser/parser.go"},
			},
			want: true,
		},
		{
			// The executor named the files it wrote with no test alongside.
			// That list IS the obligation, whatever else ran.
			name: "untestedPathsAlwaysOweATest",
			ret: observation.Return{
				Agent:    "coder",
				Output:   "Added the helper.",
				Changed:  []string{"internal/parser/parser.go"},
				Tests:    passedVerification("tests"),
				Untested: []string{"internal/parser/parser.go"},
			},
			want: true,
		},
		{
			name: "markdownOnlyTurnOwesNothing",
			ret: observation.Return{
				Agent:   "coder",
				Output:  "Documented the flag.",
				Changed: []string{"README.md"},
			},
			want: false,
		},
		{
			name: "testFileOnlyTurnOwesNothing",
			ret: observation.Return{
				Agent:   "coder",
				Output:  "Added coverage.",
				Changed: []string{"internal/parser/parser_test.go"},
			},
			want: false,
		},
		{
			name: "noChangeOwesNothing",
			ret:  observation.Return{Agent: "coder", Output: "Nothing to change."},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := pendingTestOwed(tc.ret); got != tc.want {
				t.Errorf("pendingTestOwed = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestContinuationSummary_NeverSaysSuccessfullyForUnverifiedStep pins the
// symptom observed in the chat session of 2026-09-17 21:24: a coder step
// reported "Wrote 1 file(s) ... Requested behavior remains unverified (no
// acceptance contract)." and the lines under it read "All 1 steps complete."
// then "Completed 1 steps successfully."
//
// The old summary was `fmt.Sprintf("Completed %d steps successfully.", n)`,
// emitted whenever the kernel derived no NEXT step — a statement about running
// out of work, not about the work.
func TestContinuationSummary_NeverSaysSuccessfullyForUnverifiedStep(t *testing.T) {
	t.Parallel()

	unverified := observation.Return{
		Output:  "Wrote 1 file(s): internal/core/shards/manager_tools.go",
		Outcome: "/unverified",
		Stage:   "checks_passed",
		Changed: []string{"internal/core/shards/manager_tools.go"},
		Build:   passedVerification("build"),
	}

	got := continuationSummary(1, unverified)

	if strings.Contains(strings.ToLower(got), "successfully") {
		t.Errorf("an unverified step must never be summarized as successful, got %q", got)
	}
	if !strings.Contains(strings.ToLower(got), "not verified") {
		t.Errorf("the summary must state the verdict, got %q", got)
	}
	// The evidence, not just the verdict: what built, what tested, what was
	// not verified.
	for _, want := range []string{"build passed", "tests did not run", "no acceptance contract"} {
		if !strings.Contains(got, want) {
			t.Errorf("summary must name the evidence %q, got %q", want, got)
		}
	}

	if outcome := continuationOutcomeFor(unverified, nil); outcome != continuationUnverified {
		t.Errorf("an /unverified step must render as continuationUnverified, got %v", outcome)
	}
}

// TestContinuationSummary_HollowAndDone covers the other verdicts so the
// unverified case above is not passing by rendering one string for everything.
func TestContinuationSummary_HollowAndDone(t *testing.T) {
	t.Parallel()

	hollow := observation.Return{Output: "Here is my plan.", Outcome: "/hollow"}
	got := continuationSummary(2, hollow)
	if strings.Contains(strings.ToLower(got), "successfully") {
		t.Errorf("a hollow step must never be summarized as successful, got %q", got)
	}
	if !strings.Contains(got, "never performed") {
		t.Errorf("a hollow summary must say the work was not performed, got %q", got)
	}
	if outcome := continuationOutcomeFor(hollow, nil); outcome != continuationUnverified {
		t.Errorf("a /hollow step must not render as completed, got %v", outcome)
	}

	done := observation.Return{
		Output:     "Implemented and verified.",
		Outcome:    "/done",
		Changed:    []string{"internal/core/kernel.go"},
		Build:      passedVerification("build"),
		Tests:      passedVerification("tests"),
		Acceptance: &observation.Acceptance{Status: "verified", Contract: "c1"},
	}
	gotDone := continuationSummary(1, done)
	if !strings.Contains(gotDone, "verified") {
		t.Errorf("a verified step must say so, got %q", gotDone)
	}
	if outcome := continuationOutcomeFor(done, nil); outcome != continuationCompleted {
		t.Errorf("a /done step must render as completed, got %v", outcome)
	}
}

// TestInjectShardResultFacts_DerivesContinuationThroughRealKernel runs the
// derived facts through the loaded Mangle corpus: the fact shapes are
// unchanged, so the existing rules in
// internal/core/defaults/policy/codedom_continuation.mg must still derive the
// follow-up work — and must no longer derive it from prose.
func TestInjectShardResultFacts_DerivesContinuationThroughRealKernel(t *testing.T) {
	t.Run("unverifiedWriteWithNoTestRunOwesATesterStep", func(t *testing.T) {
		m := newKernelBackedModel(t)
		m.injectShardResultFacts("coder", "add the handler", observation.Return{
			Output:  "Added the handler.",
			Outcome: "/unverified",
			Changed: []string{"internal/core/handler.go"},
		}, nil)

		assertShardResultStatus(t, m, "/unverified")
		if n := queryLen(t, m, "pending_test"); n != 1 {
			t.Fatalf("expected one pending_test derived from the evidence, got %d", n)
		}
		if n := queryLen(t, m, "has_pending_subtask"); n == 0 {
			t.Fatal("the corpus must derive a pending subtask from shard_result + pending_test")
		}
		if n := queryLen(t, m, "should_auto_continue"); n == 0 {
			t.Fatal("should_auto_continue must derive when a subtask is pending")
		}
	})

	t.Run("testedWriteOwesNothingEvenWithoutTheWordTest", func(t *testing.T) {
		m := newKernelBackedModel(t)
		m.injectShardResultFacts("coder", "add the handler", observation.Return{
			Output:  "Reworked the parser so the offsets line up.",
			Outcome: "/unverified",
			Changed: []string{"internal/core/handler.go"},
			Tests:   passedVerification("tests"),
		}, nil)

		if n := queryLen(t, m, "pending_test"); n != 0 {
			t.Fatalf("a turn whose tests ran and passed owes no test, got %d pending_test", n)
		}
		if n := queryLen(t, m, "has_pending_subtask"); n != 0 {
			t.Fatalf("no obligation means no pending subtask, got %d", n)
		}
	})

	t.Run("prosewithTODOOnAVerifiedTurnDerivesNothing", func(t *testing.T) {
		m := newKernelBackedModel(t)
		m.injectShardResultFacts("coder", "add the handler", observation.Return{
			Output:  "Done. TODO in the tracker: document the new flag.",
			Outcome: "/done",
			Changed: []string{"internal/core/handler.go"},
			Tests:   passedVerification("tests"),
		}, nil)

		assertShardResultStatus(t, m, "/complete")
		if n := queryLen(t, m, "has_pending_subtask"); n != 0 {
			t.Fatalf("the word TODO must not create follow-up work, got %d pending subtasks", n)
		}
	})

	t.Run("structuredFindingsOweAFix", func(t *testing.T) {
		m := newKernelBackedModel(t)
		m.injectShardResultFacts("coder", "add the handler", observation.Return{
			Output:   "Added the handler.",
			Outcome:  "/unverified",
			Changed:  []string{"internal/core/handler.go"},
			Tests:    passedVerification("tests"),
			Findings: []observation.Finding{{File: "internal/core/handler.go", Line: 12, Severity: "high", Message: "unchecked error"}},
		}, nil)

		if n := queryLen(t, m, "pending_fix"); n != 1 {
			t.Fatalf("expected one pending_fix derived from structured findings, got %d", n)
		}
		if n := queryLen(t, m, "has_pending_subtask"); n == 0 {
			t.Fatal("the corpus must derive a coder subtask from pending_fix")
		}
	})

	t.Run("prosewithIssueButNoFindingsOwesNoReview", func(t *testing.T) {
		m := newKernelBackedModel(t)
		m.injectShardResultFacts("reviewer", "review the package", observation.Return{
			Output:  "No issue found anywhere in this package.",
			Outcome: "/unverified",
		}, nil)

		if n := queryLen(t, m, "pending_fix"); n != 0 {
			t.Fatalf("the substring \"issue\" must not create a fix obligation, got %d", n)
		}
	})
}

// newKernelBackedModel is a Model with a real kernel carrying the loaded
// default corpus — the rules under test are the shipped ones, not a stub.
func newKernelBackedModel(t *testing.T) *Model {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	m := NewTestModel()
	m.kernel = k
	return &m
}

func queryLen(t *testing.T, m *Model, predicate string) int {
	t.Helper()
	facts, err := m.kernel.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	return len(facts)
}

func assertShardResultStatus(t *testing.T, m *Model, want string) {
	t.Helper()
	facts, err := m.kernel.Query("shard_result")
	if err != nil {
		t.Fatalf("query shard_result: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("expected one shard_result fact, got %d", len(facts))
	}
	got := types.ExtractString(facts[0].Args[1])
	if got != want {
		t.Fatalf("shard_result status = %q, want %q", got, want)
	}
}
