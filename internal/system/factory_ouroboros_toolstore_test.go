package system

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
)

type factoryDelegationProbeKernel struct {
	*core.RealKernel
	armed     atomic.Bool
	processed chan struct{}
}

type factoryShutdownLLM struct{ *MockLLMClient }

func (m *factoryShutdownLLM) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	return m.Complete(ctx, system+user)
}

func (k *factoryDelegationProbeKernel) Query(predicate string) ([]core.Fact, error) {
	if predicate == "delegate_task" && k.armed.CompareAndSwap(true, false) {
		return []core.Fact{{Predicate: predicate, Args: []any{"/tool_generator", "factory_listener_probe", "/pending"}}}, nil
	}
	return k.RealKernel.Query(predicate)
}

func (k *factoryDelegationProbeKernel) Assert(f core.Fact) error {
	if f.Predicate == "tool_generation_failed" || f.Predicate == "tool_delegation_complete" {
		select {
		case k.processed <- struct{}{}:
		default:
		}
	}
	return k.RealKernel.Assert(f)
}

// TestBootOuroborosToolStoreWiring pins the Cortex-factory Ouroboros/tool
// wiring that used to live only in the TUI boot
// (cmd/nerd/chat/session_shared_boot.go): ToolStore, generated-tool hydration
// from disk, and the kernel-listener + Dreamer-queue goroutines.
//
// One boot per test: two BootCortexWithConfig calls inside one test function
// hang the package (see factory_kernel_shards_test.go), so this test boots
// exactly once.
func TestBootOuroborosToolStoreWiring(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("Failed to create .nerd dir: %v", err)
	}

	// On-disk format HydrateToolsFromDisk expects: one file per tool under
	// .nerd/tools/.compiled; the file name (minus .exe) becomes the tool name
	// (see internal/core/tool_registry.go RestoreFromDisk).
	compiledDir := filepath.Join(workspace, ".nerd", "tools", ".compiled")
	if err := os.MkdirAll(compiledDir, 0755); err != nil {
		t.Fatalf("Failed to create compiled tools dir: %v", err)
	}
	const genToolName = "factory-ouroboros-probe-tool"
	if err := os.WriteFile(filepath.Join(compiledDir, genToolName), []byte("#!/bin/sh\necho probe\n"), 0755); err != nil {
		t.Fatalf("Failed to write generated tool fixture: %v", err)
	}

	realKernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	mockKernel := &factoryDelegationProbeKernel{RealKernel: realKernel, processed: make(chan struct{}, 1)}
	var queueProbe atomic.Bool
	queueStarted, queueCanceled, queueRelease := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	releaseQueue := func() { releaseOnce.Do(func() { close(queueRelease) }) }
	defer releaseQueue()
	mockLLM := &MockLLMClient{
		CompleteFunc: func(ctx context.Context, prompt string) (string, error) {
			if queueProbe.CompareAndSwap(true, false) {
				close(queueStarted)
				<-ctx.Done()
				close(queueCanceled)
				<-queueRelease
				return "", ctx.Err()
			}
			return "OK", nil
		},
	}
	mockUserConfig := config.DefaultUserConfig()
	mockUserConfig.Embedding = &config.EmbeddingConfig{Provider: "none"}

	cortex, err := BootCortexWithConfig(context.Background(), BootConfig{
		Workspace: workspace,
		APIKey:    "test-key",
		DisableSystemShards: []string{
			"constitution_gate",
			"perception_firewall",
			"executive_policy",
			"world_model_ingestor",
			"session_planner",
			"tactile_router",
			"campaign_runner",
			"mangle_repair",
			"legislator",
		},
		UserConfigOverride: mockUserConfig,
		LLMClientOverride:  &factoryShutdownLLM{mockLLM},
		KernelOverride:     mockKernel,
	})
	if err != nil {
		t.Fatalf("BootCortexWithConfig failed: %v", err)
	}
	t.Cleanup(func() {
		releaseQueue()
		_ = cortex.Close()
	})

	if cortex.ToolStore == nil {
		t.Error("Cortex.ToolStore should be non-nil after boot")
	}
	if cortex.VirtualStore == nil {
		t.Fatal("VirtualStore should be wired after boot")
	}
	registry := cortex.VirtualStore.GetToolRegistry()
	if registry == nil {
		t.Fatal("VirtualStore.GetToolRegistry() should be non-nil after boot")
	}
	if _, ok := registry.GetTool(genToolName); !ok {
		t.Errorf("generated tool %q should be present in the tool registry after boot", genToolName)
	}
	// Exercise the factory-owned listener through a real delegation. The fake
	// model cannot synthesize a tool, so an explicit failure is the expected
	// processed outcome; a dormant listener emits neither success nor failure.
	mockKernel.armed.Store(true)
	select {
	case <-mockKernel.processed:
	case <-time.After(15 * time.Second):
		t.Fatal("factory listener never processed delegation")
	}

	// The listener and consumer goroutines must exit on Close: assert Close
	// returns without hanging.
	queueProbe.Store(true)
	cortex.OuroborosQueue <- core.ToolNeed{Name: "shutdown_queue_probe", Description: "Generate a tool for the shutdown queue probe"}
	select {
	case <-queueStarted:
	case <-time.After(15 * time.Second):
		t.Fatal("factory queue never entered tool generation")
	}
	serviceDone := cortex.ouroborosDone
	listenerDone := cortex.Orchestrator.StartKernelListener(t.Context(), 2*time.Second)
	done := make(chan error, 1)
	go func() {
		done <- cortex.Close()
	}()
	select {
	case <-queueCanceled:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not cancel the active queue consumer")
	}
	select {
	case <-listenerDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not join the kernel listener")
	}
	select {
	case <-serviceDone:
		t.Fatal("shutdown signaled completion before the queue consumer exited")
	default:
	}
	releaseQueue()
	select {
	case <-serviceDone:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not join the released queue consumer within 5s")
	}
	// The five-second service lifecycle contract above is independent of disk
	// flush latency. Windows runners can spend more than five seconds inside
	// sqlite3_close_v2 after both service goroutines have exited. Still require
	// successful, joined resource cleanup; TempDir cleanup detects open handles.
	const resourceCleanupWait = 4 * closeStepTimeout
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Close: %v", err)
		}
	case <-time.After(resourceCleanupWait):
		stacks := make([]byte, 1<<20)
		n := runtime.Stack(stacks, true)
		t.Fatalf("Cortex.Close() did not release resources within %v; shutdown stacks:\n%s", resourceCleanupWait, stacks[:n])
	}
}
