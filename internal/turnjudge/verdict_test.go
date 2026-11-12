package turnjudge

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The verdict sends the report whole as shared state, one question per
// target with that target's whole message, and returns the answers in the
// targets' order; a job with no targets is one question on the whole report.
func TestJudgeDiagnosis_RequestShape(t *testing.T) {
	for _, tc := range verdictCases {
		a := &recordingAsker{p: 0.2}
		probs, err := JudgeDiagnosis(context.Background(), a, tc.lane, tc.report, tc.targets, 20)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		want := max(len(tc.targets), 1)
		if len(probs) != want || len(a.seen) != 1 || len(a.seen[0].questions) != want {
			t.Fatalf("%s: %d answers over %d requests, want %d in one", tc.name, len(probs), len(a.seen), want)
		}
		state := payload(t, a.seen[0].state)
		if !strings.Contains(state, payload(t, tc.report)[1:len(payload(t, tc.report))-1]) {
			t.Fatalf("%s: the report is not in the state whole: %s", tc.name, state)
		}
		qs := payload(t, a.seen[0].questions)
		for _, target := range tc.targets {
			text := target.Message
			if text == "" {
				text = target.Key
			}
			if !strings.Contains(qs, payload(t, text)[1:len(payload(t, text))-1]) {
				t.Fatalf("%s: target %q is not asked about: %s", tc.name, text, qs)
			}
		}
		if tc.lane == DiagnosisSpeed && !strings.Contains(qs, "take less time") {
			t.Fatalf("%s: the optimize lane is not asked about speed: %s", tc.name, qs)
		}
	}
}

// A failed call is an error and no answers, never a "no".
func TestJudgeDiagnosis_FailureIsNoEvidence(t *testing.T) {
	probs, err := JudgeDiagnosis(context.Background(), &recordingAsker{fail: true}, DiagnosisDefect, "report", nil, 20)
	if err == nil || probs != nil {
		t.Fatalf("a failed call returned %v, %v", probs, err)
	}
}

// The facts are keyed by job and target, a job-level verdict under "*".
func TestVerdictFacts_KeyedByJobAndTarget(t *testing.T) {
	facts := VerdictFacts("job-1", nil, []float64{0.01})
	if len(facts) != 1 || facts[0].Predicate != VerdictPredicate ||
		facts[0].Args[0] != types.MangleString("job-1") || facts[0].Args[1] != types.MangleString(DiagnosisJobKey) {
		t.Fatalf("job-level facts = %v", facts)
	}
	facts = VerdictFacts("job-1", []DiagnosisTarget{{Key: "a"}, {Key: "b"}}, []float64{0.9, 0.1})
	if len(facts) != 2 || facts[1].Args[1] != types.MangleString("b") {
		t.Fatalf("per-target facts = %v", facts)
	}
}
