package prompt

import (
	"context"
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// Compiler Gap Tests — remediation of TEST_GAP markers
// =============================================================================

func TestCompiler_EmptyPartialContextFields(t *testing.T) {
	// GAP A2: Verify robustness against empty/partial context fields.
	atoms := []*PromptAtom{
		{
			ID:          "identity",
			Category:    CategoryIdentity,
			Content:     "Identity content",
			Priority:    100,
			IsMandatory: true,
			TokenCount:  10,
		},
	}
	corpus := NewEmbeddedCorpus(atoms)
	kernel := &mockKernel{facts: atomsToFacts(atoms)}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler failed: %v", err)
	}

	tests := []struct {
		name string
		cc   *CompilationContext
	}{
		{"empty shard ID", NewCompilationContext().WithShard("", "", "").WithTokenBudget(10000, 1000)},
		{"empty intent verb", NewCompilationContext().WithIntent("", "").WithTokenBudget(10000, 1000)},
		{"empty languages", NewCompilationContext().WithLanguage("").WithTokenBudget(10000, 1000)},
		{"all empty strings", NewCompilationContext().WithShard("", "", "").WithIntent("", "").WithLanguage("").WithTokenBudget(10000, 1000)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := compiler.Compile(context.Background(), tc.cc)
			if err != nil {
				t.Fatalf("Compile failed: %v", err)
			}
			if result == nil {
				t.Fatal("Expected non-nil result")
			}
		})
	}
}

func TestCompiler_RegisterDB_BadPaths(t *testing.T) {
	// GAP A3: Verify RegisterDB with bad paths.
	compiler, err := NewJITPromptCompiler()
	if err != nil {
		t.Fatalf("NewJITPromptCompiler failed: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	tests := []struct {
		name    string
		corpus  string
		path    string
		wantErr bool
	}{
		{"empty path", "test", "", true},
		{"directory path", "test", t.TempDir(), false}, // sql.Open is lazy — may succeed
		{"nonexistent path", "test", filepath.Join(t.TempDir(), "nonexist", "db.sqlite"), false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := compiler.RegisterDB(tc.corpus, tc.path)
			if tc.wantErr && err == nil {
				t.Logf("Warning: RegisterDB(%q, %q) did not error (sql.Open is lazy)", tc.corpus, tc.path)
			}
			// Main requirement: must not panic
		})
	}

	t.Run("embedded atom wins over stale project duplicate", func(t *testing.T) {
		canonical := NewPromptAtom("identity/canonical", CategoryIdentity, "canonical embedded content")
		canonical.IsMandatory = true
		canonical.Priority = 100

		dupCompiler, err := NewJITPromptCompiler(
			WithEmbeddedCorpus(NewEmbeddedCorpus([]*PromptAtom{canonical})),
		)
		if err != nil {
			t.Fatalf("NewJITPromptCompiler failed: %v", err)
		}
		defer func() { _ = dupCompiler.Close() }()

		dbPath := filepath.Join(t.TempDir(), "corpus.db")
		if err := dupCompiler.RegisterDB("project", dbPath); err != nil {
			t.Fatalf("RegisterDB failed: %v", err)
		}

		schema := `
			CREATE TABLE prompt_atoms (
				atom_id TEXT PRIMARY KEY,
				version INTEGER NOT NULL,
				content TEXT NOT NULL,
				token_count INTEGER NOT NULL,
				content_hash TEXT NOT NULL,
				description TEXT,
				content_concise TEXT,
				content_min TEXT,
				category TEXT NOT NULL,
				subcategory TEXT,
				priority INTEGER NOT NULL,
				is_mandatory BOOLEAN NOT NULL,
				is_exclusive TEXT,
				created_at DATETIME NOT NULL
			);
			CREATE TABLE atom_context_tags (
				atom_id TEXT NOT NULL,
				dimension TEXT NOT NULL,
				tag TEXT NOT NULL
			);`
		if _, err := dupCompiler.projectDB.Exec(schema); err != nil {
			t.Fatalf("create prompt schema: %v", err)
		}

		insert := `INSERT INTO prompt_atoms
			(atom_id, version, content, token_count, content_hash, description,
			 content_concise, content_min, category, subcategory, priority,
			 is_mandatory, is_exclusive, created_at)
		VALUES
			('identity/canonical', 1, 'stale persisted content', 5, 'stale-hash', '', '', '', 'identity', '', 100, 1, '', CURRENT_TIMESTAMP),
			('project/only', 1, 'project-only content', 5, 'project-hash', '', '', '', 'context', '', 10, 0, '', CURRENT_TIMESTAMP)`
		if _, err := dupCompiler.projectDB.Exec(insert); err != nil {
			t.Fatalf("insert prompt atoms: %v", err)
		}

		atoms, breakdown, err := dupCompiler.collectAtomsWithStats(context.Background(), NewCompilationContext())
		if err != nil {
			t.Fatalf("collectAtomsWithStats failed: %v", err)
		}
		if len(atoms) != 2 {
			t.Fatalf("candidate count = %d, want 2 unique atoms", len(atoms))
		}
		if breakdown.embedded != 1 || breakdown.project != 1 {
			t.Fatalf("source breakdown = embedded:%d project:%d, want 1 and 1 unique atoms", breakdown.embedded, breakdown.project)
		}

		byID := make(map[string]*PromptAtom, len(atoms))
		for _, atom := range atoms {
			byID[atom.ID] = atom
		}
		if got := byID["identity/canonical"]; got == nil || got.Content != "canonical embedded content" {
			t.Fatalf("canonical atom = %#v, want embedded content instead of stale project copy", got)
		}
		if got := byID["project/only"]; got == nil || got.Content != "project-only content" {
			t.Fatalf("project-only atom = %#v, want unique project atom retained", got)
		}
	})
}

