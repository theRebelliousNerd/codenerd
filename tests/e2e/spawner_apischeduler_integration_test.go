//go:build integration

package e2e_test

import (
	"context"
	"errors"
	"fmt"
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
	"codenerd/internal/types"
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
	scheduler := newTestScheduler(t, core.APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    time.Second,
		AdaptiveConcurrency:   false,
	})

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
	if agent.GetID() == "" {
		t.Fatal("Spawner returned an agent with an empty ID")
	}

	if err = acquire(scheduler, ctx, agent.GetID()); err != nil {
		t.Fatalf("Failed to acquire slot: %v", err)
	}
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 1 {
		t.Fatalf("scheduler holds %d slots after one acquire, want 1", metrics.ActiveSlots)
	}

	scheduler.ReleaseAPISlot(agent.GetID())
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 {
		t.Fatalf("scheduler holds %d slots after release, want 0", metrics.ActiveSlots)
	}
	if state, ok := scheduler.GetShardState(agent.GetID()); !ok {
		t.Fatal("scheduler lost the spawned agent's shard state")
	} else if state.APICallCount != 1 {
		t.Fatalf("spawned agent completed %d API calls, want 1", state.APICallCount)
	}
}

// TestE2E_SpawnerAPIScheduler_Temporal_CancelWhileWaiting verifies that if a subagent context
// is cancelled while waiting in the APIScheduler queue, the scheduler cleans up the waiter
// and does not leak memory or slots.
func TestE2E_SpawnerAPIScheduler_Temporal_CancelWhileWaiting(t *testing.T) {
	t.Parallel()

	scheduler := newTestScheduler(t, core.APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    5 * time.Second,
	})

	// Hold the single slot indefinitely
	if err := acquire(scheduler, context.Background(), "holder_agent"); err != nil {
		t.Fatalf("holder could not take the only slot: %v", err)
	}

	// Attempt to acquire with a short-lived context
	shortCtx, shortCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer shortCancel()

	if err := acquire(scheduler, shortCtx, "waiting_agent"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Expected DeadlineExceeded, got: %v", err)
	}

	// The wait queue should be empty, preventing memory leaks
	sasWaitForQueueDrained(t, scheduler, 2*time.Second)
	scheduler.ReleaseAPISlot("holder_agent")
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 || metrics.WaitingForSlot != 0 {
		t.Fatalf("scheduler leaked slots after cancellation: %+v", metrics)
	}
}

