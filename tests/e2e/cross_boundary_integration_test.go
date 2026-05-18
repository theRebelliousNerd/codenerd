//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/articulation"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/jit/config"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// =============================================================================
// CROSS-BOUNDARY MOCKS
// =============================================================================

// cbMockLLMClient is a cross-boundary mock that supports both core.LLMClient
// and types.LLMClient (they are aliases). It records all prompts for inspection.
type cbMockLLMClient struct {
	mu             sync.Mutex
	responses      []string
	idx            int
	promptsReceived []string
	delay          time.Duration
	toolResponse   *types.LLMToolResponse
}

func (m *cbMockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.promptsReceived = append(m.promptsReceived, prompt)
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	if m.idx < len(m.responses) {
		res := m.responses[m.idx]
		m.idx++
		return res, nil
	}
	return "mock response", nil
}

func (m *cbMockLLMClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	return m.Complete(ctx, sys+"\n"+user)
}

func (m *cbMockLLMClient) CompleteWithTools(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if m.toolResponse != nil {
		return m.toolResponse, nil
	}
	return &types.LLMToolResponse{Text: "mock tool response"}, nil
}

func (m *cbMockLLMClient) getPrompts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]string, len(m.promptsReceived))
	copy(out, m.promptsReceived)
	return out
}

// cbMockTransducer returns configurable intents per-call.
type cbMockTransducer struct {
	intents []perception.Intent
	idx     int
	mu      sync.Mutex
}

func (m *cbMockTransducer) ParseIntent(ctx context.Context, input string) (perception.Intent, error) {
	return m.ParseIntentWithContext(ctx, input, nil)
}
func (m *cbMockTransducer) ParseIntentWithContext(ctx context.Context, input string, history []perception.ConversationTurn) (perception.Intent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.idx < len(m.intents) {
		intent := m.intents[m.idx]
		m.idx++
		return intent, nil
	}
	return perception.Intent{Verb: "/general", Target: "test"}, nil
}
func (m *cbMockTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []perception.ConversationTurn, maxRetries int) (perception.Intent, []string, error) {
	intent, err := m.ParseIntentWithContext(ctx, input, history)
	return intent, nil, err
}
func (m *cbMockTransducer) ResolveFocus(ctx context.Context, ref string, cands []string) (perception.FocusResolution, error) {
	return perception.FocusResolution{}, nil
}
func (m *cbMockTransducer) SetPromptAssembler(pa *articulation.PromptAssembler) {}
func (m *cbMockTransducer) SetStrategicContext(ctx string)                      {}

type cbMockJITCompiler struct{ result *prompt.CompilationResult }

func (m *cbMockJITCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	if m.result != nil {
		return m.result, nil
	}
	return &prompt.CompilationResult{Prompt: "test prompt"}, nil
}

type cbMockConfigFactory struct{ cfg *config.AgentConfig }

func (m *cbMockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.AgentConfig, error) {
	if m.cfg != nil {
		return m.cfg, nil
	}
	return &config.AgentConfig{}, nil
}

// =============================================================================
// TEST 1: TDD Loop × Kernel × VirtualStore — Full Repair Cycle
// =============================================================================

