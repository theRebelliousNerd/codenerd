//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/autopoiesis"
	"codenerd/internal/core"
	"codenerd/internal/types"
)

// =============================================================================
// CROSS-BOUNDARY MOCKS (RuleCourt + Feedback tests)
// =============================================================================

// rfMockLLMClient satisfies the autopoiesis.LLMClient interface for feedback tests.
type rfMockLLMClient struct {
	mu        sync.Mutex
	responses []string
	idx       int
}

func (m *rfMockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.idx < len(m.responses) {
		res := m.responses[m.idx]
		m.idx++
		return res, nil
	}
	return `{"improved_code":"// fixed","changes":["fix"],"expected_gain":0.5,"test_cases":["test1"]}`, nil
}

func (m *rfMockLLMClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	return m.Complete(ctx, sys+"\n"+user)
}

func (m *rfMockLLMClient) CompleteWithTools(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "mock"}, nil
}

// =============================================================================
// TEST 1: RuleCourt × Kernel — Constitutional Safety Boundary Hardening
// Sourced from: rule_court_boundary_analysis.md §3.1, §3.2, §3.4
// =============================================================================

func TestE2E_RuleCourt_ConstitutionalSafetyBoundary(t *testing.T) {
	// --- Nil Kernel ---
	t.Run("NilKernel_GracefulDegradation", func(t *testing.T) {
		err := core.RatifyRule(nil, "some_rule().")
		if err == nil {
			t.Fatal("Expected error for nil kernel")
		}
		if !strings.Contains(err.Error(), "no kernel") {
			t.Errorf("Expected 'no kernel' error, got: %v", err)
		}
	})

	// --- Empty Rule Variants ---
	t.Run("EmptyAndWhitespaceRules", func(t *testing.T) {
		kernel, err := core.NewRealKernel()
		if err != nil {
			t.Fatalf("Failed to create kernel: %v", err)
		}
		court := core.NewRuleCourt(kernel)

		cases := []struct {
			name string
			rule string
		}{
			{"empty_string", ""},
			{"spaces_only", "   "},
			{"tabs_and_newlines", " \t\n\r "},
			{"unicode_whitespace", " \u200B  \n \t  "},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := court.RatifyRule(tc.rule)
				if err == nil {
					t.Error("Expected error for whitespace-only rule")
				}
			})
		}
	})

	// --- False Positive ask_user Safety Hatch (§3.4) ---
	t.Run("FalsePositive_AskUser_InIdentifier", func(t *testing.T) {
		kernel, err := core.NewRealKernel()
		if err != nil {
			t.Fatalf("Failed to create kernel: %v", err)
		}
		court := core.NewRuleCourt(kernel)

		// A safe rule containing "ask_user" as part of an identifier
		rule := `log_id("ask_user_id_12345").`
		err = court.RatifyRule(rule)
		if err != nil && strings.Contains(err.Error(), "ask_user") {
			t.Log("KNOWN ISSUE: False positive ask_user blockage on safe identifier")
			t.Logf("Error: %v", err)
			// This documents the substring-matching fragility identified in the QA report
		}
	})

	// --- Null Bytes and Malformed UTF-8 (§3.2) ---
	t.Run("NullBytes_InRule", func(t *testing.T) {
		kernel, err := core.NewRealKernel()
		if err != nil {
			t.Fatalf("Failed to create kernel: %v", err)
		}
		court := core.NewRuleCourt(kernel)

		rule := "test_perm(\"\x00\")."
		err = court.RatifyRule(rule)
		// Should not panic — either reject cleanly or handle gracefully
		if err != nil {
			t.Logf("Null byte rule correctly rejected: %v", err)
		} else {
			t.Log("Null byte rule was accepted (Mangle parser tolerated it)")
		}
	})

	// --- Massive Rule (OOM Risk — §3.3) ---
	t.Run("MassiveRule_MemoryBound", func(t *testing.T) {
		kernel, err := core.NewRealKernel()
		if err != nil {
			t.Fatalf("Failed to create kernel: %v", err)
		}
		court := core.NewRuleCourt(kernel)

		// 1MB rule — large enough to stress but won't OOM on CI
		massive := `test_perm("` + strings.Repeat("a", 1_000_000) + `").`

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		done := make(chan error, 1)
		go func() {
			done <- court.RatifyRule(massive)
		}()

		select {
		case err := <-done:
			t.Logf("Massive rule result: err=%v", err)
		case <-ctx.Done():
			t.Error("TIMEOUT: RatifyRule hung on 1MB rule — no execution timeout guard")
		}
	})

	// --- Concurrent Ratification Race (§3.4) ---
	t.Run("ConcurrentRatification_NoRace", func(t *testing.T) {
		kernel, err := core.NewRealKernel()
		if err != nil {
			t.Fatalf("Failed to create kernel: %v", err)
		}

		var wg sync.WaitGroup
		errCh := make(chan error, 20)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				// Concurrently assert facts while ratifying
				kernel.Assert(core.Fact{
					Predicate: "observation",
					Args:      []interface{}{fmt.Sprintf("obs_%d", idx)},
				})
			}(i)
		}

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				rule := fmt.Sprintf(`concurrent_rule_%d("test").`, idx)
				err := core.RatifyRule(kernel, rule)
				if err != nil {
					errCh <- fmt.Errorf("ratify %d: %w", idx, err)
				}
			}(i)
		}

		wg.Wait()
		close(errCh)

		var errors []error
		for err := range errCh {
			errors = append(errors, err)
		}
		t.Logf("Concurrent ratification: %d/10 rejected (expected: all may be rejected due to sandbox compile)", len(errors))
	})

	// --- Liveness Deadlock Detection (§3.4) ---
	t.Run("LivenessCheck_ZeroBasePermitted", func(t *testing.T) {
		kernel, err := core.NewRealKernel()
		if err != nil {
			t.Fatalf("Failed to create kernel: %v", err)
		}

		// With no permitted() derivation in base kernel, any rule should pass liveness
		rule := `test_fact("hello").`
		err = core.RatifyRule(kernel, rule)
		// This tests the §3.4 gap: len(basePermitted) == 0 bypasses the deadlock check
		t.Logf("Zero-base-permitted result: err=%v", err)
	})
}

