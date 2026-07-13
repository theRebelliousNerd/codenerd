package prompt

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"
)

type panicCompilationScope struct {
	closed *atomic.Int32
}

func (s *panicCompilationScope) Query(string) ([]Fact, error) {
	panic("synthetic selector panic")
}

func (s *panicCompilationScope) AssertBatch([]any) error { return nil }

func (s *panicCompilationScope) Close() error {
	s.closed.Add(1)
	return nil
}

type panicScopeProvider struct {
	closed atomic.Int32
}

func (p *panicScopeProvider) Query(string) ([]Fact, error) { return nil, nil }
func (p *panicScopeProvider) AssertBatch([]any) error      { return nil }
func (p *panicScopeProvider) NewCompilationScope() (KernelCompilationScope, error) {
	return &panicCompilationScope{closed: &p.closed}, nil
}

func TestJITPromptCompiler_CompilationScopeClosesAfterSelectorPanic(t *testing.T) {
	provider := &panicScopeProvider{}
	atoms := []*PromptAtom{
		{ID: "identity", Category: CategoryIdentity, Content: "identity", IsMandatory: true, TokenCount: 1},
		{ID: "protocol", Category: CategoryProtocol, Content: "protocol", IsMandatory: true, TokenCount: 1},
		{ID: "safety", Category: CategorySafety, Content: "safety", IsMandatory: true, TokenCount: 1},
		{ID: "method", Category: CategoryMethodology, Content: "method", IsMandatory: true, TokenCount: 1},
	}
	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(NewEmbeddedCorpus(atoms)),
		WithKernel(provider),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}

	_, err = compiler.Compile(context.Background(), NewCompilationContext())
	if err == nil {
		t.Fatal("Compile() error = nil, want recovered selector panic")
	}
	if !strings.Contains(err.Error(), "selector panic") {
		t.Fatalf("Compile() error = %v, want recovered selector panic", err)
	}
	if provider.closed.Load() != 1 {
		t.Fatalf("compilation scope closed %d times, want 1", provider.closed.Load())
	}
}
