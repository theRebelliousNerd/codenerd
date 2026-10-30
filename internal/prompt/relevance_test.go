package prompt

import (
	"codenerd/internal/store"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// fixedSearcher scores atoms from a table.
type fixedSearcher map[string]float64

func (f fixedSearcher) Search(_ context.Context, _ string, _ int) ([]SearchResult, error) {
	out := make([]SearchResult, 0, len(f))
	for id, s := range f {
		out = append(out, SearchResult{AtomID: id, Score: s})
	}
	return out, nil
}

func (fixedSearcher) EmbedQuery(context.Context, string) (store.QueryEmbedding, error) {
	return store.QueryEmbedding{}, nil
}

// countingJudge answers from a table and counts what it was asked.
type countingJudge struct {
	mu      sync.Mutex
	answers map[string]float64
	err     error
	delay   time.Duration
	calls   int
	asked   []string
	lastReq AtomRelevanceRequest
}

func (j *countingJudge) JudgeAtomRelevance(ctx context.Context, req AtomRelevanceRequest, atoms []*PromptAtom) (map[string]float64, error) {
	j.mu.Lock()
	j.calls++
	j.lastReq = req
	for _, a := range atoms {
		j.asked = append(j.asked, a.ID)
	}
	j.mu.Unlock()
	if j.delay > 0 {
		select {
		case <-time.After(j.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if j.err != nil {
		return nil, j.err
	}
	out := map[string]float64{}
	for _, a := range atoms {
		if p, ok := j.answers[a.ID]; ok {
			out[a.ID] = p
		}
	}
	return out, nil
}

// recordingKernel keeps every asserted fact and answers no query.
type recordingKernel struct {
	mu    sync.Mutex
	facts []string
}

func (k *recordingKernel) Query(string) ([]Fact, error) { return nil, nil }
func (k *recordingKernel) AssertBatch(facts []any) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, f := range facts {
		k.facts = append(k.facts, fmt.Sprint(f))
	}
	return nil
}

func (k *recordingKernel) with(prefix string) []string {
	k.mu.Lock()
	defer k.mu.Unlock()
	var out []string
	for _, f := range k.facts {
		if strings.HasPrefix(f, prefix) {
			out = append(out, f)
		}
	}
	return out
}

// relevanceCorpus is one mandatory skeleton atom plus twelve flesh atoms
// scored 0.90 down to 0.35; the relative floor (mean + 1.5 sd) keeps only the
// top two as vector slots.
func relevanceCorpus() ([]*PromptAtom, fixedSearcher) {
	atoms := []*PromptAtom{{ID: "identity/core", Category: CategoryIdentity, Content: "id", IsMandatory: true}}
	scores := fixedSearcher{}
	for i := range 12 {
		id := fmt.Sprintf("flesh/%02d", i)
		atoms = append(atoms, &PromptAtom{ID: id, Category: CategoryExemplar, Content: "body " + id, ContentHash: HashContent("body " + id)})
		scores[id] = 0.90 - float64(i)*0.05
	}
	return atoms, scores
}

func relevanceSelector(judge AtomRelevanceJudge, pool int, timeout time.Duration) *AtomSelector {
	_, scores := relevanceCorpus()
	s := NewAtomSelector()
	s.SetVectorSearcher(scores)
	if judge != nil {
		s.SetAtomRelevance(AtomRelevance{Judge: judge, Pool: pool, Timeout: timeout, YesPercent: 70, NoPercent: 30})
	}
	return s
}

func relevanceContext() *CompilationContext {
	cc := NewCompilationContext().WithSemanticQuery("fix the nil pointer in broker.go", 10)
	cc.IntentVerb = "/fix"
	cc.IntentTarget = "internal/approval/broker.go"
	cc.Language = "/go"
	cc.ShardType = "/coder"
	return cc
}

func TestAtomRelevance_OffAssertsOnlyTheVectorSlots(t *testing.T) {
	atoms, _ := relevanceCorpus()
	k := &recordingKernel{}
	if _, err := relevanceSelector(nil, 0, 0).runSelectionFor(context.Background(), atoms, relevanceContext(), k); err != nil {
		t.Fatal(err)
	}
	hits, slots := k.with("vector_hit("), k.with("vector_slot(")
	if len(hits) == 0 || len(hits) != len(slots) {
		t.Fatalf("Jev off: vector_hit %d, vector_slot %d; want the same non-empty set", len(hits), len(slots))
	}
	if got := k.with("atom_relevance"); len(got) != 0 {
		t.Fatalf("Jev off asserted relevance facts: %v", got)
	}
}

func TestAtomRelevance_JudgesThePoolAndAssertsBands(t *testing.T) {
	atoms, _ := relevanceCorpus()
	judge := &countingJudge{answers: map[string]float64{}}
	for i := range 12 {
		judge.answers[fmt.Sprintf("flesh/%02d", i)] = float64(i) / 12
	}
	k := &recordingKernel{}
	if _, err := relevanceSelector(judge, 6, time.Second).runSelectionFor(context.Background(), atoms, relevanceContext(), k); err != nil {
		t.Fatal(err)
	}
	if len(judge.asked) != 6 {
		t.Fatalf("judged %d atoms, want the pool of 6: %v", len(judge.asked), judge.asked)
	}
	for _, id := range judge.asked {
		if id == "identity/core" {
			t.Fatal("a mandatory skeleton atom was put to Jev")
		}
	}
	if hits := k.with("vector_hit("); len(hits) != 6 {
		t.Fatalf("vector_hit covers %d atoms, want the pool of 6", len(hits))
	}
	if slots := k.with("vector_slot("); len(slots) >= 6 || len(slots) == 0 {
		t.Fatalf("vector_slot covers %d atoms, want the vector channel's own slots", len(slots))
	}
	if rel := k.with("atom_relevance("); len(rel) != 6 {
		t.Fatalf("atom_relevance rows = %d, want 6", len(rel))
	}
	th := k.with("atom_relevance_threshold(")
	if len(th) != 2 || !strings.Contains(strings.Join(th, " "), "(/yes, 70)") || !strings.Contains(strings.Join(th, " "), "(/no, 30)") {
		t.Fatalf("threshold rows = %v", th)
	}
	if r := judge.lastReq; r.Request == "" || r.Intent != "fix" || r.Target != "internal/approval/broker.go" || r.Language != "go" || r.Agent != "coder" {
		t.Fatalf("request state = %+v", r)
	}
}

func TestAtomRelevance_OneJudgmentPerTurn(t *testing.T) {
	atoms, _ := relevanceCorpus()
	judge := &countingJudge{answers: map[string]float64{}}
	for i := range 12 {
		judge.answers[fmt.Sprintf("flesh/%02d", i)] = 0.8
	}
	s := relevanceSelector(judge, 6, time.Second)
	for range 3 {
		if _, err := s.runSelectionFor(context.Background(), atoms, relevanceContext(), &recordingKernel{}); err != nil {
			t.Fatal(err)
		}
	}
	if judge.calls != 1 {
		t.Fatalf("three compiles of one turn called Jev %d times, want 1", judge.calls)
	}
	other := relevanceContext()
	other.SemanticQuery = "a different request"
	if _, err := s.runSelectionFor(context.Background(), atoms, other, &recordingKernel{}); err != nil {
		t.Fatal(err)
	}
	if judge.calls != 2 {
		t.Fatalf("a new request reused the last turn's answers (calls=%d)", judge.calls)
	}
}

func TestAtomRelevance_FailureOrLatenessIsNoEvidence(t *testing.T) {
	atoms, _ := relevanceCorpus()
	for name, judge := range map[string]*countingJudge{
		"failed":     {err: errors.New("503")},
		"late":       {delay: time.Second, answers: map[string]float64{}},
		"incomplete": {answers: map[string]float64{"flesh/00": 0.9}},
	} {
		t.Run(name, func(t *testing.T) {
			k := &recordingKernel{}
			if _, err := relevanceSelector(judge, 6, 20*time.Millisecond).runSelectionFor(context.Background(), atoms, relevanceContext(), k); err != nil {
				t.Fatal(err)
			}
			if rel := k.with("atom_relevance"); len(rel) != 0 {
				t.Fatalf("a %s judgment asserted %v", name, rel)
			}
			hits, slots := k.with("vector_hit("), k.with("vector_slot(")
			if len(hits) != len(slots) {
				t.Fatalf("a %s judgment widened recall: %d hits vs %d slots", name, len(hits), len(slots))
			}
		})
	}
}

func TestRecallPool_SlotsFirstThenMostSimilarEligible(t *testing.T) {
	atoms, scores := relevanceCorpus()
	s := NewAtomSelector()
	flesh := filterFleshAtoms(atoms, relevanceContext())
	slots := map[string]float64{"flesh/00": 0.90}
	pool := s.recallPool(scores, slots, flesh, relevanceContext(), 4)
	for _, id := range []string{"flesh/00", "flesh/01", "flesh/02", "flesh/03"} {
		if _, ok := pool[id]; !ok {
			t.Fatalf("pool %v lacks %s", pool, id)
		}
	}
	if len(pool) != 4 {
		t.Fatalf("pool has %d atoms, want 4", len(pool))
	}
	if got := s.recallPool(scores, slots, flesh, relevanceContext(), 0); len(got) != 1 {
		t.Fatalf("a pool smaller than the slots still keeps every slot, got %v", got)
	}
}

func TestProbabilityPercent(t *testing.T) {
	for p, want := range map[float64]int64{-1: 0, 0: 0, 0.296: 30, 0.7: 70, 1: 100, 2: 100} {
		if got := probabilityPercent(p); got != want {
			t.Errorf("probabilityPercent(%v) = %d, want %d", p, got, want)
		}
	}
}

// runSelectionFor runs one selection against the given kernel.
func (s *AtomSelector) runSelectionFor(ctx context.Context, atoms []*PromptAtom, cc *CompilationContext, k KernelQuerier) ([]*ScoredAtom, error) {
	merged, _, err := s.runSelection(ctx, atoms, cc, k, false)
	return merged, err
}

// queryLog records the predicates queried.
type queryLog struct{ queries []string }

func (q *queryLog) Query(p string) ([]Fact, error) {
	q.queries = append(q.queries, p)
	return []Fact{{Predicate: "x", Args: []any{"a", "/judged_applies"}}}, nil
}
func (q *queryLog) AssertBatch([]any) error { return nil }

// TestRelevanceAdmission_ReportsTheKernelsRows: after a judged compile the
// selector reads the kernel's admission table rather than recomputing the
// bands in Go.
func TestRelevanceAdmission_ReportsTheKernelsRows(t *testing.T) {
	q := &queryLog{}
	logRelevanceAdmission(q, relevanceContext())
	if strings.Join(q.queries, ";") != "flesh_admitted(Atom, Row);flesh_refused(Atom, Row)" {
		t.Fatalf("queried %v", q.queries)
	}
}

func TestCarry_RememberedInPromptOrderAndRejoinsThePool(t *testing.T) {
	s := NewAtomSelector()
	cc := relevanceContext()
	mk := func(id string) *ScoredAtom { return &ScoredAtom{Atom: &PromptAtom{ID: id}} }
	s.rememberCarry(cc, []*ScoredAtom{mk("z/new"), mk("b/kept"), mk("a/new"), mk("c/kept")}, map[string]int{"c/kept": 0, "b/kept": 1})
	if got := strings.Join(s.carry.get(carryKey(cc)), ","); got != "c/kept,b/kept,a/new,z/new" {
		t.Fatalf("carried order = %s", got)
	}
	other := relevanceContext()
	other.IntentTarget = "another/file.go"
	if len(s.carry.get(carryKey(other))) != 0 {
		t.Fatal("a different task inherited the carry")
	}

	atoms, scores := relevanceCorpus()
	flesh := filterFleshAtoms(atoms, cc)
	pool := map[string]float64{"flesh/00": 0.9}
	got := carriedInto(pool, scores, []string{"flesh/11", "gone/atom", "identity/core"}, flesh, cc)
	if strings.Join(got, ",") != "flesh/11" {
		t.Fatalf("carried = %v; an atom no longer eligible, or mandatory, is not carried", got)
	}
	if _, ok := pool["flesh/11"]; !ok {
		t.Fatal("a carried atom did not rejoin the recall pool")
	}
}

func TestAssembler_CarriedFleshPrecedesNewFlesh(t *testing.T) {
	a := NewFinalAssembler()
	atoms := []*OrderedAtom{
		{Atom: &PromptAtom{ID: "new", Category: CategoryLanguage, Content: "NEW"}, Order: 0},
		{Atom: &PromptAtom{ID: "kept", Category: CategoryExemplar, Content: "KEPT"}, Order: 1, Carried: true},
	}
	out, err := a.Assemble(atoms, NewCompilationContext())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(out, "KEPT") > strings.Index(out, "NEW") {
		t.Fatalf("carried flesh must precede new flesh whatever its category:\n%s", out)
	}
}
