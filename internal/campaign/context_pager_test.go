package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/types"
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"
)

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify NewContextPager behavior when kernel or llmClient is nil.
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

// TODO: TEST_GAP: [State Conflicts] Verify SetBudget behavior under concurrent conditions when other methods are actively querying and updating tokens.
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
	// REMEDIATED: TEST_GAP: Type Coercion - scopedDocsForPhase when kernel returns non-string arguments (int, bool, nil) or Mangle Atoms instead of strings.
	// REMEDIATED: TEST_GAP: Performance/Extremes - scopedDocsForPhase with 1,000,000 phase_context_scope facts to check linear scan performance.
	kernel.Assert(core.Fact{
		Predicate: "phase_context_scope",
		Args:      []any{"test_phase", "scoped_doc.md"},
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
	// REMEDIATED: TEST_GAP: Null/Empty - ActivatePhase with nil phase (should handle gracefully)
	// REMEDIATED: TEST_GAP: Null/Empty - ActivatePhase with phase containing nil Tasks slice
	// REMEDIATED: TEST_GAP: Null/Empty - ActivatePhase with phase containing Tasks with nil Artifacts
	// REMEDIATED: TEST_GAP: User Request Extremes - ActivatePhase with malformed Phase IDs (spaces, special chars) injected into predicates
	// REMEDIATED: TEST_GAP: User Request Extremes - ActivatePhase with 100,000+ artifacts to check for timeouts in boosting loop

	// REMEDIATED: TEST_GAP: State Conflicts - Double Activation: Call ActivatePhase twice and verify idempotency of activation scores

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

func TestActivatePhase_NilTasks(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{}
	cp := NewContextPager(kernel, llm, 100000)
	ctx := context.Background()

	// Setup phase with nil Tasks slice
	phase := &Phase{
		ID:             "phase_nil_tasks",
		Name:           "Phase with Nil Tasks",
		ContextProfile: "default", // will fallback to default profile
		Tasks:          nil,
	}

	// Call ActivatePhase
	err := cp.ActivatePhase(ctx, phase)
	if err != nil {
		t.Fatalf("ActivatePhase failed to handle nil Tasks slice gracefully: %v", err)
	}

	// Since tasks is nil, artifactCount should be 0, and no phase_context_atom assertions for artifacts should happen.
	// The code handles nil slices in range loops automatically in Go.
	// Just reaching here without a panic and err == nil is considered a pass for this specific null/empty case.
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
		Args:      []any{phaseID, "some_atom", 100},
	})

	// TEST_GAP: Null/Empty - CompressPhase with nil phase
	nilSummary, nilCount, _, err := cp.CompressPhase(ctx, nil)
	if err != nil {
		t.Fatalf("CompressPhase failed with nil phase: %v", err)
	}
	if nilSummary != "" || nilCount != 0 {
		t.Errorf("CompressPhase with nil phase returned unexpected results: summary=%s, count=%d", nilSummary, nilCount)
	}

	// TEST_GAP: Null/Empty - CompressPhase with a phase with 0 tasks
	emptyPhase := &Phase{
		ID:    "empty1",
		Name:  "Empty Phase",
		Tasks: []Task{},
	}
	emptySummary, emptyCount, _, err := cp.CompressPhase(ctx, emptyPhase)
	if err != nil {
		t.Fatalf("CompressPhase failed with empty phase: %v", err)
	}
	if !strings.Contains(emptySummary, "no recorded accomplishments") || emptyCount != 0 {
		t.Errorf("CompressPhase with empty phase returned unexpected results: summary=%s, count=%d", emptySummary, emptyCount)
	}

	// Run Compression
	// REMEDIATED: TEST_GAP: Null/Empty - CompressPhase with empty accomplishments list (ensure fallback formatting works gracefully)
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

// TODO: TEST_GAP: [User Request Extremes] Verify performance and memory allocation of CompressPhase when phase contains millions of facts to compress.
func TestCompressPhase_MassiveAccomplishments(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			// Ensure the prompt size is massive as expected (at least 10MB)
			if len(prompt) < 10*1024*1024 {
				return "", fmt.Errorf("prompt size %d is less than 10MB", len(prompt))
			}
			return "Phase summary: Massive output processed successfully.", nil
		},
	}
	cp := NewContextPager(kernel, llm, 100000)
	ctx := context.Background()

	phaseID := "phase_massive"

	// Create a task with a very long description and many tasks to exceed 10MB
	// 11,000 tasks * ~1KB description = ~11MB
	var tasks []Task
	longDesc := strings.Repeat("A", 1024)
	for i := range 11000 {
		tasks = append(tasks, Task{
			ID:          fmt.Sprintf("task%d", i),
			Description: longDesc,
			Status:      TaskCompleted,
		})
	}

	phase := &Phase{
		ID:    phaseID,
		Name:  "Massive Phase",
		Tasks: tasks,
	}

	// Simulate existing phase atoms
	kernel.Assert(core.Fact{
		Predicate: "phase_context_atom",
		Args:      []any{phaseID, "some_atom", 100},
	})

	// Run Compression
	summary, count, _, err := cp.CompressPhase(ctx, phase)
	if err != nil {
		t.Fatalf("CompressPhase failed: %v", err)
	}

	if summary != "Phase summary: Massive output processed successfully." {
		t.Errorf("Unexpected summary: %s", summary)
	}
	if count != 1 {
		t.Errorf("Expected 1 original atom, got %d", count)
	}

	// Verify context_compression fact
	compressionStored := false
	for _, f := range kernel.Facts {
		if f.Predicate == "context_compression" && f.Args[0] == phaseID {
			compressionStored = true
			if f.Args[1] != summary {
				t.Errorf("Stored summary mismatch, got: %v", f.Args[1])
			}
			break
		}
	}
	if !compressionStored {
		t.Error("Expected context_compression fact to be asserted")
	}
}

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify behavior of PrefetchNextTasks when tasks is an empty slice or limit is 0 or negative.
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
	kernel.Assert(core.Fact{Predicate: "dom_node", Args: []any{"div"}})
	kernel.Assert(core.Fact{Predicate: "visible_text", Args: []any{"hello"}})
	kernel.Assert(core.Fact{Predicate: "other_fact", Args: []any{"keep"}})

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

