package campaign

import (
	"codenerd/internal/core"
	"context"
	"fmt"
	"strings"
	"testing"
)

func TestNewContextPager(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{}

	// Test default budget
	cp := NewContextPager(kernel, llm, 0)
	if cp.totalBudget != 200000 {
		t.Errorf("expected default budget 200000, got %d", cp.totalBudget)
	}

	// Test custom budget
	cp2 := NewContextPager(kernel, llm, 100000)
	if cp2.totalBudget != 100000 {
		t.Errorf("expected custom budget 100000, got %d", cp2.totalBudget)
	}

	// Test reserve calculations (100k)
	// core=5%, phase=30%, history=15%, working=40%, prefetch=10%
	if cp2.coreReserve != 5000 {
		t.Errorf("expected core reserve 5000, got %d", cp2.coreReserve)
	}
	if cp2.phaseReserve != 30000 {
		t.Errorf("expected phase reserve 30000, got %d", cp2.phaseReserve)
	}
}

func TestSetBudget(t *testing.T) {
	cp := NewContextPager(&MockKernel{}, &MockLLMClient{}, 100000)
	cp.SetBudget(50000)

	if cp.totalBudget != 50000 {
		t.Errorf("expected updated budget 50000, got %d", cp.totalBudget)
	}
	// Verify recalculation
	if cp.workingReserve != 20000 { // 40% of 50000
		t.Errorf("expected working reserve 20000, got %d", cp.workingReserve)
	}
}

func TestActivatePhase(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{}
	cp := NewContextPager(kernel, llm, 100000)
	ctx := context.Background()

	// 1. Setup Phase and Context Profile
	profileID := "profile1"
	profile := ContextProfile{
		ID:              profileID,
		RequiredSchemas: []string{"schema1", "schema2"},
		RequiredTools:   []string{"tool1"},
		FocusPatterns:   []string{"*.go", "*.md"},
	}
	// Inject profile fact into kernel
	kernel.Assert(profile.ToFacts()[0])

	// Inject scoped docs fact
	// Predicate: phase_context_scope(Layer, Doc)
	// Phase Name: "Test Phase" -> Normalized: "test_phase"
	kernel.Assert(core.Fact{
		Predicate: "phase_context_scope",
		Args:      []interface{}{"test_phase", "scoped_doc.md"},
	})

	phase := &Phase{
		ID:             "phase1",
		Name:           "Test Phase",
		ContextProfile: profileID,
		Tasks: []Task{
			{
				ID: "task1",
				Artifacts: []TaskArtifact{
					{Path: "src/main.go"},
				},
			},
		},
	}

	// 2. Activate Phase
	err := cp.ActivatePhase(ctx, phase)
	if err != nil {
		t.Fatalf("ActivatePhase failed: %v", err)
	}

	// 3. Verify Assertions
	// Should have boosted focus patterns
	patternBoosted := false
	for _, f := range kernel.Facts {
		if f.Predicate == "activation" && len(f.Args) > 0 {
			arg0 := fmt.Sprintf("%v", f.Args[0])
			if strings.Contains(arg0, "file_pattern") && strings.Contains(arg0, "*.go") {
				patternBoosted = true
				break
			}
		}
	}
	if !patternBoosted {
		t.Error("Expected activation boost for *.go pattern")
	}

	// Should have boosted scoped docs
	scopedDocBoosted := false
	for _, f := range kernel.Facts {
		if f.Predicate == "phase_context_atom" && len(f.Args) > 1 {
			arg1 := fmt.Sprintf("%v", f.Args[1])
			if strings.Contains(arg1, "scoped_doc.md") {
				scopedDocBoosted = true
				break
			}
		}
	}
	if !scopedDocBoosted {
		t.Error("Expected phase_context_atom for scoped_doc.md")
	}

	// Should have boosted task artifacts
	artifactBoosted := false
	for _, f := range kernel.Facts {
		if f.Predicate == "phase_context_atom" && len(f.Args) > 1 {
			arg1 := fmt.Sprintf("%v", f.Args[1])
			if strings.Contains(arg1, "src/main.go") {
				artifactBoosted = true
				break
			}
		}
	}
	if !artifactBoosted {
		t.Error("Expected phase_context_atom for src/main.go")
	}

	// Should have suppressed irrelevant schemas
	// "vector_recall" is in the default irrelevant list and NOT in RequiredSchemas
	vectorSuppressed := false
	for _, f := range kernel.Facts {
		if f.Predicate == "activation" && len(f.Args) > 1 {
			if f.Args[0] == "vector_recall" && fmt.Sprintf("%v", f.Args[1]) == "-100" {
				vectorSuppressed = true
				break
			}
		}
	}
	if !vectorSuppressed {
		t.Error("Expected suppression of vector_recall schema")
	}
}

