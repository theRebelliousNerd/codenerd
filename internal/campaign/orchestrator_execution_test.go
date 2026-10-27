package campaign

import (
	"codenerd/internal/core"
	"codenerd/internal/session"
	"codenerd/internal/types"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

type rejectedCampaignResetKernel struct {
	core.Kernel
	cause error
}

func (k *rejectedCampaignResetKernel) Transaction() types.KernelTransaction {
	return rejectedCampaignResetTransaction{cause: k.cause}
}

type rejectedCampaignResetTransaction struct{ cause error }

func (rejectedCampaignResetTransaction) Retract(string)                          {}
func (rejectedCampaignResetTransaction) RetractFact(types.Fact)                  {}
func (rejectedCampaignResetTransaction) RetractExactFact(types.Fact)             {}
func (rejectedCampaignResetTransaction) RetractPredicateSet(map[string]struct{}) {}
func (rejectedCampaignResetTransaction) Assert(types.Fact)                       {}
func (tx rejectedCampaignResetTransaction) Commit() error                        { return tx.cause }

// TODO: Null/Undefined/Empty: What happens if o.campaign is initialized but has an empty ID or title?
// TODO: Null/Undefined/Empty: What if o.config is partially initialized (e.g., CampaignTimeout == 0 vs negative timeout)?
// TODO: Null/Undefined/Empty: What if o.pauseCh is nil in isPaused state?
// TODO: Null/Undefined/Empty: What if o.contextPager is uninitialized (nil) and the campaign transitions to a new phase?
// TODO: Null/Undefined/Empty: What happens when the active phase has zero tasks? Does the orchestrator immediately complete the phase, or does it hang waiting for a task that will never be scheduled?
// TODO: Null/Undefined/Empty: What happens if o.kernel is nil during runHeartbeatLoop?
// TODO: Type Coercion: Block Reason Coercion: The getCampaignBlockReason() relies on Mangle facts. If the block reason contains non-printable characters or enormous strings, does Go logging fail or truncate?
// TODO: Type Coercion: Heartbeat Timestamp Coercion: The runHeartbeatLoop pushes time.Now().Unix() to Mangle. Does the Mangle schema for campaign_heartbeat enforce int64? Can float precision errors happen?
// TODO: Type Coercion/Invalid State: What happens if Mangle returns a currentPhase that isn't actually in o.campaign.Phases?
// TODO: Type Coercion: Fact Type Mismatch. A fact is asserted as an Atom but retrieved as a String. Mangle treats these as distinct types, which will cause join failures. Verify type strictness.
// TODO: User request Extremes: A campaign with 1,000,000 phases or tasks. Context cancellations during massive campaigns.
// TODO: User Request Extremes: High-frequency context switching: The context pager is invoked on phase transitions. If phases complete instantly (empty phases), does the rapid switching overwhelm the LLM client or memory?
// TODO: User Request Extremes: Massive file path deduplication: If a phase has millions of artifact files, does the orchestrator deduplicate them efficiently without O(N^2) latency spikes?
// TODO: User Request Extremes: Extremely long phase names: Can the logging subsystem gracefully handle phase names that are 10MB strings?
// TODO: User Request Extremes: Deeply nested directories: If the workspace config points to a repository with extremely deep nesting, will the orchestrator's file monitoring crash with path too long errors?
// TODO: State Conflicts (Race Conditions): ctx.Done() during Run immediately triggers o.updateCampaignStatus(StatusPaused) and o.saveCampaign(). What if a separate goroutine is pausing or canceling the orchestrator at the exact same moment runHeartbeatLoop tries to autosave?
// TODO: State Conflicts: What if o.campaign gets set to nil concurrently? o.mu.RLock() protects o.campaign.ID, but then RetractFact/Assert are called outside the lock using campaignID. Is it possible RetractFact runs *after* a new campaign is loaded?
// TODO: State Conflicts: State Desynchronization: If the Mangle kernel's transaction commit fails, but the orchestrator's internal Go state has already advanced, how is the mismatch reconciled?
// TODO: State Conflicts: Parallel Phase Execution: If multiple phases are forced active simultaneously via corrupted logic, does runPhase cause race conditions on shared orchestrator maps?
// TODO: Execution Profiles: The Infinite Loop State. The Mangle program fails to stratify or enters an infinite loop. The orchestrator's kernel.Query calls must have explicit timeouts to prevent freezing.
// TODO: Execution Profiles: The Silent Failure State. Mangle returns zero facts when it should return exactly one (no active phases, but campaign not complete). The orchestrator must gracefully fail or auto-heal.
// TODO: User Request Extremes: Performance of Autosaving. During a 1,000,000 task campaign, o.saveCampaign() holds the orchestrator lock while serializing JSON to disk. This synchronous I/O blocks the Run loop and APIs, causing significant performance degradation and timeouts.
// TODO: State Conflicts: Concurrency with runHeartbeatLoop. If context is cancelled, Run() saves campaign while holding lock. If runHeartbeatLoop also triggers autosave concurrently, we need to ensure the writes are atomic to prevent campaign file corruption.
// TODO: State Conflicts: State Desynchronization in runHeartbeatLoop. If Mangle transaction commit fails due to SQLite lock, the heartbeat isn't recorded but the process continues.

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify Orchestrator.Run and runHeartbeatLoop safely handle an initialized campaign with an empty ID ("") without polluting the root filesystem during autosave.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify runHeartbeatLoop safely handles o.config.HeartbeatEvery or AutosaveEvery being 0 or negative without panicking time.NewTicker.
// TODO: TEST_GAP: [Type Coercion] Verify campaign_heartbeat assertions use correct Atom vs String types matching the Mangle schema to prevent silent join failures.
// TODO: TEST_GAP: [Type Coercion] Verify time.Now().Unix() int64 values do not overflow or lose precision when asserted into Mangle's type system.
// TODO: TEST_GAP: [User Request Extremes] Verify performance of o.saveCampaign() during autosave with 1,000,000 tasks to ensure it does not hold o.mu.Lock() long enough to block the run loop or API queries.
// TODO: TEST_GAP: [User Request Extremes] Verify context pager and memory stability when rapidly cycling through 100,000 empty phases.
// TODO: TEST_GAP: [State Conflicts] Verify that concurrent ctx.Done() and autosaveTicker firing do not corrupt the campaign.json file if saveCampaign lacks atomic rename guarantees.
// TODO: TEST_GAP: [State Conflicts] Verify state desynchronization handling when tx.Commit() in runHeartbeatLoop fails (e.g. SQLite database is locked), as the error is currently ignored.

func TestFinalizeCancellationPersistsPausedCampaign(t *testing.T) {
	nerdDir := filepath.Join(t.TempDir(), ".nerd")
	o := &Orchestrator{
		nerdDir: nerdDir,
		kernel:  &MockKernel{},
		campaign: &Campaign{
			ID:     "campaign_timeout",
			Title:  "timeout persistence",
			Status: StatusActive,
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o.finalizeCancellation(ctx)

	path := filepath.Join(nerdDir, "campaigns", "campaign_timeout.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read persisted campaign: %v", err)
	}
	var persisted Campaign
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode persisted campaign: %v", err)
	}
	if persisted.Status != StatusPaused {
		t.Fatalf("persisted status = %s, want %s", persisted.Status, StatusPaused)
	}
}

func TestFinalizeCancellationDoesNotPauseWithoutCancellation(t *testing.T) {
	o := &Orchestrator{
		nerdDir: filepath.Join(t.TempDir(), ".nerd"),
		kernel:  &MockKernel{},
		campaign: &Campaign{
			ID:     "campaign_active",
			Status: StatusActive,
		},
	}

	o.finalizeCancellation(context.Background())
	if o.campaign.Status != StatusActive {
		t.Fatalf("status = %s, want active for ordinary return", o.campaign.Status)
	}
	if _, err := os.Stat(filepath.Join(o.nerdDir, "campaigns", "campaign_active.json")); !os.IsNotExist(err) {
		t.Fatalf("ordinary return unexpectedly persisted cancellation state: %v", err)
	}
}

func TestResetInProgress_KernelRejectionDoesNotDivergeLiveCampaign(t *testing.T) {
	commitErr := errors.New("kernel reset transaction rejected")
	kernel := &MockKernel{AssertErr: commitErr}
	kernel.Facts = []core.Fact{
		{Predicate: "campaign_phase", Args: []any{"phase_live", "campaign_reset_failure", "Live phase", 0, string(PhaseInProgress), ""}},
		{Predicate: "campaign_task", Args: []any{"task_live", "phase_live", "work", string(TaskInProgress), string(TaskTypeResearch)}},
	}
	o := &Orchestrator{
		kernel:  kernel,
		nerdDir: filepath.Join(t.TempDir(), ".nerd"),
		campaign: &Campaign{
			ID:     "campaign_reset_failure",
			Status: StatusActive,
			Phases: []Phase{{
				ID:     "phase_live",
				Status: PhaseInProgress,
				Tasks: []Task{{
					ID:      "task_live",
					PhaseID: "phase_live",
					Status:  TaskInProgress,
					Type:    TaskTypeResearch,
				}},
			}},
		},
	}

	// A restart reset is a single kernel transaction. If it is rejected, the
	// orchestrator must not publish its proposed /pending state to memory or the
	// snapshot while the executive still holds /in_progress.
	o.mu.Lock()
	if err := o.saveCampaign(); err != nil {
		o.mu.Unlock()
		t.Fatalf("save pre-reset snapshot: %v", err)
	}
	err := o.resetInProgress()
	o.mu.Unlock()
	if !errors.Is(err, commitErr) {
		t.Fatalf("reset error = %v, want kernel cause %v", err, commitErr)
	}

	if got := o.campaign.Phases[0].Tasks[0].Status; got != TaskInProgress {
		t.Fatalf("task status after rejected kernel reset = %s, want unchanged %s", got, TaskInProgress)
	}
	if got := o.campaign.Phases[0].Status; got != PhaseInProgress {
		t.Fatalf("phase status after rejected kernel reset = %s, want unchanged %s", got, PhaseInProgress)
	}
	assertCampaignStateFact(t, kernel, "campaign_phase", "phase_live", 4, string(PhaseInProgress))
	tasks, err := kernel.Query("campaign_task")
	if err != nil || len(tasks) != 1 {
		t.Fatalf("query kernel task after rejected reset: rows=%v err=%v", tasks, err)
	}
	if got := tasks[0].Args[3]; got != string(TaskInProgress) {
		t.Fatalf("kernel task status = %v, want unchanged %s", got, TaskInProgress)
	}
	data, err := os.ReadFile(filepath.Join(o.nerdDir, "campaigns", "campaign_reset_failure.json"))
	if err != nil {
		t.Fatalf("read persisted campaign: %v", err)
	}
	var persisted Campaign
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode persisted campaign: %v", err)
	}
	if got := persisted.Phases[0].Tasks[0].Status; got != TaskInProgress {
		t.Fatalf("persisted task status after rejected kernel reset = %s, want unchanged %s", got, TaskInProgress)
	}
	if got := persisted.Phases[0].Status; got != PhaseInProgress {
		t.Fatalf("persisted phase status after rejected kernel reset = %s, want unchanged %s", got, PhaseInProgress)
	}
}