// -----------------------------------------------------------------------------
// Marathon 23: Context Pager Gaps
// -----------------------------------------------------------------------------

func TestContextPager_ResetFailure(t *testing.T) {
	kernel := &MockKernel{RetractErr: fmt.Errorf("transaction commit failed")}
	cp := NewContextPager(kernel, &MockLLMClient{}, 100000)

	kernel.Assert(core.Fact{Predicate: "activation", Args: []any{"file1.go", 100}})

	// Should not panic, but log an error and continue
	cp.ResetPhaseContext()

	// Ghost facts persist
	if len(kernel.Facts) != 1 {
		t.Errorf("Expected ghost facts to persist, got %d", len(kernel.Facts))
	}
}

func TestContextPager_NilLLMClient(t *testing.T) {
	kernel := &MockKernel{}
	cp := NewContextPager(kernel, nil, 100000)

	phase := &Phase{
		ID:    "phase1",
		Name:  "Phase",
		Tasks: []Task{{ID: "task1", Status: TaskCompleted, Description: "done"}},
	}

	summary, _, _, err := cp.CompressPhase(context.Background(), phase)
	if err != nil {
		t.Fatalf("Expected graceful handling, got error: %v", err)
	}
	if summary == "" || strings.Contains(summary, "panic") {
		t.Errorf("Expected fallback summary, got %q", summary)
	}
}

