package session

import (
	"context"
	"sync"
	"testing"

	"codenerd/internal/tools"
)

func registryTaskSpawner() *Spawner {
	return NewSpawner(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, nil, nil, &MockTransducer{}, DefaultSpawnerConfig())
}

func TestTaskRegistryOwnership(t *testing.T) {
	t.Run("ordinary default remains dynamic global", func(t *testing.T) {
		parent := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, nil, nil, &MockTransducer{})
		s := registryTaskSpawner()
		NewJITExecutor(parent, s, &MockTransducer{})
		a, err := s.Spawn(context.Background(), SpawnRequest{Name: "default"})
		if err != nil || a == nil {
			t.Fatal(err)
		}
		if a.executor.modular.Load() != nil || a.executor.modularTools() != tools.Global() {
			t.Fatal("unconfigured child captured or changed the global registry")
		}
	})
	t.Run("explicit scope follows composition and future children", func(t *testing.T) {
		parent := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, nil, nil, &MockTransducer{})
		first, second := tools.NewRegistry(), tools.NewRegistry()
		parent.SetModularTools(first)
		s := registryTaskSpawner()
		NewJITExecutor(parent, s, &MockTransducer{})
		if parent.CloneForTask().modularTools() != first {
			t.Fatal("inline task lost its configured registry")
		}
		spawn := func(name string) *SubAgent {
			a, err := s.Spawn(context.Background(), SpawnRequest{Name: name})
			if err != nil {
				t.Fatal(err)
			}
			return a
		}
		original := spawn("original")
		if original.executor.modularTools() != first {
			t.Error("isolated task lost the parent registry and fell back to another owner")
		}
		parent.SetModularTools(second)
		if spawn("before-recomposition").executor.modularTools() != first {
			t.Error("changing the parent retroactively changed the composed spawner")
		}
		NewJITExecutor(parent, s, &MockTransducer{})
		if spawn("after-recomposition").executor.modularTools() != second {
			t.Error("recomposition did not forward the new explicit registry")
		}
		if original.executor.modularTools() != first {
			t.Error("recomposition rewired an existing child")
		}
	})
}

func TestTaskRegistryIndependentOwners(t *testing.T) {
	var wg sync.WaitGroup
	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			private := tools.NewRegistry()
			parent := NewExecutor(&MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, nil, nil, &MockTransducer{})
			parent.SetModularTools(private)
			s := registryTaskSpawner()
			NewJITExecutor(parent, s, &MockTransducer{})
			a, err := s.Spawn(context.Background(), SpawnRequest{Name: "private"})
			if err != nil || a == nil {
				t.Error(err)
				return
			}
			if a.executor.modularTools() != private {
				t.Error("isolated task crossed registry owners")
			}
		}()
	}
	wg.Wait()
}