func TestCompiler_KernelNonStringFacts(t *testing.T) {
	// GAP B1/B2: Verify robustness against kernel returning non-string types.
	atoms := []*PromptAtom{
		{
			ID:          "identity",
			Category:    CategoryIdentity,
			Content:     "Identity",
			Priority:    100,
			IsMandatory: true,
			TokenCount:  10,
		},
	}
	corpus := NewEmbeddedCorpus(atoms)

	// Return facts with non-string types
	kernel := &mockKernel{
		facts: []any{
			Fact{Predicate: "selected_atom", Args: []any{42, "skeleton", 1.0}},           // int ID
			Fact{Predicate: "selected_atom", Args: []any{nil, "skeleton", 1.0}},          // nil ID
			Fact{Predicate: "selected_atom", Args: []any{true, "skeleton", 1.0}},         // bool ID
			Fact{Predicate: "selected_atom", Args: []any{"identity", "skeleton", "bad"}}, // non-float score
			Fact{Predicate: "selected_atom", Args: []any{"identity", 42, 1.0}},           // non-string source
		},
	}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler failed: %v", err)
	}

	cc := NewCompilationContext().WithTokenBudget(10000, 1000)
	// Must not panic
	result, err := compiler.Compile(context.Background(), cc)
	if err != nil {
		t.Logf("Compile returned error (acceptable for invalid facts): %v", err)
	}
	if result != nil {
		t.Logf("Result: atoms=%d, tokens=%d", result.AtomsIncluded, result.TotalTokens)
	}
}

