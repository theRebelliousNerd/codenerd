package chat

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"

	"codenerd/internal/observation"
	"codenerd/internal/types"
)

// C-03 in Docs/journeys/04-contract-registry.md: "the set of status atoms
// written equals the set consumed". S1 rewrote the producer
// (shardResultStatus) and left the consumer (codedom_continuation.mg)
// untouched. The shape of shard_result/5 did not change; the VOCABULARY did:
// /unverified was written and joined by nothing, /tests_needed and
// /review_needed were joined and written by nothing, and a verified turn was
// written /complete while the tester step was reachable only from
// /code_generated (REVIEW-wave1 F8).
//
// The contract now: work owed is never read from the status word. Obligations
// (pending_test, pending_fix) are raised from evidence and discharged by the
// shard result that acts on them; the status is joined only to route an
// /incomplete step back to its shard. Every status the producer writes is
// therefore either joined by a rule or terminal for the step, and every status
// a rule joins is one the producer writes.

// pendingTestOwed is evidence-driven and independent of the status atom: a turn
// that wrote production Go with no test alongside it owes a test whatever
// verdict the kernel reached. The obligation must reach the tester on a
// verified turn exactly as on an unverified one.
func TestPendingTestOnAVerifiedTurnStillReachesTheTester(t *testing.T) {
	m := newKernelBackedModel(t)

	m.injectShardResultFacts("coder", "add the retry", observation.Return{
		Output:   "Added the retry and ran the suite.",
		Outcome:  "/done",
		Changed:  []string{"internal/broker/compression.go"},
		Tests:    passedVerification("tests"),
		Untested: []string{"internal/broker/compression.go"},
	}, nil)

	if n := queryLen(t, m, "pending_test"); n != 1 {
		t.Fatalf("precondition: the evidence owes a test, got %d pending_test facts", n)
	}
	assertShardResultStatus(t, m, "/complete")

	subtasks := querySubtasks(t, m)
	if len(subtasks) != 1 || subtasks[0].shard != "/tester" {
		t.Fatalf("pending_test was asserted on a verified turn and the tester step did not derive: %v", subtasks)
	}
	if subtasks[0].description != testObligationPrefix+"add the retry" {
		t.Errorf("the tester is asked %q; the obligation's description is the key its own result must discharge", subtasks[0].description)
	}
}

// The tester's result for the obligation's description discharges it. Until
// 2026-09-18 nothing did: the coder's shard_result and the pending_test both
// survived the tester step, the rule re-derived, and the continuation ran the
// tester again until max_continuation_steps(10) -- a count as the completion
// criterion.
func TestTestObligationIsDischargedByTheTestersResult(t *testing.T) {
	m := newKernelBackedModel(t)

	m.injectShardResultFacts("coder", "add the retry", observation.Return{
		Output:   "Added the retry.",
		Outcome:  "/unverified",
		Changed:  []string{"internal/broker/compression.go"},
		Untested: []string{"internal/broker/compression.go"},
	}, nil)
	subtasks := querySubtasks(t, m)
	if len(subtasks) != 1 || subtasks[0].shard != "/tester" {
		t.Fatalf("precondition: one tester step owed, got %v", subtasks)
	}

	// The continuation dispatches the tester with the obligation's description
	// and injects its result under that same description.
	m.injectShardResultFacts("tester", subtasks[0].description, observation.Return{
		Output:  "Wrote compression_test.go covering the retry.",
		Outcome: "/done",
		Changed: []string{"internal/broker/compression_test.go"},
		Tests:   passedVerification("tests"),
	}, nil)

	if got := querySubtasks(t, m); len(got) != 0 {
		t.Errorf("the tester acted on the obligation and it is still owed: %v", got)
	}
	if n := queryLen(t, m, "should_auto_continue"); n != 0 {
		t.Errorf("should_auto_continue still derives after every obligation was discharged")
	}
}

// Structured findings raise a FIX obligation, and it goes to the coder. Until
// 2026-09-18 the description said "Fix N finding(s)" and the rule routed it to
// the reviewer: the same fact asked one shard to do another's work. The
// coder's result for the description discharges it; the findings its second
// pass leaves behind are reported, not chased -- one round per obligation.
func TestFindingsOweAFixToTheCoderOnce(t *testing.T) {
	m := newKernelBackedModel(t)

	m.injectShardResultFacts("coder", "add the retry", observation.Return{
		Output:   "Added the retry.",
		Outcome:  "/done",
		Changed:  []string{"internal/broker/compression.go"},
		Tests:    passedVerification("tests"),
		Findings: []observation.Finding{{File: "internal/broker/compression.go", Line: 12, Severity: "high", Message: "unchecked error"}},
	}, nil)
	subtasks := querySubtasks(t, m)
	if len(subtasks) != 1 || subtasks[0].shard != "/coder" {
		t.Fatalf("findings must owe a fix to the coder, got %v", subtasks)
	}
	if subtasks[0].description != fixObligationPrefix+"add the retry" {
		t.Fatalf("fix description = %q", subtasks[0].description)
	}

	// The coder's fix pass still leaves a finding. Its obligation description
	// is built from the ROOT task, so it is the one already discharged.
	m.injectShardResultFacts("coder", subtasks[0].description, observation.Return{
		Output:   "Handled the error; one nit remains.",
		Outcome:  "/done",
		Changed:  []string{"internal/broker/compression.go"},
		Tests:    passedVerification("tests"),
		Findings: []observation.Finding{{File: "internal/broker/compression.go", Line: 40, Severity: "low", Message: "naming"}},
	}, nil)

	if n := queryLen(t, m, "pending_fix"); n != 2 {
		t.Fatalf("both rounds raised an obligation, got %d pending_fix facts", n)
	}
	if got := querySubtasks(t, m); len(got) != 0 {
		t.Errorf("a second fix round was dispatched for the same work: %v", got)
	}
}