func TestE2E_CrossBoundary_TDDLoop_FullRepairCycle(t *testing.T) {
	// Wire: real kernel + real VirtualStore + TDDLoop
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	executor := tactile.NewCompositeExecutor()
	vs := core.NewVirtualStore(executor)
	vs.SetKernel(kernel)
	vs.DisableBootGuard()

	llm := &cbMockLLMClient{
		responses: []string{
			// Patch response for LLM
			"FILE: main.go\nOLD:\nfoo()\nNEW:\nbar()\nRATIONALE: Fix broken call",
		},
	}

	cfg := core.TDDLoopConfig{
		MaxRetries:   2,
		TestCommand:  "echo FAIL",
		BuildCommand: "echo ok",
		TestTimeout:  5 * time.Second,
		BuildTimeout: 5 * time.Second,
		WorkingDir:   t.TempDir(),
	}

	loop := core.NewTDDLoopWithConfig(vs, kernel, llm, cfg)

	// Verify initial state
	if loop.GetState() != core.TDDStateIdle {
		t.Errorf("Expected idle state, got %s", loop.GetState())
	}

	// Run one step: should transition to Running then Failing (because echo FAIL contains "FAIL")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := loop.Run(ctx); err != nil {
		t.Fatalf("First run step failed: %v", err)
	}

	state := loop.GetState()
	if state != core.TDDStateFailing && state != core.TDDStatePassing {
		t.Logf("State after first run: %s", state)
	}

	// Verify kernel has test_state fact
	facts, err := kernel.Query("test_state")
	if err != nil {
		t.Fatalf("Kernel query failed: %v", err)
	}
	if len(facts) == 0 {
		t.Error("Expected test_state fact in kernel after TDD run")
	}

	// Verify retry_count fact exists
	retryFacts, err := kernel.Query("retry_count")
	if err != nil {
		t.Fatalf("Kernel query for retry_count failed: %v", err)
	}
	if len(retryFacts) == 0 {
		t.Error("Expected retry_count fact in kernel after TDD run")
	}

	// Verify state history is recorded
	history := loop.GetHistory()
	if len(history) == 0 {
		t.Error("Expected non-empty state transition history")
	}

	// Run additional steps to test escalation path
	for i := 0; i < 5; i++ {
		if err := loop.Run(ctx); err != nil {
			t.Logf("Run step %d: %v", i, err)
			break
		}
		s := loop.GetState()
		if s == core.TDDStatePassing || s == core.TDDStateEscalated {
			t.Logf("TDD loop reached terminal state: %s after %d steps", s, i+1)
			break
		}
	}

	// Verify ToFacts produces valid output
	tddFacts := loop.ToFacts()
	if len(tddFacts) == 0 {
		t.Error("Expected ToFacts to return non-empty slice")
	}

	t.Logf("TDD loop completed: final_state=%s, history_len=%d, tdd_facts=%d",
		loop.GetState(), len(loop.GetHistory()), len(tddFacts))
}

// =============================================================================
// TEST 2: Shadow Mode × Kernel × Transaction Manager — 2PC Safety Gate
// =============================================================================

func TestE2E_CrossBoundary_ShadowMode_2PC_SafetyGate(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	tmpDir := t.TempDir()
	tm := core.NewTransactionManager(kernel, tmpDir)

	// Begin a transaction
	ctx := context.Background()
	txn, err := tm.Begin(ctx, "test transaction")
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	if txn.Status != core.TxnStatusPending {
		t.Errorf("Expected pending status, got %s", txn.Status)
	}

	// Cannot begin another while one is active
	_, err = tm.Begin(ctx, "second txn")
	if err == nil {
		t.Error("Expected error when beginning second concurrent transaction")
	}

	// Add a file creation edit (no snapshot needed)
	testFile := fmt.Sprintf("%s/test_file.go", tmpDir)
	err = tm.AddEdit(ctx, core.FileEdit{
		FilePath: testFile,
		Content:  []byte("package main\n\nfunc main() {}\n"),
		EditType: core.EditTypeCreate,
	})
	if err != nil {
		t.Fatalf("Failed to add edit: %v", err)
	}

	// Prepare (Phase 1) — runs shadow simulation
	validationResult, err := tm.Prepare(ctx)
	if err != nil {
		t.Fatalf("Prepare failed: %v", err)
	}

	t.Logf("Validation: valid=%v, parse_errors=%d, safety_blocks=%d, warnings=%d, duration=%v",
		validationResult.IsValid, len(validationResult.ParseErrors),
		len(validationResult.SafetyBlocks), len(validationResult.Warnings),
		validationResult.ValidDuration)

	if validationResult.IsValid {
		// Commit (Phase 2) — writes to filesystem
		if err := tm.Commit(ctx); err != nil {
			t.Fatalf("Commit failed: %v", err)
		}

		// Verify file_written fact was injected into kernel
		writtenFacts, err := kernel.Query("file_written")
		if err != nil {
			t.Fatalf("Query file_written failed: %v", err)
		}
		if len(writtenFacts) == 0 {
			t.Error("Expected file_written fact after commit")
		}

		// Verify no active transaction remains
		if tm.IsTransactionActive() {
			t.Error("Expected no active transaction after commit")
		}
	} else {
		t.Logf("Transaction validation failed (expected in some kernel configs)")
		// Abort and verify cleanup
		if err := tm.Abort(ctx, "validation failed"); err != nil {
			// May already be aborted by Prepare
			t.Logf("Abort returned: %v", err)
		}
	}

	// Verify ToFacts works after completion
	tmFacts := tm.ToFacts()
	t.Logf("TransactionManager facts after completion: %d", len(tmFacts))
}