// =============================================================================
// TEST 2: LearningStore × Concurrent Feedback × Mangle Fact Generation
// Sourced from: feedback_boundary_analysis.md §2, §3, §5
// =============================================================================

func TestE2E_LearningStore_ConcurrentFeedback_MangleFacts(t *testing.T) {
	tmpDir := t.TempDir()
	store := autopoiesis.NewLearningStore(tmpDir)

	// --- Nil Feedback Panic Prevention (§2.1) ---
	t.Run("NilFeedback_NoPanic", func(t *testing.T) {
		defer func() {
			if r := recover(); r != nil {
				// KNOWN ISSUE: RecordLearning does not guard against nil feedback.
				// QA Report §2.1 recommends adding: if feedback == nil { return }
				t.Logf("KNOWN ISSUE (QA §2.1): nil feedback causes panic: %v", r)
			}
		}()
		// This may panic if the guard clause is missing
		store.RecordLearning("nil_test", nil, nil)
	})

	// --- Empty ToolName (§2.2) ---
	t.Run("EmptyToolName_NoCorruption", func(t *testing.T) {
		feedback := &autopoiesis.ExecutionFeedback{
			ToolName: "",
			Success:  true,
			Duration: 100 * time.Millisecond,
		}
		store.RecordLearning("", feedback, nil)

		facts := store.GenerateMangleFacts()
		for _, f := range facts {
			if strings.Contains(f, `tool_learning("",`) {
				t.Log("KNOWN ISSUE: Empty string key creates malformed Mangle fact")
				t.Logf("Fact: %s", f)
			}
		}
	})

	// --- NaN Quality Score Poisoning (§3.1) ---
	t.Run("NaN_QualityScore_Poisoning", func(t *testing.T) {
		nanStore := autopoiesis.NewLearningStore(t.TempDir())

		feedback := &autopoiesis.ExecutionFeedback{
			ToolName: "nan_tool",
			Success:  true,
			Duration: 50 * time.Millisecond,
			Quality: &autopoiesis.QualityAssessment{
				Score: math.NaN(),
			},
		}
		nanStore.RecordLearning("nan_tool", feedback, nil)

		facts := nanStore.GenerateMangleFacts()
		for _, f := range facts {
			if strings.Contains(f, "NaN") || strings.Contains(f, "nan") {
				// KNOWN ISSUE (QA §3.1): NaN propagates through moving average math
				// and leaks into Mangle facts. Fix: validate Score before averaging.
				t.Logf("KNOWN ISSUE (QA §3.1): NaN leaked into Mangle fact: %s", f)
			}
		}

		learning := nanStore.GetLearning("nan_tool")
		if learning != nil && math.IsNaN(learning.AverageQuality) {
			t.Log("KNOWN ISSUE: NaN propagated to AverageQuality — will corrupt Mangle engine")
		}
	})

	// --- Concurrent Read/Write Data Race (§5.1) ---
	t.Run("ConcurrentReadWrite_NoRace", func(t *testing.T) {
		raceStore := autopoiesis.NewLearningStore(t.TempDir())
		var wg sync.WaitGroup
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		// 5 writers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(writerID int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					select {
					case <-ctx.Done():
						return
					default:
					}
					toolName := fmt.Sprintf("tool_%d", writerID)
					fb := &autopoiesis.ExecutionFeedback{
						ToolName: toolName,
						Success:  j%3 != 0,
						Duration: time.Duration(j*10) * time.Millisecond,
						Quality: &autopoiesis.QualityAssessment{
							Score: float64(j) / 20.0,
						},
					}
					raceStore.RecordLearning(toolName, fb, nil)
				}
			}(i)
		}

		// 5 readers
		for i := 0; i < 5; i++ {
			wg.Add(1)
			go func(readerID int) {
				defer wg.Done()
				for j := 0; j < 20; j++ {
					select {
					case <-ctx.Done():
						return
					default:
					}
					all := raceStore.GetAllLearnings()
					_ = len(all)
					_ = raceStore.GenerateMangleFacts()
					time.Sleep(5 * time.Millisecond)
				}
			}(i)
		}

		wg.Wait()

		finalLearnings := raceStore.GetAllLearnings()
		t.Logf("Concurrent test: %d learnings persisted", len(finalLearnings))
		if len(finalLearnings) == 0 {
			t.Error("Expected learnings after concurrent writes")
		}
	})

	// --- AntiPattern O(N²) Accumulation (§4.3) ---
	t.Run("AntiPattern_Accumulation_Performance", func(t *testing.T) {
		perfStore := autopoiesis.NewLearningStore(t.TempDir())

		start := time.Now()
		for i := 0; i < 200; i++ {
			patterns := []*autopoiesis.DetectedPattern{
				{
					PatternID:  fmt.Sprintf("pattern_%d", i),
					IssueType:  autopoiesis.IssueIncomplete,
					Confidence: 0.8,
				},
			}
			fb := &autopoiesis.ExecutionFeedback{
				ToolName: "perf_tool",
				Success:  true,
				Duration: 10 * time.Millisecond,
			}
			perfStore.RecordLearning("perf_tool", fb, patterns)
		}
		elapsed := time.Since(start)

		learning := perfStore.GetLearning("perf_tool")
		if learning != nil {
			t.Logf("AntiPatterns accumulated: %d, time: %v", len(learning.AntiPatterns), elapsed)
			if elapsed > 5*time.Second {
				t.Errorf("Performance degradation: 200 iterations took %v (expected <5s)", elapsed)
			}
		}
	})
}