func TestCompressPhase(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "Phase summary: Did some work.", nil
		},
	}
	cp := NewContextPager(kernel, llm, 100000)
	ctx := context.Background()

	phaseID := "phase1"
	phase := &Phase{
		ID:   phaseID,
		Name: "Completed Phase",
		Tasks: []Task{
			{
				ID:          "task1",
				Description: "Write code",
				Status:      TaskCompleted,
				Artifacts: []TaskArtifact{
					{Path: "code.go"},
				},
			},
		},
	}

	// Simulate existing phase atoms
	kernel.Assert(core.Fact{
		Predicate: "phase_context_atom",
		Args:      []interface{}{phaseID, "some_atom", 100},
	})

	// Run Compression
	summary, count, _, err := cp.CompressPhase(ctx, phase)
	if err != nil {
		t.Fatalf("CompressPhase failed: %v", err)
	}

	if summary != "Phase summary: Did some work." {
		t.Errorf("Unexpected summary: %s", summary)
	}
	if count != 1 {
		t.Errorf("Expected 1 original atom, got %d", count)
	}

	// Verify Assertions
	// Should see context_compression fact
	compressionStored := false
	for _, f := range kernel.Facts {
		if f.Predicate == "context_compression" && f.Args[0] == phaseID {
			compressionStored = true
			if f.Args[1] != summary {
				t.Errorf("Stored summary mismatch")
			}
			break
		}
	}
	if !compressionStored {
		t.Error("Expected context_compression fact to be asserted")
	}

	// Should see deactivation of old facts
	deactivationSeen := false
	for _, f := range kernel.Facts {
		if f.Predicate == "activation" && f.Args[0] == "some_atom" && fmt.Sprintf("%v", f.Args[1]) == "-100" {
			deactivationSeen = true
			break
		}
	}
	if !deactivationSeen {
		t.Error("Expected activation reduction for phase facts")
	}
}

func TestPrefetchNextTasks(t *testing.T) {
	kernel := &MockKernel{}
	cp := NewContextPager(kernel, &MockLLMClient{}, 100000)
	ctx := context.Background()

	tasks := []Task{
		{
			ID: "task1",
			Artifacts: []TaskArtifact{
				{Path: "next.go"},
			},
		},
		{
			ID: "task2", // Should be ignored if limit is 1
			Artifacts: []TaskArtifact{
				{Path: "later.go"},
			},
		},
	}

	err := cp.PrefetchNextTasks(ctx, tasks, 1)
	if err != nil {
		t.Fatalf("PrefetchNextTasks failed: %v", err)
	}

	// Verify activation boost for task1 artifact
	boosted := false
	for _, f := range kernel.Facts {
		if f.Predicate == "activation" {
			arg0 := fmt.Sprintf("%v", f.Args[0])
			if strings.Contains(arg0, "next.go") {
				boosted = true
			}
			if strings.Contains(arg0, "later.go") {
				t.Error("Should not have boosted task2 artifact")
			}
		}
	}

	if !boosted {
		t.Error("Expected activation boost for next.go")
	}
}

func TestPruneIrrelevant(t *testing.T) {
	kernel := &MockKernel{}
	cp := NewContextPager(kernel, &MockLLMClient{}, 100000)

	// Setup some facts to prune
	kernel.Assert(core.Fact{Predicate: "dom_node", Args: []interface{}{"div"}})
	kernel.Assert(core.Fact{Predicate: "visible_text", Args: []interface{}{"hello"}})
	kernel.Assert(core.Fact{Predicate: "other_fact", Args: []interface{}{"keep"}})

	// Profile that does NOT require browser
	profile := &ContextProfile{
		ID:              "backend_profile",
		RequiredSchemas: []string{"file_topology"},
	}

	err := cp.PruneIrrelevant(profile)
	if err != nil {
		t.Fatalf("PruneIrrelevant failed: %v", err)
	}

	// Verify suppression
	domSuppressed := false
	textSuppressed := false

	for _, f := range kernel.Facts {
		if f.Predicate == "activation" && fmt.Sprintf("%v", f.Args[1]) == "-200" {
			if f.Args[0] == "dom_node" {
				domSuppressed = true
			}
			if f.Args[0] == "visible_text" {
				textSuppressed = true
			}
			if f.Args[0] == "other_fact" {
				t.Error("Should not have suppressed other_fact")
			}
		}
	}

	if !domSuppressed || !textSuppressed {
		t.Error("Expected browser predicates to be suppressed")
	}
}
