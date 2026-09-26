//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/jit/config"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// =============================================================================
// CROSS-BOUNDARY MOCKS
// =============================================================================

// cbMockLLMClient is a cross-boundary mock that supports both core.LLMClient
// and types.LLMClient (they are aliases). It records all prompts for inspection.
type cbMockLLMClient struct {
	mu              sync.Mutex
	responses       []string
	idx             int
	promptsReceived []string
	delay           time.Duration
	toolResponse    *types.LLMToolResponse
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

func (m *cbMockLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	errCh := make(chan error, 1)
	ch <- "mock streaming response"
	close(ch)
	close(errCh)
	return ch, errCh
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
func (m *cbMockTransducer) SetPromptAssembler(pa perception.PromptAssembler) {}
func (m *cbMockTransducer) SetStrategicContext(ctx string)                   {}

type cbMockJITCompiler struct{ result *prompt.CompilationResult }

func (m *cbMockJITCompiler) Compile(ctx context.Context, cc *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	if m.result != nil {
		return m.result, nil
	}
	return &prompt.CompilationResult{Prompt: "test prompt"}, nil
}

type cbMockConfigFactory struct {
	cfg *config.EffectiveAgentRuntimeConfig
}

func (m *cbMockConfigFactory) Generate(ctx context.Context, result *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	if m.cfg != nil {
		return m.cfg, nil
	}
	return &config.EffectiveAgentRuntimeConfig{}, nil
}

// =============================================================================
// TEST 2: Shadow Mode × Kernel × Transaction Manager — 2PC Safety Gate
// =============================================================================

func TestE2E_CrossBoundary_ShadowMode_2PC_SafetyGate(t *testing.T) {
	t.Parallel()
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

	// The safety gate rejects this transaction (writing outside the
	// workspace): invalid with at least one safety block, deterministically.
	// The commit path is covered by TransactionManager unit tests; this test
	// owns the rejection path end to end.
	if validationResult.IsValid {
		t.Fatalf("Expected the safety gate to reject the out-of-workspace write, got valid")
	}
	if len(validationResult.SafetyBlocks) == 0 {
		t.Fatalf("Expected safety blocks on rejection, got %+v", validationResult)
	}
	t.Logf("Gate rejected as expected: %s", validationResult.SafetyBlocks[0])

	// Abort and verify cleanup: nothing written, no active transaction.
	if err := tm.Abort(ctx, "validation failed"); err != nil {
		// May already be aborted by Prepare
		t.Logf("Abort returned: %v", err)
	}
	if _, statErr := os.Stat(testFile); !os.IsNotExist(statErr) {
		t.Errorf("Rejected transaction must not write its file: %s", testFile)
	}
	if tm.IsTransactionActive() {
		t.Error("Expected no active transaction after abort")
	}

	// Verify ToFacts works after completion
	tmFacts := tm.ToFacts()
	t.Logf("TransactionManager facts after completion: %d", len(tmFacts))
}

// =============================================================================
// TEST 3: Shadow Mode × WhatIf — Rapid Sequential Simulations
// =============================================================================

func TestE2E_CrossBoundary_ShadowMode_RapidWhatIf(t *testing.T) {
	t.Parallel()
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

	// Every simulation must succeed: a silent WhatIf failure is a dropped
	// safety analysis, and successCount==0 would divide by zero below.
	if successCount != 25 {
		t.Fatalf("Expected 25/25 WhatIf simulations to succeed, got %d", successCount)
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
	t.Parallel()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	executor := tactile.NewCompositeExecutorWithConfig(tactile.DefaultExecutorConfig())
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

	// Hammer RouteAction from 10 goroutines until the guard drops. Looping
	// (not one shot + sleep) makes overlap deterministic: every route
	// attempted during the active window must block, and the window is
	// long enough that attempts during it are guaranteed.
	var wg sync.WaitGroup
	var blockedCount atomic.Int64
	stop := make(chan struct{})

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			n := 0
			for {
				select {
				case <-stop:
					return
				default:
				}
				a := core.Fact{
					Predicate: "next_action",
					Args:      []interface{}{fmt.Sprintf("concurrent_%d_%d", idx, n), "/echo", "test"},
				}
				n++
				_, err := vs.RouteAction(context.Background(), a)
				if err != nil && strings.Contains(err.Error(), "boot guard") {
					blockedCount.Add(1)
				}
			}
		}(i)
	}

	time.Sleep(100 * time.Millisecond)

	// Disable boot guard
	vs.DisableBootGuard()

	if vs.IsBootGuardActive() {
		t.Error("Expected boot guard to be disabled")
	}

	close(stop)
	wg.Wait()

	t.Logf("Boot guard blocked %d concurrent RouteAction calls", blockedCount.Load())
	if blockedCount.Load() == 0 {
		t.Error("Expected routes during the active window to be blocked by boot guard")
	}

	// Post-disable, the same path must no longer raise the boot guard.
	a := core.Fact{
		Predicate: "next_action",
		Args:      []interface{}{"post_disable_probe", "/echo", "test"},
	}
	if _, err := vs.RouteAction(context.Background(), a); err != nil && strings.Contains(err.Error(), "boot guard") {
		t.Errorf("Boot guard still blocking after disable: %v", err)
	}

	// Verify audit metrics are available
	metrics := vs.GetAuditMetrics()
	t.Logf("Audit metrics: %+v", metrics)
}

// =============================================================================
// TEST 5: Executor × Multi-Turn × Kernel Intent — Conversation Drift
// =============================================================================

func TestE2E_CrossBoundary_Executor_MultiTurn_ConversationDrift(t *testing.T) {
	t.Parallel()
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

	// A real VirtualStore, a write tool, and a tool call in every response.
	//
	// The intent cycle here is the point of the test — /explain, /fix, /test,
	// /review is what "conversation drift" means — and checkHollowSuccess
	// (8e9507d) refuses /fix and /test turns that complete no tool call. This
	// executor was built with a nil VirtualStore, so there was no executive
	// gate at all and a write tool could never have run even if one were
	// offered. See write_turn_fixture_test.go for why the fix is a real write
	// turn rather than narrowing the verbs.
	vs := core.NewVirtualStore(nil)
	wireDreamer(vs, kernel)
	cf := &cbMockConfigFactory{cfg: &config.EffectiveAgentRuntimeConfig{
		AllowedTools: writeTurnAllowedTools(),
	}}
	lc := &cbMockLLMClient{toolResponse: &types.LLMToolResponse{
		Text:      "working",
		ToolCalls: []types.ToolCall{writeTurnCall(t)},
	}}

	exec := session.NewExecutor(kernel, vs, lc, jc, cf, tr)
	exec.SetConfig(writeTurnExecutorConfig(t))

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
	if len(intentFacts) == 0 {
		t.Error("Expected user_intent facts to accumulate in the kernel over 20 turns")
	}

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
// TEST 7: Executor × VirtualStore × Tool Registry — Dual Registry Routing
// =============================================================================

func TestE2E_CrossBoundary_Executor_DualRegistryToolRouting(t *testing.T) {
	t.Parallel()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	executor := tactile.NewCompositeExecutorWithConfig(tactile.DefaultExecutorConfig())
	vs := core.NewVirtualStore(executor)
	vs.SetKernel(kernel)

	// Create an Ouroboros tool registry and register a custom tool
	ouroborosReg := core.NewToolRegistry(t.TempDir())

	tr := &cbMockTransducer{intents: []perception.Intent{{Verb: "/coder", Target: "test"}}}
	jc := &cbMockJITCompiler{}
	cf := &cbMockConfigFactory{cfg: &config.EffectiveAgentRuntimeConfig{}}
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

	// Register a tool in the Ouroboros registry and execute it: dual
	// routing is only real if the registry resolves AND runs.
	if err := ouroborosReg.RegisterTool("echo-dual", "echo", "/generalist"); err != nil {
		t.Fatalf("Ouroboros register failed: %v", err)
	}
	out, err := ouroborosReg.ExecuteTool(context.Background(), "echo-dual", "dual-ping")
	if err != nil {
		t.Fatalf("Ouroboros execute failed: %v", err)
	}
	if !strings.Contains(out, "dual-ping") {
		t.Fatalf("Expected echoed input from Ouroboros tool, got %q", out)
	}

	// Verify the VirtualStore tool registry is a separate instance holding
	// different tools: the Ouroboros registration must not leak across.
	vsRegistry := vs.GetToolRegistry()
	if vsRegistry == nil {
		t.Fatal("Expected non-nil VirtualStore tool registry")
	}
	if vsRegistry == ouroborosReg {
		t.Fatal("VirtualStore and Ouroboros registries must be separate instances")
	}
	if _, found := vsRegistry.GetTool("echo-dual"); found {
		t.Error("Ouroboros registration leaked into the VirtualStore registry")
	}
}

// =============================================================================
// TEST 8: Kernel × Concurrent Assert/Query — Stress Test
// =============================================================================

func TestE2E_CrossBoundary_Kernel_ConcurrentAssertQuery_Stress(t *testing.T) {
	t.Parallel()
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
	t.Logf("Final observation count: %d (expected 200)", len(finalFacts))

	// All 200 asserts succeeded (no error reached errCh) with distinct
	// arguments, so the kernel must hold exactly 200 — no drops, no dupes.
	if len(finalFacts) != 200 {
		t.Errorf("Expected exactly 200 observations after concurrent assertions, got %d", len(finalFacts))
	}
}

// ResolveAllowedTools projects the same fixture envelope before JIT selection.
func (m *cbMockConfigFactory) ResolveAllowedTools(ctx context.Context, intents ...string) ([]string, error) {
	resolved, err := m.Generate(ctx, &prompt.CompilationResult{}, intents...)
	if err != nil || resolved == nil {
		return nil, err
	}
	return append([]string(nil), resolved.AllowedTools...), nil
}