func TestContextPager_MalformedProfileID(t *testing.T) {
	kernel := &MockKernel{}
	cp := NewContextPager(kernel, &MockLLMClient{}, 100000)

	// Extremely long ID
	longID := strings.Repeat("A", 10000)
	kernel.Assert(core.Fact{
		Predicate: "context_profile",
		Args:      []any{longID, "schema1", "", ""},
	})

	prof, err := cp.getContextProfile(longID)
	if err != nil {
		t.Fatalf("Failed to handle long profile ID: %v", err)
	}
	if prof.RequiredSchemas[0] != "schema1" {
		t.Errorf("Expected schema1, got %v", prof.RequiredSchemas)
	}

	// Non-UTF8
	binaryID := string([]byte{0xff, 0xfe, 0xfd})
	kernel.Assert(core.Fact{
		Predicate: "context_profile",
		Args:      []any{binaryID, "schema2", "", ""},
	})

	prof2, err := cp.getContextProfile(binaryID)
	if err != nil {
		t.Fatalf("Failed to handle binary profile ID: %v", err)
	}
	if prof2.RequiredSchemas[0] != "schema2" {
		t.Errorf("Expected schema2, got %v", prof2.RequiredSchemas)
	}
}

func TestContextPager_GetUsageConcurrent(t *testing.T) {
	kernel := &ThreadSafeMockKernel{}
	cp := NewContextPager(kernel, &MockLLMClient{}, 100000)

	phase := &Phase{
		ID:    "test-phase",
		Name:  "Test Phase",
		Tasks: []Task{{Description: "Task 1", Artifacts: []TaskArtifact{{Path: "file1.go"}}}},
	}

	var wg sync.WaitGroup
	// Concurrently SetBudget, ActivatePhase, and GetUsage
	for i := range 100 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%3 == 0 {
				cp.SetBudget(50000)
			} else if i%3 == 1 {
				_ = cp.ActivatePhase(context.Background(), phase)
			} else {
				_, _, _ = cp.GetUsage()
			}
		}(i)
	}
	wg.Wait()
}

// TODO: TEST_GAP: [Type Coercion] Verify handling of malformed phase IDs containing special characters or non-string representations in getContextProfile.
func TestGetContextProfile_Malformed(t *testing.T) {
	kernel := &MockKernel{}
	cp := NewContextPager(kernel, &MockLLMClient{}, 100000)

	// Inject malformed profiles
	// 1. Not enough arguments
	kernel.Assert(core.Fact{
		Predicate: "context_profile",
		Args:      []any{"short_profile", "schema1"},
	})

	// 2. Nil arguments
	kernel.Assert(core.Fact{
		Predicate: "context_profile",
		Args:      []any{"nil_profile", nil, nil, nil},
	})

	// 3. Non-string arguments
	kernel.Assert(core.Fact{
		Predicate: "context_profile",
		Args:      []any{"type_profile", 123, true, 45.6},
	})

	// 4. Non-comma-separated strings
	kernel.Assert(core.Fact{
		Predicate: "context_profile",
		Args:      []any{"space_profile", "schema1 schema2", "tool1", "pattern1"},
	})

	// Test 1: Short Profile (Should return error because it's skipped in loop)
	_, err := cp.getContextProfile("short_profile")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error for short_profile, got %v", err)
	}

	// Test 2: Nil Profile
	// ExtractString(nil) -> "". strings.Split("", ",") -> [""]
	prof, err := cp.getContextProfile("nil_profile")
	if err != nil {
		t.Fatalf("Unexpected error for nil_profile: %v", err)
	}
	if len(prof.RequiredSchemas) != 1 || prof.RequiredSchemas[0] != "" {
		t.Errorf("Expected empty schema slice from nil, got %q", prof.RequiredSchemas)
	}
	if len(prof.RequiredTools) != 1 || prof.RequiredTools[0] != "" {
		t.Errorf("Expected empty tools slice from nil, got %q", prof.RequiredTools)
	}

	// Test 3: Type Profile
	// ExtractString converts int/bool/float to string representation
	prof, err = cp.getContextProfile("type_profile")
	if err != nil {
		t.Fatalf("Unexpected error for type_profile: %v", err)
	}
	if len(prof.RequiredSchemas) != 1 || prof.RequiredSchemas[0] != "123" {
		t.Errorf("Expected [\"123\"], got %q", prof.RequiredSchemas)
	}
	// ExtractString(true) -> "/true"
	if len(prof.RequiredTools) != 1 || prof.RequiredTools[0] != "/true" {
		t.Errorf("Expected [\"/true\"], got %q", prof.RequiredTools)
	}
	// ExtractString(45.6) -> "45.6"
	if len(prof.FocusPatterns) != 1 || prof.FocusPatterns[0] != "45.6" {
		t.Errorf("Expected [\"45.6\"], got %q", prof.FocusPatterns)
	}

	// Test 4: Space Profile
	// Space-separated strings should not be split correctly
	prof, err = cp.getContextProfile("space_profile")
	if err != nil {
		t.Fatalf("Unexpected error for space_profile: %v", err)
	}
	if len(prof.RequiredSchemas) != 1 || prof.RequiredSchemas[0] != "schema1 schema2" {
		t.Errorf("Expected [\"schema1 schema2\"], got %q", prof.RequiredSchemas)
	}
}