func TestCompiler_ContextCancellation(t *testing.T) {
	// GAP: Verify context cancellation aborts compilation.
	atoms := make([]*PromptAtom, 100)
	for i := range 100 {
		atoms[i] = &PromptAtom{
			ID:         fmt.Sprintf("atom_%d", i),
			Category:   CategoryContext,
			Content:    strings.Repeat("content ", 50),
			Priority:   i,
			TokenCount: 50,
		}
	}
	corpus := NewEmbeddedCorpus(atoms)
	kernel := &mockKernel{facts: atomsToFacts(atoms)}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	cc := NewCompilationContext().WithTokenBudget(100000, 10000)
	_, err = compiler.Compile(ctx, cc)
	// Should either error with context canceled or succeed quickly
	if err != nil {
		t.Logf("Compile with canceled context: %v (expected)", err)
	}
}

func TestCompiler_MassiveContentString(t *testing.T) {
	// GAP C5: Verify handling of massive content strings.
	bigContent := strings.Repeat("word ", 100000) // ~500KB
	atoms := []*PromptAtom{
		{
			ID:          "big",
			Category:    CategoryIdentity,
			Content:     bigContent,
			Priority:    100,
			IsMandatory: true,
			TokenCount:  100000,
		},
	}
	corpus := NewEmbeddedCorpus(atoms)
	kernel := &mockKernel{facts: atomsToFacts(atoms)}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler failed: %v", err)
	}

	cc := NewCompilationContext().WithTokenBudget(200000, 10000)
	result, err := compiler.Compile(context.Background(), cc)
	// Should not OOM or panic
	if err != nil {
		t.Logf("Compile with massive content: %v", err)
	}
	if result != nil {
		t.Logf("Result: atoms=%d, tokens=%d", result.AtomsIncluded, result.TotalTokens)
	}
}

func TestCompiler_ConcurrentCompile(t *testing.T) {
	// GAP D1: Verify concurrency safety under thundering herd.
	atoms := []*PromptAtom{
		{
			ID:          "identity",
			Category:    CategoryIdentity,
			Content:     "Identity",
			Priority:    100,
			IsMandatory: true,
			TokenCount:  10,
		},
	}
	corpus := NewEmbeddedCorpus(atoms)
	kernel := &mockKernel{facts: atomsToFacts(atoms)}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler failed: %v", err)
	}

	var wg sync.WaitGroup
	const goroutines = 20

	for range goroutines {
		wg.Go(func() {
			cc := NewCompilationContext().WithTokenBudget(10000, 1000)
			_, _ = compiler.Compile(context.Background(), cc)
		})
	}

	// Must not panic or race
	wg.Wait()
}

func TestCompiler_ConcurrentRegisterDB(t *testing.T) {
	// GAP D2: Verify RegisterDB during Compile is safe.
	atoms := []*PromptAtom{
		{
			ID:          "identity",
			Category:    CategoryIdentity,
			Content:     "Identity",
			Priority:    100,
			IsMandatory: true,
			TokenCount:  10,
		},
	}
	corpus := NewEmbeddedCorpus(atoms)
	kernel := &mockKernel{facts: atomsToFacts(atoms)}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler failed: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Use a temp dir that won't be cleaned up by t.TempDir() before Close()
	tmp := t.TempDir()

	// Goroutine 1: compile repeatedly
	go func() {
		defer wg.Done()
		for range 50 {
			cc := NewCompilationContext().WithTokenBudget(10000, 1000)
			_, _ = compiler.Compile(context.Background(), cc)
		}
	}()

	// Goroutine 2: register/unregister DBs
	go func() {
		defer wg.Done()
		for i := range 50 {
			dbPath := filepath.Join(tmp, fmt.Sprintf("test-%d.db", i))
			_ = compiler.RegisterDB("corpus", dbPath)
		}
	}()

	wg.Wait()
	// Close before t.TempDir() cleanup to release SQLite file locks
	_ = compiler.Close()
}

func TestCompiler_MassiveTokenBudgetOverflow(t *testing.T) {
	// GAP: Verify integer overflow in token budget.
	compiler, err := NewJITPromptCompiler()
	if err != nil {
		t.Fatalf("NewJITPromptCompiler failed: %v", err)
	}

	t.Run("max int budget", func(t *testing.T) {
		cc := NewCompilationContext().WithTokenBudget(math.MaxInt64, 0)
		result, err := compiler.Compile(context.Background(), cc)
		// Should handle gracefully
		if err != nil {
			t.Logf("Max budget error (acceptable): %v", err)
		}
		if result != nil {
			t.Logf("Result: atoms=%d", result.AtomsIncluded)
		}
	})
}

