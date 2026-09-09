package prompt

import (
	"testing"
)

type fakeStrategyProvider struct {
	atoms  []*PromptAtom
	calls  int
	panics bool
}

func (f *fakeStrategyProvider) StrategyAtoms(cc *CompilationContext) []*PromptAtom {
	f.calls++
	if f.panics {
		panic("provider exploded")
	}
	return f.atoms
}

func TestCollectStrategyAtoms_NoProviderIsNoSource(t *testing.T) {
	c := &JITPromptCompiler{}
	if got := c.collectStrategyAtoms(&CompilationContext{}); got != nil {
		t.Errorf("an unregistered provider produced %d atoms", len(got))
	}
}

func TestRegisterStrategyProvider_NilRemovesTheSource(t *testing.T) {
	c := &JITPromptCompiler{}
	p := &fakeStrategyProvider{atoms: []*PromptAtom{NewPromptAtom("strategy/x", CategoryMethodology, "body")}}

	c.RegisterStrategyProvider(p)
	if got := c.collectStrategyAtoms(&CompilationContext{}); len(got) != 1 {
		t.Fatalf("registered provider yielded %d atoms, want 1", len(got))
	}

	c.RegisterStrategyProvider(nil)
	if got := c.collectStrategyAtoms(&CompilationContext{}); got != nil {
		t.Errorf("clearing the provider left %d atoms; the five-source pipeline must fall back to four", len(got))
	}
}

// TestCollectStrategyAtoms_ContainsAPanickingProvider: the strategy source is
// the newest and least load-bearing of the five, and a prompt without its
// strategies is still a working prompt. Losing the turn to it would not be.
func TestCollectStrategyAtoms_ContainsAPanickingProvider(t *testing.T) {
	c := &JITPromptCompiler{}
	c.RegisterStrategyProvider(&fakeStrategyProvider{panics: true})

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("a panicking strategy provider escaped compilation: %v", r)
		}
	}()
	if got := c.collectStrategyAtoms(&CompilationContext{}); got != nil {
		t.Errorf("a panicking provider returned %d atoms, want none", len(got))
	}
}
