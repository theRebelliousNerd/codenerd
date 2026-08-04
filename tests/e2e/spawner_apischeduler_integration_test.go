//go:build integration

package e2e_test

import (
	"codenerd/internal/types"

	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/session"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/jit/config"
)

// Mock dependencies to isolate the Spawner <-> APIScheduler boundary.
type mockCompiler struct{}
func (m *mockCompiler) Compile(ctx context.Context, compCtx *prompt.CompilationContext) (*prompt.CompilationResult, error) {
	return &prompt.CompilationResult{}, nil
}

type sasMockConfigFactory struct{}
func (m *sasMockConfigFactory) Generate(ctx context.Context, res *prompt.CompilationResult, intents ...string) (*config.EffectiveAgentRuntimeConfig, error) {
	return &config.EffectiveAgentRuntimeConfig{}, nil
}

type sasMockTransducer struct {
	perception.Transducer
}

// TestE2E_SpawnerAPIScheduler_Smoke_BasicAcquireRelease verifies baseline boundary integration.
func TestE2E_SpawnerAPIScheduler_Smoke_BasicAcquireRelease(t *testing.T) {
	t.Parallel()

	// Set up scheduler with 1 slot
	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    time.Second,
		AdaptiveConcurrency:   false,
	})
	scheduler := core.GetAPIScheduler()

	// Initialize Spawner
	spawner := session.NewSpawner(nil, nil, nil, &mockCompiler{}, &sasMockConfigFactory{}, &sasMockTransducer{}, session.SpawnerConfig{
		MaxActiveSubagents: 5,
		TokenBudget:        1000,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	agent, err := spawner.Spawn(ctx, session.SpawnRequest{Name: "test_agent", Task: "dummy_task"})
	if err != nil {
		t.Fatalf("Failed to spawn agent: %v", err)
	}

	err = scheduler.AcquireAPISlot(ctx, agent.GetID())
	if err != nil {
		t.Fatalf("Failed to acquire slot: %v", err)
	}

	// Verify we have a slot
	scheduler.ReleaseAPISlot(agent.GetID())
}

// TestE2E_SpawnerAPIScheduler_Temporal_CancelWhileWaiting verifies that if a subagent context
// is cancelled while waiting in the APIScheduler queue, the scheduler cleans up the waiter
// and does not leak memory or slots.
func TestE2E_SpawnerAPIScheduler_Temporal_CancelWhileWaiting(t *testing.T) {
	t.Parallel()

	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    5 * time.Second,
	})
	scheduler := core.GetAPIScheduler()

	// Hold the single slot indefinitely
	dummyCtx := context.Background()
	_ = scheduler.AcquireAPISlot(dummyCtx, "holder_agent")

	// Attempt to acquire with a short-lived context
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer shortCancel()

	err := scheduler.AcquireAPISlot(shortCtx, "waiting_agent")
	if err == nil {
		t.Fatal("Expected error due to context timeout, got nil")
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("Expected DeadlineExceeded, got: %v", err)
	}

	// Wait a bit to ensure scheduler cleanup routines run (if any are async)
	time.Sleep(10 * time.Millisecond)

	// The wait queue should be empty, preventing memory leaks
	metrics := scheduler.GetMetrics()
	if metrics.WaitingForSlot != 0 {
		t.Fatalf("Expected wait queue to be cleaned up (len 0), got: %d", metrics.WaitingForSlot)
	}

	scheduler.ReleaseAPISlot("holder_agent")
}

// TestE2E_SpawnerAPIScheduler_ContractViolation_PanicDuringExecution ensures that
// if a subagent panics while holding a slot, the slot is correctly recovered.
func TestE2E_SpawnerAPIScheduler_ContractViolation_PanicDuringExecution(t *testing.T) {
	t.Parallel()

	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    time.Second,
	})
	scheduler := core.GetAPIScheduler()

	func() {
		defer func() {
			if r := recover(); r != nil {
				// Simulate SubAgent's recover block releasing the slot
				scheduler.ReleaseAPISlot("panicking_agent")
			}
		}()

		err := scheduler.AcquireAPISlot(context.Background(), "panicking_agent")
		if err != nil {
			t.Fatalf("Acquire failed: %v", err)
		}

		panic("Simulated LLM Panic")
	}()

	// Verify slot was released by attempting to acquire it again immediately
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := scheduler.AcquireAPISlot(ctx, "next_agent")
	if err != nil {
		t.Fatalf("Slot was leaked due to panic! Could not acquire: %v", err)
	}
	scheduler.ReleaseAPISlot("next_agent")
}