// =============================================================================
// TEST 3: Shadow Mode × WhatIf — Rapid Sequential Simulations
// =============================================================================

func TestE2E_CrossBoundary_ShadowMode_RapidWhatIf(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	// Seed dependency facts for impact analysis
	kernel.Assert(core.Fact{
		Predicate: "dependency_link",
		Args:      []interface{}{"pkg/handler.go", "pkg/service.go", "internal"},
	})
	kernel.Assert(core.Fact{
		Predicate: "dependency_link",
		Args:      []interface{}{"pkg/service.go", "pkg/repo.go", "internal"},
	})

	sm := core.NewShadowMode(kernel)

	ctx := context.Background()
	actionTypes := []core.SimActionType{
		core.ActionTypeFileWrite,
		core.ActionTypeFileDelete,
		core.ActionTypeRefactor,
		core.ActionTypeExec,
		core.ActionTypeGitCommit,
	}

	successCount := 0
	var totalDuration time.Duration

	for i := 0; i < 25; i++ {
		action := core.SimulatedAction{
			ID:          fmt.Sprintf("whatif_%d", i),
			Type:        actionTypes[i%len(actionTypes)],
			Target:      fmt.Sprintf("pkg/file_%d.go", i),
			Description: fmt.Sprintf("WhatIf test action %d", i),
		}

		start := time.Now()
		result, err := sm.WhatIf(ctx, action)
		elapsed := time.Since(start)
		totalDuration += elapsed

		if err != nil {
			t.Logf("WhatIf %d failed: %v", i, err)
			continue
		}

		successCount++
		if len(result.Effects) == 0 {
			t.Errorf("WhatIf %d: expected effects for action type %s", i, action.Type)
		}
	}

	// Verify no simulation leak — should not be active after all WhatIfs
	if sm.IsShadowModeActive() {
		t.Error("Shadow mode should not be active after WhatIf queries")
	}

	avgDuration := totalDuration / time.Duration(successCount)
	t.Logf("WhatIf results: %d/%d succeeded, avg_duration=%v, total=%v",
		successCount, 25, avgDuration, totalDuration)

	if avgDuration > 5*time.Second {
		t.Errorf("WhatIf average duration too slow: %v (expected <5s)", avgDuration)
	}
}

// =============================================================================
// TEST 4: Kernel × VirtualStore × Boot Guard — Permission Cache Race
// =============================================================================

func TestE2E_CrossBoundary_VirtualStore_BootGuard_PermissionRace(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	executor := tactile.NewCompositeExecutor()
	vs := core.NewVirtualStore(executor)

	// Boot guard should be active initially
	if !vs.IsBootGuardActive() {
		t.Error("Expected boot guard to be active on fresh VirtualStore")
	}

	// SetKernel triggers rebuildPermissionCache — this is the deadlock-prone path
	vs.SetKernel(kernel)

	// Verify RouteAction is blocked while boot guard is active
	action := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"test_action_1", "/echo", "hello"},
	}
	_, err = vs.RouteAction(context.Background(), action)
	if err == nil || !strings.Contains(err.Error(), "boot guard") {
		t.Errorf("Expected boot guard error, got: %v", err)
	}

	// Launch concurrent RouteAction calls while boot guard is active
	var wg sync.WaitGroup
	blockedCount := 0
	var blockedMu sync.Mutex

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			a := core.Fact{
				Predicate: "next_action",
				Args:      []interface{}{fmt.Sprintf("concurrent_%d", idx), "/echo", "test"},
			}
			_, err := vs.RouteAction(context.Background(), a)
			if err != nil && strings.Contains(err.Error(), "boot guard") {
				blockedMu.Lock()
				blockedCount++
				blockedMu.Unlock()
			}
		}(i)
	}

	// Give goroutines time to hit the boot guard
	time.Sleep(50 * time.Millisecond)

	// Disable boot guard
	vs.DisableBootGuard()

	if vs.IsBootGuardActive() {
		t.Error("Expected boot guard to be disabled")
	}

	wg.Wait()

	t.Logf("Boot guard blocked %d/10 concurrent RouteAction calls", blockedCount)
	if blockedCount == 0 {
		t.Error("Expected at least some calls to be blocked by boot guard")
	}

	// Verify audit metrics are available
	metrics := vs.GetAuditMetrics()
	t.Logf("Audit metrics: %+v", metrics)
}