// TestE2E_SpawnerAPIScheduler_ContractViolation_PanicDuringExecution ensures
// that when a spawned agent's LLM call panics, the scheduled-call wrapper
// converts the panic to an error and releases the agent's slot.
func TestE2E_SpawnerAPIScheduler_ContractViolation_PanicDuringExecution(t *testing.T) {
	t.Parallel()

	scheduler := newTestScheduler(t, core.APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    time.Second,
	})
	spawner := session.NewSpawner(nil, nil, nil, &mockCompiler{}, &sasMockConfigFactory{}, &sasMockTransducer{}, session.SpawnerConfig{
		MaxActiveSubagents: 2,
		TokenBudget:        1000,
	})
	agent, err := spawner.Spawn(context.Background(), session.SpawnRequest{Name: "panicking_agent", Task: "panic task"})
	if err != nil {
		t.Fatalf("Failed to spawn agent: %v", err)
	}
	scheduler.RegisterShard(agent.GetID(), "e2e")

	scheduled := &core.ScheduledLLMCall{Scheduler: scheduler, ShardID: agent.GetID(), Client: &panickingClient{}}
	if _, err := scheduled.CompleteWithSystem(context.Background(), "sys", "user"); err == nil {
		t.Fatal("expected an error from a panicking LLM call, got success")
	} else if !strings.Contains(err.Error(), "panic during LLM call") {
		t.Fatalf("panic lost its contract wording: %v", err)
	}
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 {
		t.Fatalf("Slot leaked due to panic! ActiveSlots=%d", metrics.ActiveSlots)
	}

	// Verify the grant path is still healthy by acquiring immediately.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := acquire(scheduler, ctx, "next_agent"); err != nil {
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

	scheduler := newTestScheduler(t, core.APISchedulerConfig{
		MaxConcurrentAPICalls: numSlots,
		SlotAcquireTimeout:    10 * time.Second, // Long enough for 500 fast tasks
	})

	spawner := session.NewSpawner(nil, nil, nil, &mockCompiler{}, &sasMockConfigFactory{}, &sasMockTransducer{}, session.SpawnerConfig{
		MaxActiveSubagents: numSpawns + 10,
		TokenBudget:        100000,
	})

	var wg sync.WaitGroup
	var successCount int32
	agentIDs := make([]string, numSpawns)
	spawnErrs := make([]error, numSpawns)
	slotErrs := make([]error, numSpawns)

	for i := 0; i < numSpawns; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			agent, err := spawner.Spawn(ctx, session.SpawnRequest{
				Name:       fmt.Sprintf("mass_agent_%d", id),
				Task:       "mass spawn slot check",
				Type:       session.SubAgentTypeEphemeral,
				IntentVerb: "/fix",
			})
			if err != nil {
				spawnErrs[id] = err
				return
			}
			agentIDs[id] = agent.GetID()

			if err := acquire(scheduler, ctx, agent.GetID()); err != nil {
				slotErrs[id] = err
				return
			}

			// Simulate tiny burst of work
			time.Sleep(1 * time.Millisecond)

			scheduler.ReleaseAPISlot(agent.GetID())
			atomic.AddInt32(&successCount, 1)
		}(i)
	}

	wg.Wait()

	for i := range agentIDs {
		if spawnErrs[i] != nil {
			t.Fatalf("spawn %d failed: %v", i, spawnErrs[i])
		}
		if slotErrs[i] != nil {
			t.Fatalf("spawn %d failed to acquire a slot: %v", i, slotErrs[i])
		}
	}
	seen := make(map[string]struct{}, numSpawns)
	for _, agentID := range agentIDs {
		if agentID == "" {
			t.Fatal("a mass spawn returned an empty agent ID")
		}
		if _, dup := seen[agentID]; dup {
			t.Fatalf("duplicate agent ID %q in mass spawn", agentID)
		}
		seen[agentID] = struct{}{}
	}
	if int(successCount) != numSpawns {
		t.Fatalf("Expected %d successful runs, got %d. Some waiters timed out or scheduler dropped them.", numSpawns, successCount)
	}
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 || metrics.WaitingForSlot != 0 {
		t.Fatalf("scheduler leaked slots after mass spawn: %+v", metrics)
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
	const attempts = 100
	spawnErrs := make([]error, attempts)

	ctx := context.Background()

	for i := 0; i < attempts; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := spawner.Spawn(ctx, session.SpawnRequest{Name: "test_type", Task: "task"})
			spawnErrs[id] = err
		}(i)
	}

	wg.Wait()

	successes := 0
	for i, err := range spawnErrs {
		if err == nil {
			successes++
			continue
		}
		if !strings.Contains(err.Error(), "max active subagents reached") {
			t.Fatalf("spawn %d failed with a non-capacity error: %v", i, err)
		}
	}
	if successes != maxSpawns {
		t.Fatalf("State Corruption! Expected exactly %d successful spawns, but got %d", maxSpawns, successes)
	}
}

// TestE2E_SpawnerAPIScheduler_Recovery_DoubleRelease asserts that APIScheduler
// does not panic or grant hallucinated slots if a subagent bug causes double release.
func TestE2E_SpawnerAPIScheduler_Recovery_DoubleRelease(t *testing.T) {
	t.Parallel()

	scheduler := newTestScheduler(t, core.APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    time.Second,
	})

	ctx := context.Background()
	if err := acquire(scheduler, ctx, "clumsy_agent"); err != nil {
		t.Fatalf("clumsy_agent could not acquire: %v", err)
	}

	// Valid release
	scheduler.ReleaseAPISlot("clumsy_agent")

	// Invalid double release - should be handled gracefully (e.g. log error, but no panic)
	scheduler.ReleaseAPISlot("clumsy_agent")
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 {
		t.Fatalf("double release corrupted the counter: ActiveSlots=%d", metrics.ActiveSlots)
	}

	// Verify slot count isn't corrupted (should still just be 1 slot available)
	if err := acquire(scheduler, ctx, "test_agent_1"); err != nil {
		t.Fatalf("test_agent_1 could not acquire after double release: %v", err)
	}

	// This second acquire should timeout, proving we don't have 2 slots now
	ctx2, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := acquire(scheduler, ctx2, "test_agent_2"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Double release corrupted slot tracking, artificially increasing slot capacity! err=%v", err)
	}
	scheduler.ReleaseAPISlot("test_agent_1")
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 {
		t.Fatalf("scheduler leaked slots after double-release check: %+v", metrics)
	}
}