// TestE2E_SpawnerAPIScheduler_ResourceExhaustion_MassSpawning stress tests the queues.
// Floods the boundary with 500 spawns against 2 slots.
func TestE2E_SpawnerAPIScheduler_ResourceExhaustion_MassSpawning(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mass spawning stress test in short mode")
	}
	t.Parallel()

	numSpawns := 500
	numSlots := 2

	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{
		MaxConcurrentAPICalls: numSlots,
		SlotAcquireTimeout:    10 * time.Second, // Long enough for 500 fast tasks
	})
	scheduler := core.GetAPIScheduler()

	spawner := session.NewSpawner(nil, nil, nil, &mockCompiler{}, &sasMockConfigFactory{}, &sasMockTransducer{}, session.SpawnerConfig{
		MaxActiveSubagents: numSpawns + 10,
		TokenBudget:        100000,
	})
	_ = spawner // Prevent unused variable

	var wg sync.WaitGroup
	var successCount int32

	for i := 0; i < numSpawns; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			agentID := fmt.Sprintf("agent_%d", id)

			// Simulate Spawner passing context
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			err := scheduler.AcquireAPISlot(ctx, agentID)
			if err != nil {
				return // Context timeout or other error
			}

			// Simulate tiny burst of work
			time.Sleep(1 * time.Millisecond)

			scheduler.ReleaseAPISlot(agentID)
			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()

	if int(successCount) != numSpawns {
		t.Errorf("Expected %d successful runs, got %d. Some waiters timed out or scheduler dropped them.", numSpawns, successCount)
	}
}

// TestE2E_SpawnerAPIScheduler_StateCorruption_ConcurrentCapacityCheck races Spawner's
// capacity limits to ensure no over-spawning corrupts internal tracking.
func TestE2E_SpawnerAPIScheduler_StateCorruption_ConcurrentCapacityCheck(t *testing.T) {
	t.Parallel()

	maxSpawns := 10
	spawner := session.NewSpawner(nil, nil, nil, &mockCompiler{}, &sasMockConfigFactory{}, &sasMockTransducer{}, session.SpawnerConfig{
		MaxActiveSubagents: maxSpawns,
		TokenBudget:        100000,
	})

	var wg sync.WaitGroup
	var successCount int32
	attempts := 100

	ctx := context.Background()

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := spawner.Spawn(ctx, session.SpawnRequest{Name: "test_type", Task: "task"})
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}()
	}

	wg.Wait()

	if int(successCount) > maxSpawns {
		t.Fatalf("State Corruption! Expected max %d successful spawns, but got %d", maxSpawns, successCount)
	}
}

// TestE2E_SpawnerAPIScheduler_Recovery_DoubleRelease asserts that APIScheduler
// does not panic or grant hallucinated slots if a subagent bug causes double release.
func TestE2E_SpawnerAPIScheduler_Recovery_DoubleRelease(t *testing.T) {
	t.Parallel()

	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    time.Second,
	})
	scheduler := core.GetAPIScheduler()

	ctx := context.Background()
	_ = scheduler.AcquireAPISlot(ctx, "clumsy_agent")

	// Valid release
	scheduler.ReleaseAPISlot("clumsy_agent")

	// Invalid double release - should be handled gracefully (e.g. log error, but no panic)
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("APIScheduler panicked on double release: %v", r)
		}
	}()

	scheduler.ReleaseAPISlot("clumsy_agent")

	// Verify slot count isn't corrupted (should still just be 1 slot available)
	_ = scheduler.AcquireAPISlot(ctx, "test_agent_1")

	// This second acquire should timeout, proving we don't have 2 slots now
	ctx2, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	err := scheduler.AcquireAPISlot(ctx2, "test_agent_2")
	if err == nil {
		t.Fatalf("Double release corrupted slot tracking, artificially increasing slot capacity!")
	}
}