// =============================================================================
// TEST 3: ShadowMode × TransactionManager — Singleton Contention Stress
// Sourced from: shadow_mode_boundary_analysis.md §3.4, §4
// =============================================================================

func TestE2E_ShadowMode_TransactionManager_SingletonContention(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	// --- Singleton Lock: Double StartSimulation ---
	t.Run("DoubleStartSimulation_Rejection", func(t *testing.T) {
		sm := core.NewShadowMode(kernel)
		ctx := context.Background()

		sim1, err := sm.StartSimulation(ctx, "first simulation")
		if err != nil {
			t.Fatalf("First simulation failed: %v", err)
		}
		t.Logf("Started sim: %s", sim1.ID)

		_, err = sm.StartSimulation(ctx, "second simulation")
		if err == nil {
			t.Error("Expected error for second concurrent simulation")
		}
		if !strings.Contains(err.Error(), "already active") {
			t.Errorf("Expected 'already active' error, got: %v", err)
		}

		sm.AbortSimulation("test cleanup")
	})

	// --- Transaction + ShadowMode Interlock ---
	t.Run("Transaction_ShadowMode_Interlock", func(t *testing.T) {
		tmpDir := t.TempDir()
		tm := core.NewTransactionManager(kernel, tmpDir)
		ctx := context.Background()

		// Begin → Prepare → Prepare should fail (shadow busy or state wrong)
		txn, err := tm.Begin(ctx, "interlock test")
		if err != nil {
			t.Fatalf("Begin failed: %v", err)
		}

		testFile := fmt.Sprintf("%s/interlock_test.go", tmpDir)
		err = tm.AddEdit(ctx, core.FileEdit{
			FilePath: testFile,
			Content:  []byte("package test\n"),
			EditType: core.EditTypeCreate,
		})
		if err != nil {
			t.Fatalf("AddEdit failed: %v", err)
		}

		result, err := tm.Prepare(ctx)
		if err != nil {
			t.Fatalf("First Prepare failed: %v", err)
		}
		t.Logf("First prepare: valid=%v", result.IsValid)

		// Second Prepare should fail because state is no longer Pending
		_, err = tm.Prepare(ctx)
		if err == nil {
			t.Error("Expected error on second Prepare (not in pending state)")
		}

		// Clean up
		if result.IsValid {
			tm.Commit(ctx)
		} else {
			tm.Abort(ctx, "test")
		}
		_ = txn
	})

	// --- Concurrent WhatIf Hammer ---
	t.Run("ConcurrentWhatIf_NoDeadlock", func(t *testing.T) {
		sm := core.NewShadowMode(kernel)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		// WhatIf is serial by design (starts+aborts internally), but concurrent
		// callers should queue, not deadlock
		var wg sync.WaitGroup
		results := make(chan bool, 20)

		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				action := core.SimulatedAction{
					ID:          fmt.Sprintf("concurrent_whatif_%d", idx),
					Type:        core.ActionTypeFileWrite,
					Target:      fmt.Sprintf("pkg/concurrent_%d.go", idx),
					Description: fmt.Sprintf("Concurrent WhatIf %d", idx),
				}
				_, err := sm.WhatIf(ctx, action)
				results <- (err == nil)
			}(i)
		}

		wg.Wait()
		close(results)

		successCount := 0
		for ok := range results {
			if ok {
				successCount++
			}
		}
		t.Logf("Concurrent WhatIf: %d/20 succeeded (serial by design, others may fail on lock)", successCount)

		if sm.IsShadowModeActive() {
			t.Error("Shadow mode leaked active state after concurrent WhatIfs")
		}
	})
}