// TestE2E_SpawnerAPIScheduler_CascadingFailure_SchedulerStall ensures that when
// every slot is held, a waiter is cut loose by SlotAcquireTimeout well before
// its own (longer) context deadline, so a stalled scheduler aborts subagents
// promptly instead of pinning them for the whole turn.
func TestE2E_SpawnerAPIScheduler_CascadingFailure_SchedulerStall(t *testing.T) {
	t.Parallel()

	scheduler := newTestScheduler(t, core.APISchedulerConfig{
		MaxConcurrentAPICalls: 1,
		SlotAcquireTimeout:    50 * time.Millisecond,
	})
	if err := acquire(scheduler, context.Background(), "holder"); err != nil {
		t.Fatalf("holder could not take the only slot: %v", err)
	}
	defer scheduler.ReleaseAPISlot("holder")

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	err := acquire(scheduler, ctx, "stalled_agent")
	duration := time.Since(start)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Expected DeadlineExceeded acquiring a held slot, got: %v", err)
	}
	if duration > 500*time.Millisecond {
		t.Fatalf("Subagent stalled for %v waiting for slot; SlotAcquireTimeout (50ms) should have cut it loose.", duration)
	}
	sasWaitForQueueDrained(t, scheduler, 2*time.Second)
}

// TestE2E_SpawnerAPIScheduler_PriorityInversion_Prevention proves that a
// high-priority spawn jumps a crowded wait queue. Every agent registers at
// normal priority; only the urgent agent carries PriorityHigh in its context,
// so the grant order pins the context override rather than the registered
// default.
func TestE2E_SpawnerAPIScheduler_PriorityInversion_Prevention(t *testing.T) {
	t.Parallel()
	scheduler := newTestScheduler(t, core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 5 * time.Second})
	spawner := session.NewSpawner(nil, nil, nil, &mockCompiler{}, &sasMockConfigFactory{}, &sasMockTransducer{}, session.SpawnerConfig{
		MaxActiveSubagents: 5,
		TokenBudget:        1000,
	})

	lowIDs := make([]string, 3)
	for i := range lowIDs {
		agent, err := spawner.Spawn(context.Background(), session.SpawnRequest{
			Name:       fmt.Sprintf("low_prio_%d", i),
			Task:       "background work",
			Type:       session.SubAgentTypeEphemeral,
			IntentVerb: "/fix",
		})
		if err != nil {
			t.Fatalf("failed to spawn low-priority agent %d: %v", i, err)
		}
		lowIDs[i] = agent.GetID()
	}
	highAgent, err := spawner.Spawn(context.Background(), session.SpawnRequest{
		Name:       "high_prio_agent",
		Task:       "urgent work",
		Type:       session.SubAgentTypeEphemeral,
		IntentVerb: "/fix",
	})
	if err != nil {
		t.Fatalf("failed to spawn high-priority agent: %v", err)
	}

	if err := acquire(scheduler, context.Background(), "blocking_agent"); err != nil {
		t.Fatalf("blocking_agent could not take the only slot: %v", err)
	}

	lowAcquired := make(chan string, len(lowIDs))
	lowErrs := make(chan error, len(lowIDs))
	var wg sync.WaitGroup
	for _, agentID := range lowIDs {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := acquire(scheduler, ctx, id); err != nil {
				lowErrs <- err
				return
			}
			lowAcquired <- id
			scheduler.ReleaseAPISlot(id)
		}(agentID)
	}
	waitForSchedulerWaiters(t, scheduler, len(lowIDs))

	highAcquired := make(chan struct{})
	highErrs := make(chan error, 1)
	releaseHigh := make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		ctx = types.WithSpawnPriority(ctx, types.PriorityHigh)
		if err := acquire(scheduler, ctx, highAgent.GetID()); err != nil {
			highErrs <- err
			return
		}
		close(highAcquired)
		<-releaseHigh
		scheduler.ReleaseAPISlot(highAgent.GetID())
	}()
	waitForSchedulerWaiters(t, scheduler, len(lowIDs)+1)

	scheduler.ReleaseAPISlot("blocking_agent")

	select {
	case <-highAcquired:
	case err := <-highErrs:
		t.Fatalf("high-priority waiter failed: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("high-priority waiter never acquired the released slot")
	}
	select {
	case id := <-lowAcquired:
		t.Fatalf("low-priority waiter %s acquired ahead of a queued high-priority waiter", id)
	case <-time.After(100 * time.Millisecond):
		// Expected: the normal waiters stay queued while high holds the slot.
	}
	close(releaseHigh)

	timeout := time.After(5 * time.Second)
	for got := 0; got < len(lowIDs); {
		select {
		case <-lowAcquired:
			got++
		case err := <-lowErrs:
			t.Fatalf("low-priority waiter failed: %v", err)
		case <-timeout:
			t.Fatalf("only %d/%d low-priority waiters acquired after high released", got, len(lowIDs))
		}
	}
	wg.Wait()
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 || metrics.WaitingForSlot != 0 {
		t.Fatalf("scheduler leaked slots after priority check: %+v", metrics)
	}
}

