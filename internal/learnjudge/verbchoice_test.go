package learnjudge

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"codenerd/internal/typesafe"
)

// verbAsker answers the verb choice with pick at confidence and the fit noul
// with fits.
type verbAsker struct {
	pick             string
	confidence, fits float64
	err              error
	state            any
	questions        map[string]typesafe.Question
}

func (a *verbAsker) Ask(_ context.Context, state any, questions map[string]typesafe.Question) (*typesafe.Response, error) {
	a.state, a.questions = state, questions
	if a.err != nil {
		return nil, a.err
	}
	raw, _ := json.Marshal(map[string]any{"model": "jev-test", "answers": map[string]any{
		ChosenVerb: map[string]any{"type": "choice", "choice": a.pick, "confidence": a.confidence, "probabilities": map[string]float64{a.pick: a.confidence}},
		VerbFits:   map[string]any{"type": "noul", "noul": a.fits},
	}, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}})
	var resp typesafe.Response
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

var testVerbs = []VerbOption{
	{Verb: "/fix", Means: []string{"fix", "repair", "correct", "patch", "resolve bug", "debug it", "make it work", "sort out", "too many"}},
	{Verb: "/explain", Means: []string{"explain", "describe"}},
}

func TestChooseVerb_OneRequestAsksTheChoiceAndWhetherAnyVerbFits(t *testing.T) {
	a := &verbAsker{pick: "fix", confidence: 0.99, fits: 0.93}
	got, err := ChooseVerb(context.Background(), a, "fix oversized prompt atoms", testVerbs)
	if err != nil || got != (VerbChoice{Verb: "/fix", Confidence: 0.99, Fits: 0.93}) {
		t.Fatalf("got %+v err=%v", got, err)
	}
	if len(a.questions) != 2 {
		t.Fatalf("asked %d questions, want the choice and its companion", len(a.questions))
	}
	criteria := a.questions[ChosenVerb].(typesafe.Choice).Criteria
	if len(criteria) != 2 || criteria["fix"] == nil || criteria["explain"] == nil {
		t.Fatalf("options are the verbs without their slash: %v", criteria)
	}
	if desc := criteria["fix"].(string); desc != "The user asks codeNERD to fix, repair, correct, patch, resolve bug, debug it, make it work, sort out." {
		t.Fatalf("a verb's description is its first %d synonyms: %q", verbMeansLimit, desc)
	}
}

func TestChooseVerb_AFailedCallIsAnErrorNotAPick(t *testing.T) {
	if _, err := ChooseVerb(context.Background(), &verbAsker{err: errors.New("503")}, "fix it", testVerbs); err == nil {
		t.Fatal("a failed call must be an error")
	}
	if _, err := ChooseVerb(context.Background(), nil, "fix it", testVerbs); err == nil {
		t.Fatal("no client must be an error")
	}
	if _, err := ChooseVerb(context.Background(), &verbAsker{}, "", testVerbs); err == nil {
		t.Fatal("an empty request must be an error")
	}
}
