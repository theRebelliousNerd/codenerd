package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// Sweep finding F4: the post-edit rounds ran in a fixed Go order, and six of
// them decided from the written paths' extensions whether they applied -- a
// second answer to turn_owes_gate. The kernel's turn_next_round is the
// schedule now (policy/turn_rounds.mg).

// roundsExecutor is an executor on a real kernel loaded with the policy
// corpus.
func roundsExecutor(t *testing.T) *Executor {
	t.Helper()
	e := createTestExecutor(t)
	e.kernel = realKernel(t)
	return e
}

// schedule walks the kernel's schedule for result, marking each round ran,
// and returns the rounds in the order the kernel named them.
func schedule(t *testing.T, e *Executor, result *ExecutionResult) []string {
	t.Helper()
	var got []string
	for i := 0; i < 20; i++ {
		next, err := e.nextPostEditRound(result)
		if err != nil {
			t.Fatalf("nextPostEditRound: %v", err)
		}
		if next == "" {
			return got
		}
		got = append(got, next)
		if err := e.markRoundRan(result, next); err != nil {
			t.Fatal(err)
		}
	}
	t.Fatalf("the schedule did not end: %v", got)
	return nil
}

func TestPostEditRounds_AGoFixOwesEveryRoundInOrder(t *testing.T) {
	e := roundsExecutor(t)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}, SuccessfulWriteTools: 1, WrittenPaths: []string{"internal/x/x.go"}}
	want := []string{"/build", "/test", "/critic", "/coverage", "/pinned", "/vet", "/removed_tests"}
	if got := schedule(t, e, result); strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("rounds = %v, want %v", got, want)
	}
}

// A write with no recorded path owes the build and the tests (the policy's
// no-path arm). touchedGoFiles(nil) was false, so no round ran and the
// verdict could only be /unverified.
func TestPostEditRounds_AWriteWithNoRecordedPathOwesTheBuild(t *testing.T) {
	e := roundsExecutor(t)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}, SuccessfulWriteTools: 1}
	got := schedule(t, e, result)
	if len(got) == 0 || got[0] != "/build" {
		t.Fatalf("rounds = %v, want the build first", got)
	}
}

// A document write owes no Go round and no test run.
func TestPostEditRounds_ADocumentWriteOwesNoGoRound(t *testing.T) {
	e := roundsExecutor(t)
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/create"}, SuccessfulWriteTools: 1, WrittenPaths: []string{"Docs/x.md"}}
	for _, round := range schedule(t, e, result) {
		if round != "/removed_tests" {
			t.Errorf("a .md write was scheduled %s", round)
		}
	}
}

// The schedule is policy: a round the policy owes and the executor has no
// driver for is an error, not skipped -- a new round is a rule and a driver.
func TestPostEditRounds_ARoundWithNoDriverIsAnError(t *testing.T) {
	e := roundsExecutor(t)
	rk, ok := e.kernel.(interface{ AppendPolicy(string) })
	if !ok {
		t.Fatalf("kernel %T cannot take policy", e.kernel)
	}
	rk.AppendPolicy("round_order(/probe, 0).\nturn_round_owed(Turn, /probe) :- turn_wrote(Turn).\n")
	result := &ExecutionResult{Intent: perception.Intent{Verb: "/fix"}, SuccessfulWriteTools: 1, WrittenPaths: []string{"internal/x/x.go"}}
	_, _, err := e.verifyCompletedToolTurn(context.Background(), nil, "", nil, &types.LLMToolResponse{Text: "done"}, nil, nil, result)
	if err == nil || !strings.Contains(err.Error(), "/probe") {
		t.Fatalf("verifyCompletedToolTurn = %v, want an error naming the round with no driver", err)
	}
}

// A turn with writes and no kernel to ask is refused; a turn with none owes
// nothing.
func TestPostEditRounds_NoKernel(t *testing.T) {
	e := createTestExecutor(t)
	e.kernel = nil
	if _, err := e.nextPostEditRound(&ExecutionResult{SuccessfulWriteTools: 1}); err == nil {
		t.Fatal("a turn with writes was scheduled with no kernel")
	}
	if next, err := e.nextPostEditRound(&ExecutionResult{}); err != nil || next != "" {
		t.Fatalf("a turn with no writes: %q, %v", next, err)
	}
}
