package learnjudge

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func TestRequestFragments_SplitsAsksAndKeepsTheirWording(t *testing.T) {
	for _, tc := range []struct {
		request string
		want    []string
	}{
		{"add a --json flag to nerd status and document it in the README",
			[]string{"add a --json flag to nerd status", "document it in the README"}},
		{"The chat drops the last line. Do not touch the renderer. Find where the reply is cut and fix it with a test",
			[]string{"The chat drops the last line", "Do not touch the renderer", "Find where the reply is cut", "fix it with a test"}},
		{"1. rename Timeout\n2. update the docs; then run the tests",
			[]string{"rename Timeout", "update the docs", "run the tests"}},
		// A one-word piece is a modifier of the clause before it.
		{"make the output read cleanly and consistently", []string{"make the output read cleanly and consistently"}},
		{"fix the parser", []string{"fix the parser"}},
		{"   ", nil},
	} {
		if got := RequestFragments(tc.request); !slices.Equal(got, tc.want) {
			t.Errorf("RequestFragments(%q) = %q, want %q", tc.request, got, tc.want)
		}
	}
}

func TestCheckDelivery_AsksCoverageAndOneBlockerQuestionOverOneState(t *testing.T) {
	a := &autopoiesisAsker{answers: map[string]float64{
		"asked_0": 0.99, "delivered_0": 0.97,
		"asked_1": 0.98, "delivered_1": 0.05,
		DeliveryNamesBlocker: 0.02,
	}}
	in := DeliveryInput{
		Request:   "add a --json flag and document it",
		Reports:   []string{"Added the flag.", "Still working."},
		Changed:   []string{"cmd/status.go"},
		Fragments: []string{"add a --json flag", "document it"},
	}
	got, err := CheckDelivery(context.Background(), a, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.questions) != 5 {
		t.Fatalf("asked %d questions, want two per fragment and one blocker question", len(a.questions))
	}
	state := a.state.(map[string]any)
	result, _ := state["result"].(string)
	if !strings.Contains(result, "Step 1 report:\nAdded the flag.") || !strings.Contains(result, "Step 2 report:\nStill working.") {
		t.Fatalf("result = %q: each step's report is its own block", result)
	}
	if !slices.Equal(state["files_changed"].([]string), in.Changed) {
		t.Fatalf("files_changed = %v", state["files_changed"])
	}
	if got.ResultCut || got.Blocked != 0.02 || len(got.Verdicts) != 2 || got.Verdicts[1].Delivered != 0.05 || got.Verdicts[1].Fragment != "document it" {
		t.Fatalf("reading = %+v", got)
	}

	// No change list: the field is left out rather than sent empty.
	in.Changed = nil
	if _, err := CheckDelivery(context.Background(), a, in); err != nil {
		t.Fatal(err)
	}
	if _, ok := a.state.(map[string]any)["files_changed"]; ok {
		t.Fatal("an empty change list was sent")
	}
}

func TestCheckDelivery_AFailedCallIsAnError(t *testing.T) {
	a := &autopoiesisAsker{err: errors.New("503")}
	if _, err := CheckDelivery(context.Background(), a, DeliveryInput{Request: "x y", Fragments: []string{"x y"}}); err == nil {
		t.Fatal("a failed call returned a reading")
	}
	if _, err := CheckDelivery(context.Background(), &autopoiesisAsker{}, DeliveryInput{Request: "x"}); err == nil {
		t.Fatal("no fragments returned a reading")
	}
	if _, err := CheckDelivery(context.Background(), nil, DeliveryInput{Request: "x", Fragments: []string{"x"}}); err == nil {
		t.Fatal("no client returned a reading")
	}
}

func TestDeliveryFacts_ProjectsWholePercentsPerRound(t *testing.T) {
	r := DeliveryReading{Verdicts: []CoverageVerdict{{Fragment: "document it", Asked: 0.98, Delivered: 0.05}}, Blocked: 0.71, ResultCut: true}
	facts := DeliveryFacts(types.MangleAtom("/delivery_1"), 2, r)
	if len(facts) != 3 {
		t.Fatalf("facts = %v", facts)
	}
	if f := facts[0]; f.Predicate != DeliveryReadingPredicate || f.Args[1] != int64(2) || f.Args[2] != "document it" || f.Args[3] != int64(98) || f.Args[4] != int64(5) {
		t.Fatalf("reading fact = %v", f)
	}
	if f := facts[1]; f.Predicate != DeliveryBlockerPredicate || f.Args[2] != int64(71) {
		t.Fatalf("blocker fact = %v", f)
	}
	if facts[2].Predicate != DeliveryCutPredicate {
		t.Fatalf("cut fact = %v", facts[2])
	}
}
