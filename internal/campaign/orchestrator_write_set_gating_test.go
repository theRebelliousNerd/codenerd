package campaign

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

type safeKernel struct {
	mu    sync.RWMutex
	facts []core.Fact
}

func (k *safeKernel) LoadFacts(facts []core.Fact) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.facts = append(k.facts, facts...)
	return nil
}

func (k *safeKernel) Query(predicate string) ([]core.Fact, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make([]core.Fact, 0)
	for _, f := range k.facts {
		if f.Predicate == predicate {
			out = append(out, f)
		}
	}
	return out, nil
}

func (k *safeKernel) QueryAll() (map[string][]core.Fact, error) {
	k.mu.RLock()
	defer k.mu.RUnlock()
	out := make(map[string][]core.Fact)
	for _, f := range k.facts {
		out[f.Predicate] = append(out[f.Predicate], f)
	}
	return out, nil
}

func (k *safeKernel) Assert(fact core.Fact) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.facts = append(k.facts, fact)
	return nil
}

func (k *safeKernel) AssertBatch(facts []core.Fact) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.facts = append(k.facts, facts...)
	return nil
}

func (k *safeKernel) Retract(predicate string) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	filtered := k.facts[:0]
	for _, f := range k.facts {
		if f.Predicate != predicate {
			filtered = append(filtered, f)
		}
	}
	k.facts = filtered
	return nil
}

func (k *safeKernel) RetractFact(fact core.Fact) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	filtered := k.facts[:0]
	for _, f := range k.facts {
		if f.Predicate != fact.Predicate {
			filtered = append(filtered, f)
			continue
		}
		if len(fact.Args) == 0 || fact.Args == nil {
			continue
		}
		if !matchFactArgsPrefix(f.Args, fact.Args) {
			filtered = append(filtered, f)
		}
	}
	k.facts = filtered
	return nil
}

func (k *safeKernel) UpdateSystemFacts() error { return nil }
func (k *safeKernel) Reset() {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.facts = nil
}
func (k *safeKernel) AppendPolicy(policy string) {}
func (k *safeKernel) RetractExactFactsBatch(facts []core.Fact) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	for _, target := range facts {
		filtered := k.facts[:0]
		for _, f := range k.facts {
			if f.Predicate == target.Predicate && reflect.DeepEqual(f.Args, target.Args) {
				continue
			}
			filtered = append(filtered, f)
		}
		k.facts = filtered
	}
	return nil
}

func (k *safeKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	k.mu.Lock()
	defer k.mu.Unlock()
	filtered := k.facts[:0]
	for _, f := range k.facts {
		if _, drop := predicates[f.Predicate]; !drop {
			filtered = append(filtered, f)
		}
	}
	k.facts = filtered
	return nil
}

func matchFactArgsPrefix(actual []interface{}, prefix []interface{}) bool {
	if len(prefix) > len(actual) {
		return false
	}
	for i := range prefix {
		if !reflect.DeepEqual(actual[i], prefix[i]) {
			return false
		}
	}
	return true
}

type noopLLM struct{}

func (n *noopLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return "ok", nil
}

func (n *noopLLM) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "ok", nil
}

func (n *noopLLM) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "ok", StopReason: "end_turn"}, nil
}

type gatingTaskExecutor struct {
	execute func(ctx context.Context, intent string, task string) (string, error)
}

func (g *gatingTaskExecutor) Execute(ctx context.Context, intent string, task string) (string, error) {
	if g.execute != nil {
		return g.execute(ctx, intent, task)
	}
	return "", nil
}

func (g *gatingTaskExecutor) ExecuteWithContext(ctx context.Context, intent string, task string, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error) {
	return g.Execute(ctx, intent, task)
}

func (g *gatingTaskExecutor) ExecuteAsync(ctx context.Context, intent string, task string) (string, error) {
	return g.Execute(ctx, intent, task)
}

func (g *gatingTaskExecutor) GetResult(taskID string) (string, bool, error) {
	return "", false, nil
}

func (g *gatingTaskExecutor) WaitForResult(ctx context.Context, taskID string) (string, error) {
	return "", nil
}

