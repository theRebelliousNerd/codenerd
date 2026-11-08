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

func TestAtomicityRequest_TooLargeIsReportedNeverCut(t *testing.T) {
	huge := atom("huge", strings.Repeat("word ", 40000*4))
	if _, _, err := AtomicityRequest(huge, nil); err == nil {
		t.Fatal("an oversized atom must be an error, never cut to fit")
	} else if !strings.Contains(err.Error(), "one Jev question holds") {
		t.Fatalf("error = %v", err)
	}
	if _, _, err := AtomicityRequest(nil, nil); err == nil {
		t.Fatal("a missing atom must be an error")
	}
}

func TestJudgeAtomicity_NilAskerIsError(t *testing.T) {
	if _, err := JudgeAtomicity(context.Background(), nil, atom("a", "x"), nil); err == nil {
		t.Fatal("a missing Jev client must be an error, never a reading")
	}
}

func TestJudgeCorpusAtomicity_SortsAndIsolatesFailures(t *testing.T) {
	asker := readings(1, 0.3, 0.3, 0.1)
	atoms := []*prompt.PromptAtom{atom("b", "check errors wrap"), atom("a", "check errors wrap"), {ID: "bad", Content: "   "}}
	results := JudgeCorpusAtomicity(context.Background(), asker, atoms, nil, 0)
	if len(results) != 3 {
		t.Fatalf("results = %v", results)
	}
	if results[0].AtomID != "a" || results[1].AtomID != "b" || results[2].AtomID != "bad" {
		t.Fatalf("results must sort by atom ID: %v", results)
	}
	for _, r := range results[:2] {
		if r.Atomicity == nil || r.Err != "" {
			t.Fatalf("result %v must carry a reading", r)
		}
	}
	if results[2].Atomicity != nil || results[2].Err == "" {
		t.Fatalf("one bad atom must fail only itself: %+v", results[2])
	}
}

func TestScoreImportance_MissingProbabilityFailsItsAtom(t *testing.T) {
	f := &fakeAsker{answer: func(string, typesafe.Question) string {
		return `{"type":"score","score":1,"confidence":0.5,"legend":{"0":"a"},"probabilities":{"0":1}}`
	}}
	results := ScoreImportance(context.Background(), f, []*prompt.PromptAtom{atom("a", "x")}, 1)
	if len(results) != 1 || results[0].Err == nil || !strings.Contains(results[0].Err.Error(), "lacks level") {
		t.Fatalf("a mistyped distribution must fail its atom: %+v", results)
	}
}

func TestStaleAtoms_NilLedgerReturnsAllSorted(t *testing.T) {
	got := StaleAtoms(nil, []*prompt.PromptAtom{atom("b", "y"), atom("a", "x")})
	if len(got) != 2 || got[0].ID != "a" || got[1].ID != "b" {
		t.Fatalf("nil ledger must stale every atom in ID order: %v", got)
	}
	if got := StaleAtoms(nil, nil); len(got) != 0 {
		t.Fatalf("empty pool must stay empty: %v", got)
	}
}

func TestRoundTo_ThreePlaces(t *testing.T) {
	if got := roundTo(1.23456, 3); got != 1.235 {
		t.Fatalf("roundTo = %v", got)
	}
	if got := roundTo(1.2, 3); got != 1.2 {
		t.Fatalf("roundTo = %v", got)
	}
}

func TestGuidanceOf_StripsSlashesAndOmitsEmpty(t *testing.T) {
	a := &prompt.PromptAtom{ID: "x", Category: prompt.CategoryLanguage, Content: "do x", ContentHash: prompt.HashContent("do x"), ShardTypes: []string{"/coder"}, Languages: []string{"/go"}}
	g := guidanceOf(a)
	if agents, _ := g["agents"].([]string); len(agents) != 1 || agents[0] != "coder" {
		t.Fatalf("agents = %v", g["agents"])
	}
	if _, ok := g["description"]; ok {
		t.Fatalf("empty description must be omitted: %v", g)
	}
	if _, ok := g["frameworks"]; ok {
		t.Fatalf("empty selectors must be omitted: %v", g)
	}
}

func TestPlanRelevance_RejectsBadLimitsAndOversizedGuidance(t *testing.T) {
	req := prompt.AtomRelevanceRequest{Request: "fix errors"}
	if _, err := (RelevanceJudge{Asker: yesAsker(), MaxAtoms: -1}).planRelevance(context.Background(), req, relevanceAtoms(1)); err == nil {
		t.Fatal("a negative limit must be an error")
	}
	huge := atom("huge", strings.Repeat("word ", 40000*4))
	if _, err := (RelevanceJudge{Asker: yesAsker(), MaxAtoms: 100, MaxInputTokens: 100000}).planRelevance(context.Background(), req, []*prompt.PromptAtom{huge}); err == nil {
		t.Fatal("oversized guidance must be refused before any Ask call")
	} else if !strings.Contains(err.Error(), "one Jev question holds") {
		t.Fatalf("error = %v", err)
	}
	plan, err := (RelevanceJudge{Asker: yesAsker()}).planRelevance(context.Background(), req, nil)
	if err != nil || len(plan.batches) != 0 || plan.inputTokens != 0 {
		t.Fatalf("empty pool = %+v, %v", plan, err)
	}
	if err := (&RelevanceBudgetError{Resource: "atoms", Required: 3, Limit: 2}).Error(); !strings.Contains(err, "atom relevance budget exceeded") {
		t.Fatalf("budget error = %q", err)
	}
}
