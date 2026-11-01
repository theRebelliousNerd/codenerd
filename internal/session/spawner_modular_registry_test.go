package session

import (
	"context"
	"sync"
	"testing"

	"codenerd/internal/tools"
)

func TestSpawnerModularRegistryConfiguration(t *testing.T) {
	first, second := tools.NewRegistry(), tools.NewRegistry()
	parent := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, nil, nil, &MockTransducer{})
	s := registryTaskSpawner()
	s.SetModularTools(first)
	NewJITExecutor(parent, s, &MockTransducer{})
	a, err := s.Spawn(context.Background(), SpawnRequest{Name: "direct"})
	if err != nil || a == nil || a.executor.modularTools() != first {
		t.Fatalf("unconfigured parent replaced direct spawner ownership: %v", err)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Error("nil explicit registry was accepted")
			}
		}()
		s.SetModularTools(nil)
	}()
	if s.currentModularTools() != first {
		t.Fatal("rejected nil configuration changed ownership")
	}
	s.SetModularTools(second)
	if a.executor.modularTools() != first {
		t.Fatal("setter rewired an existing executor")
	}
	if b, err := s.Spawn(context.Background(), SpawnRequest{Name: "updated"}); err != nil || b == nil || b.executor.modularTools() != second {
		t.Fatalf("future child did not see direct update: %v", err)
	}
}

func TestSpawnerModularRegistryConcurrentSnapshots(t *testing.T) {
	first, second := tools.NewRegistry(), tools.NewRegistry()
	cfg := DefaultSpawnerConfig()
	cfg.MaxActiveSubagents = 64
	s := NewSpawner(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, nil, nil, &MockTransducer{}, cfg)
	s.SetModularTools(first)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for range 64 {
			s.SetModularTools(second)
			s.SetModularTools(first)
		}
	}()
	for range 32 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			a, err := s.Spawn(context.Background(), SpawnRequest{Name: "concurrent"})
			if err != nil || a == nil {
				t.Error(err)
				return
			}
			r := a.executor.modularTools()
			if r != first && r != second {
				t.Error("spawn escaped configured owners during update")
			}
		}()
	}
	wg.Wait()
}
