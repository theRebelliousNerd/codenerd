package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"testing"
	"time"
)

// The manager, the store, and the fact emitter are all reachable from several
// goroutines at once in production: discovery runs detached from Connect, usage
// recording runs detached from CallTool, and the compiler reads the store while
// both are in flight. These tests exist to give `go test -race` something real
// to observe on those paths.
//
//	go test -race ./internal/mcp/ -run 'Concurrent'

func TestConcurrentDiscoverAndCall_WhenRacing_ShouldNotCorruptState(t *testing.T) {
	store, err := NewMCPToolStore("file::memory:?cache=shared", nil)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })

	transport := &mockTransport{
		connected: true,
		tools: []MCPToolSchema{
			{Name: "alpha", Description: "tool alpha", InputSchema: json.RawMessage(`{}`)},
			{Name: "beta", Description: "tool beta", InputSchema: json.RawMessage(`{}`)},
		},
		callResult: &MCPCallResult{Success: true, Output: json.RawMessage(`{"ok":true}`), LatencyMs: 3},
	}

	manager := NewMCPClientManager(store, &countingAnalyzer{}, map[string]MCPServerConfig{})
	manager.SetFactEmitter(NewFactEmitter(&recordingKernel{}))
	manager.servers["srv"] = &MCPServerConnection{
		Server:    &MCPServer{ID: "srv", Status: ServerStatusConnected},
		Transport: transport,
	}

	ctx := context.Background()

	// Persist the catalog before racing. RecordToolUsage is an UPDATE, so a
	// call that lands before its tool row exists silently records nothing —
	// real behaviour, but it would make the assertion below flaky rather than
	// telling us anything about concurrency safety.
	if err := manager.DiscoverTools(ctx, "srv"); err != nil {
		t.Fatalf("initial DiscoverTools: %v", err)
	}

	var wg sync.WaitGroup

	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				if err := manager.DiscoverTools(ctx, "srv"); err != nil {
					t.Errorf("DiscoverTools: %v", err)
					return
				}
			}
		}()
	}

	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				if _, err := manager.CallTool(ctx, "srv/alpha", map[string]any{"n": 1}); err != nil {
					t.Errorf("CallTool: %v", err)
					return
				}
			}
		}()
	}

	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 5 {
				manager.GetConnectedServers()
				manager.GetAllTools()
				if _, err := store.GetAllTools(ctx); err != nil {
					t.Errorf("GetAllTools: %v", err)
					return
				}
			}
		}()
	}

	wg.Wait()

	// CallTool records usage in a detached goroutine; give it a bounded window
	// to land so the assertion is not itself racy.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tool, err := store.GetTool(ctx, "srv/alpha")
		if err != nil {
			t.Fatalf("GetTool: %v", err)
		}
		if tool != nil && tool.UsageCount > 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("usage counters never landed for srv/alpha")
}

func TestConcurrentFactEmission_WhenRacing_ShouldKeepTrackedSetConsistent(t *testing.T) {
	emitter := NewFactEmitter(&recordingKernel{})

	var wg sync.WaitGroup
	for i := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			toolID := fmt.Sprintf("srv/tool%d", i)
			for range 10 {
				emitter.EmitTool(&MCPTool{
					ToolID:          toolID,
					ServerID:        "srv",
					Name:            toolID,
					Categories:      []string{"filesystem"},
					Capabilities:    []string{"/read"},
					Domain:          "/general",
					ShardAffinities: map[string]int{"coder": 50},
					RegisteredAt:    time.Unix(1700000000, 0),
				})
				emitter.EmitServerStatus("srv", ServerStatusConnected)
				_ = emitter.EmittedFactCount()
			}
		}()
	}
	wg.Wait()

	// 8 tools x 7 facts each (registered, name, capability, category, domain,
	// affinity ... no description/condensed/analyzed here) plus one status.
	facts := emitter.EmittedFacts()
	if len(facts) == 0 {
		t.Fatal("no facts tracked")
	}
	seen := map[string]bool{}
	for _, f := range facts {
		if seen[f] {
			t.Errorf("duplicate tracked fact after concurrent emission: %q", f)
		}
		seen[f] = true
	}

	emitter.RetractServer("srv")
	if got := emitter.EmittedFactCount(); got != 0 {
		t.Errorf("RetractServer left %d facts tracked", got)
	}
}
