// Package system provides the core initialization and factory logic for the Cortex.
// It acts as the "Motherboard" that wires all components together.
package system

import (
	"codenerd/internal/autopoiesis"
	"codenerd/internal/browser"
	"codenerd/internal/core"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	prsync "codenerd/internal/prompt/sync"
	"codenerd/internal/shards"
	"codenerd/internal/shards/system"
	"database/sql"
	"strings"

	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/usage"
	"codenerd/internal/world"
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/mangle/ast"
	"github.com/google/mangle/parse"
	_ "github.com/mattn/go-sqlite3" // SQLite driver for project corpus
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
	JITCompiler    *prompt.JITPromptCompiler
}

// BootCortex initializes the entire system stack for a given workspace.
// This ensures consistent wiring across CLI, TUI, and Workers.
func BootCortex(ctx context.Context, workspace string, apiKey string, disableSystemShards []string) (*Cortex, error) {
	if workspace == "" {
		workspace, _ = os.Getwd()
	}

	// 0. Initialize Logging System (critical for debugging)
	if err := logging.Initialize(workspace); err != nil {
		// Non-fatal - continue without file logging
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize logging: %v\n", err)
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

	// Learning Layer - Autopoiesis persistence (§8.3)
	learningStorePath := filepath.Join(workspace, ".nerd", "shards")
	var learningStore *store.LearningStore
	if ls, err := store.NewLearningStore(learningStorePath); err == nil {
		learningStore = ls
	} else {
		// Non-fatal, but worth logging
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize learning store: %v\n", err)
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
	if learningStore != nil {
		virtualStore.SetLearningStore(learningStore)
	}

	shardManager := core.NewShardManager()
	shardManager.SetParentKernel(kernel)
	shardManager.SetLLMClient(llmClient)

	// 3. Autopoiesis & Tools
	autopoiesisConfig := autopoiesis.DefaultConfig(workspace)
	poiesis := autopoiesis.NewOrchestrator(llmClient, autopoiesisConfig)
	bridge := core.NewAutopoiesisBridge(kernel)
	poiesis.SetKernel(bridge)

	// Wire Ouroboros as ToolGenerator for coder shard self-tool routing
	if ouroborosLoop := poiesis.GetOuroborosLoop(); ouroborosLoop != nil {
		virtualStore.SetToolGenerator(ouroborosLoop)
	}

	// 4. Browser Physics
	browserCfg := browser.DefaultConfig()
	browserCfg.SessionStore = filepath.Join(workspace, ".nerd", "browser", "sessions.json")
	var browserMgr *browser.SessionManager
	// We need a Mangle engine for the browser manager
	if engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil); err == nil {
		browserMgr = browser.NewSessionManager(browserCfg, engine)
		// Browser will be started lazily when needed
	}

	// 5. JIT Prompt Compiler & Distributed Storage
	// Initialize Embedding Engine (required for AtomLoader)
	var embeddingEngine embedding.EmbeddingEngine
	if apiKey != "" {
		// Use user-mandated model
		if engine, err := embedding.NewGenAIEngine(apiKey, "models/embedding-001", "RETRIEVAL_DOCUMENT"); err == nil {
			embeddingEngine = engine
		} else {
			fmt.Fprintf(os.Stderr, "Warning: Failed to init embedding engine: %v\n", err)
		}
	}

	// Initialize Atom Loader
	atomLoader := prompt.NewAtomLoader(embeddingEngine)

	// Sync Agents (Distributed Storage)
	// This ensures .nerd/shards/*.db are up-to-date with .nerd/agents/*.yaml
	synchronizer := prsync.NewAgentSynchronizer(workspace, atomLoader)
	if err := synchronizer.SyncAll(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Agent sync failed: %v\n", err)
	}

	// Initialize JIT Prompt Compiler
	// Load embedded corpus from internal/prompt/atoms/ (baked into binary)
	embeddedCorpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded corpus: %w", err)
	}

	// Build compiler options
	compilerOpts := []prompt.CompilerOption{
		prompt.WithKernel(NewKernelAdapter(kernel)),
		prompt.WithEmbeddedCorpus(embeddedCorpus),
	}

	// Load project corpus.db if it exists (user-defined atoms)
	corpusPath := filepath.Join(workspace, ".nerd", "prompts", "corpus.db")
	if _, statErr := os.Stat(corpusPath); statErr == nil {
		projectDB, dbErr := sql.Open("sqlite3", corpusPath)
		if dbErr == nil {
			compilerOpts = append(compilerOpts, prompt.WithProjectDB(projectDB))
			logging.Get(logging.CategoryContext).Info("Registered project corpus: %s", corpusPath)
		} else {
			logging.Get(logging.CategoryContext).Warn("Failed to open project corpus: %v", dbErr)
		}
	}

	jitCompiler, err := prompt.NewJITPromptCompiler(compilerOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to init JIT compiler: %w", err)
	}

	// Register Shards (The Critical Fix)
	regCtx := shards.RegistryContext{
		Kernel:       kernel,
		LLMClient:    llmClient,
		VirtualStore: virtualStore,
		Workspace:    workspace,
		JITCompiler:  jitCompiler,
	}
	shards.RegisterAllShardFactories(shardManager, regCtx)

	// Wire JIT Registrars for future dynamic registration
	shardManager.SetJITRegistrar(prompt.CreateJITDBRegistrar(jitCompiler))
	shardManager.SetJITUnregistrar(prompt.CreateJITDBUnregistrar(jitCompiler))

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
		JITCompiler:    jitCompiler,
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

// KernelAdapter adapts core.Kernel to prompt.KernelQuerier.
// It handles type conversion between []interface{} and []core.Fact.
type KernelAdapter struct {
	kernel core.Kernel
}

// NewKernelAdapter creates a new KernelAdapter for the given kernel.
// This adapter bridges core.Kernel to prompt.KernelQuerier interface,
// enabling the JIT Prompt Compiler to query the Mangle kernel for
// skeleton atom selection.
func NewKernelAdapter(kernel core.Kernel) *KernelAdapter {
	return &KernelAdapter{kernel: kernel}
}

func (ka *KernelAdapter) Query(predicate string) ([]prompt.Fact, error) {
	facts, err := ka.kernel.Query(predicate)
	if err != nil {
		return nil, err
	}
	// Convert []core.Fact to []prompt.Fact
	result := make([]prompt.Fact, len(facts))
	for i, f := range facts {
		result[i] = prompt.Fact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return result, nil
}

func (ka *KernelAdapter) AssertBatch(facts []interface{}) error {
	var coreFacts []core.Fact
	for _, f := range facts {
		switch v := f.(type) {
		case core.Fact:
			coreFacts = append(coreFacts, v)
		case string:
			// Parse string fact
			// Mangle parser expects full clause syntax, typically ending with dot
			input := v
			if !strings.HasSuffix(input, ".") {
				input += "."
			}

			parsed, err := parse.Unit(strings.NewReader(input))
			if err != nil {
				return fmt.Errorf("failed to parse fact string '%s': %w", v, err)
			}

			if len(parsed.Clauses) != 1 {
				return fmt.Errorf("expected 1 clause in fact string, got %d", len(parsed.Clauses))
			}

			atom := parsed.Clauses[0].Head
			args := make([]interface{}, len(atom.Args))
			for i, arg := range atom.Args {
				switch t := arg.(type) {

				case ast.Constant:
					// Mangle name constants must start with /
					// If the symbol doesn't start with /, treat it as a plain string
					// to avoid ToAtom failures later
					if strings.HasPrefix(t.Symbol, "/") {
						args[i] = core.MangleAtom(t.Symbol)
					} else {
						args[i] = t.Symbol
					}
				default:
					args[i] = t.String()
				}
			}

			coreFacts = append(coreFacts, core.Fact{
				Predicate: atom.Predicate.Symbol,
				Args:      args,
			})
		default:
			return fmt.Errorf("unsupported fact type: %T", f)
		}
	}
	return ka.kernel.LoadFacts(coreFacts)
}