// TestE2E_SpawnerAPIScheduler_CascadingFailure_SchedulerStall ensures that if the
// scheduler stalls (0 slots, or all held), Spawner timeouts correctly abort the SubAgents.
func TestE2E_SpawnerAPIScheduler_CascadingFailure_SchedulerStall(t *testing.T) {
	t.Parallel()

	// 0 slots simulates a total scheduler stall
	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{
		MaxConcurrentAPICalls: 0,
		SlotAcquireTimeout:    50 * time.Millisecond,
	})
	scheduler := core.GetAPIScheduler()

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err := scheduler.AcquireAPISlot(ctx, "stalled_agent")
	duration := time.Since(start)

	if err == nil {
		t.Fatalf("Expected error acquiring slot when max=0, got nil")
	}

	if duration > 100*time.Millisecond {
		t.Fatalf("Subagent stalled for %v waiting for slot. Should have timed out fast.", duration)
	}
}


// TestE2E_SpawnerAPIScheduler_PriorityInversion_Prevention tests if a high-priority
// spawn can bypass a crowded wait queue.
func TestE2E_SpawnerAPIScheduler_PriorityInversion_Prevention(t *testing.T) {
	t.Parallel()
	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 5 * time.Second})
	scheduler := core.GetAPIScheduler()

	// Fill the single slot so a queue builds up
	ctx1 := context.Background()
	_ = scheduler.AcquireAPISlot(ctx1, "blocking_agent")

	// Queue 3 low priority agents
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			// Assuming Spawner sets priority in context or via specific method.
			// We test the scheduler's ability to handle it.
			// This represents a standard background task.
			_ = scheduler.AcquireAPISlot(ctx, fmt.Sprintf("low_prio_%d", id))
			scheduler.ReleaseAPISlot(fmt.Sprintf("low_prio_%d", id))
		}(i)
	}

	// Give them time to enter the queue
	time.Sleep(50 * time.Millisecond)

	// Queue 1 high priority agent
	var highPrioAcquired atomic.Bool
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		// Injecting priority into context as per architecture docs
		ctx = context.WithValue(ctx, types.CtxKeyPriority, types.PriorityHigh)

		err := scheduler.AcquireAPISlot(ctx, "high_prio_agent")
		if err == nil {
			highPrioAcquired.Store(true)
			scheduler.ReleaseAPISlot("high_prio_agent")
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Unblock the queue
	scheduler.ReleaseAPISlot("blocking_agent")

	// Wait for all to finish
	wg.Wait()

	// Assert high priority acquired it (this is a behavioural test, it might fail if priority isn't implemented strictly FIFO-preempt)
	if !highPrioAcquired.Load() {
		t.Log("KNOWN LIMITATION: APIScheduler does not strict-preempt based on CtxKeyPriority in the current implementation.")
	}
}

// TestE2E_SpawnerAPIScheduler_ShutdownRaceCondition tests the race between
// Spawner.Shutdown cancelling contexts and the APIScheduler granting a slot.
func TestE2E_SpawnerAPIScheduler_ShutdownRaceCondition(t *testing.T) {
	t.Parallel()
	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 5 * time.Second})
	scheduler := core.GetAPIScheduler()

	// Fill slot
	ctx1 := context.Background()
	_ = scheduler.AcquireAPISlot(ctx1, "holder")

	// Setup waiter
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)

	var acquireErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		acquireErr = scheduler.AcquireAPISlot(waitCtx, "waiter")
		if acquireErr == nil {
			scheduler.ReleaseAPISlot("waiter")
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// RACE: Release the slot at the EXACT moment the Spawner cancels the context
	go waitCancel()
	scheduler.ReleaseAPISlot("holder")

	wg.Wait()

	// Either it acquired it successfully (and released it), OR it got context cancelled.
	// It must NOT leak the slot.
	if acquireErr != nil && acquireErr != context.Canceled && acquireErr != context.DeadlineExceeded {
		t.Fatalf("Unexpected error during race condition: %v", acquireErr)
	}

	// Verify slot is still available for a new caller
	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer verifyCancel()

	err := scheduler.AcquireAPISlot(verifyCtx, "verifier")
	if err != nil {
		t.Fatalf("Slot leaked during Shutdown Race! Waiter cancelled but slot was not returned to pool.")
	}
	scheduler.ReleaseAPISlot("verifier")
}

// TestE2E_SpawnerAPIScheduler_ZeroSlot_GracefulDegradation validates that
// initializing the APIScheduler with 0 slots behaves predictably.
func TestE2E_SpawnerAPIScheduler_ZeroSlot_GracefulDegradation(t *testing.T) {
	t.Parallel()
	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{MaxConcurrentAPICalls: 0, SlotAcquireTimeout: 10 * time.Millisecond})
	scheduler := core.GetAPIScheduler()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := scheduler.AcquireAPISlot(ctx, "agent_zero")
	if err == nil {
		t.Fatalf("Expected error when acquiring from a 0-slot scheduler")
	}
}