// TestE2E_SpawnerAPIScheduler_ShutdownRaceCondition tests the race between
// Spawner.Shutdown cancelling contexts and the APIScheduler granting a slot.
func TestE2E_SpawnerAPIScheduler_ShutdownRaceCondition(t *testing.T) {
	t.Parallel()
	scheduler := newTestScheduler(t, core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 5 * time.Second})

	// Fill slot
	if err := acquire(scheduler, context.Background(), "holder"); err != nil {
		t.Fatalf("holder could not take the only slot: %v", err)
	}

	// Setup waiter
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)

	var acquireErr error
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		acquireErr = acquire(scheduler, waitCtx, "waiter")
		if acquireErr == nil {
			scheduler.ReleaseAPISlot("waiter")
		}
	}()
	waitForSchedulerWaiters(t, scheduler, 1)

	// RACE: Release the slot at the EXACT moment the Spawner cancels the context
	go waitCancel()
	scheduler.ReleaseAPISlot("holder")

	wg.Wait()

	// Either it acquired it successfully (and released it), OR it got context cancelled.
	// It must NOT leak the slot.
	if acquireErr != nil && !errors.Is(acquireErr, context.Canceled) && !errors.Is(acquireErr, context.DeadlineExceeded) {
		t.Fatalf("Unexpected error during race condition: %v", acquireErr)
	}

	// Verify slot is still available for a new caller
	verifyCtx, verifyCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer verifyCancel()

	if err := acquire(scheduler, verifyCtx, "verifier"); err != nil {
		t.Fatalf("Slot leaked during Shutdown Race! Waiter cancelled but slot was not returned to pool.")
	}
	scheduler.ReleaseAPISlot("verifier")
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 || metrics.WaitingForSlot != 0 {
		t.Fatalf("scheduler leaked slots during shutdown race: %+v", metrics)
	}
}

// TestE2E_SpawnerAPIScheduler_UnsetCeiling_UsesDefault: a zero
// MaxConcurrentAPICalls means "not configured", not "no calls". The scheduler
// takes its default ceiling rather than stalling every caller, which is what
// an absent core_limits.max_concurrent_api in config.json has to mean.
func TestE2E_SpawnerAPIScheduler_UnsetCeiling_UsesDefault(t *testing.T) {
	t.Parallel()
	scheduler := newTestScheduler(t, core.APISchedulerConfig{MaxConcurrentAPICalls: 0, SlotAcquireTimeout: 10 * time.Millisecond})

	want := core.DefaultAPISchedulerConfig().MaxConcurrentAPICalls
	if got := scheduler.EffectiveMaxSlots(); got != want {
		t.Fatalf("an unset ceiling produced %d slots, want default %d", got, want)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	if err := acquire(scheduler, ctx, "agent_zero"); err != nil {
		t.Fatalf("acquire under the default ceiling failed: %v", err)
	}
	scheduler.ReleaseAPISlot("agent_zero")
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 {
		t.Fatalf("scheduler leaked slots under the default ceiling: %+v", metrics)
	}
}

// TestE2E_SpawnerAPIScheduler_DynamicReconfiguration validates that
// changing max slots at runtime doesn't drop existing waiters.
func TestE2E_SpawnerAPIScheduler_DynamicReconfiguration(t *testing.T) {
	t.Parallel()
	scheduler := newTestScheduler(t, core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 5 * time.Second})

	if err := acquire(scheduler, context.Background(), "holder1"); err != nil {
		t.Fatalf("holder1 could not take the only slot: %v", err)
	}

	// Queue a waiter
	var wg sync.WaitGroup
	var waitErr error
	waitCtx, waitCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer waitCancel()

	wg.Add(1)
	go func() {
		defer wg.Done()
		waitErr = acquire(scheduler, waitCtx, "waiter1")
		if waitErr == nil {
			scheduler.ReleaseAPISlot("waiter1")
		}
	}()
	waitForSchedulerWaiters(t, scheduler, 1)

	// Dynamically update max calls to 2 (simulating recovery from rate limiting)
	scheduler.UpdateMaxConcurrentAPICalls(2)

	wg.Wait()

	if waitErr != nil {
		t.Fatalf("Waiter should have been granted a slot dynamically, but failed: %v", waitErr)
	}
	if got := scheduler.EffectiveMaxSlots(); got != 2 {
		t.Fatalf("dynamic reconfiguration left %d slots, want 2", got)
	}

	scheduler.ReleaseAPISlot("holder1")
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 || metrics.WaitingForSlot != 0 {
		t.Fatalf("scheduler leaked slots after dynamic reconfiguration: %+v", metrics)
	}
}

