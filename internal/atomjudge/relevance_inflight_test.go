package atomjudge

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/prompt"
)

func concurrentRelevanceCorpus() []*prompt.PromptAtom {
	identity := prompt.NewPromptAtom("identity/core", prompt.CategoryIdentity, "Keep mandatory obligations.")
	identity.IsMandatory = true
	atoms := []*prompt.PromptAtom{identity}
	for i := range 12 {
		atoms = append(atoms, atom(fmt.Sprintf("flesh/%02d", i), "guidance"))
	}
	return atoms
}

func concurrentRelevanceSearcher(searched chan<- struct{}) budgetSearcher {
	return func(string) []prompt.SearchResult {
		var out []prompt.SearchResult
		for i := range 12 {
			score := .70 - float64(i)*.025
			if i < 2 {
				score = .99 - float64(i)*.01
			}
			out = append(out, prompt.SearchResult{AtomID: fmt.Sprintf("flesh/%02d", i), Score: score})
		}
		searched <- struct{}{}
		return out
	}
}

func TestRelevanceInFlight_DistinctCompilerKeysKeepPlanBudgets(t *testing.T) {
	const n = 4
	for _, tc := range []struct {
		name               string
		maxAtoms, maxInput int
		wantCalls          int32
	}{
		{"complete", 6, 10000, 1},
		{"whole-pool-atom-refusal", 5, 10000, 0},
		{"whole-pool-input-refusal", 6, 1, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			searched := make(chan struct{}, n)
			c := budgetCompiler(t, concurrentRelevanceCorpus(), concurrentRelevanceSearcher(searched))
			f := &gatedRelevanceAsker{started: make(chan struct{}, n), release: make(chan struct{})}
			var once sync.Once
			release := func() { once.Do(func() { close(f.release) }) }
			c.SetAtomRelevance(RelevanceFor(f, &config.TypeSafeConfig{AtomRelevancePool: 6,
				AtomRelevanceTimeout: "10s", AtomRelevanceMaxAtoms: tc.maxAtoms,
				AtomRelevanceMaxInputTokens: tc.maxInput, AtomRelevanceMaxConcurrentBatches: 1}))
			results := make([]*prompt.CompilationResult, n)
			errs := make([]error, n)
			var wg sync.WaitGroup
			keys := map[string]bool{}
			for i := range n {
				cc := budgetContext("fix concurrent relevance")
				cc.TokenBudget += i
				keys[cc.Hash()] = true
				wg.Add(1)
				go func() {
					defer wg.Done()
					results[i], errs[i] = c.Compile(context.Background(), cc)
				}()
			}
			t.Cleanup(func() { release(); wg.Wait() })
			if len(keys) != n {
				t.Fatal("compiles did not have distinct prompt-cache identities")
			}
			for range n {
				awaitStart(t, searched)
			}
			if tc.wantCalls > 0 {
				awaitStart(t, f.started)
			}
			release()
			wg.Wait()
			for i, result := range results {
				if errs[i] != nil || result == nil {
					t.Fatalf("compile %d: %v", i, errs[i])
				}
				ids := includedIDs(result)
				if !slices.Contains(ids, "identity/core") || !slices.Contains(ids, "flesh/00") {
					t.Fatalf("compile %d lost mandatory/vector evidence: %v", i, ids)
				}
				if slices.Contains(ids, "flesh/02") != (tc.wantCalls > 0) {
					t.Fatalf("compile %d did not preserve judged versus vector-only selection: %v", i, ids)
				}
			}
			if f.calls.Load() != tc.wantCalls || f.peak.Load() > 1 {
				t.Errorf("actual Ask calls=%d peak=%d; want calls=%d peak<=1", f.calls.Load(), f.peak.Load(), tc.wantCalls)
			}
		})
	}
}

func TestRelevanceInFlight_CompilerShutdownJoinsOwnedWork(t *testing.T) {
	const n = 4
	searched := make(chan struct{}, n)
	c := budgetCompiler(t, concurrentRelevanceCorpus(), concurrentRelevanceSearcher(searched))
	f := &gatedRelevanceAsker{started: make(chan struct{}, n), release: make(chan struct{})}
	c.SetAtomRelevance(RelevanceFor(f, &config.TypeSafeConfig{AtomRelevancePool: 6, AtomRelevanceTimeout: "1h"}))
	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cc := budgetContext("stop active relevance")
			cc.TokenBudget += i
			_, errs[i] = c.Compile(context.Background(), cc)
		}()
	}
	t.Cleanup(func() { close(f.release); _ = c.Close(); wg.Wait() })
	for range n {
		awaitStart(t, searched)
	}
	awaitStart(t, f.started)
	done := make(chan struct{})
	go func() { _ = c.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("compiler shutdown did not cancel and join relevance ownership")
	}
	wg.Wait()
	for i, err := range errs {
		if err == nil {
			t.Errorf("compile %d returned success after owner shutdown", i)
		}
	}
	if f.calls.Load() != 1 || f.active.Load() != 0 {
		t.Errorf("shutdown dispatched queued work or left an active Ask: calls=%d active=%d", f.calls.Load(), f.active.Load())
	}
}

func TestRelevanceInFlight_CallerCancellationKeepsCompilerOwner(t *testing.T) {
	searched := make(chan struct{}, 2)
	c := budgetCompiler(t, concurrentRelevanceCorpus(), concurrentRelevanceSearcher(searched))
	f := &gatedRelevanceAsker{started: make(chan struct{}, 2), release: make(chan struct{})}
	var once sync.Once
	release := func() { once.Do(func() { close(f.release) }) }
	c.SetAtomRelevance(RelevanceFor(f, &config.TypeSafeConfig{AtomRelevancePool: 6, AtomRelevanceTimeout: "1h"}))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstDone := make(chan error, 1)
	go func() { _, err := c.Compile(ctx, budgetContext("shared owner survives caller")); firstDone <- err }()
	awaitStart(t, searched)
	awaitStart(t, f.started)
	secondDone := make(chan struct{})
	var result *prompt.CompilationResult
	var secondErr error
	go func() {
		defer close(secondDone)
		cc := budgetContext("shared owner survives caller")
		cc.TokenBudget++
		result, secondErr = c.Compile(context.Background(), cc)
	}()
	t.Cleanup(func() { release(); _ = c.Close(); <-secondDone })
	awaitStart(t, searched)
	cancel()
	select {
	case err := <-firstDone:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("canceled caller returned %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("canceled caller remained tied to compiler-owned work")
	}
	if f.active.Load() != 1 {
		t.Fatalf("caller cancellation revoked compiler-owned Ask or duplicated it: %d active", f.active.Load())
	}
	release()
	select {
	case <-secondDone:
	case <-time.After(10 * time.Second):
		t.Fatal("healthy compile did not receive its complete judgment")
	}
	if secondErr != nil || result == nil || !slices.Contains(includedIDs(result), "flesh/02") || f.calls.Load() != 1 {
		t.Fatalf("healthy compiler owner lost/co-ran judgment: error=%v calls=%d result=%v", secondErr, f.calls.Load(), result)
	}
}