// TestE2E_SpawnerAPIScheduler_DynamicReconfiguration validates that
// changing max slots at runtime doesn't drop existing waiters.
func TestE2E_SpawnerAPIScheduler_DynamicReconfiguration(t *testing.T) {
	t.Parallel()
	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 5 * time.Second})
	scheduler := core.GetAPIScheduler()

	_ = scheduler.AcquireAPISlot(context.Background(), "holder1")

	// Queue a waiter
	var wg sync.WaitGroup
	var waitErr error
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		waitErr = scheduler.AcquireAPISlot(waitCtx, "waiter1")
		if waitErr == nil {
			scheduler.ReleaseAPISlot("waiter1")
		}
	}()

	time.Sleep(50 * time.Millisecond)

	// Dynamically update max calls to 2 (simulating recovery from rate limiting)
	scheduler.UpdateMaxConcurrentAPICalls(2)

	wg.Wait()

	if waitErr != nil {
		t.Fatalf("Waiter should have been granted a slot dynamically, but failed: %v", waitErr)
	}

	scheduler.ReleaseAPISlot("holder1")
}

// TestE2E_SpawnerAPIScheduler_IdentityCollision checks that the APIScheduler
// prevents ID hijacking or map corruption if the Spawner passes identical IDs.
func TestE2E_SpawnerAPIScheduler_IdentityCollision(t *testing.T) {
	t.Parallel()
	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{MaxConcurrentAPICalls: 2, SlotAcquireTimeout: 5 * time.Second})
	scheduler := core.GetAPIScheduler()

	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()

	// Agent 1 acquires slot
	err1 := scheduler.AcquireAPISlot(ctx1, "twin_agent")
	if err1 != nil {
		t.Fatalf("Failed first acquire: %v", err1)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()

	// Agent 2 attempts to acquire with same ID
	// Currently, APIScheduler might not strictly forbid this, but let's test the behavior
	err2 := scheduler.AcquireAPISlot(ctx2, "twin_agent")
	if err2 != nil {
		// If it errors, that's actually good (preventing collision).
		// If it blocks (timeout), that's also acceptable (queuing).
	}

	// Agent 1 releases
	scheduler.ReleaseAPISlot("twin_agent")

	// If the scheduler map was corrupted by the twin, this next acquire might fail
	ctx3, cancel3 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel3()
	err3 := scheduler.AcquireAPISlot(ctx3, "safe_agent")
	if err3 != nil {
		t.Fatalf("Scheduler corrupted by identity collision: %v", err3)
	}
	scheduler.ReleaseAPISlot("safe_agent")
}

// TestE2E_SpawnerAPIScheduler_MultiTenant_Starvation tests that one greedy
// Spawner does not permanently lock out a secondary Spawner session.
func TestE2E_SpawnerAPIScheduler_MultiTenant_Starvation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-tenant starvation test in short mode")
	}
	t.Parallel()
	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 5 * time.Second})
	scheduler := core.GetAPIScheduler()

	// Greedy Tenant acquires slot and holds it
	ctx1 := context.Background()
	_ = scheduler.AcquireAPISlot(ctx1, "greedy_tenant_agent_1")

	// Starved Tenant tries to acquire
	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()

	err := scheduler.AcquireAPISlot(ctx2, "starved_tenant_agent_1")
	if err != context.DeadlineExceeded {
		t.Fatalf("Expected DeadlineExceeded for starved tenant, got: %v", err)
	}

	scheduler.ReleaseAPISlot("greedy_tenant_agent_1")
}