// TestE2E_SpawnerAPIScheduler_IdentityCollision pins the scheduler's actual
// identity contract: slots are anonymous counters, so a duplicate ID consumes
// a second free slot instead of hijacking the first grant, and releasing both
// grants returns the pool to a consistent empty state.
func TestE2E_SpawnerAPIScheduler_IdentityCollision(t *testing.T) {
	t.Parallel()
	scheduler := newTestScheduler(t, core.APISchedulerConfig{MaxConcurrentAPICalls: 2, SlotAcquireTimeout: 5 * time.Second})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if err := acquire(scheduler, ctx, "twin_agent"); err != nil {
		t.Fatalf("Failed first acquire: %v", err)
	}
	if err := acquire(scheduler, ctx, "twin_agent"); err != nil {
		t.Fatalf("duplicate ID unexpectedly failed while a second slot was free: %v", err)
	}
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 2 {
		t.Fatalf("duplicate ID holds %d slots, want 2", metrics.ActiveSlots)
	}

	scheduler.ReleaseAPISlot("twin_agent")
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 1 {
		t.Fatalf("one twin release left %d slots, want 1", metrics.ActiveSlots)
	}

	if err := acquire(scheduler, ctx, "safe_agent"); err != nil {
		t.Fatalf("Scheduler corrupted by identity collision: %v", err)
	}
	scheduler.ReleaseAPISlot("safe_agent")
	scheduler.ReleaseAPISlot("twin_agent")
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 || metrics.WaitingForSlot != 0 {
		t.Fatalf("scheduler leaked slots after identity collision: %+v", metrics)
	}
}

// TestE2E_SpawnerAPIScheduler_MultiTenant_Starvation tests that one greedy
// tenant cannot permanently lock out a secondary tenant: the waiter times out
// while the slot is held, then succeeds once the greedy holder releases.
func TestE2E_SpawnerAPIScheduler_MultiTenant_Starvation(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping multi-tenant starvation test in short mode")
	}
	t.Parallel()
	scheduler := newTestScheduler(t, core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 5 * time.Second})

	if err := acquire(scheduler, context.Background(), "greedy_tenant_agent_1"); err != nil {
		t.Fatalf("greedy tenant could not take the only slot: %v", err)
	}

	ctx2, cancel2 := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel2()
	if err := acquire(scheduler, ctx2, "starved_tenant_agent_1"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Expected DeadlineExceeded for starved tenant, got: %v", err)
	}
	sasWaitForQueueDrained(t, scheduler, 2*time.Second)

	scheduler.ReleaseAPISlot("greedy_tenant_agent_1")
	retryCtx, retryCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer retryCancel()
	if err := acquire(scheduler, retryCtx, "starved_tenant_agent_1"); err != nil {
		t.Fatalf("starved tenant could not acquire after greedy release: %v", err)
	}
	scheduler.ReleaseAPISlot("starved_tenant_agent_1")
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 || metrics.WaitingForSlot != 0 {
		t.Fatalf("scheduler leaked slots after multi-tenant check: %+v", metrics)
	}
}