// ThreadSafeMockKernel is a wrapper around MockKernel to allow concurrent access in tests.
type ThreadSafeMockKernel struct {
	MockKernel
	mu sync.Mutex
}

func (m *ThreadSafeMockKernel) Query(predicate string) ([]core.Fact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.MockKernel.Query(predicate)
}

func (m *ThreadSafeMockKernel) AssertBatch(facts []core.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.MockKernel.AssertBatch(facts)
}

func (m *ThreadSafeMockKernel) Assert(fact core.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.MockKernel.Assert(fact)
}

func TestActivatePhase_Concurrent(t *testing.T) {
	kernel := &ThreadSafeMockKernel{}
	cp := NewContextPager(kernel, &MockLLMClient{}, 100000)
	ctx := context.Background()

	phase := &Phase{
		ID:   "test-phase",
		Name: "Test Phase",
		Tasks: []Task{
			{Description: "Task 1", Artifacts: []TaskArtifact{{Path: "file1.go"}}},
			{Description: "Task 2", Artifacts: []TaskArtifact{{Path: "file2.go"}}},
		},
	}

	var wg sync.WaitGroup
	// Run 10 goroutines concurrently calling ActivatePhase
	for range 10 {
		wg.Go(func() {
			err := cp.ActivatePhase(ctx, phase)
			if err != nil {
				t.Errorf("ActivatePhase failed: %v", err)
			}
		})
	}
	wg.Wait()

	// Verify usedTokens is correctly computed without race condition corruption
	used, total, _ := cp.GetUsage()

	// Phase base estimate is 100
	// 2 tasks * 50 tokens = 100
	// 2 artifacts * 20 tokens = 40
	// Total estimate = 240
	expectedUsed := 240
	if used != expectedUsed {
		t.Errorf("Expected usedTokens %d, got %d", expectedUsed, used)
	}
	if total != 100000 {
		t.Errorf("Expected total budget 100000, got %d", total)
	}
}

func TestActivatePhase_ExtremeBudget(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{}
	cp := NewContextPager(kernel, llm, 100) // Very small budget
	ctx := context.Background()

	// estimatePhaseTokens = 100 + len(Tasks)*(50 + len(Artifacts)*20)
	// With 1 task and 2 artifacts: 100 + 1 * (50 + 40) = 190
	phase := &Phase{
		ID:   "phase_huge",
		Name: "Huge Phase",
		Tasks: []Task{
			{
				ID:          "task1",
				Description: "Write a lot of code",
				Artifacts: []TaskArtifact{
					{Path: "file1.go"},
					{Path: "file2.go"},
				},
			},
		},
	}

	err := cp.ActivatePhase(ctx, phase)
	if err == nil || !strings.Contains(err.Error(), "exceeds total budget") {
		t.Errorf("Expected ActivatePhase to fail with budget error, got: %v", err)
	}
}