func TestRunPhase_WriteSetGatesConflictingMutations(t *testing.T) {
	mockKernel := &safeKernel{}
	_ = mockKernel.Assert(core.Fact{Predicate: "eligible_task", Args: []interface{}{"/task_1"}})
	_ = mockKernel.Assert(core.Fact{Predicate: "eligible_task", Args: []interface{}{"/task_2"}})

	workspace := t.TempDir()
	conflictPath := filepath.Join(workspace, "internal", "conflict.go")
	if err := os.MkdirAll(filepath.Dir(conflictPath), 0o755); err != nil {
		t.Fatalf("failed to create write_set directory: %v", err)
	}
	if err := os.WriteFile(conflictPath, []byte("package internal\n"), 0o644); err != nil {
		t.Fatalf("failed to seed write_set file: %v", err)
	}

	firstStarted := make(chan struct{}, 1)
	releaseFirst := make(chan struct{})
	var firstStartOnce sync.Once

	var mu sync.Mutex
	started := make([]string, 0, 2)
	executor := &gatingTaskExecutor{
		execute: func(ctx context.Context, intent string, task string) (string, error) {
			mu.Lock()
			started = append(started, task)
			mu.Unlock()

			if strings.Contains(task, "task-one") {
				firstStartOnce.Do(func() { close(firstStarted) })
				select {
				case <-ctx.Done():
					return "", ctx.Err()
				case <-releaseFirst:
				}
			}
			return "ok", nil
		},
	}

	orch := NewOrchestrator(OrchestratorConfig{
		Workspace:           workspace,
		Kernel:              mockKernel,
		LLMClient:           &noopLLM{},
		TaskExecutor:        executor,
		MaxParallelTasks:    2,
		DisableTimeouts:     true,
		WriteSetLockTimeout: 40 * time.Millisecond,
		WriteSetLockRetry:   20 * time.Millisecond,
		WriteSetLockPoll:    5 * time.Millisecond,
	})

	orch.campaign = &Campaign{
		ID:          "/campaign_locking",
		Type:        CampaignTypeCustom,
		Title:       "Locking",
		Goal:        "serialize conflicting writes",
		Status:      StatusActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		TotalPhases: 1,
		TotalTasks:  2,
		Phases: []Phase{{
			ID:     "/phase_1",
			Name:   "Phase 1",
			Order:  0,
			Status: PhaseInProgress,
			Tasks: []Task{
				{
					ID:          "/task_1",
					PhaseID:     "/phase_1",
					Description: "task-one",
					Status:      TaskPending,
					Type:        TaskTypeFileModify,
					Priority:    PriorityNormal,
					Shard:       "coder",
					WriteSet:    []string{"internal/conflict.go"},
				},
				{
					ID:          "/task_2",
					PhaseID:     "/phase_1",
					Description: "task-two",
					Status:      TaskPending,
					Type:        TaskTypeFileModify,
					Priority:    PriorityNormal,
					Shard:       "coder",
					WriteSet:    []string{"internal/conflict.go"},
				},
			},
		}},
	}

	done := make(chan error, 1)
	go func() {
		done <- orch.runPhase(context.Background(), &orch.campaign.Phases[0])
	}()

	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first task did not start")
	}

	time.Sleep(120 * time.Millisecond)
	mu.Lock()
	startedBeforeRelease := len(started)
	mu.Unlock()
	if startedBeforeRelease != 1 {
		t.Fatalf("expected only one task start before releasing lock, got %d starts: %v", startedBeforeRelease, started)
	}

	close(releaseFirst)

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("runPhase failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("runPhase timed out")
	}

	mu.Lock()
	totalStarts := len(started)
	mu.Unlock()
	if totalStarts != 2 {
		t.Fatalf("expected both tasks to run sequentially, got %d starts: %v", totalStarts, started)
	}
}

func TestComputeWriteSetLockRetryDelay_BoundedAndContextAware(t *testing.T) {
	orch := &Orchestrator{
		config: OrchestratorConfig{
			WriteSetLockRetry: 10 * time.Second,
		},
	}

	delay := orch.computeWriteSetLockRetryDelay(context.Background(), 35*time.Millisecond)
	if delay != 35*time.Millisecond {
		t.Fatalf("expected retry delay to clamp to lock timeout, got %v", delay)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
	defer cancel()
	delay = orch.computeWriteSetLockRetryDelay(ctx, 5*time.Second)
	if delay <= 0 || delay > 25*time.Millisecond {
		t.Fatalf("expected retry delay to honor context deadline, got %v", delay)
	}

	canceledCtx, canceled := context.WithCancel(context.Background())
	canceled()
	delay = orch.computeWriteSetLockRetryDelay(canceledCtx, time.Second)
	if delay != 0 {
		t.Fatalf("expected canceled context to yield zero retry delay, got %v", delay)
	}
}
