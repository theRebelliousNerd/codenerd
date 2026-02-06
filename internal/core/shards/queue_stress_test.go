package shards_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core/shards"
	"codenerd/internal/types"
)

// MockLimitsEnforcer allows everything
type MockLimitsEnforcer struct{}

func (m *MockLimitsEnforcer) CheckShardLimit(activeCount int) error { return nil }
func (m *MockLimitsEnforcer) CheckMemory() error                    { return nil }
func (m *MockLimitsEnforcer) GetAvailableShardSlots(activeCount int) int {
	return 100 // plenty of slots
}

// MockShardManager to intercept spawns without real overhead
type MockShardManager struct {
	shards.ShardManager // Embed for interface compatibility if needed, but we override critical methods
	spawnCount          int
	mu                  sync.Mutex
}

func (m *MockShardManager) SpawnAsyncWithContext(ctx context.Context, typeName, task string, sessionCtx *types.SessionContext) (string, error) {
	m.mu.Lock()
	m.spawnCount++
	id := fmt.Sprintf("mock-shard-%d", m.spawnCount)
	m.mu.Unlock()

	// Simulate work
	go func() {
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Millisecond):
			// Done
		}
	}()

	return id, nil
}

func (m *MockShardManager) GetResult(id string) (types.ShardResult, bool) {
	return types.ShardResult{
		ShardID:   id,
		Result:    "Success",
		Timestamp: time.Now(),
	}, true
}

func TestSpawnQueue_Stress(t *testing.T) {
	// Setup
	// We can't easily mock the struct method receiver, so we have to use the real one or refactor.
	// but we can mock its dependencies to make it fast.

	// Wait, SpawnQueue takes *ShardManager. We can't inject a mock manager unless we mock the methods specifically
	// or if SpawnQueue took an interface.
	// Looking at spawn_queue.go: type SpawnQueue struct { ... shardManager *ShardManager ... }
	// It's coupled to the concrete type.
	// However, ShardManager has a factory system. We can register a dummy factory that does nothing.

	sm := shards.NewShardManager()
	sm.SetLimitsEnforcer(&MockLimitsEnforcer{})

	// Register a dummy "stress_test" shard type that does nothing and returns immediately
	sm.RegisterShard("stress_test", func(id string, config types.ShardConfig) types.ShardAgent {
		return &MockShardAgent{ID: id}
	})

	// Configure Queue
	cfg := shards.DefaultSpawnQueueConfig()
	cfg.MaxQueueSize = 200
	cfg.MaxQueuePerPriority = 200
	cfg.WorkerCount = 10 // Increase workers for stress test

	sq := shards.NewSpawnQueue(sm, &MockLimitsEnforcer{}, cfg)
	sm.SetSpawnQueue(sq) // Link back

	sq.Start()
	defer sq.Stop()

	// Stress Test Parameters
	requestCount := 150

	var wg sync.WaitGroup
	start := time.Now()

	errCh := make(chan error, requestCount)

	for i := 0; i < requestCount; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_, err := sm.SpawnWithPriority(context.Background(), "stress_test", fmt.Sprintf("task %d", id), nil, types.PriorityNormal)
			if err != nil {
				errCh <- err
			}
		}(i)
	}

	wg.Wait()
	duration := time.Since(start)
	close(errCh)

	// Analyze results
	errCount := 0
	for err := range errCh {
		t.Logf("Spawn error: %v", err)
		errCount++
	}

	t.Logf("Processed %d requests in %v", requestCount, duration)
	t.Logf("Throughput: %.2f req/sec", float64(requestCount)/duration.Seconds())

	if errCount > 0 {
		t.Errorf("Encountered %d errors during stress test", errCount)
	}
}

// MockShardAgent
type MockShardAgent struct {
	ID string
}

func (m *MockShardAgent) Execute(ctx context.Context, task string) (string, error) {
	time.Sleep(10 * time.Millisecond) // Simulate tiny work
	return "Mock execution complete", nil
}
func (m *MockShardAgent) GetID() string                               { return m.ID }
func (m *MockShardAgent) GetState() types.ShardState                  { return types.ShardStateIdle }
func (m *MockShardAgent) GetConfig() types.ShardConfig                { return types.ShardConfig{} }
func (m *MockShardAgent) Stop() error                                 { return nil }
func (m *MockShardAgent) SetParentKernel(k types.Kernel)              {}
func (m *MockShardAgent) SetLLMClient(client types.LLMClient)         {}
func (m *MockShardAgent) SetSessionContext(ctx *types.SessionContext) {}