// A fix step that writes untested Go owes a test for the root task, not for
// "Fix the findings reported in: ...". Without obligationRoot the descriptions
// nest one prefix per round and no shard result ever discharges them.
func TestObligationDescriptionsDoNotNest(t *testing.T) {
	cases := map[string]string{
		"add the retry":                                              "add the retry",
		testObligationPrefix + "add the retry":                       "add the retry",
		fixObligationPrefix + "add the retry":                        "add the retry",
		testObligationPrefix + fixObligationPrefix + "add the retry": "add the retry",
	}
	for in, want := range cases {
		if got := obligationRoot(in); got != want {
			t.Errorf("obligationRoot(%q) = %q, want %q", in, got, want)
		}
	}
}

// An /incomplete step is retried once. Two results for the same shard and
// description mean the retry happened; a third dispatch would be the same
// call again, which is a stall.
func TestIncompleteStepIsRetriedOnce(t *testing.T) {
	m := newKernelBackedModel(t)

	hollow := observation.Return{Output: "Here is the plan.", Outcome: "/hollow"}
	m.injectShardResultFacts("coder", "add the retry", hollow, nil)
	subtasks := querySubtasks(t, m)
	if len(subtasks) != 1 || subtasks[0].shard != "/coder" || subtasks[0].description != "add the retry" {
		t.Fatalf("a hollow step must be owed back to the same shard once, got %v", subtasks)
	}

	m.injectShardResultFacts("coder", "add the retry", hollow, nil)
	if n := queryLen(t, m, "shard_result"); n != 2 {
		t.Fatalf("precondition: two results for the same work, got %d", n)
	}
	if got := querySubtasks(t, m); len(got) != 0 {
		t.Errorf("the same incomplete step was dispatched a third time: %v", got)
	}
}

// The closed-vocabulary check the registry asks for, with the consumed side
// read from the policy file rather than transcribed, so it fails whenever
// either side moves.
func TestShardResultStatusVocabularyIsClosed(t *testing.T) {
	written := map[string]bool{}
	for _, c := range []struct {
		ret observation.Return
		err error
	}{
		{ret: observation.Return{Outcome: "/failed"}},
		{ret: observation.Return{Outcome: "/hollow"}},
		{ret: observation.Return{Outcome: "/done"}},
		{ret: observation.Return{Outcome: "/unverified", Changed: []string{"a.go"}}},
		{ret: observation.Return{Outcome: "/unverified"}},
		{ret: observation.Return{}},
		{ret: observation.Return{}, err: errForVocabulary{}},
	} {
		written[shardResultStatus(c.ret, c.err)] = true
	}

	// Terminal for the step by design: no continuation is owed by the status
	// itself. What such a step still owes is carried by the obligations.
	terminal := map[string]bool{"/failed": true, "/complete": true, "/unverified": true}

	consumed := statusAtomsJoinedByContinuationPolicy(t)
	if len(consumed) == 0 {
		t.Fatal("no shard_result status literal found in codedom_continuation.mg; the parser has drifted")
	}

	var unconsumed, unwritten []string
	for s := range written {
		if terminal[s] {
			continue
		}
		if !consumed[s] {
			unconsumed = append(unconsumed, s)
		}
	}
	for s := range consumed {
		if !written[s] {
			unwritten = append(unwritten, s)
		}
	}
	sort.Strings(unconsumed)
	sort.Strings(unwritten)
	if len(unconsumed) > 0 {
		t.Errorf("shardResultStatus writes %v, which no rule in codedom_continuation.mg joins and "+
			"which is not terminal: the continuation cannot see those steps", unconsumed)
	}
	if len(unwritten) > 0 {
		t.Errorf("codedom_continuation.mg joins %v, which no producer writes: those rules can never fire", unwritten)
	}
}

// statusAtomsJoinedByContinuationPolicy reads every status literal that a rule
// in codedom_continuation.mg places in shard_result's second argument.
func statusAtomsJoinedByContinuationPolicy(t *testing.T) map[string]bool {
	t.Helper()
	path := filepath.Join("..", "..", "..", "internal", "core", "defaults", "policy", "codedom_continuation.mg")
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	re := regexp.MustCompile(`(?m)^\s*shard_result\([^,]*,\s*(/[a-z_]+)\s*,`)
	out := map[string]bool{}
	for _, m := range re.FindAllStringSubmatch(string(src), -1) {
		out[m[1]] = true
	}
	return out
}

type pendingSubtask struct {
	id, description, shard string
}

func querySubtasks(t *testing.T, m *Model) []pendingSubtask {
	t.Helper()
	facts, err := m.kernel.Query("has_pending_subtask")
	if err != nil {
		t.Fatalf("query has_pending_subtask: %v", err)
	}
	out := make([]pendingSubtask, 0, len(facts))
	for _, f := range facts {
		out = append(out, pendingSubtask{
			id:          types.ExtractString(f.Args[0]),
			description: types.ExtractString(f.Args[1]),
			shard:       types.ExtractString(f.Args[2]),
		})
	}
	return out
}

type errForVocabulary struct{}

func (errForVocabulary) Error() string { return "the shard returned an error" }