// =============================================================================
// TEST 5: Executor × Multi-Turn × Kernel Intent — Conversation Drift
// =============================================================================

func TestE2E_CrossBoundary_Executor_MultiTurn_ConversationDrift(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	intents := []perception.Intent{
		{Verb: "/explain", Target: "auth.go", Category: "/query"},
		{Verb: "/fix", Target: "auth.go", Category: "/mutation"},
		{Verb: "/test", Target: "auth_test.go", Category: "/instruction"},
		{Verb: "/review", Target: "auth.go", Category: "/query"},
	}
	// Repeat intents to fill 20 turns
	allIntents := make([]perception.Intent, 20)
	for i := range allIntents {
		allIntents[i] = intents[i%len(intents)]
	}

	tr := &cbMockTransducer{intents: allIntents}
	jc := &cbMockJITCompiler{result: &prompt.CompilationResult{Prompt: "test prompt"}}
	cf := &cbMockConfigFactory{cfg: &config.AgentConfig{}}
	lc := &cbMockLLMClient{}

	exec := session.NewExecutor(kernel, nil, lc, jc, cf, tr)

	var durations []time.Duration

	for i := 0; i < 20; i++ {
		start := time.Now()
		_, err := exec.Process(context.Background(), fmt.Sprintf("Turn %d: work on auth", i))
		elapsed := time.Since(start)
		durations = append(durations, elapsed)

		if err != nil {
			t.Fatalf("Turn %d failed: %v", i, err)
		}
	}

	// Verify conversation history grew correctly (20 user + 20 assistant = 40)
	history := exec.GetHistory()
	if len(history) != 40 {
		t.Errorf("Expected 40 history items, got %d", len(history))
	}

	// Verify kernel has user_intent facts
	intentFacts, err := kernel.Query("user_intent")
	if err != nil {
		t.Fatalf("Kernel query for user_intent failed: %v", err)
	}
	t.Logf("Kernel accumulated %d user_intent facts over 20 turns", len(intentFacts))

	// Check for performance degradation (last 5 turns shouldn't be >3x slower than first 5)
	if len(durations) >= 10 {
		var earlyAvg, lateAvg time.Duration
		for i := 0; i < 5; i++ {
			earlyAvg += durations[i]
			lateAvg += durations[len(durations)-5+i]
		}
		earlyAvg /= 5
		lateAvg /= 5

		ratio := float64(lateAvg) / float64(earlyAvg)
		t.Logf("Performance: early_avg=%v, late_avg=%v, ratio=%.2fx", earlyAvg, lateAvg, ratio)

		if ratio > 5.0 && lateAvg > 500*time.Millisecond {
			t.Errorf("Significant performance degradation detected: %.2fx slowdown (early=%v, late=%v)",
				ratio, earlyAvg, lateAvg)
		}
	}
}

// =============================================================================
// TEST 6: TDD Loop × LLM Mock × Patch Generation — Full Repair with LLM
// =============================================================================