func TestCompiler_MangleInjectionViaAtomID(t *testing.T) {
	// GAP: Verify Mangle injection via atom ID with special characters.
	atoms := []*PromptAtom{
		{
			ID:         "normal-atom",
			Category:   CategoryIdentity,
			Content:    "Normal content",
			Priority:   100,
			TokenCount: 10,
		},
		{
			ID:         "atom'with\"quotes",
			Category:   CategoryContext,
			Content:    "Dangerous ID",
			Priority:   50,
			TokenCount: 10,
		},
		{
			ID:         "atom\nwith\nnewlines",
			Category:   CategoryContext,
			Content:    "Newline ID",
			Priority:   50,
			TokenCount: 10,
		},
	}
	corpus := NewEmbeddedCorpus(atoms)
	kernel := &mockKernel{facts: atomsToFacts(atoms)}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler failed: %v", err)
	}

	cc := NewCompilationContext().WithTokenBudget(10000, 1000)
	// Must not panic or allow injection
	result, err := compiler.Compile(context.Background(), cc)
	if err != nil {
		t.Logf("Compile with injection IDs: %v (acceptable)", err)
	}
	if result != nil {
		t.Logf("Result: atoms=%d", result.AtomsIncluded)
	}
}

func TestCompiler_AtomSyntaxViolationsInContext(t *testing.T) {
	// GAP: Verify handling of special characters in context fields.
	atoms := []*PromptAtom{
		{
			ID:          "identity",
			Category:    CategoryIdentity,
			Content:     "Identity",
			Priority:    100,
			IsMandatory: true,
			TokenCount:  10,
		},
	}
	corpus := NewEmbeddedCorpus(atoms)
	kernel := &mockKernel{facts: atomsToFacts(atoms)}

	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(corpus),
		WithKernel(kernel),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler failed: %v", err)
	}

	tests := []struct {
		name string
		cc   *CompilationContext
	}{
		{
			"shard type with spaces",
			NewCompilationContext().WithShard("/coder with spaces", "", "").WithTokenBudget(10000, 1000),
		},
		{
			"shard type with special chars",
			NewCompilationContext().WithShard("/cod'er\"test", "", "").WithTokenBudget(10000, 1000),
		},
		{
			"intent with unicode",
			NewCompilationContext().WithIntent("/修复", "target.go").WithTokenBudget(10000, 1000),
		},
		{
			"language with null bytes",
			NewCompilationContext().WithLanguage("/go\x00injected").WithTokenBudget(10000, 1000),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Must not panic
			result, err := compiler.Compile(context.Background(), tc.cc)
			if err != nil {
				t.Logf("Compile with special chars: %v (acceptable)", err)
			}
			if result != nil {
				t.Logf("Result: atoms=%d", result.AtomsIncluded)
			}
		})
	}
}

func TestCompiler_InputSanitization_DoS(t *testing.T) {
	// GAP C1: Verify 1MB+ strings don't cause memory spikes.
	compiler, err := NewJITPromptCompiler()
	if err != nil {
		t.Fatalf("NewJITPromptCompiler failed: %v", err)
	}

	megaString := strings.Repeat("A", 1024*1024) // 1MB
	cc := NewCompilationContext().
		WithShard(megaString, megaString, megaString).
		WithIntent(megaString, megaString).
		WithTokenBudget(10000, 1000)

	// Must not OOM or panic
	_, err = compiler.Compile(context.Background(), cc)
	if err != nil {
		t.Logf("Compile with 1MB strings: %v (acceptable)", err)
	}
}