func TestActivatePhase_NilPhase(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{}
	cp := NewContextPager(kernel, llm, 100000)
	ctx := context.Background()

	err := cp.ActivatePhase(ctx, nil)
	if err != nil {
		t.Fatalf("ActivatePhase with nil phase failed: %v", err)
	}

	used, _, _ := cp.GetUsage()
	if used != 0 {
		t.Errorf("Expected 0 used tokens, got %d", used)
	}

	if len(kernel.Facts) != 0 {
		t.Errorf("Expected 0 facts asserted, got %d", len(kernel.Facts))
	}
}

func TestCompressPhase_EmptyAccomplishments(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			return "Should not be called", nil
		},
	}
	cp := NewContextPager(kernel, llm, 100000)
	ctx := context.Background()

	phaseID := "phase_empty"
	phase := &Phase{
		ID:   phaseID,
		Name: "Empty Phase",
		Tasks: []Task{
			{
				ID:          "task_pending",
				Description: "Not done yet",
				Status:      TaskPending,
				Artifacts: []TaskArtifact{
					{Path: "code.go"},
				},
			},
		},
	}

	kernel.Assert(core.Fact{
		Predicate: "phase_context_atom",
		Args:      []any{phaseID, "some_atom", 100},
	})

	summary, count, _, err := cp.CompressPhase(ctx, phase)
	if err != nil {
		t.Fatalf("CompressPhase failed: %v", err)
	}

	expectedSummary := "Phase 'Empty Phase' completed with no recorded accomplishments."
	if summary != expectedSummary {
		t.Errorf("Unexpected summary: got %q, want %q", summary, expectedSummary)
	}
	if count != 1 {
		t.Errorf("Expected 1 original atom, got %d", count)
	}
}

func TestActivatePhase_ExtremeTaskCount(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{}
	cp := NewContextPager(kernel, llm, 1000000)

	// Set a reasonable timeout for 10k tasks
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	profileID := "extreme_profile"
	profile := ContextProfile{
		ID:              profileID,
		RequiredSchemas: []string{"schema1"},
		RequiredTools:   []string{"tool1"},
		FocusPatterns:   []string{"*.go"},
	}
	kernel.Assert(profile.ToFacts()[0])

	phase := &Phase{
		ID:             "huge_phase",
		Name:           "Performance Test Phase",
		ContextProfile: profileID,
	}

	taskCount := 10000
	phase.Tasks = make([]Task, taskCount)
	for i := range taskCount {
		phase.Tasks[i] = Task{
			ID: fmt.Sprintf("task_%d", i),
			Artifacts: []TaskArtifact{
				{Path: fmt.Sprintf("src/file_%d.go", i)},
			},
		}
	}

	start := time.Now()
	err := cp.ActivatePhase(ctx, phase)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("ActivatePhase failed on large input: %v", err)
	}

	t.Logf("ActivatePhase with %d tasks took %v", taskCount, duration)

	// Verify assertions
	var artifactCount int
	for _, f := range kernel.Facts {
		if f.Predicate == "phase_context_atom" {
			artifactCount++
		}
	}

	// 10000 artifacts pushed.
	if artifactCount != 10000 {
		t.Errorf("Expected 10000 artifacts, got %d", artifactCount)
	}
}
func TestCompressPhase_MassiveOutput(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			if len(prompt) < 10*1024*1024 {
				return "", fmt.Errorf("prompt size %d is less than 10MB", len(prompt))
			}
			return "Phase summary: Massive output processed successfully.", nil
		},
	}
	cp := NewContextPager(kernel, llm, 100000)
	ctx := context.Background()

	phaseID := "phase_massive"

	// Create a task with a very long description and many tasks to exceed 10MB
	// 11,000 tasks * ~1KB description = ~11MB
	var tasks []Task
	longDesc := strings.Repeat("A", 1024)
	for i := range 11000 {
		tasks = append(tasks, Task{
			ID:          fmt.Sprintf("task%d", i),
			Description: longDesc,
			Status:      TaskCompleted,
		})
	}

	phase := &Phase{
		ID:    phaseID,
		Name:  "Massive Phase",
		Tasks: tasks,
	}

	// Simulate existing phase atoms
	kernel.Assert(core.Fact{
		Predicate: "phase_context_atom",
		Args:      []any{phaseID, "some_atom", 100},
	})

	// Run Compression
	summary, count, _, err := cp.CompressPhase(ctx, phase)
	if err != nil {
		t.Fatalf("CompressPhase failed: %v", err)
	}

	if summary != "Phase summary: Massive output processed successfully." {
		t.Errorf("Unexpected summary: %s", summary)
	}
	if count != 1 {
		t.Errorf("Expected 1 original atom, got %d", count)
	}

	// Verify context_compression fact
	compressionStored := false
	for _, f := range kernel.Facts {
		if f.Predicate == "context_compression" && f.Args[0] == phaseID {
			compressionStored = true
			if f.Args[1] != summary {
				t.Errorf("Stored summary mismatch, got: %v", f.Args[1])
			}
			break
		}
	}
	if !compressionStored {
		t.Error("Expected context_compression fact to be asserted")
	}
}

