package chat

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// recordingSystemShard is a registered system implementation that records
// the task it was given.
type recordingSystemShard struct {
	id    string
	cfg   types.ShardConfig
	calls *atomic.Int64
}

func (s *recordingSystemShard) Execute(_ context.Context, task string) (string, error) {
	s.calls.Add(1)
	return "ratified: " + task, nil
}
func (s *recordingSystemShard) GetID() string                           { return s.id }
func (s *recordingSystemShard) GetState() types.ShardState              { return types.ShardStateCompleted }
func (s *recordingSystemShard) GetConfig() types.ShardConfig            { return s.cfg }
func (s *recordingSystemShard) Stop() error                             { return nil }
func (s *recordingSystemShard) SetParentKernel(types.Kernel)            {}
func (s *recordingSystemShard) SetLLMClient(types.LLMClient)            {}
func (s *recordingSystemShard) SetSessionContext(*types.SessionContext) {}

// countingExecutor is the JIT task executor, counting what reached it.
type countingExecutor struct{ calls atomic.Int64 }

func (e *countingExecutor) Execute(context.Context, session.TaskRequest) (string, error) {
	e.calls.Add(1)
	return "generic advice", nil
}
func (e *countingExecutor) ExecuteWithContext(ctx context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (string, error) {
	return e.Execute(ctx, req)
}
func (e *countingExecutor) ExecuteAsync(context.Context, session.TaskRequest) (string, error) {
	return "", nil
}
func (e *countingExecutor) GetResult(string) (string, bool, error) { return "", false, nil }
func (e *countingExecutor) WaitForResult(context.Context, string) (string, error) {
	return "", nil
}

// /legislate promises synthesis and ratification, which the registered
// Legislator system shard performs (compile, stratify, RuleCourt, hot-load).
// The TUI sent it to the generic JIT executor instead, which turned the name
// into the verb /legislate and ran an ordinary task (audit
// tui-legislator-dispatch-parity). A registered system profile now runs
// through the shard manager, as the Cortex front door does.
func TestSpawnTask_ARegisteredSystemShardRunsItsImplementation(t *testing.T) {
	t.Parallel()
	var shardCalls atomic.Int64
	sm := coreshards.NewShardManager()
	sm.RegisterShard("legislator", func(id string, cfg types.ShardConfig) types.ShardAgent {
		return &recordingSystemShard{id: id, cfg: cfg, calls: &shardCalls}
	})
	sm.DefineProfile("legislator", types.ShardConfig{Name: "legislator", Type: types.ShardTypeSystem, Timeout: time.Minute})
	exec := &countingExecutor{}
	m := NewTestModel()
	m.shardMgr = sm
	m.taskExecutor = exec

	out, err := m.spawnTask(context.Background(), "legislator", "Stop using fmt.Printf; use log.Info instead.")
	if err != nil {
		t.Fatal(err)
	}
	if shardCalls.Load() != 1 || exec.calls.Load() != 0 {
		t.Fatalf("legislator ran the system shard %d time(s) and the JIT executor %d time(s); want 1 and 0 (output %q)", shardCalls.Load(), exec.calls.Load(), out)
	}

	// A persona with no system profile still goes to the JIT executor.
	if _, err := m.spawnTask(context.Background(), "coder", "rename a variable"); err != nil {
		t.Fatal(err)
	}
	if exec.calls.Load() != 1 || shardCalls.Load() != 1 {
		t.Fatalf("coder: system %d, JIT %d; want the JIT executor", shardCalls.Load(), exec.calls.Load())
	}
}