func TestRun_RestartResetRejectionStopsBeforeTaskEffects(t *testing.T) {
	orch, kernel := newPolicyFailureOrchestrator(t, nil, Task{
		ID: "/task_restart_reject", Description: "recover interrupted work", Type: TaskTypeResearch,
	})
	markCampaignTaskInProgress(t, orch, kernel, "/task_restart_reject")
	orch.mu.Lock()
	if err := orch.saveCampaign(); err != nil {
		orch.mu.Unlock()
		t.Fatalf("save pre-run snapshot: %v", err)
	}
	orch.mu.Unlock()

	commitErr := errors.New("restart transaction refused")
	orch.kernel = &rejectedCampaignResetKernel{Kernel: kernel, cause: commitErr}
	var executions atomic.Int32
	orch.taskExecutor = &MockTaskExecutor{ExecuteFunc: func(context.Context, session.TaskRequest) (string, error) {
		executions.Add(1)
		return "recovered findings", nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	err := orch.Run(ctx)
	if !errors.Is(err, commitErr) {
		t.Fatalf("Run error = %v, want reset commit cause %v", err, commitErr)
	}
	if got := executions.Load(); got != 0 {
		t.Fatalf("executor calls after rejected reset = %d, want 0", got)
	}
	if got := orch.campaign.Phases[0].Status; got != PhaseInProgress {
		t.Fatalf("live phase status after rejected reset = %s, want unchanged %s", got, PhaseInProgress)
	}
	if got := orch.campaign.Phases[0].Tasks[0].Status; got != TaskInProgress {
		t.Fatalf("live task status after rejected reset = %s, want unchanged %s", got, TaskInProgress)
	}
	assertCampaignStateFact(t, kernel, "campaign_phase", "/phase_failure_policy", 4, string(PhaseInProgress))
	assertCampaignStateFact(t, kernel, "campaign_task", "/task_restart_reject", 3, string(TaskInProgress))
	assertPersistedCampaignStatus(t, orch, PhaseInProgress, TaskInProgress)
}

func TestRun_SuccessfulRestartResetSchedulesInterruptedTask(t *testing.T) {
	orch, kernel := newPolicyFailureOrchestrator(t, nil, Task{
		ID: "/task_restart_success", Description: "recover interrupted work", Type: TaskTypeResearch,
	})
	markCampaignTaskInProgress(t, orch, kernel, "/task_restart_success")
	orch.mu.Lock()
	if err := orch.saveCampaign(); err != nil {
		orch.mu.Unlock()
		t.Fatalf("save interrupted campaign before restart: %v", err)
	}
	campaignID := orch.campaign.ID
	orch.mu.Unlock()
	if err := orch.LoadCampaign(campaignID); err != nil {
		t.Fatalf("reload interrupted campaign before restart: %v", err)
	}
	var executions atomic.Int32
	orch.taskExecutor = &MockTaskExecutor{ExecuteFunc: func(context.Context, session.TaskRequest) (string, error) {
		executions.Add(1)
		return "recovered findings", nil
	}}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := orch.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := executions.Load(); got != 1 {
		t.Fatalf("executor calls after successful reset = %d, want 1", got)
	}
	if got := orch.campaign.Status; got != StatusCompleted {
		t.Fatalf("campaign status = %s, want %s", got, StatusCompleted)
	}
	if got := orch.campaign.Phases[0].Status; got != PhaseCompleted {
		t.Fatalf("phase status = %s, want %s", got, PhaseCompleted)
	}
	if got := orch.campaign.Phases[0].Tasks[0].Status; got != TaskCompleted {
		t.Fatalf("task status = %s, want %s", got, TaskCompleted)
	}
	assertCampaignStateFact(t, kernel, "campaign_phase", "/phase_failure_policy", 4, string(PhaseCompleted))
	assertCampaignStateFact(t, kernel, "campaign_task", "/task_restart_success", 3, string(TaskCompleted))
	assertPersistedCampaignStatus(t, orch, PhaseCompleted, TaskCompleted)
}

func markCampaignTaskInProgress(t *testing.T, orch *Orchestrator, kernel core.Kernel, taskID string) {
	t.Helper()
	orch.mu.Lock()
	var fact core.Fact
	for pi := range orch.campaign.Phases {
		phase := &orch.campaign.Phases[pi]
		for ti := range phase.Tasks {
			task := &phase.Tasks[ti]
			if task.ID != taskID {
				continue
			}
			task.Status = TaskInProgress
			fact = core.Fact{Predicate: "campaign_task", Args: []any{task.ID, task.PhaseID, task.Description, string(TaskInProgress), string(task.Type)}}
		}
	}
	orch.mu.Unlock()
	if len(fact.Args) == 0 {
		t.Fatalf("task %s not found in campaign", taskID)
	}
	if err := kernel.RetractFact(core.Fact{Predicate: "campaign_task", Args: []any{taskID}}); err != nil {
		t.Fatalf("retract pending task fact: %v", err)
	}
	if err := kernel.Assert(fact); err != nil {
		t.Fatalf("assert in-progress task fact: %v", err)
	}
}

func assertCampaignStateFact(t *testing.T, kernel core.Kernel, predicate, id string, statusArg int, want string) {
	t.Helper()
	facts, err := kernel.Query(predicate)
	if err != nil {
		t.Fatalf("query %s: %v", predicate, err)
	}
	for _, fact := range facts {
		if len(fact.Args) > statusArg && types.ExtractString(fact.Args[0]) == id {
			if got := types.ExtractString(fact.Args[statusArg]); got != want {
				t.Fatalf("%s(%s) status = %s, want %s", predicate, id, got, want)
			}
			return
		}
	}
	t.Fatalf("%s(%s) fact not found in kernel: %v", predicate, id, facts)
}

func assertPersistedCampaignStatus(t *testing.T, orch *Orchestrator, wantPhase PhaseStatus, wantTask TaskStatus) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(orch.nerdDir, "campaigns", orch.campaign.ID+".json"))
	if err != nil {
		t.Fatalf("read persisted campaign: %v", err)
	}
	var persisted Campaign
	if err := json.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode persisted campaign: %v", err)
	}
	if got := persisted.Phases[0].Status; got != wantPhase {
		t.Fatalf("persisted phase status = %s, want %s", got, wantPhase)
	}
	if got := persisted.Phases[0].Tasks[0].Status; got != wantTask {
		t.Fatalf("persisted task status = %s, want %s", got, wantTask)
	}
}