func TestActivatePhase_TasksWithNilArtifacts(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{}
	cp := NewContextPager(kernel, llm, 100000)
	ctx := context.Background()

	phase := &Phase{
		ID:             "phase_nil_artifacts",
		Name:           "Test Phase Nil Artifacts",
		ContextProfile: "profile1",
		Tasks: []Task{
			{
				ID:        "task1",
				Artifacts: nil,
			},
		},
	}

	err := cp.ActivatePhase(ctx, phase)
	if err != nil {
		t.Fatalf("ActivatePhase failed when tasks have nil artifacts: %v", err)
	}

	// Verify it processed correctly. No artifact-based atoms should be generated.
	for _, f := range kernel.Facts {
		if f.Predicate == "phase_context_atom" {
			arg1 := fmt.Sprintf("%v", f.Args[1])
			if strings.Contains(arg1, "file_topology") {
				t.Errorf("Did not expect file_topology facts for nil artifacts, got: %v", f)
			}
		}
	}
}

func TestScopedDocsForPhase_PerformanceExtremes(t *testing.T) {
	kernel := &MockKernel{}
	cp := NewContextPager(kernel, &MockLLMClient{}, 100000)

	numFacts := 1000000
	kernel.Facts = make([]core.Fact, 0, numFacts+2)

	// Pre-populate with a large number of irrelevant facts
	for range numFacts {
		kernel.Facts = append(kernel.Facts, core.Fact{
			Predicate: "phase_context_scope",
			Args:      []any{"other_phase", "irrelevant_doc.md"},
		})
	}

	// Add the expected facts for our target phase
	targetPhase := "target_phase"
	expectedDocs := []string{"doc1.md", "doc2.md"}
	for _, doc := range expectedDocs {
		kernel.Facts = append(kernel.Facts, core.Fact{
			Predicate: "phase_context_scope",
			Args:      []any{targetPhase, doc},
		})
	}

	start := time.Now()
	docs := cp.scopedDocsForPhase(targetPhase)
	elapsed := time.Since(start)

	if len(docs) != len(expectedDocs) {
		t.Errorf("Expected %d docs, got %d", len(expectedDocs), len(docs))
	}

	// Verify the correct docs were returned
	for _, expected := range expectedDocs {
		found := slices.Contains(docs, expected)
		if !found {
			t.Errorf("Expected doc %s not found in result", expected)
		}
	}

	// Performance assertion: check if it completes within 5 seconds.
	// Normally this should take less than 100ms.
	if elapsed > 5*time.Second {
		t.Errorf("Linear scan took too long: %v (expected < 5s)", elapsed)
	}
}

