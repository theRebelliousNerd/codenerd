package prompt_evolution

import (
	"sync"
	"testing"
	"time"

	"codenerd/internal/prompt"
)

func TestSetOnAtomPromoted_FiresOnPromotion(t *testing.T) {
	tempDir := t.TempDir()
	pe, err := NewPromptEvolver(tempDir, &mockLLMClient{}, nil)
	if err != nil {
		t.Fatalf("NewPromptEvolver failed: %v", err)
	}
	defer pe.Close()

	atomID := "test/promoted/callback"
	ga := &GeneratedAtom{
		Atom: &prompt.PromptAtom{
			ID:       atomID,
			Category: prompt.CategoryMethodology,
			Content:  "callback test content",
		},
		Source:     "test",
		SourceIDs:  []string{"task-1"},
		Confidence: 0.9,
		CreatedAt:  time.Now(),
	}
	if err := pe.storeEvolvedAtom(ga); err != nil {
		t.Fatalf("storeEvolvedAtom failed: %v", err)
	}

	var mu sync.Mutex
	var count int
	var gotID string
	var gotTime time.Time

	pe.SetOnAtomPromoted(func(id string, promotedAt time.Time) {
		mu.Lock()
		defer mu.Unlock()
		count++
		gotID = id
		gotTime = promotedAt
	})

	if err := pe.PromoteAtom(atomID); err != nil {
		t.Fatalf("PromoteAtom failed: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if count != 1 {
		t.Fatalf("expected callback to fire exactly once, got %d", count)
	}
	if gotID != atomID {
		t.Fatalf("expected callback atomID %q, got %q", atomID, gotID)
	}
	if gotTime.IsZero() {
		t.Fatal("expected non-zero promotedAt, got zero time")
	}
}

func TestPromoteAtom_NilCallbackIsSilent(t *testing.T) {
	tempDir := t.TempDir()
	pe, err := NewPromptEvolver(tempDir, &mockLLMClient{}, nil)
	if err != nil {
		t.Fatalf("NewPromptEvolver failed: %v", err)
	}
	defer pe.Close()

	atomID := "test/nil/callback"
	ga := &GeneratedAtom{
		Atom: &prompt.PromptAtom{
			ID:       atomID,
			Category: prompt.CategoryMethodology,
			Content:  "nil callback test content",
		},
		Source:     "test",
		SourceIDs:  []string{"task-nil"},
		Confidence: 0.5,
		CreatedAt:  time.Now(),
	}
	if err := pe.storeEvolvedAtom(ga); err != nil {
		t.Fatalf("storeEvolvedAtom failed: %v", err)
	}

	// Assert no panic when callback is nil (default).
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PromoteAtom panicked with nil callback: %v", r)
		}
	}()

	if err := pe.PromoteAtom(atomID); err != nil {
		t.Fatalf("PromoteAtom with nil callback returned error: %v", err)
	}

	// Verify atom was still promoted despite nil callback.
	promoted := pe.GetPromotedAtoms()
	if len(promoted) != 1 {
		t.Fatalf("expected 1 promoted atom, got %d", len(promoted))
	}
	if promoted[0].Atom.ID != atomID {
		t.Fatalf("expected promoted atom ID %q, got %q", atomID, promoted[0].Atom.ID)
	}
}

func TestAtomPromotedCallback_RunsWithoutEvolverLock(t *testing.T) {
	tempDir := t.TempDir()
	pe, err := NewPromptEvolver(tempDir, &mockLLMClient{}, nil)
	if err != nil {
		t.Fatalf("NewPromptEvolver failed: %v", err)
	}
	defer pe.Close()

	atomID := "test/callback/deadlock"
	ga := &GeneratedAtom{
		Atom: &prompt.PromptAtom{
			ID:       atomID,
			Category: prompt.CategoryMethodology,
			Content:  "deadlock test content",
		},
		Source:     "test",
		SourceIDs:  []string{"task-deadlock"},
		Confidence: 0.9,
		CreatedAt:  time.Now(),
	}
	if err := pe.storeEvolvedAtom(ga); err != nil {
		t.Fatalf("storeEvolvedAtom failed: %v", err)
	}

	var mu sync.Mutex
	callbackRan := false
	var callbackAtoms []*GeneratedAtom

	pe.SetOnAtomPromoted(func(id string, promotedAt time.Time) {
		// This re-enters the evolver while the callback is executing.
		// promoteAtomLocked returns the callback rather than calling it while
		// holding pe.mu, so this should not deadlock.
		atoms := pe.GetPromotedAtoms()
		mu.Lock()
		callbackRan = true
		callbackAtoms = atoms
		mu.Unlock()
	})

	done := make(chan error, 1)
	go func() {
		done <- pe.PromoteAtom(atomID)
	}()

	// Timeout exists so a regression that reintroduces the lock fails the test instead of hanging the whole suite.
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("PromoteAtom failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("PromoteAtom did not return in time - likely deadlock: callback invoked while holding pe.mu")
	}

	mu.Lock()
	defer mu.Unlock()
	if !callbackRan {
		t.Fatal("callback did not run or GetPromotedAtoms blocked due to deadlock")
	}
	if callbackAtoms == nil {
		t.Fatal("expected GetPromotedAtoms to return non-nil inside callback")
	}
	found := false
	for _, ga := range callbackAtoms {
		if ga.Atom.ID == atomID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected promoted atom %q to be visible inside callback via GetPromotedAtoms", atomID)
	}
}

func TestPromoteAtom_UnknownAtomDoesNotFireCallback(t *testing.T) {
	tempDir := t.TempDir()
	pe, err := NewPromptEvolver(tempDir, &mockLLMClient{}, nil)
	if err != nil {
		t.Fatalf("NewPromptEvolver failed: %v", err)
	}
	defer pe.Close()

	var mu sync.Mutex
	fired := false
	pe.SetOnAtomPromoted(func(id string, promotedAt time.Time) {
		mu.Lock()
		fired = true
		mu.Unlock()
	})

	err = pe.PromoteAtom("nonexistent/atom/id")
	if err == nil {
		t.Fatal("expected PromoteAtom to return error for unknown atom, got nil")
	}

	mu.Lock()
	defer mu.Unlock()
	if fired {
		t.Fatal("callback fired for unknown atom, expected not to fire")
	}
}