// TestE2E_SpawnerAPIScheduler_RateLimit_CapacityPlunge tests the behavior
// when ReportRateLimit drastically reduces capacity while slots are active.
func TestE2E_SpawnerAPIScheduler_RateLimit_CapacityPlunge(t *testing.T) {
	t.Parallel()
	scheduler := newTestScheduler(t, core.APISchedulerConfig{
		MaxConcurrentAPICalls: 5,
		AdaptiveConcurrency:   true,
	})

	// Fill 3 slots
	for i := 0; i < 3; i++ {
		if err := acquire(scheduler, context.Background(), fmt.Sprintf("holder_%d", i)); err != nil {
			t.Fatalf("holder_%d could not acquire: %v", i, err)
		}
	}

	// Trigger rate limit penalty heavily, dropping max slots below active slots
	for i := 0; i < 10; i++ {
		scheduler.ReportRateLimit()
	}

	metrics := scheduler.GetMetrics()
	if metrics.MaxSlots != 1 || metrics.ActiveSlots != 3 {
		t.Fatalf("capacity plunge left max=%d active=%d, want max=1 active=3", metrics.MaxSlots, metrics.ActiveSlots)
	}

	// Now try to queue a new one. It should block because Active > Max
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := acquire(scheduler, ctx, "new_waiter"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Expected waiter to be blocked by reduced capacity, got err: %v", err)
	}
	sasWaitForQueueDrained(t, scheduler, 2*time.Second)

	// Release the active slots. The scheduler shouldn't panic about Active > Max.
	for i := 0; i < 3; i++ {
		scheduler.ReleaseAPISlot(fmt.Sprintf("holder_%d", i))
	}
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 || metrics.WaitingForSlot != 0 {
		t.Fatalf("scheduler leaked slots after capacity plunge: %+v", metrics)
	}
}

// TestE2E_SpawnerAPIScheduler_ReportSuccess_Recovery tests that after
// a rate limit penalty, successful API calls gradually restore capacity.
// Recovery requires a quiet window after the last rate limit, so the test
// configures a short window instead of the 30s production default.
func TestE2E_SpawnerAPIScheduler_ReportSuccess_Recovery(t *testing.T) {
	t.Parallel()
	scheduler := newTestScheduler(t, core.APISchedulerConfig{
		MaxConcurrentAPICalls: 5,
		AdaptiveConcurrency:   true,
		AdaptiveRecoverAfter:  5 * time.Millisecond,
	})

	// Penalize
	for i := 0; i < 10; i++ {
		scheduler.ReportRateLimit()
	}
	if metrics := scheduler.GetMetrics(); metrics.MaxSlots != 1 {
		t.Fatalf("penalized capacity is %d, want the floor of 1", metrics.MaxSlots)
	}
	if base := scheduler.BaseMaxSlots(); base != 5 {
		t.Fatalf("adaptive plunge forgot the configured base: base=%d", base)
	}

	// Reward: one quiet success restores one slot, capped at the base ceiling.
	for want := 2; want <= 5; want++ {
		time.Sleep(10 * time.Millisecond)
		scheduler.ReportSuccess()
		if got := scheduler.GetMetrics().MaxSlots; got != want {
			t.Fatalf("recovered capacity is %d, want %d", got, want)
		}
	}
	scheduler.ReportSuccess()
	if got := scheduler.GetMetrics().MaxSlots; got != 5 {
		t.Fatalf("recovery overshot the base ceiling: %d", got)
	}
}

// TestE2E_SpawnerAPIScheduler_Piggyback_Reentrance_Deadlock tests the scenario
// where an agent holds a slot, but needs another slot to fulfill a recursive task.
func TestE2E_SpawnerAPIScheduler_Piggyback_Reentrance_Deadlock(t *testing.T) {
	t.Parallel()
	scheduler := newTestScheduler(t, core.APISchedulerConfig{MaxConcurrentAPICalls: 1, SlotAcquireTimeout: 50 * time.Millisecond})

	ctx1, cancel1 := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel1()

	// SubAgent acquires primary slot
	err1 := acquire(scheduler, ctx1, "parent_agent")
	if err1 != nil {
		t.Fatalf("Failed primary acquire: %v", err1)
	}

	// Piggyback triggers a sub-task requiring another slot
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()

	err2 := acquire(scheduler, ctx2, "child_agent_of_parent")

	// In a 1-slot system, this MUST deadlock/timeout, proving that recursive
	// calls need reserved capacity or priority overrides.
	if !errors.Is(err2, context.DeadlineExceeded) {
		t.Fatalf("Expected recursive call to deadlock/timeout, got: %v", err2)
	}
	sasWaitForQueueDrained(t, scheduler, 2*time.Second)

	scheduler.ReleaseAPISlot("parent_agent")
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 || metrics.WaitingForSlot != 0 {
		t.Fatalf("scheduler leaked slots after reentrance check: %+v", metrics)
	}
}