func TestActivatePhase_ExtremeArtifacts(t *testing.T) {
	kernel := &MockKernel{}
	llm := &MockLLMClient{}
	cp := NewContextPager(kernel, llm, 3000000)
	ctx := context.Background()

	// Create a phase with 100,000 artifacts
	numArtifacts := 100000
	artifacts := make([]TaskArtifact, numArtifacts)
	for i := range numArtifacts {
		artifacts[i] = TaskArtifact{Path: fmt.Sprintf("src/file_%d.go", i)}
	}

	phase := &Phase{
		ID:             "extreme_phase",
		Name:           "Extreme Artifact Phase",
		ContextProfile: "profile1",
		Tasks: []Task{
			{
				ID:        "task1",
				Artifacts: artifacts,
			},
		},
	}

	// This should run without timing out and correctly assert all facts
	err := cp.ActivatePhase(ctx, phase)
	if err != nil {
		t.Fatalf("ActivatePhase failed: %v", err)
	}

	// Verify that the facts were asserted.
	// ActivatePhase asserts:
	// - focus patterns for the default profile (which has 1 pattern: "**/*")
	// - 100,000 phase_context_atom facts for the artifacts
	// - 5 activation facts to suppress irrelevant schemas
	// So we expect 1 + 100000 + 5 = 100006 facts in total.

	// We count specifically the phase_context_atom facts to be robust
	artifactFactCount := 0
	for _, f := range kernel.Facts {
		if f.Predicate == "phase_context_atom" {
			artifactFactCount++
		}
	}

	if artifactFactCount != numArtifacts {
		t.Errorf("Expected %d artifact facts to be asserted, got %d", numArtifacts, artifactFactCount)
	}
}

func TestScopedDocsForPhase_TypeCoercion(t *testing.T) {
	kernel := &MockKernel{}
	cp := NewContextPager(kernel, &MockLLMClient{}, 100000)

	// Inject scoped docs facts with non-string arguments
	// 1. Int as phase, string as doc
	kernel.Assert(core.Fact{
		Predicate: "phase_context_scope",
		Args:      []any{123, "doc_int.md"},
	})

	// 2. Bool as phase, string as doc
	kernel.Assert(core.Fact{
		Predicate: "phase_context_scope",
		Args:      []any{true, "doc_bool.md"},
	})

	// 3. String as phase, int as doc
	kernel.Assert(core.Fact{
		Predicate: "phase_context_scope",
		Args:      []any{"test_phase", 456},
	})

	// 4. String as phase, bool as doc
	kernel.Assert(core.Fact{
		Predicate: "phase_context_scope",
		Args:      []any{"test_phase", false},
	})

	// 5. String as phase, nil as doc
	kernel.Assert(core.Fact{
		Predicate: "phase_context_scope",
		Args:      []any{"test_phase", nil},
	})

	// 6. Mangle Atom as phase, Mangle Atom as doc
	kernel.Assert(core.Fact{
		Predicate: "phase_context_scope",
		Args:      []any{types.MangleAtom("/atom_phase"), types.MangleAtom("/atom_doc")},
	})

	// Test 1: Int as phase
	// ExtractString(123) -> "123", normalizeLayerName("123") -> "123"
	docs := cp.scopedDocsForPhase("123")
	if len(docs) != 1 || docs[0] != "doc_int.md" {
		t.Errorf("Expected [\"doc_int.md\"], got %v", docs)
	}

	// Test 2: Bool as phase
	// ExtractString(true) -> "/true", normalizeLayerName("/true") -> "_true"
	docs = cp.scopedDocsForPhase("/true")
	if len(docs) != 1 || docs[0] != "doc_bool.md" {
		t.Errorf("Expected [\"doc_bool.md\"], got %v", docs)
	}

	// Test 3: String as phase, mixed doc types
	// "test_phase" normalizes to "test_phase"
	// 456 -> "456"
	// false -> "/false"
	// nil -> "" (should be skipped)
	docs = cp.scopedDocsForPhase("test_phase")

	// We expect 2 docs because nil is skipped
	if len(docs) != 2 {
		t.Errorf("Expected 2 docs for test_phase, got %d: %v", len(docs), docs)
	} else {
		// Check for presence, order may vary if we used map iteration but here facts are linear
		expectedDocs := map[string]bool{"456": false, "/false": false}
		for _, d := range docs {
			if _, ok := expectedDocs[d]; ok {
				expectedDocs[d] = true
			} else {
				t.Errorf("Unexpected doc found: %s", d)
			}
		}
		for k, v := range expectedDocs {
			if !v {
				t.Errorf("Expected doc missing: %s", k)
			}
		}
	}

	// Test 4: Mangle Atom as phase
	// ExtractString(/atom_phase) -> "/atom_phase", normalizeLayerName("/atom_phase") -> "_atom_phase"
	docs = cp.scopedDocsForPhase("/atom_phase")
	if len(docs) != 1 || docs[0] != "/atom_doc" {
		t.Errorf("Expected [\"/atom_doc\"], got %v", docs)
	}
}

