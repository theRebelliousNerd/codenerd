package atomjudge

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// fakeAsker answers every question with the function's answer JSON and keeps
// each request's state and questions as sent.
type fakeAsker struct {
	mu     sync.Mutex
	answer func(name string, q typesafe.Question) string
	err    error
	states []any
	bodies []map[string]typesafe.Question
}

func (f *fakeAsker) Ask(_ context.Context, state any, questions map[string]typesafe.Question) (*typesafe.Response, error) {
	f.mu.Lock()
	f.states = append(f.states, state)
	f.bodies = append(f.bodies, questions)
	f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	answers := map[string]json.RawMessage{}
	for name, q := range questions {
		answers[name] = json.RawMessage(f.answer(name, q))
	}
	raw, _ := json.Marshal(map[string]any{"model": "jev-test", "answers": answers, "usage": map[string]int{"input_tokens": 1, "output_tokens": 0}})
	var r typesafe.Response
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

func atom(id, content string) *prompt.PromptAtom {
	return &prompt.PromptAtom{ID: id, Category: prompt.CategoryLanguage, Content: content, ContentHash: prompt.HashContent(content), Languages: []string{"/go"}}
}

func TestScoreImportance_LevelsAreTheWrittenMeaningsAndRecordsKeepTheDistribution(t *testing.T) {
	f := &fakeAsker{answer: func(string, typesafe.Question) string {
		return `{"type":"score","score":3.2,"confidence":0.5,"legend":{"0":"a"},"probabilities":{"0":0,"1":0,"2":0.1,"3":0.6,"4":0.3}}`
	}}
	atoms := []*prompt.PromptAtom{atom("a", "check errors"), atom("b", "wrap with %w")}
	results := ScoreImportance(context.Background(), f, atoms, 2)
	if len(results) != 2 {
		t.Fatalf("%d results", len(results))
	}
	for _, r := range results {
		if r.Err != nil || r.Record.Level != prompt.ImportanceCorrectness || r.Model != "jev-test" || len(r.Record.Probabilities) != 5 {
			t.Fatalf("result %+v", r)
		}
	}
	body, _ := json.Marshal(f.bodies[0]["atom_0"])
	for _, meaning := range prompt.ImportanceMeanings() {
		if !strings.Contains(string(body), meaning) {
			t.Fatalf("score criteria lack the level meaning %q: %s", meaning, body)
		}
	}
	if !strings.Contains(string(body), `"type":"score"`) || !strings.Contains(string(body), "check errors") {
		t.Fatalf("question is not a Score over the atom's text: %s", body)
	}
}

func TestScoreImportance_AFailedRequestLeavesItsAtomsUnscored(t *testing.T) {
	f := &fakeAsker{err: errors.New("429")}
	results := ScoreImportance(context.Background(), f, []*prompt.PromptAtom{atom("a", "x")}, 1)
	if len(results) != 1 || results[0].Err == nil {
		t.Fatalf("a failed request produced %+v", results)
	}
}

func TestBatches_RespectTheBudgetAndReportOversizedAtoms(t *testing.T) {
	var atoms []*prompt.PromptAtom
	for i := range 90 {
		atoms = append(atoms, atom(fmt.Sprintf("a%02d", i), "short"))
	}
	huge := atom("huge", strings.Repeat("word ", 40000*4))
	groups, tooLarge := batches(append(atoms, huge))
	if len(tooLarge) != 1 || tooLarge[0].AtomID != "huge" {
		t.Fatalf("oversized atoms = %+v; an atom too large for a question must be reported, never cut", tooLarge)
	}
	total := 0
	for _, g := range groups {
		if len(g) > batchMaxQuestions {
			t.Fatalf("a batch of %d questions", len(g))
		}
		total += len(g)
	}
	if total != 90 {
		t.Fatalf("batched %d of 90 atoms", total)
	}
}

func TestRelevanceJudge_StateIsTheRequestAndEachAtomIsItsOwnQuestion(t *testing.T) {
	f := &fakeAsker{answer: func(string, typesafe.Question) string { return `{"type":"noul","noul":0.8,"confidence":0.6}` }}
	req := prompt.AtomRelevanceRequest{Request: "fix the panic", Intent: "fix", Target: "broker.go", Language: "go", Agent: "coder"}
	got, err := RelevanceJudge{Asker: f}.JudgeAtomRelevance(context.Background(), req, []*prompt.PromptAtom{atom("a", "x"), atom("b", "y")})
	if err != nil || got["a"] != 0.8 || got["b"] != 0.8 {
		t.Fatalf("answers %v, %v", got, err)
	}
	state, _ := json.Marshal(f.states[0])
	for _, want := range []string{`"text":"fix the panic"`, `"intent":"fix"`, `"target":"broker.go"`, `"agent":"coder"`} {
		if !strings.Contains(string(state), want) {
			t.Fatalf("state %s lacks %s", state, want)
		}
	}
	if strings.Contains(string(state), "frameworks") {
		t.Fatalf("state carries an empty field: %s", state)
	}
	q, _ := json.Marshal(f.bodies[0]["atom_1"])
	if !strings.Contains(string(q), `"type":"noul"`) || !strings.Contains(string(q), `"id":"b"`) {
		t.Fatalf("question for b: %s", q)
	}
}

func TestRelevanceJudge_AnyMissingAnswerFailsTheWhole(t *testing.T) {
	f := &fakeAsker{answer: func(name string, _ typesafe.Question) string {
		if name == "atom_1" {
			return `{"type":"score","score":1,"probabilities":{"0":0,"1":1}}`
		}
		return `{"type":"noul","noul":0.9}`
	}}
	if got, err := (RelevanceJudge{Asker: f}).JudgeAtomRelevance(context.Background(), prompt.AtomRelevanceRequest{Request: "r"}, []*prompt.PromptAtom{atom("a", "x"), atom("b", "y")}); err == nil {
		t.Fatalf("a mistyped answer became evidence: %v", got)
	}
}

func TestRelevanceFor_SwitchAndThresholds(t *testing.T) {
	f := &fakeAsker{}
	if r := RelevanceFor(nil, &config.TypeSafeConfig{}); r.Judge != nil {
		t.Fatal("no Jev, yet a judge")
	}
	off := false
	if r := RelevanceFor(f, &config.TypeSafeConfig{AtomRelevance: &off}); r.Judge != nil {
		t.Fatal("atom_relevance: false, yet a judge")
	}
	r := RelevanceFor(f, &config.TypeSafeConfig{JudgmentYesPercent: 75, JudgmentNoPercent: 25, AtomRelevancePool: 9,
		AtomRelevanceMaxAtoms: 15, AtomRelevanceMaxInputTokens: 12345, AtomRelevanceMaxConcurrentBatches: 3})
	if r.Judge == nil || r.YesPercent != 75 || r.NoPercent != 25 || r.Pool != 9 || r.Timeout <= 0 {
		t.Fatalf("resolved %+v", r)
	}
	j := r.Judge.(RelevanceJudge)
	if j.MaxAtoms != 15 || j.MaxInputTokens != 12345 || j.MaxConcurrentBatches != 3 {
		t.Fatalf("work limits did not reach the judge: %+v", j)
	}
}

// TestEmbeddedLedgerCoversEveryAtom: every embedded atom has a build-time
// importance score for exactly its current content, answered to the current
// question. Refresh with `go run ./cmd/tools/prompt_builder -score-importance`.
func TestEmbeddedLedgerCoversEveryAtom(t *testing.T) {
	ledger, err := prompt.EmbeddedImportanceLedger()
	if err != nil {
		t.Fatal(err)
	}
	if ledger.Scale != ImportanceScaleFingerprint() {
		t.Fatalf("ledger answered scale %q, the question is now %q: rescore every atom", ledger.Scale, ImportanceScaleFingerprint())
	}
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatal(err)
	}
	stale := StaleAtoms(ledger, corpus.All())
	if len(stale) > 0 {
		ids := make([]string, 0, min(len(stale), 10))
		for _, a := range stale[:min(len(stale), 10)] {
			ids = append(ids, a.ID)
		}
		t.Fatalf("%d embedded atoms lack a current importance score (first %v); regenerate them with the regenerate tool (generator atom-importance) or `go run ./cmd/tools/prompt_builder -score-importance`", len(stale), ids)
	}
}