// TestE2E_SpawnerAPIScheduler_OODALoop_LatencyBudget verifies that the scheduler
// does not artificially inflate wait times beyond the mathematical queue time.
func TestE2E_SpawnerAPIScheduler_OODALoop_LatencyBudget(t *testing.T) {
	t.Parallel()
	scheduler := newTestScheduler(t, core.APISchedulerConfig{MaxConcurrentAPICalls: 2, SlotAcquireTimeout: 5 * time.Second})

	// Fill slots with deterministic 50ms holds
	holdErrs := make(chan error, 2)
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			holder := fmt.Sprintf("hold_%d", id)
			if err := acquire(scheduler, context.Background(), holder); err != nil {
				holdErrs <- err
				return
			}
			time.Sleep(50 * time.Millisecond)
			scheduler.ReleaseAPISlot(holder)
		}(i)
	}
	sasWaitForActiveSlots(t, scheduler, 2, 2*time.Second)

	// The third waiter should get it in ~50ms
	start := time.Now()
	err := acquire(scheduler, context.Background(), "waiter")
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Failed to acquire: %v", err)
	}
	scheduler.ReleaseAPISlot("waiter")

	if duration > 200*time.Millisecond {
		t.Fatalf("Latency budget exceeded! Expected ~50ms, got %v", duration)
	}

	wg.Wait()
	select {
	case err := <-holdErrs:
		t.Fatalf("slot holder failed: %v", err)
	default:
	}
	if metrics := scheduler.GetMetrics(); metrics.ActiveSlots != 0 || metrics.WaitingForSlot != 0 {
		t.Fatalf("scheduler leaked slots after latency check: %+v", metrics)
	}
}

// ResolveAllowedTools projects the same fixture envelope before JIT selection.
func (m *sasMockConfigFactory) ResolveAllowedTools(ctx context.Context, intents ...string) ([]string, error) {
	resolved, err := m.Generate(ctx, &prompt.CompilationResult{}, intents...)
	if err != nil || resolved == nil {
		return nil, err
	}
	return append([]string(nil), resolved.AllowedTools...), nil
}

// newTestScheduler gives one test its own scheduler. These tests used to
// share the process-global scheduler under t.Parallel, each reconfiguring it
// before the sync.Once that builds it had run, so only the first test's
// configuration ever applied and a 0-slot test could starve a 2-slot test's
// waiters.
func newTestScheduler(t *testing.T, cfg core.APISchedulerConfig) *core.APIScheduler {
	t.Helper()
	s := core.NewAPIScheduler(cfg)
	t.Cleanup(s.Stop)
	return s
}

// acquire registers id on first use, the way NewScheduledLLMCall registers a
// real client's shard, then acquires. The scheduler refuses an unregistered
// id rather than inventing state for it.
func acquire(s *core.APIScheduler, ctx context.Context, id string) error {
	if _, ok := s.GetShardState(id); !ok {
		s.RegisterShard(id, "e2e")
	}
	return s.AcquireAPISlot(ctx, id)
}

// sasWaitForQueueDrained polls until no waiter remains queued (or fails).
// Cancellation cleanup is synchronous on the current path, but polling keeps
// the assertion honest if that ever becomes asynchronous.
func sasWaitForQueueDrained(t *testing.T, scheduler *core.APIScheduler, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if got := scheduler.GetMetrics().WaitingForSlot; got == 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for the scheduler queue to drain")
}

// sasWaitForActiveSlots polls until want slots are held (or fails). Queue
// arrival is asynchronous; polling beats a blind sleep and keeps the
// latency-budget precondition deterministic instead of racy.
func sasWaitForActiveSlots(t *testing.T, scheduler *core.APIScheduler, want int, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if got := scheduler.GetMetrics().ActiveSlots; got >= want {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d active slots", want)
}