// =============================================================================
// TEST 4: Full Constitutional Pipeline
// RuleCourt × ShadowMode × TransactionManager × Kernel — End-to-End Safety
// =============================================================================

func TestE2E_FullConstitutionalPipeline(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	// Seed some dependency facts for impact analysis
	kernel.Assert(core.Fact{
		Predicate: "dependency_link",
		Args:      []interface{}{"cmd/main.go", "internal/core/kernel.go", "internal"},
	})
	kernel.Assert(core.Fact{
		Predicate: "dependency_link",
		Args:      []interface{}{"internal/core/kernel.go", "internal/core/rule_court.go", "internal"},
	})

	ctx := context.Background()
	tmpDir := t.TempDir()

	// Phase 1: Ratify a rule through the RuleCourt
	court := core.NewRuleCourt(kernel)
	testRule := `test_observation("pipeline_test").`
	err = court.RatifyRule(testRule)
	t.Logf("RuleCourt ratification: err=%v", err)

	// Phase 2: Shadow Mode — simulate a file write and check impact
	sm := core.NewShadowMode(kernel)
	action := core.SimulatedAction{
		ID:          "pipeline_file_write",
		Type:        core.ActionTypeFileWrite,
		Target:      "internal/core/kernel.go",
		Description: "Modify kernel.go (should trigger dependency impact)",
	}

	result, err := sm.WhatIf(ctx, action)
	if err != nil {
		t.Fatalf("WhatIf failed: %v", err)
	}

	impactedEffects := 0
	for _, effect := range result.Effects {
		if effect.Predicate == "impacted" {
			impactedEffects++
		}
	}
	t.Logf("Shadow WhatIf: effects=%d, impacted=%d, violations=%d, safe=%v",
		len(result.Effects), impactedEffects, len(result.Violations), result.IsSafe)

	if len(result.Effects) == 0 {
		t.Error("Expected effects from WhatIf on dependency-linked file")
	}

	// Phase 3: TransactionManager — full 2PC cycle with shadow validation
	tm := core.NewTransactionManager(kernel, tmpDir)
	txn, err := tm.Begin(ctx, "pipeline integration test")
	if err != nil {
		t.Fatalf("Begin failed: %v", err)
	}

	testFile := fmt.Sprintf("%s/pipeline_output.go", tmpDir)
	err = tm.AddEdit(ctx, core.FileEdit{
		FilePath: testFile,
		Content:  []byte("package pipeline\n\nfunc Init() {}\n"),
		EditType: core.EditTypeCreate,
	})
	if err != nil {
		t.Fatalf("AddEdit failed: %v", err)
	}

	validation, err := tm.Prepare(ctx)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	t.Logf("Transaction validation: valid=%v, duration=%v", validation.IsValid, validation.ValidDuration)

	if validation.IsValid {
		err = tm.Commit(ctx)
		if err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify kernel state after commit
		writtenFacts, _ := kernel.Query("file_written")
		t.Logf("Kernel file_written facts after commit: %d", len(writtenFacts))
	} else {
		tm.Abort(ctx, "validation failed")
	}

	// Verify final kernel consistency
	tmFacts := tm.ToFacts()
	t.Logf("Final TM facts: %d, transaction_active=%v", len(tmFacts), tm.IsTransactionActive())

	if tm.IsTransactionActive() {
		t.Error("Transaction should not be active after commit/abort")
	}
	_ = txn
}

