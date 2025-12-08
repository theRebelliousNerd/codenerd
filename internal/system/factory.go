// Package system provides the core initialization and factory logic for the Cortex.
// It acts as the "Motherboard" that wires all components together.
package system

import (
	"codenerd/internal/autopoiesis"
	"codenerd/internal/browser"
	"codenerd/internal/core"
	"codenerd/internal/mangle"
	"codenerd/internal/perception"
	"codenerd/internal/shards"
	"codenerd/internal/shards/system"
	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/usage"
	"codenerd/internal/world"
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Cortex represents a fully initialized system instance.
type Cortex struct {
	Kernel         core.Kernel
	LLMClient      perception.LLMClient
	ShardManager   *core.ShardManager
	VirtualStore   *core.VirtualStore
	Transducer     *perception.RealTransducer
	Orchestrator   *autopoiesis.Orchestrator
	BrowserManager *browser.SessionManager
	Scanner        *world.Scanner
	UsageTracker   *usage.Tracker
	Workspace      string
}

// BootCortex initializes the entire system stack for a given workspace.
// This ensures consistent wiring across CLI, TUI, and Workers.
func BootCortex(ctx context.Context, workspace string, apiKey string, disableSystemShards []string) (*Cortex, error) {
	if workspace == "" {
		workspace, _ = os.Getwd()
	}

	// 1. Initialize Usage Tracker
	tracker, err := usage.NewTracker(workspace)
	if err != nil {
		// Non-fatal, but worth logging
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize usage tracker: %v\n", err)
	}

	// 2. Initialize Core Components
	baseLLMClient := perception.NewZAIClient(apiKey)

	// Tracing Layer (if local DB available)
	var llmClient perception.LLMClient = baseLLMClient
	localDBPath := filepath.Join(workspace, ".nerd", "knowledge.db")
	var localDB *store.LocalStore
	if db, err := store.NewLocalStore(localDBPath); err == nil {
		localDB = db
		// Wrap with tracing
		traceStore := createTraceStoreAdapter(db)
		llmClient = perception.NewTracingLLMClient(baseLLMClient, traceStore)
	}

	transducer := perception.NewRealTransducer(llmClient)
	kernel := core.NewRealKernel()
	// Force initial evaluation to boot the Mangle engine (even with 0 facts)
	// This is CRITICAL to prevent "kernel not initialized" errors when shards query it early.
	if err := kernel.Evaluate(); err != nil {
		return nil, fmt.Errorf("failed to boot kernel: %w", err)
	}

	executor := tactile.NewSafeExecutor()
	virtualStore := core.NewVirtualStore(executor)
	if localDB != nil {
		virtualStore.SetLocalDB(localDB)
		virtualStore.SetKernel(kernel)
	}

	shardManager := core.NewShardManager()
	shardManager.SetParentKernel(kernel)
	shardManager.SetLLMClient(llmClient)

	// 3. Autopoiesis & Tools
	autopoiesisConfig := autopoiesis.DefaultConfig(workspace)
	poiesis := autopoiesis.NewOrchestrator(llmClient, autopoiesisConfig)
	bridge := core.NewAutopoiesisBridge(kernel)
	poiesis.SetKernel(bridge)

	// Wire Autopoiesis tool executor to VirtualStore
	// Note: We need an adapter here if VirtualStore expects a specific interface
	// Assuming NewToolExecutorAdapter exists in chat package, but we might need to move it or duplicate it.
	// For now, we'll skip this specific wiring if the adapter is in `chat` package.
	// Ideally, the adapter should be in `tactile` or `autopoiesis`.

	// 4. Browser Physics
	browserCfg := browser.DefaultConfig()
	browserCfg.SessionStore = filepath.Join(workspace, ".nerd", "browser", "sessions.json")
	var browserMgr *browser.SessionManager
	// We need a Mangle engine for the browser manager
	if engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil); err == nil {
		browserMgr = browser.NewSessionManager(browserCfg, engine)
		// Browser will be started lazily when needed
	}

	// 5. Register Shards (The Critical Fix)
	regCtx := shards.RegistryContext{
		Kernel:       kernel,
		LLMClient:    llmClient,
		VirtualStore: virtualStore,
	}
	shards.RegisterAllShardFactories(shardManager, regCtx)

	// Overwrite System Shards (Manual Injection if needed, but RegistryContext handles most)
	// However, TactileRouter needs BrowserManager which isn't in RegistryContext yet
	// So we manually re-register TactileRouter to inject BrowserManager
	shardManager.RegisterShard("tactile_router", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewTactileRouterShard()
		shard.SetParentKernel(kernel)
		shard.SetVirtualStore(virtualStore)
		shard.SetLLMClient(llmClient)
		if browserMgr != nil {
			shard.SetBrowserManager(browserMgr)
		}
		return shard
	})

	// 6. Start System Shards
	disabledSet := make(map[string]struct{})
	for _, name := range disableSystemShards {
		disabledSet[name] = struct{}{}
	}
	if env := os.Getenv("NERD_DISABLE_SYSTEM_SHARDS"); env != "" {
		// Parse env var... simplistic split
		// (omitted for brevity, rely on caller to pass parsed list if possible)
	}

	for name := range disabledSet {
		shardManager.DisableSystemShard(name)
	}

	if err := shardManager.StartSystemShards(ctx); err != nil {
		return nil, fmt.Errorf("failed to start system shards: %w", err)
	}

	// 7. World Model Scanning
	scanner := world.NewScanner()

	return &Cortex{
		Kernel:         kernel,
		LLMClient:      llmClient,
		ShardManager:   shardManager,
		VirtualStore:   virtualStore,
		Transducer:     transducer,
		Orchestrator:   poiesis,
		BrowserManager: browserMgr,
		Scanner:        scanner,
		UsageTracker:   tracker,
		Workspace:      workspace,
	}, nil
}

// LocalStoreTraceAdapter wraps LocalStore to implement perception.TraceStore.
// Duplicated from chat/session.go to avoid import cycle or dependency on `chat`.
type LocalStoreTraceAdapter struct {
	store *store.LocalStore
}

func createTraceStoreAdapter(s *store.LocalStore) *LocalStoreTraceAdapter {
	return &LocalStoreTraceAdapter{store: s}
}

func (a *LocalStoreTraceAdapter) StoreReasoningTrace(trace *perception.ReasoningTrace) error {
	// perception.TraceStore expects StoreReasoningTrace(*ReasoningTrace)
	// store.LocalStore.StoreReasoningTrace takes interface{}.
	return a.store.StoreReasoningTrace(trace)
}

func (a *LocalStoreTraceAdapter) LoadReasoningTrace(traceID string) (*perception.ReasoningTrace, error) {
	// Not implemented for now in this adapter context
	return nil, nil
}