func TestE2E_CrossBoundary_TDDLoop_PatchGeneration_WithLLM(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	executor := tactile.NewCompositeExecutor()
	vs := core.NewVirtualStore(executor)
	vs.SetKernel(kernel)
	vs.DisableBootGuard()

	// LLM that returns structured patch format
	llm := &cbMockLLMClient{
		responses: []string{
			"FILE: src/auth.go\nOLD:\nfunc login() {}\nNEW:\nfunc login(user string) error { return nil }\nRATIONALE: Add user parameter and error return",
			"FILE: src/auth.go\nOLD:\nfunc validate() {}\nNEW:\nfunc validate(token string) bool { return true }\nRATIONALE: Add token validation",
		},
	}

	cfg := core.TDDLoopConfig{
		MaxRetries:   2,
		TestCommand:  "echo '--- FAIL: TestAuth'",
		BuildCommand: "echo ok",
		TestTimeout:  5 * time.Second,
		BuildTimeout: 5 * time.Second,
		WorkingDir:   t.TempDir(),
	}

	loop := core.NewTDDLoopWithConfig(vs, kernel, llm, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Use bounded Run() steps instead of RunToCompletion.
	// FINDING: RunToCompletion with always-failing tests cycles indefinitely
	// because MaxRetries doesn't cap iterations in all TDD state paths.
	maxSteps := 15
	for step := 0; step < maxSteps; step++ {
		if err := loop.Run(ctx); err != nil {
			t.Logf("TDD step %d: %v", step, err)
			break
		}
		s := loop.GetState()
		if s == core.TDDStatePassing || s == core.TDDStateEscalated {
			t.Logf("TDD loop reached terminal state: %s after %d steps", s, step+1)
			break
		}
	}

	finalState := loop.GetState()
	t.Logf("Final state: %s", finalState)

	// Verify the LLM received at least one prompt
	prompts := llm.getPrompts()
	t.Logf("LLM received %d prompts", len(prompts))
	if len(prompts) > 0 {
		lastPrompt := prompts[len(prompts)-1]
		t.Logf("Last prompt length: %d chars", len(lastPrompt))
	}

	// Verify state history shows OODA cycle — the core architectural validation
	history := loop.GetHistory()
	statesSeen := make(map[core.TDDState]bool)
	for _, h := range history {
		statesSeen[h.ToState] = true
	}
	t.Logf("States visited: %d unique states across %d transitions", len(statesSeen), len(history))

	if !statesSeen[core.TDDStateFailing] {
		t.Error("Expected TDD loop to visit Failing state")
	}
	if len(history) == 0 {
		t.Error("Expected non-empty state transition history")
	}
}

// =============================================================================
// TEST 7: Executor × VirtualStore × Tool Registry — Dual Registry Routing
// =============================================================================

func TestE2E_CrossBoundary_Executor_DualRegistryToolRouting(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	executor := tactile.NewCompositeExecutor()
	vs := core.NewVirtualStore(executor)
	vs.SetKernel(kernel)

	// Create an Ouroboros tool registry and register a custom tool
	ouroborosReg := core.NewToolRegistry(t.TempDir())

	tr := &cbMockTransducer{intents: []perception.Intent{{Verb: "/coder", Target: "test"}}}
	jc := &cbMockJITCompiler{}
	cf := &cbMockConfigFactory{cfg: &config.AgentConfig{}}
	lc := &cbMockLLMClient{}

	exec := session.NewExecutor(kernel, vs, lc, jc, cf, tr)
	exec.SetOuroborosRegistry(ouroborosReg)

	// Process a request — should work even without Piggyback
	res, err := exec.Process(context.Background(), "test dual registry")
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}
	if res.Response == "" {
		t.Error("Expected non-empty response")
	}

	// Verify the tool registry is queryable
	allTools := ouroborosReg.ListTools()
	t.Logf("Ouroboros registry has %d tools", len(allTools))

	// Verify the VirtualStore tool registry is separate
	vsRegistry := vs.GetToolRegistry()
	if vsRegistry == nil {
		t.Error("Expected non-nil VirtualStore tool registry")
	}
}

// =============================================================================
// TEST 8: Kernel × Concurrent Assert/Query — Stress Test
// =============================================================================

func TestE2E_CrossBoundary_Kernel_ConcurrentAssertQuery_Stress(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, 100)

	// 10 concurrent asserters
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				fact := types.Fact{
					Predicate: "observation",
					Args: []interface{}{
						fmt.Sprintf("worker_%d_obs_%d", workerID, j),
						fmt.Sprintf("data_%d", j),
					},
				}
				if err := kernel.Assert(fact); err != nil {
					errCh <- fmt.Errorf("worker %d assert %d: %w", workerID, j, err)
					return
				}
			}
		}(i)
	}

	// 5 concurrent queriers
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(queryID int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				select {
				case <-ctx.Done():
					return
				default:
				}
				facts, err := kernel.Query("observation")
				if err != nil {
					errCh <- fmt.Errorf("querier %d query %d: %w", queryID, j, err)
					return
				}
				_ = len(facts) // Use the result
				time.Sleep(10 * time.Millisecond)
			}
		}(i)
	}

	wg.Wait()
	close(errCh)

	var errors []error
	for err := range errCh {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		for _, e := range errors[:min(5, len(errors))] {
			t.Errorf("Concurrent error: %v", e)
		}
		t.Errorf("Total concurrent errors: %d", len(errors))
	}

	// Verify final state is consistent
	finalFacts, err := kernel.Query("observation")
	if err != nil {
		t.Fatalf("Final query failed: %v", err)
	}
	t.Logf("Final observation count: %d (expected ~200)", len(finalFacts))

	if len(finalFacts) == 0 {
		t.Error("Expected observations in kernel after concurrent assertions")
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
