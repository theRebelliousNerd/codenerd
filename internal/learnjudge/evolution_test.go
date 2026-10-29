package learnjudge

import (
	"context"
	"errors"
	"strings"
	"testing"

	"codenerd/internal/typesafe"
)

// One verdict request asks the fixed questions plus one per changed file; the
// state carries only what the record has, and bounds a long output.
func TestTaskVerdictRequest_Shape(t *testing.T) {
	state, qs := TaskVerdictRequest(TaskRecord{Request: "fix Max", ChangedFiles: []string{"a.go", "b.go"}, Output: strings.Repeat("x", taskOutputLimit+10)})
	for _, name := range []string{DidWhatWasAsked, StoppedEarly, FailureKind, FailureKindFits, "file_needed_0", "file_needed_1"} {
		if _, ok := qs[name]; !ok {
			t.Fatalf("question %s missing: %v", name, qs)
		}
	}
	if c, ok := qs[FailureKind].(typesafe.Choice); !ok || len(c.Criteria) != len(FailureKinds) {
		t.Fatalf("failure kind is not a Choice over every kind: %#v", qs[FailureKind])
	}
	result := state["result"].(map[string]any)
	if _, ok := result["build_errors"]; ok {
		t.Fatal("an empty build error list went into the state")
	}
	if n, _ := result["final_output_omitted_bytes"].(int); n != 10 {
		t.Fatalf("output omission = %v, want 10", result["final_output_omitted_bytes"])
	}
}

func TestProblemTypeAndSharedCause_FailLoudly(t *testing.T) {
	failing := &autopoiesisAsker{err: errors.New("503")}
	if _, err := JudgeProblemType(context.Background(), failing, "fix it", ""); err == nil {
		t.Fatal("a failed problem-type call must be an error")
	}
	if _, err := JudgeProblemType(context.Background(), failing, "  ", ""); err == nil {
		t.Fatal("an empty task must be refused before a call")
	}
	if _, err := JudgeSharedCause(context.Background(), failing, []GroupFailure{{Request: "x"}}); err == nil {
		t.Fatal("a failed shared-cause call must be an error")
	}
	state, _ := SharedCauseRequest(make([]GroupFailure, groupMaxFailures+3))
	if state["more_failures_omitted"] != 3 || len(state["failures"].([]map[string]any)) != groupMaxFailures {
		t.Fatalf("group bound: %v", state["more_failures_omitted"])
	}
	_, qs := ProblemTypeRequest("fix the race", "/coder")
	if c, ok := qs[ProblemKind].(typesafe.Choice); !ok || len(c.Criteria) != len(ProblemKinds) {
		t.Fatal("problem kind is not a Choice over every kind")
	}
}

// One question per promoted atom, at most atomMaxComparisons, and the state
// says how many were left out.
func TestAtomCheckRequest_ShapeAndBound(t *testing.T) {
	existing := make([]AtomText, atomMaxComparisons+2)
	for i := range existing {
		existing[i] = AtomText{ID: "a", Content: "x"}
	}
	state, qs := AtomCheckRequest(AtomText{ID: "c", Content: "do y"}, existing)
	if len(qs) != atomMaxComparisons+1 || state["existing_atoms_omitted"] != 2 {
		t.Fatalf("questions %d omitted %v", len(qs), state["existing_atoms_omitted"])
	}
	if _, err := CheckAtom(context.Background(), &autopoiesisAsker{err: errors.New("503")}, AtomText{ID: "c", Content: "do y"}, nil); err == nil {
		t.Fatal("a failed atom check must be an error")
	}
}