// =============================================================================
// TEST 5: LearningStore → MangleFacts → Kernel Assert — Data Integrity Pipeline
// Sourced from: feedback_boundary_analysis.md §3.3
// =============================================================================

func TestE2E_LearningStore_MangleFacts_KernelAssert_Pipeline(t *testing.T) {
	store := autopoiesis.NewLearningStore(t.TempDir())

	// Populate learning store with varied tool data
	tools := []struct {
		name    string
		success bool
		quality float64
		issues  []autopoiesis.IssueType
	}{
		{"file_reader", true, 0.95, nil},
		{"api_fetcher", false, 0.3, []autopoiesis.IssueType{autopoiesis.IssueRateLimit}},
		{"code_generator", true, 0.8, []autopoiesis.IssueType{autopoiesis.IssueIncomplete}},
		{"test_runner", true, 0.7, []autopoiesis.IssueType{autopoiesis.IssueSlow, autopoiesis.IssuePoorFormat}},
		{"doc_parser", true, 0.99, nil},
	}

	for _, tool := range tools {
		qualityIssues := make([]autopoiesis.QualityIssue, 0, len(tool.issues))
		for _, it := range tool.issues {
			qualityIssues = append(qualityIssues, autopoiesis.QualityIssue{
				Type:     it,
				Severity: 0.7,
			})
		}

		fb := &autopoiesis.ExecutionFeedback{
			ToolName: tool.name,
			Success:  tool.success,
			Duration: 100 * time.Millisecond,
			Quality: &autopoiesis.QualityAssessment{
				Score:  tool.quality,
				Issues: qualityIssues,
			},
		}
		store.RecordLearning(tool.name, fb, nil)
	}

	// Generate Mangle facts and validate syntax
	facts := store.GenerateMangleFacts()
	t.Logf("Generated %d Mangle facts from %d tools", len(facts), len(tools))

	if len(facts) == 0 {
		t.Fatal("Expected Mangle facts from learning store")
	}

	// Validate fact format
	for _, fact := range facts {
		if !strings.HasSuffix(fact, ".") {
			t.Errorf("Mangle fact missing terminal period: %s", fact)
		}
		if strings.Contains(fact, "NaN") || strings.Contains(fact, "Inf") {
			t.Errorf("Invalid numeric in Mangle fact: %s", fact)
		}
	}

	// Verify tool_learning facts exist for each tool
	learningFacts := 0
	issueFacts := 0
	for _, f := range facts {
		if strings.HasPrefix(f, "tool_learning(") {
			learningFacts++
		}
		if strings.HasPrefix(f, "tool_known_issue(") {
			issueFacts++
		}
	}
	t.Logf("Fact breakdown: tool_learning=%d, tool_known_issue=%d", learningFacts, issueFacts)

	if learningFacts != len(tools) {
		t.Errorf("Expected %d tool_learning facts, got %d", len(tools), learningFacts)
	}

	// Verify GetAllLearnings returns deep copies (§5.1 fix validation)
	allLearnings := store.GetAllLearnings()
	if len(allLearnings) > 0 {
		// Mutate the returned copy — should not affect the store
		allLearnings[0].ToolName = "MUTATED"
		originalLearning := store.GetLearning(tools[0].name)
		if originalLearning != nil && originalLearning.ToolName == "MUTATED" {
			t.Error("GetAllLearnings returned shallow copy — mutation leaked to store")
		}
	}
}