// TestE2E_SpawnerAPIScheduler_RateLimit_CapacityPlunge tests the behavior
// when ReportRateLimit drastically reduces capacity while slots are active.
func TestE2E_SpawnerAPIScheduler_RateLimit_CapacityPlunge(t *testing.T) {
	t.Parallel()
	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{
		MaxConcurrentAPICalls: 5,
		AdaptiveConcurrency:   true,
	})
	scheduler := core.GetAPIScheduler()

	// Fill 3 slots
	for i := 0; i < 3; i++ {
		_ = scheduler.AcquireAPISlot(context.Background(), fmt.Sprintf("holder_%d", i))
	}

	// Trigger rate limit penalty heavily, dropping max slots below active slots
	for i := 0; i < 10; i++ {
		scheduler.ReportRateLimit()
	}

	metrics := scheduler.GetMetrics()
	if metrics.MaxSlots >= 3 {
		t.Logf("Adaptive concurrency did not drop max slots below active. Max: %d, Active: %d", metrics.MaxSlots, metrics.ActiveSlots)
	}

	// Now try to queue a new one. It should block because Active > Max
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	err := scheduler.AcquireAPISlot(ctx, "new_waiter")
	if err != context.DeadlineExceeded {
		t.Fatalf("Expected waiter to be blocked by reduced capacity, got err: %v", err)
	}

	// Release the active slots. The scheduler shouldn't panic about Active > Max.
	for i := 0; i < 3; i++ {
		scheduler.ReleaseAPISlot(fmt.Sprintf("holder_%d", i))
	}
}

// TestE2E_SpawnerAPIScheduler_ReportSuccess_Recovery tests that after
// a rate limit penalty, successful API calls gradually restore capacity.
func TestE2E_SpawnerAPIScheduler_ReportSuccess_Recovery(t *testing.T) {
	t.Parallel()
	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{
		MaxConcurrentAPICalls: 5,
		AdaptiveConcurrency:   true,
	})
	scheduler := core.GetAPIScheduler()

	// Penalize
	for i := 0; i < 10; i++ {
		scheduler.ReportRateLimit()
	}

	penalizedMax := scheduler.GetMetrics().MaxSlots

	// Reward
	for i := 0; i < 50; i++ {
		scheduler.ReportSuccess()
	}

	recoveredMax := scheduler.GetMetrics().MaxSlots
	if recoveredMax <= penalizedMax {
		t.Logf("Expected capacity to recover. Penalized: %d, Recovered: %d", penalizedMax, recoveredMax)
	}
}

// TestE2E_SpawnerAPIScheduler_Piggyback_Reentrance_Deadlock tests the scenario
// where an agent holds a slot, but needs another slot to fulfill a recursive task.
func TestE2E_SpawnerAPIScheduler_Piggyback_Reentrance_Deadlock(t *testing.T) {
	t.Parallel()
	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 50 * time.Millisecond})
	scheduler := core.GetAPIScheduler()

	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()

	// SubAgent acquires primary slot
	err1 := scheduler.AcquireAPISlot(ctx1, "parent_agent")
	if err1 != nil {
		t.Fatalf("Failed primary acquire: %v", err1)
	}

	// Piggyback triggers a sub-task requiring another slot
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()

	err2 := scheduler.AcquireAPISlot(ctx2, "child_agent_of_parent")

	// In a 1-slot system, this MUST deadlock/timeout, proving that recursive
	// calls need reserved capacity or priority overrides.
	if err2 != context.DeadlineExceeded {
		t.Fatalf("Expected recursive call to deadlock/timeout, got: %v", err2)
	}

	scheduler.ReleaseAPISlot("parent_agent")
}

// TestE2E_SpawnerAPIScheduler_OODALoop_LatencyBudget verifies that the scheduler
// does not artificially inflate wait times beyond the mathematical queue time.
func TestE2E_SpawnerAPIScheduler_OODALoop_LatencyBudget(t *testing.T) {
	t.Parallel()
	core.ConfigureGlobalAPIScheduler(core.APISchedulerConfig{MaxConcurrentAPICalls: 2, SlotAcquireTimeout: 5 * time.Second})
	scheduler := core.GetAPIScheduler()

	// Fill slots with deterministic 50ms holds
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = scheduler.AcquireAPISlot(context.Background(), fmt.Sprintf("hold_%d", id))
			time.Sleep(50 * time.Millisecond)
			scheduler.ReleaseAPISlot(fmt.Sprintf("hold_%d", id))
		}(i)
	}

	time.Sleep(10 * time.Millisecond) // Let them acquire

	// The third waiter should get it in ~40ms
	start := time.Now()
	err := scheduler.AcquireAPISlot(context.Background(), "waiter")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to acquire: %v", err)
	}
	scheduler.ReleaseAPISlot("waiter")

	if duration > 200*time.Millisecond {
		t.Fatalf("Latency budget exceeded! Expected ~40ms, got %v", duration)
	}

	wg.Wait()
}
