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

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
)

// fakeJev answers every noul question with the probability its name maps to
// (0.5 when unlisted) and every choice with an even split over its options,
// recording the request it saw.
func fakeJev(t *testing.T, answers map[string]float64) (*typesafe.Client, *map[string]any) {
	return fakeJevChoosing(t, answers, nil)
}

// fakeJevChoosing is fakeJev with each listed choice answered by its
// distribution.
func fakeJevChoosing(t *testing.T, answers map[string]float64, choices map[string]map[string]float64) (*typesafe.Client, *map[string]any) {
	t.Helper()
	var seen map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode request: %v", err)
		}
		seen = req
		out := map[string]any{}
		for name, q := range req["questions"].(map[string]any) {
			q := q.(map[string]any)
			if q["type"] == "choice" {
				probs := choices[name]
				if probs == nil {
					opts := q["criteria"].(map[string]any)
					probs = map[string]float64{}
					for o := range opts {
						probs[o] = 1 / float64(len(opts))
					}
				}
				best, bp := "", -1.0
				for o, p := range probs {
					if p > bp {
						best, bp = o, p
					}
				}
				out[name] = map[string]any{"type": "choice", "choice": best, "confidence": bp, "probabilities": probs}
				continue
			}
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
	c, seen := fakeJevChoosing(t, map[string]float64{ChangesFiles: 0.91, "answered_0": 0.88, ApprovesHeldWork: 0.07},
		map[string]map[string]float64{ConversationAct: {ActReply: 0.814, ActRequestForWork: 0.186}})
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
	for _, name := range []string{ConversationAct, ChangesFiles, NamesTarget, Hypothetical, AboutCodeNERD, WantsToStop, AsksAboutRunning, RulesOutChanges, AsksToCommit, ApprovesHeldWork, AddsToHeldWork, "answered_0"} {
		if _, ok := qs[name]; !ok {
			t.Errorf("question %s was not asked", name)
		}
	}
	if len(qs) != 12 {
		t.Errorf("asked %d questions, want exactly the 12 the rules read: %v", len(qs), qs)
	}
	act := qs[ConversationAct].(map[string]any)
	if act["type"] != "choice" || len(act["criteria"].(map[string]any)) != len(ConversationActs) {
		t.Errorf("the conversation act is not a choice over every act: %v", act)
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
	choice := map[string]int64{}
	for _, f := range facts {
		if f.Predicate == "jev_judgment_about" && types.ExtractString(f.Args[1]) == "which algorithm?" {
			about = true
		}
		if f.Predicate == "jev_choice" && types.ExtractString(f.Args[0]) == ConversationAct {
			choice[types.ExtractString(f.Args[1])], _ = types.ExtractInt64(f.Args[2])
		}
	}
	if !about {
		t.Error("answered_0 is not tied to its question")
	}
	// The whole distribution goes to the kernel, each option rounded to a
	// whole percent; which counts as chosen is the kernel's reading.
	if len(choice) != 2 || choice[ActReply] != 81 || choice[ActRequestForWork] != 19 {
		t.Errorf("choice projection wrong: %v", choice)
	}
	if _, ok := res.Judgments[ConversationAct]; ok {
		t.Error("a choice was read as a noul")
	}
	if !strings.Contains(res.Summary(), "conversation_act=reply_to_codenerd:0.81") {
		t.Errorf("summary does not show the top act: %s", res.Summary())
	}
}

// A choice answered with the wrong type is an error, never a guess: the
// whole turn then has no evidence.
func TestJudge_MalformedChoiceIsAnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]any
		_ = json.NewDecoder(r.Body).Decode(&req)
		out := map[string]any{}
		for name := range req["questions"].(map[string]any) {
			out[name] = map[string]any{"type": "noul", "noul": 0.9}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"model": "m", "answers": out, "usage": map[string]any{"input_tokens": 1, "output_tokens": 1}})
	}))
	t.Cleanup(srv.Close)
	c, err := typesafe.NewClient(typesafe.Options{Model: "m", BaseURL: srv.URL, APIKey: "k"})
	if err != nil {
		t.Fatal(err)
	}
	if res, err := Judge(context.Background(), c, Input{Message: "yes"}); res != nil || err == nil || !strings.Contains(err.Error(), ConversationAct) {
		t.Fatalf("a noul answer to the act choice must fail the call: %v %v", res, err)
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
	if _, ok := qs[AsksAboutRunning]; ok {
		t.Error("a progress question asked with nothing running")
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

// The commit question is about the user's message alone: its wording names
// `message`, and earlier turns ride the state as context, never as the thing
// judged (an assistant line or a pasted log that says "commit it" is not the
// user telling codeNERD to).
func TestAsksToCommit_IsAboutTheMessageNotTheContext(t *testing.T) {
	_, questions, _ := Request(Input{Message: "fix it", Recent: []Turn{{From: "assistant", Text: "I can commit it"}}})
	q, ok := questions[AsksToCommit].(typesafe.Noul)
	if !ok {
		t.Fatalf("%s is not asked as a Noul: %T", AsksToCommit, questions[AsksToCommit])
	}
	if want := "`message`"; !strings.Contains(fmt.Sprint(q.Instructions), want) {
		t.Errorf("the question does not name %s: %q", want, q.Instructions)
	}
	if !strings.Contains(fmt.Sprint(q.Criteria.False), "does not say to commit") || !strings.Contains(fmt.Sprint(q.Criteria.True), "itself says to commit") {
		t.Errorf("criteria lost the message-only reading: %+v", q.Criteria)
	}
}

// AuthorizationFrom reads the kernel's conclusion; it decides nothing and
// treats a missing judgment as no evidence.
func TestAuthorizationFrom_ReadsTheKernelsConclusionOnly(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if got := AuthorizationFrom(k); !got.IsZero() {
		t.Fatalf("a kernel with no judgments authorized %v", got.Kinds())
	}
	if got := AuthorizationFrom(nil); !got.IsZero() {
		t.Fatal("a nil kernel authorized something")
	}
	judge := func(commit, readOnly, whatIf float64) perception.Authorization {
		t.Helper()
		for _, p := range Predicates {
			_ = k.Retract(p)
		}
		res := &Result{Judgments: map[string]float64{AsksToCommit: commit, RulesOutChanges: readOnly, Hypothetical: whatIf}, Choices: map[string]map[string]float64{}, About: map[string]string{}}
		if err := config.EnsureParams(k, (&config.TypeSafeConfig{}).Params()); err != nil {
			t.Fatal(err)
		}
		if err := k.AssertBatch(res.Facts()); err != nil {
			t.Fatal(err)
		}
		return AuthorizationFrom(k)
	}
	for _, tc := range []struct {
		name                     string
		commit, readOnly, whatIf float64
		want                     bool
	}{
		{"yes", 0.98, 0.02, 0.02, true},
		{"exactly the yes threshold", 0.70, 0.02, 0.02, true},
		{"below it", 0.69, 0.02, 0.02, false},
		{"unsettled", 0.5, 0.02, 0.02, false},
		{"no", 0.03, 0.02, 0.02, false},
		{"commit but read only", 0.98, 0.95, 0.02, false},
		{"commit but a what-if", 0.98, 0.02, 0.95, false},
		{"commit, read-only unsettled", 0.98, 0.5, 0.02, true},
	} {
		if got := judge(tc.commit, tc.readOnly, tc.whatIf).Has(perception.AuthorizeCommit); got != tc.want {
			t.Errorf("%s: commit authorization = %v, want %v", tc.name, got, tc.want)
		}
	}
}
