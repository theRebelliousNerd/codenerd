package system

import (
	"context"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/prompt"
)

// The five NewPredicateSelector sites apply the resolved
// jit.predicate_limit / jit.predicate_vec_limit. Each shard gets the config
// the way it already reads config: SetJITConfig, injected after
// SetParentKernel by registration (every system-shard constructor ends with
// withJITConfig) and by the factory closures, carrying
// GetEffectiveJITConfig. The limit is observed through the retained
// selector's SelectForContext cap. The vector limit rides the same helper
// call but only binds with an indexed vector store attached, so it has no
// unit-observable effect here.
func selectorSignatureCount(t *testing.T, sel *prompt.PredicateSelector) int {
	t.Helper()
	if sel == nil {
		t.Fatal("no predicate selector attached")
	}
	sigs, err := sel.SelectForContext(context.Background(), "coder", "/fix", "mangle", "repair kernel policy")
	if err != nil {
		t.Fatalf("SelectForContext: %v", err)
	}
	return len(sigs)
}

func assertSelectorCapped(t *testing.T, sel *prompt.PredicateSelector, wantMax int) {
	t.Helper()
	if n := selectorSignatureCount(t, sel); n > wantMax {
		t.Errorf("SelectForContext returned %d signatures, want at most %d", n, wantMax)
	}
}

func TestConstitutionGateShard_PredicateLimitFollowsJITConfig(t *testing.T) {
	k := newTestKernel(t)
	wired := func() *ConstitutionGateShard {
		s := NewConstitutionGateShard()
		s.SetParentKernel(k)
		return s
	}
	// Absent config: the constructor default (jit.predicate_limit = 100).
	// The corpus must clear 7 or the cap below binds nothing.
	if n := selectorSignatureCount(t, wired().predicateSelector); n > 100 || n <= 7 {
		t.Fatalf("default selection = %d, want the default window (8..100)", n)
	}
	// Production order: kernel first, config after.
	s := wired()
	s.SetJITConfig(config.JITConfig{PredicateLimit: 7, PredicateVecLimit: 11})
	assertSelectorCapped(t, s.predicateSelector, 7)
	// Reverse order: config first, kernel after.
	r := NewConstitutionGateShard()
	r.SetJITConfig(config.JITConfig{PredicateLimit: 7, PredicateVecLimit: 11})
	r.SetParentKernel(k)
	assertSelectorCapped(t, r.predicateSelector, 7)
	// Zero config (a zero RegistryContext): defaults, never a zero cap.
	z := wired()
	z.SetJITConfig(config.JITConfig{})
	if n := selectorSignatureCount(t, z.predicateSelector); n > 100 || n <= 7 {
		t.Errorf("zero-config selection = %d, want the default window (8..100)", n)
	}
}

func TestExecutivePolicyShard_PredicateLimitFollowsJITConfig(t *testing.T) {
	k := newTestKernel(t)
	wired := func() *ExecutivePolicyShard {
		s := NewExecutivePolicyShard()
		s.SetParentKernel(k)
		return s
	}
	if n := selectorSignatureCount(t, wired().predicateSelector); n > 100 || n <= 7 {
		t.Fatalf("default selection = %d, want the default window (8..100)", n)
	}
	s := wired()
	s.SetJITConfig(config.JITConfig{PredicateLimit: 7, PredicateVecLimit: 11})
	assertSelectorCapped(t, s.predicateSelector, 7)
	r := NewExecutivePolicyShard()
	r.SetJITConfig(config.JITConfig{PredicateLimit: 7, PredicateVecLimit: 11})
	r.SetParentKernel(k)
	assertSelectorCapped(t, r.predicateSelector, 7)
	z := wired()
	z.SetJITConfig(config.JITConfig{})
	if n := selectorSignatureCount(t, z.predicateSelector); n > 100 || n <= 7 {
		t.Errorf("zero-config selection = %d, want the default window (8..100)", n)
	}
}

func TestLegislatorShard_PredicateLimitFollowsJITConfig(t *testing.T) {
	k := newTestKernel(t)
	wired := func() *LegislatorShard {
		s := NewLegislatorShard()
		s.SetParentKernel(k)
		return s
	}
	if n := selectorSignatureCount(t, wired().predicateSelector); n > 100 || n <= 7 {
		t.Fatalf("default selection = %d, want the default window (8..100)", n)
	}
	s := wired()
	s.SetJITConfig(config.JITConfig{PredicateLimit: 7, PredicateVecLimit: 11})
	assertSelectorCapped(t, s.predicateSelector, 7)
	r := NewLegislatorShard()
	r.SetJITConfig(config.JITConfig{PredicateLimit: 7, PredicateVecLimit: 11})
	r.SetParentKernel(k)
	assertSelectorCapped(t, r.predicateSelector, 7)
	z := wired()
	z.SetJITConfig(config.JITConfig{})
	if n := selectorSignatureCount(t, z.predicateSelector); n > 100 || n <= 7 {
		t.Errorf("zero-config selection = %d, want the default window (8..100)", n)
	}
}

func TestMangleRepairShard_PredicateLimitFollowsJITConfig(t *testing.T) {
	k := newTestKernel(t)
	corpus := k.GetPredicateCorpus()
	if corpus == nil {
		t.Fatal("test kernel has no predicate corpus")
	}
	// Production order: kernel, corpus, config.
	s := NewMangleRepairShard()
	s.SetParentKernel(k)
	s.SetCorpus(corpus)
	if n := selectorSignatureCount(t, s.predicateSelector); n > 100 || n <= 7 {
		t.Fatalf("default selection = %d, want the default window (8..100)", n)
	}
	s.SetJITConfig(config.JITConfig{PredicateLimit: 7, PredicateVecLimit: 11})
	assertSelectorCapped(t, s.predicateSelector, 7)
	// Reverse order: config before corpus.
	r := NewMangleRepairShard()
	r.SetJITConfig(config.JITConfig{PredicateLimit: 7, PredicateVecLimit: 11})
	r.SetParentKernel(k)
	r.SetCorpus(corpus)
	assertSelectorCapped(t, r.predicateSelector, 7)
}
