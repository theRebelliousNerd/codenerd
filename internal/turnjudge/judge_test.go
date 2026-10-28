package turnjudge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// fakeJev answers every noul question with the probability its name maps to
// (0.5 when unlisted), recording the request it saw.
func fakeJev(t *testing.T, answers map[string]float64) (*typesafe.Client, *map[string]any) {
	t.Helper()
	var seen map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		seen = req
		out := map[string]any{}
		for name := range req["questions"].(map[string]any) {
			p, ok := answers[name]
			if !ok {
				p = 0.5
			}
			out[name] = map[string]any{"type": "noul", "noul": p}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "jev-test", "answers": out, "usage": map[string]any{"input_tokens": 321, "output_tokens": 9}})
	}))
	t.Cleanup(srv.Close)
	c, err := typesafe.NewClient(typesafe.Options{Model: "jev-test", BaseURL: srv.URL, APIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	return c, &seen
}

func TestJudge_AsksEveryQuestionAndProjectsFacts(t *testing.T) {
	c, seen := fakeJev(t, map[string]float64{ChangesFiles: 0.91, "answered_0": 0.88, ApprovesHeldWork: 0.07})
	in := Input{
		Message:     "use a token bucket, and go ahead",
		Recent:      []Turn{{From: "user", Text: "add rate limiting"}, {From: "assistant", Text: "Which algorithm?"}},
		Brief:       &perception.Brief{Goal: "add rate limiting", OpenQuestions: []string{"which algorithm?"}},
		HeldWork:    &HeldWork{Action: "implement", Target: "rate limiting", Shard: "coder", RequestedAs: "add rate limiting to the API"},
		RunningWork: []string{"reviewer: review internal/auth"},
	}
	res, err := Judge(context.Background(), c, in)
	if err != nil {
		t.Fatal(err)
	}
	qs := (*seen)["questions"].(map[string]any)
	for _, name := range []string{ChangesFiles, NamesTarget, Hypothetical, AboutCodeNERD, WantsToStop, RulesOutChanges, ApprovesHeldWork, AddsToHeldWork, "answered_0"} {
		if _, ok := qs[name]; !ok {
			t.Errorf("question %s was not asked", name)
		}
	}
	if len(qs) != 9 {
		t.Errorf("asked %d questions, want exactly the 9 the rules read: %v", len(qs), qs)
	}
	if got := qs["answered_0"].(map[string]any)["instructions"].(map[string]any)["open_question"]; got != "which algorithm?" {
		t.Errorf("the open question is not a field of its own judgment: %v", got)
	}
	state := (*seen)["state"].(map[string]any)
	held, ok := state["held_work"].(map[string]any)
	if !ok || held["handled_by"] != "coder" || held["requested_as"] != "add rate limiting to the API" {
		t.Errorf("held work is not named fields in the state: %v", state["held_work"])
	}
	if _, ok := state["running_work"]; !ok {
		t.Error("running work missing from the state")
	}
	if res.Model != "jev-test" || res.InputTokens != 321 {
		t.Errorf("model/usage not reported: %+v", res)
	}

	facts := res.Facts()
	byName := map[string]int64{}
	for _, f := range facts {
		if f.Predicate == "jev_judgment" {
			byName[types.ExtractString(f.Args[0])], _ = types.ExtractInt64(f.Args[1])
		}
	}
	if byName[ChangesFiles] != 91 || byName[ApprovesHeldWork] != 7 {
		t.Errorf("percent projection wrong: %v", byName)
	}
	var about bool
	for _, f := range facts {
		if f.Predicate == "jev_judgment_about" && types.ExtractString(f.Args[1]) == "which algorithm?" {
			about = true
		}
	}
	if !about {
		t.Error("answered_0 is not tied to its question")
	}
}

func TestRequest_LeavesEmptyFieldsOut(t *testing.T) {
	state, qs, _ := Request(Input{Message: "hi"})
	for _, k := range []string{"recent_turns", "earlier_turns_omitted", "brief", "held_work", "running_work"} {
		if _, ok := state[k]; ok {
			t.Errorf("empty %s sent: an empty field reads as a signal", k)
		}
	}
	if _, ok := qs[ApprovesHeldWork]; ok {
		t.Error("approval asked with nothing held")
	}
	if _, ok := qs[WantsToStop]; ok {
		t.Error("a stop asked with nothing running: there is nothing to stop")
	}
	_, qs, _ = Request(Input{Message: "stop", RunningWork: []string{"  ", ""}})
	if _, ok := qs[WantsToStop]; ok {
		t.Error("blank running work counted as work")
	}
}

// Turns left out to fit the context are counted in the state, not dropped
// without a word.
func TestRequest_NamesTheTurnsItLeavesOut(t *testing.T) {
	turns := []Turn{{"user", "a"}, {"assistant", "b"}, {"user", "c"}, {"assistant", "d"}}
	state, _, _ := Request(Input{Message: "ok", Recent: turns, RecentTurns: 3})
	if got := state["earlier_turns_omitted"]; got != 1 {
		t.Fatalf("earlier_turns_omitted = %v, want 1", got)
	}
	if got := state["recent_turns"].([]Turn); len(got) != 3 || got[0].Text != "b" {
		t.Fatalf("recent_turns = %v, want the last three", got)
	}
}

// Every judgment is one condition: no question joins two clauses with
// "and", "without" or "but", so each answer means one thing. (An "or" between
// nouns, "file, function or feature", is one condition.)
func TestQuestionsAskOneConditionEach(t *testing.T) {
	state, qs, _ := Request(Input{
		Message: "m", RunningWork: []string{"w"}, HeldWork: &HeldWork{RequestedAs: "x"},
		Brief: &perception.Brief{Goal: "g", OpenQuestions: []string{"q?"}},
	})
	_ = state
	for name, q := range qs {
		raw, _ := json.Marshal(q)
		var w struct {
			Instructions any `json:"instructions"`
		}
		_ = json.Unmarshal(raw, &w)
		text := fmt.Sprint(w.Instructions)
		if strings.Contains(text, " and ") || strings.Contains(text, " without ") || strings.Contains(text, " but ") {
			t.Errorf("%s joins conditions in its instructions: %s", name, text)
		}
	}
}

func TestJudge_FailureIsAnErrorNotEvidence(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"detail":"bad key"}`))
	}))
	t.Cleanup(srv.Close)
	c, err := typesafe.NewClient(typesafe.Options{Model: "m", BaseURL: srv.URL, APIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	res, err := Judge(context.Background(), c, Input{Message: "fix it"})
	var apiErr *typesafe.APIError
	if res != nil || !errors.As(err, &apiErr) {
		t.Fatalf("a failed call must be an error and no result: %v %v", res, err)
	}
}
