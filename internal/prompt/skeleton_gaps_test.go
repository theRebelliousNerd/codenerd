package prompt

import (
	"context"
	"slices"
	"sync"
	"testing"

	"codenerd/internal/core"
)

// supersededRecorder is the production querier with every compile scope's
// mandatory_superseded answer recorded, so a test can see what the kernel
// derived inside the private scope it closes.
type supersededRecorder struct {
	*realKernelQuerier
	mu         sync.Mutex
	superseded map[string]bool
}

type recordingScope struct {
	*realKernelQuerier
	rec *supersededRecorder
}

func (s *recordingScope) Close() error { return nil }

func (s *recordingScope) Query(predicate string) ([]Fact, error) {
	rows, err := s.realKernelQuerier.Query(predicate)
	if err == nil && predicate == "mandatory_superseded(Atom)" {
		s.rec.mu.Lock()
		for _, r := range rows {
			if id, ok := r.Args[0].(string); ok {
				s.rec.superseded[id] = true
			}
		}
		s.rec.mu.Unlock()
	}
	return rows, err
}

func (r *supersededRecorder) NewCompilationScope() (KernelCompilationScope, error) {
	return &recordingScope{realKernelQuerier: &realKernelQuerier{k: r.k.Clone()}, rec: r}, nil
}

// The delegation judge's compile (internal/verification/verifier.go) has no
// protocol and no methodology atom by design: its atoms declare
// conflicts_with over the Piggyback protocol and OODA methodology. The
// 2026-10-28 dogfood log reported both categories as "has no selected atoms"
// on every judge call. A gap the kernel derived (mandatory_superseded) is the
// compile's shape; only an unexplained gap is a selection defect.
func TestSkeletonGaps_JudgeCompileSupersessionIsNotADefect(t *testing.T) {
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	embedded, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	rec := &supersededRecorder{realKernelQuerier: &realKernelQuerier{k: k}, superseded: map[string]bool{}}
	compiler, err := NewJITPromptCompiler(WithKernel(rec), WithEmbeddedCorpus(embedded))
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	cc := NewCompilationContextWithBudget(192000)
	cc.ShardType = "/judge_implementation"
	cc.OperationalMode = "/active"
	cc.IntentVerb = "/judge"
	result, err := compiler.Compile(context.Background(), cc)
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	var selected []*ScoredAtom
	for _, a := range result.IncludedAtoms {
		if a.Category == CategoryProtocol || a.Category == CategoryMethodology {
			t.Fatalf("judge compile included %s atom %s; its atoms supersede that category", a.Category, a.ID)
		}
		if isSkeletonCategory(a.Category) {
			selected = append(selected, &ScoredAtom{Atom: a})
		}
	}
	rec.mu.Lock()
	superseded := rec.superseded
	rec.mu.Unlock()
	if !superseded["protocol/piggyback/envelope"] || !superseded["methodology/ooda/core"] {
		t.Fatalf("kernel did not derive the judge's supersessions: %v", superseded)
	}

	var candidates []*PromptAtom
	for _, a := range embedded.All() {
		if a != nil && isSkeletonCategory(a.Category) {
			candidates = append(candidates, a)
		}
	}
	gaps := unexplainedSkeletonGaps(selected, candidates, superseded)
	for _, cat := range []AtomCategory{CategoryProtocol, CategoryMethodology} {
		if slices.Contains(gaps, cat) {
			t.Errorf("superseded category %s reported as an unexplained skeleton gap", cat)
		}
	}
}

// An empty skeleton category nothing superseded is still reported.
func TestSkeletonGaps_UnexplainedGapIsReported(t *testing.T) {
	selected := []*ScoredAtom{
		{Atom: &PromptAtom{ID: "identity/x", Category: CategoryIdentity}},
		{Atom: &PromptAtom{ID: "safety/x", Category: CategorySafety}},
	}
	candidates := []*PromptAtom{
		{ID: "protocol/p", Category: CategoryProtocol},
		{ID: "methodology/m", Category: CategoryMethodology},
	}
	got := unexplainedSkeletonGaps(selected, candidates, map[string]bool{"protocol/p": true})
	if !slices.Equal(got, []AtomCategory{CategoryMethodology}) {
		t.Fatalf("gaps = %v, want [methodology]", got)
	}
}