func TestActivatePhase_MalformedPhaseID(t *testing.T) {
	kernel := &MockKernel{}
	cp := NewContextPager(kernel, &MockLLMClient{}, 100000)
	ctx := context.Background()

	malformedID := "invalid phase id !@#$%\n^&*()"
	phase := &Phase{
		ID:   malformedID,
		Name: "Test Phase",
		Tasks: []Task{
			{
				ID: "task1",
				Artifacts: []TaskArtifact{
					{Path: "src/main.go"},
				},
			},
		},
	}

	err := cp.ActivatePhase(ctx, phase)
	if err != nil {
		t.Fatalf("ActivatePhase failed: %v", err)
	}

	// Verify that the fact was correctly asserted with the malformed ID
	found := false
	for _, f := range kernel.Facts {
		if f.Predicate == "phase_context_atom" && len(f.Args) > 0 {
			arg0 := fmt.Sprintf("%v", f.Args[0])
			if arg0 == malformedID {
				found = true
				break
			}
		}
	}

	if !found {
		t.Error("Expected phase_context_atom fact to be asserted with the exact malformed ID")
	}
}

func TestActivatePhase_DoubleActivation(t *testing.T) {
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

	// 2. Activate Phase for the first time
	err := cp.ActivatePhase(ctx, phase)
	if err != nil {
		t.Fatalf("First ActivatePhase failed: %v", err)
	}
	usedTokensFirstRun := cp.usedTokens

	// 3. Activate Phase again (Double Activation)
	err = cp.ActivatePhase(ctx, phase)
	if err != nil {
		t.Fatalf("Second ActivatePhase failed: %v", err)
	}

	// 4. Verify Idempotency
	if cp.usedTokens != usedTokensFirstRun {
		t.Errorf("Expected usedTokens to be idempotent, got %d on first run and %d on second run", usedTokensFirstRun, cp.usedTokens)
	}

	// Although MockKernel appends facts, we verify that the scores themselves are idempotent
	// (i.e. they are always exactly what they should be, and there's no accumulated scores like 240 instead of 120).
	for _, f := range kernel.Facts {
		if f.Predicate == "activation" && len(f.Args) > 1 {
			scoreStr := fmt.Sprintf("%v", f.Args[1])
			if scoreStr != "120" && scoreStr != "-100" && scoreStr != "-200" {
				t.Errorf("Found unexpected accumulated or malformed activation score: %s for args %v", scoreStr, f.Args[0])
			}
		}
		if f.Predicate == "phase_context_atom" && len(f.Args) > 2 {
			scoreStr := fmt.Sprintf("%v", f.Args[2])
			if scoreStr != "120" && scoreStr != "100" {
				t.Errorf("Found unexpected accumulated or malformed phase_context_atom score: %s for args %v", scoreStr, f.Args[1])
			}
		}
	}
}

func (m *MockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, forceJSON bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 1)
	errorChan := make(chan error, 1)
	go func() {
		defer close(contentChan)
		defer close(errorChan)
		res, err := m.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			errorChan <- err
			return
		}
		contentChan <- res
	}()
	return contentChan, errorChan
}
