// Package system provides the core initialization and factory logic for the Cortex.
// It acts as the "Motherboard" that wires all components together.
package system

import (
	"codenerd/internal/articulation"
	"codenerd/internal/autopoiesis"
	"codenerd/internal/browser"
	"codenerd/internal/config"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/mcp"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	prsync "codenerd/internal/prompt/sync"
	"codenerd/internal/session"
	"codenerd/internal/shards"
	"codenerd/internal/shards/system"
	"codenerd/internal/types"
	"database/sql"
	"strings"

	"codenerd/internal/sqlpragmas"
	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/usage"
	"codenerd/internal/world"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"

	"sync"
	"time"

	_ "github.com/mattn/go-sqlite3" // SQLite driver for project corpus
)

// SystemKernel extends core.Kernel with system-level lifecycle methods.
type SystemKernel interface {
	core.Kernel
	Evaluate() error
	LoadFactsFromFile(path string) error
	ConsumeBootPrompts() []core.HybridPrompt
}

// BootConfig defines configuration and dependency overrides for BootCortex.
type BootConfig struct {
	Workspace           string
	APIKey              string
	DisableSystemShards []string

	// Overrides for testing
	UserConfigOverride *config.UserConfig
	LLMClientOverride  perception.LLMClient
	KernelOverride     SystemKernel
}

// Keyed cache of Cortex instances. Each entry is keyed by a hash of
// (workspace + provider + apiKey + model) so that switching workspace,
// provider, API key, or model mid-process yields the correct instance
// instead of a stale singleton bound to the wrong context (Bug #15 fix).
//
// Failed boots are NOT cached: returning an error never inserts into the
// map, so a transient initialization failure cannot poison subsequent
// boots for the same key.
var (
	cortexCacheMu sync.RWMutex
	cortexCache   = make(map[string]*Cortex)
)

// cortexKey derives a stable cache key for a Cortex instance from the
// dimensions that change Cortex identity: workspace, provider, API key,
// and model. The components are joined with NUL bytes to avoid ambiguity
// between values that contain the separator, then SHA-256 hashed so the
// key can be safely used as a map index without leaking the API key.
func cortexKey(workspace, provider, apiKey, model string) string {
	h := sha256.New()
	h.Write([]byte(workspace + "\x00" + provider + "\x00" + apiKey + "\x00" + model))
	return hex.EncodeToString(h.Sum(nil))
}

// resolveWorkspaceRoot mirrors BootCortexWithConfig's workspace resolution
// so cache keying uses the same effective workspace path as boot.
func resolveWorkspaceRoot(workspace string) string {
	if workspace != "" {
		return workspace
	}
	if root, err := config.FindWorkspaceRoot(); err == nil && root != "" {
		return root
	}
	cwd, _ := os.Getwd()
	return cwd
}

// resolveProviderModelForKey reads the user config (best-effort) to
// determine the provider and model components of the cortex cache key.
// Errors are intentionally swallowed: if the config is unreadable the
// caller will hit the same failure mode inside BootCortex, and we still
// want to key consistently across calls.
func resolveProviderModelForKey(workspace string) (provider, model string) {
	userCfgPath := filepath.Join(workspace, ".nerd", "config.json")
	cfg, err := config.LoadUserConfig(userCfgPath)
	if err != nil || cfg == nil {
		return "", ""
	}
	return cfg.Provider, cfg.Model
}

// GetOrBootCortex returns the Cortex bound to the given workspace and
// provider context, booting it on first use. Subsequent calls with the
// same (workspace, provider, apiKey, model) tuple return the cached
// instance; calls with a different tuple boot a fresh Cortex so that
// switching workspace, provider, or credentials mid-session does not
// hand back a Cortex wired to the wrong context.
//
// Failed boots return the error to the caller and are NOT cached, so a
// transient failure (missing config, unreachable embedding service)
// will not poison subsequent attempts.
//
// IMPORTANT: This function should be used instead of BootCortex() in
// all command handlers.
func GetOrBootCortex(ctx context.Context, workspace string, apiKey string, disableSystemShards []string) (*Cortex, error) {
	ws := resolveWorkspaceRoot(workspace)
	provider, model := resolveProviderModelForKey(ws)
	key := cortexKey(ws, provider, apiKey, model)

	// Fast path: cache hit under read lock.
	cortexCacheMu.RLock()
	if existing, ok := cortexCache[key]; ok {
		cortexCacheMu.RUnlock()
		return existing, nil
	}
	cortexCacheMu.RUnlock()

	// Slow path: hold the write lock across BootCortex. This serializes
	// concurrent first-boots even across distinct keys, which is acceptable
	// because boot is heavy and rare; the simpler invariant (no torn cache,
	// no duplicate maintenance goroutines) is worth the contention.
	cortexCacheMu.Lock()
	defer cortexCacheMu.Unlock()

	// Re-check under write lock in case a concurrent caller booted it first.
	if existing, ok := cortexCache[key]; ok {
		return existing, nil
	}

	cortex, err := BootCortex(ctx, workspace, apiKey, disableSystemShards)
	if err != nil {
		// Do NOT cache failures.
		return nil, err
	}
	if cortex == nil {
		return nil, fmt.Errorf("BootCortex returned nil cortex without error")
	}

	cortex.cortexKey = key
	cortexCache[key] = cortex

	// Start background maintenance for archival, cleanup, and logging.
	// Only spawned on a fresh boot so cache hits do not leak goroutines.
	cortex.StartMaintenanceSchedule(context.Background())

	return cortex, nil
}

// ResetGlobalCortex clears every cached Cortex instance. Primarily intended
// for testing; in production prefer ResetCortexForWorkspace for surgical
// invalidation. Does not Close() the evicted instances; callers that need
// resource cleanup should Close() the Cortex they hold a reference to.
func ResetGlobalCortex() {
	cortexCacheMu.Lock()
	defer cortexCacheMu.Unlock()
	cortexCache = make(map[string]*Cortex)
}

// ResetCortexForWorkspace evicts every cached Cortex whose Workspace matches
// the given path. Use this when a workspace's configuration changes (provider
// switch, key rotation, model change) and you want the next GetOrBootCortex
// call for that workspace to boot fresh against the new config.
func ResetCortexForWorkspace(workspace string) {
	ws := resolveWorkspaceRoot(workspace)
	cortexCacheMu.Lock()
	defer cortexCacheMu.Unlock()
	for k, c := range cortexCache {
		if c != nil && c.Workspace == ws {
			delete(cortexCache, k)
		}
	}
}

// evictCortexByKey removes the given key from the cache. Used by Cortex.Close
// to keep the cache from holding pointers to torn-down instances.
func evictCortexByKey(key string) {
	if key == "" {
		return
	}
	cortexCacheMu.Lock()
	defer cortexCacheMu.Unlock()
	delete(cortexCache, key)
}

// Cortex represents a fully initialized system instance.
type Cortex struct {
	Kernel          core.Kernel
	RealKernel      *core.RealKernel
	LLMClient       perception.LLMClient
	ShardManager    *coreshards.ShardManager // For shard management (profiles, system shards). Use TaskExecutor for task execution.
	TaskExecutor    session.TaskExecutor     // For task execution (replaces direct ShardManager.Spawn calls)
	SessionExecutor *session.Executor
	SessionSpawner  *session.Spawner
	VirtualStore    *core.VirtualStore
	Executor        tactile.Executor
	Transducer      perception.Transducer
	Orchestrator    *autopoiesis.Orchestrator
	BrowserManager  *browser.SessionManager
	Scanner         *world.Scanner
	UsageTracker    *usage.Tracker
	LocalDB         *store.LocalStore
	LearningStore   *store.LearningStore
	EmbeddingEngine embedding.EmbeddingEngine
	Workspace       string
	JITCompiler     *prompt.JITPromptCompiler
	PromptAssembler *articulation.PromptAssembler

	// cortexKey is the cache key under which this Cortex is registered
	// in cortexCache (set by GetOrBootCortex). Direct BootCortex callers
	// leave it empty; Close() then becomes a no-op against the cache.
	cortexKey string
}

type missingLLMClient struct {
	err error
}

func (c *missingLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	return "", c.err
}

func (c *missingLLMClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "", c.err
}

func (c *missingLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, c.err
}

// SpawnTask is the unified entry point for task execution.
// System shards (Type S) are routed to ShardManager for lifecycle management.
// All other tasks go through TaskExecutor.
func (c *Cortex) SpawnTask(ctx context.Context, shardType string, task string) (string, error) {
	normalized := normalizeShardTypeName(shardType)

	// System shards (Type S) require ShardManager for lifecycle management
	if c.ShardManager != nil {
		if cfg, ok := c.ShardManager.GetProfile(normalized); ok && cfg.Type == types.ShardTypeSystem {
			return c.ShardManager.Spawn(ctx, normalized, task)
		}
	}

	// All other tasks go through TaskExecutor
	if c.TaskExecutor == nil {
		return "", fmt.Errorf("taskExecutor not initialized")
	}
	req := session.TaskRequest{
		IntentVerb: shardType,
		Task:       task,
	}
	return c.TaskExecutor.Execute(ctx, req)
}

// SpawnTaskWithContext spawns a task with additional session context and priority.
// This is used for dream mode, shadow mode, and other speculative execution scenarios.
func (c *Cortex) SpawnTaskWithContext(ctx context.Context, shardType string, task string, sessionCtx *types.SessionContext, priority types.SpawnPriority) (string, error) {
	normalized := normalizeShardTypeName(shardType)

	// System shards (Type S) require ShardManager for lifecycle management
	if c.ShardManager != nil {
		if cfg, ok := c.ShardManager.GetProfile(normalized); ok && cfg.Type == types.ShardTypeSystem {
			return c.ShardManager.SpawnWithPriority(ctx, normalized, task, sessionCtx, priority)
		}
	}

	// All other tasks go through TaskExecutor
	if c.TaskExecutor == nil {
		return "", fmt.Errorf("taskExecutor not initialized")
	}
	req := session.TaskRequest{
		IntentVerb: shardType,
		Task:       task,
	}
	return c.TaskExecutor.ExecuteWithContext(ctx, req, sessionCtx, priority)
}

func normalizeShardTypeName(typeName string) string {
	typeName = strings.TrimSpace(typeName)
	typeName = strings.TrimLeft(typeName, "/")
	return typeName
}

type taskDelegatorAdapter struct {
	executor session.TaskExecutor
}

func (a *taskDelegatorAdapter) Execute(ctx context.Context, intent string, task string) (string, error) {
	req := session.TaskRequest{
		IntentVerb: intent,
		Task:       task,
	}
	return a.executor.Execute(ctx, req)
}

// StartMaintenanceSchedule launches a background goroutine that periodically
// runs cold storage archival and cleanup. Call this once after boot.
// Returns a cancel function to stop the maintenance loop.
func (c *Cortex) StartMaintenanceSchedule(ctx context.Context) context.CancelFunc {
	if c.LocalDB == nil {
		logging.Get(logging.CategorySession).Warn("Maintenance schedule skipped: no LocalDB")
		return func() {}
	}

	mCtx, cancel := context.WithCancel(ctx)
	go func() {
		// Run once immediately on startup
		c.runMaintenance()

		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-mCtx.Done():
				logging.Get(logging.CategorySession).Info("Maintenance schedule stopped")
				return
			case <-ticker.C:
				c.runMaintenance()
			}
		}
	}()

	logging.Get(logging.CategorySession).Info("Maintenance schedule started (every 30 minutes)")
	return cancel
}

// runMaintenance performs a single maintenance cycle.
func (c *Cortex) runMaintenance() {
	stats, err := c.LocalDB.MaintenanceCleanup(store.MaintenanceConfig{
		ArchiveOlderThanDays:       90,
		MaxAccessCount:             5,
		PurgeArchivedOlderThanDays: 365,
		CleanActivationLogDays:     30,
		VacuumDatabase:             false, // Only vacuum on explicit request
	})
	if err != nil {
		logging.Get(logging.CategoryStore).Warn("Maintenance cycle failed: %v", err)
		return
	}
	if stats.FactsArchived > 0 || stats.FactsPurged > 0 || stats.ActivationLogsDeleted > 0 {
		logging.Get(logging.CategoryStore).Info(
			"Maintenance complete: archived=%d purged=%d logs_cleaned=%d",
			stats.FactsArchived, stats.FactsPurged, stats.ActivationLogsDeleted,
		)
	}
}

// BootCortex initializes the entire system stack for a given workspace.
// This ensures consistent wiring across CLI, TUI, and Workers.
func BootCortex(ctx context.Context, workspace string, apiKey string, disableSystemShards []string) (*Cortex, error) {
	return BootCortexWithConfig(ctx, BootConfig{
		Workspace:           workspace,
		APIKey:              apiKey,
		DisableSystemShards: disableSystemShards,
	})
}

// BootCortexWithConfig initializes the system with a configuration object.
// This allows for dependency injection during testing.

type bootContext struct {
	ctx                          context.Context
	cfg                          BootConfig
	workspace                    string
	apiKey                       string
	appCfg                       *config.UserConfig
	jitCfg                       config.JITConfig
	llmClient                    perception.LLMClient
	shardLLMClient               perception.LLMClient
	providerCfgForClassification *perception.ProviderConfig
	localDB                      *store.LocalStore
	learningStore                *store.LearningStore
	kernel                       SystemKernel
	transducer                   perception.Transducer
	virtualStore                 *core.VirtualStore
	embeddingEngine              embedding.EmbeddingEngine
	atomLoader                   *prompt.AtomLoader
	jitCompiler                  *prompt.JITPromptCompiler
	promptAssembler              *articulation.PromptAssembler
	shardManager                 *coreshards.ShardManager
	executor                     tactile.Executor
	sessionExecutor              *session.Executor
	sessionSpawner               *session.Spawner
	taskExecutor                 session.TaskExecutor
	poiesis                      *autopoiesis.Orchestrator
	browserMgr                   *browser.SessionManager
	scanner                      *world.Scanner
	tracker                      *usage.Tracker
}

func initCoreComponents(bctx *bootContext) error {
	bctx.workspace = bctx.cfg.Workspace
	bctx.apiKey = bctx.cfg.APIKey

	if bctx.workspace == "" {
		if root, err := config.FindWorkspaceRoot(); err == nil && root != "" {
			bctx.workspace = root
		} else {
			bctx.workspace, _ = os.Getwd()
		}
	}
	if perception.SharedTaxonomy != nil {
		perception.SharedTaxonomy.SetWorkspace(bctx.workspace)
	}

	if err := logging.Initialize(bctx.workspace); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize logging: %v\n", err)
	}

	tracker, err := usage.NewTracker(bctx.workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize usage tracker: %v\n", err)
	}
	bctx.tracker = tracker

	userCfgPath := filepath.Join(bctx.workspace, ".nerd", "config.json")
	var appCfg *config.UserConfig
	if bctx.cfg.UserConfigOverride != nil {
		appCfg = bctx.cfg.UserConfigOverride
	} else {
		appCfg, _ = config.LoadUserConfig(userCfgPath)
	}
	if appCfg == nil {
		appCfg = config.DefaultUserConfig()
	}
	bctx.appCfg = appCfg
	bctx.jitCfg = appCfg.GetEffectiveJITConfig()

	// LLM API scheduler policy is fully driven by config.json (api_scheduler +
	// core_limits + engine max_concurrent_calls). See config.GetEffectiveAPISchedulerPolicy.
	pol := appCfg.GetEffectiveAPISchedulerPolicy()
	schedulerCfg := core.DefaultAPISchedulerConfig()
	schedulerCfg.MaxConcurrentAPICalls = pol.MaxConcurrentAPICalls
	schedulerCfg.SlotAcquireTimeout = pol.SlotAcquireTimeout
	schedulerCfg.MinCallSpacing = pol.MinCallSpacing
	schedulerCfg.AdaptiveConcurrency = pol.AdaptiveConcurrency
	schedulerCfg.AdaptiveFloor = pol.AdaptiveFloor
	schedulerCfg.AdaptiveRecoverAfter = pol.AdaptiveRecoverAfter
	core.ConfigureGlobalAPIScheduler(schedulerCfg)
	return nil
}

func initPerceptionLayer(bctx *bootContext) error {
	userCfgPath := filepath.Join(bctx.workspace, ".nerd", "config.json")
	var baseLLMClient perception.LLMClient
	if bctx.cfg.LLMClientOverride != nil {
		baseLLMClient = bctx.cfg.LLMClientOverride
	}
	// Prefer workspace config first so engine selection wins over a ambient
	// ZAI_API_KEY (or --api-key). Previously any non-empty apiKey forced
	// NewZAIClient and bypassed engine=xai-oauth / claude-cli / codex-cli.
	if baseLLMClient == nil {
		if providerCfg, err := perception.LoadConfigJSON(userCfgPath); err == nil {
			if client, err2 := perception.NewClientFromConfig(providerCfg); err2 == nil {
				baseLLMClient = client
				bctx.providerCfgForClassification = providerCfg
				logging.Get(logging.CategoryPerception).Info(
					"LLM client from config: engine=%s provider=%s",
					providerCfg.Engine, providerCfg.Provider,
				)
			}
		}
	}
	if baseLLMClient == nil {
		if client, err := perception.NewClientFromEnv(); err == nil {
			baseLLMClient = client
		}
	}
	// Legacy CLI fallback: raw apiKey arg / --api-key only when no config engine/provider client exists.
	if baseLLMClient == nil && strings.TrimSpace(bctx.apiKey) != "" {
		baseLLMClient = perception.NewZAIClient(strings.TrimSpace(bctx.apiKey))
		logging.Get(logging.CategoryPerception).Info("LLM client from legacy apiKey arg (Z.AI)")
	}
	if baseLLMClient == nil {
		err := fmt.Errorf("no LLM client configured (missing config or env keys)")
		logging.Get(logging.CategoryContext).Error("%v", err)
		baseLLMClient = &missingLLMClient{err: err}
	}

	localDBPath := filepath.Join(bctx.workspace, ".nerd", "knowledge.db")
	var localDB *store.LocalStore
	var rawLLMClient perception.LLMClient = baseLLMClient
	if db, err := store.NewLocalStore(localDBPath); err == nil {
		localDB = db
		traceStore := createTraceStoreAdapter(db)
		rawLLMClient = perception.NewTracingLLMClient(baseLLMClient, traceStore)
	}
	bctx.localDB = localDB

	bctx.llmClient = core.NewScheduledLLMCall("main", rawLLMClient)

	// Optional worker LLM (e.g. local Ollama gemma4:12b) for shards while main
	// TUI agent stays on root provider (Grok). Falls back to main when unset.
	shardRaw := rawLLMClient
	if userCfg, uerr := config.LoadUserConfig(userCfgPath); uerr == nil && userCfg != nil {
		if worker, werr := perception.NewWorkerClientFromUserConfig(userCfg); werr != nil {
			logging.Get(logging.CategoryPerception).Warn("Worker LLM init failed: %v (shards use main client)", werr)
		} else if worker != nil {
			if localDB != nil {
				traceStore := createTraceStoreAdapter(localDB)
				shardRaw = perception.NewTracingLLMClient(worker, traceStore)
			} else {
				shardRaw = worker
			}
			logging.Get(logging.CategoryPerception).Info("Worker LLM enabled for shards/spawn/create")
		}
	}
	bctx.shardLLMClient = core.NewScheduledLLMCall("shards", shardRaw)

	if perception.SharedTaxonomy != nil {
		perception.SharedTaxonomy.SetClient(bctx.llmClient)
		if localDB != nil {
			taxStore := perception.NewTaxonomyStore(localDB)
			perception.SharedTaxonomy.SetStore(taxStore)
			if err := perception.SharedTaxonomy.EnsureDefaults(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Taxonomy defaults init failed: %v\n", err)
			}
			if err := perception.SharedTaxonomy.HydrateFromDB(); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Taxonomy rehydration failed: %v\n", err)
			}
		}
	}

	bctx.transducer = perception.NewUnderstandingTransducer(bctx.llmClient)
	return nil
}

func initStorageLayer(bctx *bootContext) error {
	learningStorePath := filepath.Join(bctx.workspace, ".nerd", "shards")
	if ls, err := store.NewLearningStore(learningStorePath); err == nil {
		bctx.learningStore = ls
	} else {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize learning store: %v\n", err)
	}
	return nil
}

func initKernel(bctx *bootContext) error {
	if bctx.cfg.KernelOverride != nil {
		bctx.kernel = bctx.cfg.KernelOverride
	} else {
		cortex := core.NewCortexKernel("cortex")
		shardConfigs := []core.KernelShardConfig{
			{Domain: "routing", OwnedPredicates: []string{"user_intent", "next_action", "routing_result", "derived_mode"}},
			{Domain: "world", OwnedPredicates: []string{"file_topology", "symbol_graph", "diagnostic", "project_profile"}},
			{Domain: "tools", OwnedPredicates: []string{"tool_capabilities", "shard_lifecycle", "shell_exec_result"}},
			{Domain: "policy", OwnedPredicates: []string{"permitted", "blocked", "constitution", "commit_barrier", "dangerous_action"}},
			{Domain: "campaign", OwnedPredicates: []string{"campaign", "campaign_phase", "campaign_task", "campaign_dependency"}},
			{Domain: "prompts", OwnedPredicates: []string{"prompt_atom", "atom_selection_score", "shard_prompt_base"}},
			{Domain: "cortex", OwnedPredicates: []string{}},
		}

		for _, scfg := range shardConfigs {
			scfg.WorkspaceRoot = bctx.workspace
			shard, err := core.NewKernelShard(scfg)
			if err != nil {
				return fmt.Errorf("failed to create shard %s: %w", scfg.Domain, err)
			}
			if err := cortex.RegisterShard(shard); err != nil {
				return fmt.Errorf("failed to register shard %s: %w", scfg.Domain, err)
			}
		}

		if err := cortex.Evaluate(); err != nil {
			return fmt.Errorf("failed to boot cortex kernel: %w", err)
		}
		bctx.kernel = cortex
	}

	if err := perception.InitPerceptionLayer(bctx.kernel, bctx.appCfg); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Perception init failed: %v\n", err)
	}

	loadedWorld := false
	if bctx.localDB != nil {
		if cached, err := bctx.localDB.LoadAllWorldFacts("fast"); err == nil && len(cached) > 0 {
			facts := make([]core.Fact, 0, len(cached))
			for _, cf := range cached {
				facts = append(facts, core.Fact{Predicate: cf.Predicate, Args: cf.Args})
			}
			if err := bctx.kernel.LoadFacts(facts); err == nil {
				loadedWorld = true
			}
		}
	}
	if !loadedWorld {
		scanPath := filepath.Join(bctx.workspace, ".nerd", "mangle", "scan.mg")
		if _, statErr := os.Stat(scanPath); statErr == nil {
			if loadErr := bctx.kernel.LoadFactsFromFile(scanPath); loadErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to load scan facts: %v\n", loadErr)
			}
		}
	}
	return nil
}

func initExecutionLayer(bctx *bootContext) error {
	bctx.executor = tactile.NewDirectExecutor()
	vsCfg := core.DefaultVirtualStoreConfig()
	vsCfg.WorkingDir = bctx.workspace
	bctx.virtualStore = core.NewVirtualStoreWithConfig(bctx.executor, vsCfg)
	bctx.virtualStore.SetKernel(bctx.kernel)
	bctx.virtualStore.DisableBootGuard()

	if bctx.localDB != nil {
		bctx.virtualStore.SetLocalDB(bctx.localDB)
		if gqAdapter := store.NewLocalStoreGraphAdapter(bctx.localDB); gqAdapter != nil {
			bctx.virtualStore.SetGraphQuery(gqAdapter)
		}
	}
	if bctx.learningStore != nil {
		bctx.virtualStore.SetLearningStore(bctx.learningStore)
	}

	var dreamColdStore core.ColdStoreSaver
	if bctx.localDB != nil {
		dreamColdStore = bctx.localDB
	}
	var dreamLearningSaver core.LearningStoreSaver
	if bctx.learningStore != nil {
		dreamLearningSaver = bctx.learningStore
	}

	dreamRouter := core.NewDreamRouter(bctx.kernel, dreamLearningSaver, dreamColdStore)
	bctx.virtualStore.SetDreamRouter(dreamRouter)

	dreamPlanMgr := core.NewDreamPlanManager(bctx.kernel)
	bctx.virtualStore.SetDreamPlanManager(dreamPlanMgr)

	var realKernel *core.RealKernel
	if rk, ok := bctx.kernel.(*core.RealKernel); ok {
		realKernel = rk
	} else if ck, ok := bctx.kernel.(*core.CortexKernel); ok {
		realKernel = ck.GetPrimaryRealKernel()
	}
	if realKernel != nil {
		transactionMgr := core.NewTransactionManager(realKernel, bctx.workspace)
		bctx.virtualStore.SetTransactionManager(transactionMgr)
	}

	if err := bctx.virtualStore.HydrateModularTools(); err != nil {
		logging.Get(logging.CategorySession).Warn("Failed to hydrate modular tools: %v", err)
	}

	worldCfg := bctx.appCfg.GetWorldConfig()
	bctx.virtualStore.SetCodeScope(NewHolographicCodeScope(bctx.workspace, bctx.kernel, bctx.localDB, worldCfg.DeepWorkers))
	fileEditor := tactile.NewFileEditor()
	fileEditor.SetWorkingDir(bctx.workspace)
	bctx.virtualStore.SetFileEditor(core.NewTactileFileEditorAdapter(fileEditor))

	return nil
}

func initIntelligenceLayer(bctx *bootContext) error {
	embedCfg := bctx.appCfg.GetEmbeddingConfig()
	engineCfg := embedding.Config{
		Provider:       embedCfg.Provider,
		OllamaEndpoint: embedCfg.OllamaEndpoint,
		OllamaModel:    embedCfg.OllamaModel,
		GenAIAPIKey:    embedCfg.GenAIAPIKey,
		GenAIModel:     embedCfg.GenAIModel,
		TaskType:       embedCfg.TaskType,
	}
	if engineCfg.Provider == "genai" && engineCfg.GenAIAPIKey == "" && bctx.apiKey != "" {
		engineCfg.GenAIAPIKey = bctx.apiKey
	}
	if engineCfg.Provider == "" {
		engineCfg = embedding.DefaultConfig()
	}
	if engine, err := embedding.NewEngine(engineCfg); err == nil {
		if checker, ok := engine.(embedding.HealthChecker); ok {
			if err := checker.HealthCheck(bctx.ctx); err != nil {
				logging.Get(logging.CategoryEmbedding).Warn("Embedding engine health check failed: %v", err)
				fmt.Fprintf(os.Stderr, "Warning: Embedding engine unavailable: %v\n", err)
			} else {
				bctx.embeddingEngine = engine
			}
		} else {
			bctx.embeddingEngine = engine
		}
	} else {
		fmt.Fprintf(os.Stderr, "Warning: Failed to init embedding engine: %v\n", err)
	}

	bctx.atomLoader = prompt.NewAtomLoader(bctx.embeddingEngine)

	if bctx.localDB != nil && bctx.embeddingEngine != nil {
		bctx.localDB.SetEmbeddingEngine(bctx.embeddingEngine)
		bctx.localDB.SetReflectionConfig(bctx.appCfg.GetReflectionConfig())
	}
	if bctx.learningStore != nil && bctx.embeddingEngine != nil {
		bctx.learningStore.SetEmbeddingEngine(bctx.embeddingEngine)
		bctx.learningStore.SetReflectionConfig(bctx.appCfg.GetReflectionConfig())
	}

	integrationsCfg := bctx.appCfg.GetIntegrations()
	serverConfigs := integrationsCfg.ToMCPServerConfigs()
	if len(serverConfigs) > 0 {
		var mcpLLMClient mcp.LLMClient
		if bctx.llmClient != nil {
			mcpLLMClient = &perceptionLLMAdapter{client: bctx.llmClient}
		}
		mcpBridge, err := mcp.NewMCPIntegrationBridge(bctx.workspace, newMCPKernelAdapter(bctx.kernel), bctx.embeddingEngine, mcpLLMClient, serverConfigs)
		if err != nil {
			logging.Get(logging.CategoryTools).Warn("Failed to init MCP bridge: %v", err)
		} else {
			for serverID := range serverConfigs {
				bctx.virtualStore.SetMCPClient(serverID, mcpBridge.GetAdapter(serverID))
				logging.Get(logging.CategoryTools).Info("Wired MCP integration: %s", serverID)
			}
			go func() {
				if err := mcpBridge.ConnectAll(context.Background()); err != nil {
					logging.Get(logging.CategoryTools).Warn("MCP auto-connect failed: %v", err)
				}
			}()
		}
	}

	if count, err := IngestHybridPrompts(bctx.ctx, bctx.workspace, bctx.kernel, bctx.atomLoader); err != nil {
		logging.Get(logging.CategoryContext).Warn("Failed to ingest hybrid prompts: %v", err)
	} else if count > 0 {
		logging.Get(logging.CategoryContext).Info("Ingested %d hybrid PROMPT atoms during boot", count)
	}

	synchronizer := prsync.NewAgentSynchronizer(bctx.workspace, bctx.atomLoader)
	if err := synchronizer.SyncAll(bctx.ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Agent sync failed: %v\n", err)
	}

	embeddedCorpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		return fmt.Errorf("failed to load embedded corpus: %w", err)
	}

	compilerCfg := prompt.DefaultCompilerConfig()
	if bctx.jitCfg.TokenBudget > 0 {
		compilerCfg.DefaultTokenBudget = bctx.jitCfg.TokenBudget
	}
	compilerCfg.DebugMode = bctx.jitCfg.DebugMode
	compilerOpts := []prompt.CompilerOption{
		prompt.WithKernel(NewKernelAdapter(bctx.kernel)),
		prompt.WithEmbeddedCorpus(embeddedCorpus),
		prompt.WithConfig(compilerCfg),
	}

	var defaultVectorSearcher *prompt.CompilerVectorSearcher
	if bctx.embeddingEngine != nil {
		defaultVectorSearcher = prompt.NewCompilerVectorSearcher(bctx.embeddingEngine)
		compilerOpts = append(compilerOpts, prompt.WithVectorSearcher(defaultVectorSearcher))
	}

	corpusPath := filepath.Join(bctx.workspace, ".nerd", "prompts", "corpus.db")
	if wrote, err := prompt.MaterializeDefaultPromptCorpus(corpusPath); err != nil {
		logging.Get(logging.CategoryContext).Warn("Failed to materialize default prompt corpus: %v", err)
	} else if wrote {
		logging.Get(logging.CategoryContext).Info("Materialized default prompt corpus to %s", corpusPath)
	}
	if _, statErr := os.Stat(corpusPath); statErr == nil {
		projectDB, dbErr := sql.Open("sqlite3", corpusPath)
		if dbErr == nil {
			sqlpragmas.ApplyDefaultPragmas(projectDB, sqlpragmas.ProfileHot)
			if err := bctx.atomLoader.EnsureSchema(bctx.ctx, projectDB); err != nil {
				logging.Get(logging.CategoryContext).Warn("Failed to ensure project corpus schema: %v", err)
				_ = projectDB.Close()
			} else {
				if embeddedCorpus != nil {
					if err := prompt.HydrateAtomContextTags(bctx.ctx, projectDB, embeddedCorpus.All()); err != nil {
						logging.Get(logging.CategoryContext).Warn("Failed to hydrate project corpus tags: %v", err)
					}
				}
				compilerOpts = append(compilerOpts, prompt.WithProjectDB(projectDB))
				logging.Get(logging.CategoryContext).Info("Registered project corpus: %s", corpusPath)
			}
		} else {
			logging.Get(logging.CategoryContext).Warn("Failed to open project corpus: %v", dbErr)
		}
	}

	jitCompiler, err := prompt.NewJITPromptCompiler(compilerOpts...)
	if err != nil {
		return fmt.Errorf("failed to init JIT compiler: %w", err)
	}
	bctx.jitCompiler = jitCompiler

	if bctx.localDB != nil {
		bctx.jitCompiler.SetLocalDB(bctx.localDB)
	}
	if bctx.learningStore != nil {
		bctx.jitCompiler.SetLearningStore(bctx.learningStore)
	}
	if defaultVectorSearcher != nil {
		defaultVectorSearcher.SetCompiler(bctx.jitCompiler)
	}

	if pa, err := articulation.NewPromptAssembler(bctx.kernel); err == nil {
		pa.SetJITCompiler(bctx.jitCompiler)
		pa.SetJITBudgets(bctx.jitCfg.TokenBudget, bctx.jitCfg.ReservedTokens, bctx.jitCfg.SemanticTopK, bctx.jitCfg.ReservedTokensFallbackRatio)
		pa.EnableJIT(bctx.jitCfg.Enabled)
		bctx.promptAssembler = pa
		bctx.transducer.SetPromptAssembler(articulation.NewPromptAssemblerAdapter(pa))
		if bctx.poiesis != nil {
			bctx.poiesis.SetPromptAssembler(pa)
		}
	}

	discoveredAgents := synchronizer.GetDiscoveredAgents()
	if len(discoveredAgents) > 0 {
		agentsOnDisk := make([]AgentOnDisk, 0, len(discoveredAgents))
		for _, a := range discoveredAgents {
			agentsOnDisk = append(agentsOnDisk, AgentOnDisk{ID: a.ID, DBPath: a.DBPath})
		}
		if _, err := SyncAgentRegistryFromDiscovered(bctx.workspace, agentsOnDisk); err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to sync .nerd/agents.json from .nerd/agents: %v", err)
		}
	}

	bctx.shardManager = coreshards.NewShardManager()
	bctx.shardManager.SetParentKernel(bctx.kernel)
	bctx.shardManager.SetLLMClient(bctx.shardLLMClient)
	bctx.virtualStore.SetShardManager(bctx.shardManager)

	for _, agent := range discoveredAgents {
		if err := prompt.RegisterAgentDBWithJIT(bctx.jitCompiler, agent.ID, agent.DBPath); err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to register agent %s with JIT: %v", agent.ID, err)
		} else {
			logging.Get(logging.CategoryContext).Info("Registered user agent '%s' with JIT compiler", agent.ID)
		}
		cfg := coreshards.DefaultSpecialistConfig(agent.ID, agent.DBPath)
		cfg.Type = types.ShardTypeUser
		bctx.shardManager.DefineProfile(agent.ID, cfg)
	}
	if len(discoveredAgents) > 0 {
		logging.Get(logging.CategoryContext).Info("Registered %d user-defined agents", len(discoveredAgents))
	}

	return nil
}

func initAutopoiesisAndBrowser(bctx *bootContext) error {
	autopoiesisConfig := autopoiesis.DefaultConfig(bctx.workspace)
	bctx.poiesis = autopoiesis.NewOrchestrator(bctx.llmClient, autopoiesisConfig)
	bridge := core.NewAutopoiesisBridge(bctx.kernel)
	bctx.poiesis.SetKernel(bridge)


	if ouroborosLoop := bctx.poiesis.GetOuroborosLoop(); ouroborosLoop != nil {
		bctx.virtualStore.SetToolGenerator(ouroborosLoop)
		bctx.virtualStore.SetToolExecutor(ouroborosLoop)
	}

	browserCfg := browser.DefaultConfig()
	browserCfg.SessionStore = filepath.Join(bctx.workspace, ".nerd", "browser", "sessions.json")
	if engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil); err == nil {
		bctx.browserMgr = browser.NewSessionManager(browserCfg, engine)
	}
	return nil
}

func initShardManagement(bctx *bootContext) error {
	coreLimits := bctx.appCfg.GetCoreLimits()
	limitsEnforcer := core.NewLimitsEnforcer(core.LimitsConfig{
		MaxTotalMemoryMB:      coreLimits.MaxTotalMemoryMB,
		MaxConcurrentShards:   coreLimits.MaxConcurrentShards,
		MaxSessionDurationMin: coreLimits.MaxSessionDurationMin,
		MaxFactsInKernel:      coreLimits.MaxFactsInKernel,
		MaxDerivedFactsLimit:  coreLimits.MaxDerivedFactsLimit,
	})
	bctx.shardManager.SetLimitsEnforcer(limitsEnforcer)

	spawnQueue := coreshards.NewSpawnQueue(bctx.shardManager, limitsEnforcer, coreshards.DefaultSpawnQueueConfig())
	bctx.shardManager.SetSpawnQueue(spawnQueue)
	_ = spawnQueue.Start()

	regLLM := bctx.llmClient
	if bctx.shardLLMClient != nil {
		regLLM = bctx.shardLLMClient
	}
	regCtx := shards.RegistryContext{
		Kernel:       bctx.kernel,
		LLMClient:    regLLM,
		VirtualStore: bctx.virtualStore,
		Workspace:    bctx.workspace,
		JITCompiler:  bctx.jitCompiler,
		JITConfig:    bctx.jitCfg,
	}
	if bctx.providerCfgForClassification != nil {
		if classClient, classErr := perception.NewClassificationClientFromConfig(bctx.providerCfgForClassification); classErr == nil && classClient != nil {
			regCtx.ClassificationClient = classClient
		}
	}
	shards.RegisterAllShardFactories(bctx.shardManager, regCtx)

	bctx.shardManager.SetJITRegistrar(prompt.CreateJITDBRegistrar(bctx.jitCompiler))
	bctx.shardManager.SetJITUnregistrar(prompt.CreateJITDBUnregistrar(bctx.jitCompiler))

	bctx.shardManager.RegisterShard("tactile_router", func(id string, _ types.ShardConfig) types.ShardAgent {
		shard := system.NewTactileRouterShard()
		shard.SetParentKernel(bctx.kernel)
		shard.SetVirtualStore(bctx.virtualStore)
		shard.SetLLMClient(regLLM)
		if setter, ok := any(shard).(interface{ SetJITConfig(config.JITConfig) }); ok {
			setter.SetJITConfig(bctx.jitCfg)
		}
		if bctx.browserMgr != nil {
			shard.SetBrowserManager(bctx.browserMgr)
		}
		if bctx.promptAssembler != nil {
			shard.SetPromptAssembler(articulation.NewPromptAssemblerAdapter(bctx.promptAssembler))
		}
		return shard
	})

	bctx.shardManager.RegisterShard("campaign_runner", func(id string, _ types.ShardConfig) types.ShardAgent {
		shard := system.NewCampaignRunnerShard()
		shard.SetParentKernel(bctx.kernel)
		shard.SetVirtualStore(bctx.virtualStore)
		shard.SetLLMClient(regLLM)
		if setter, ok := any(shard).(interface{ SetJITConfig(config.JITConfig) }); ok {
			setter.SetJITConfig(bctx.jitCfg)
		}
		shard.SetWorkspaceRoot(bctx.workspace)
		shard.SetShardManager(bctx.shardManager)
		if bctx.promptAssembler != nil {
			shard.SetPromptAssembler(articulation.NewPromptAssemblerAdapter(bctx.promptAssembler))
		}
		return shard
	})

	disabledSet := make(map[string]struct{})
	for _, name := range bctx.cfg.DisableSystemShards {
		disabledSet[name] = struct{}{}
	}
	for name := range disabledSet {
		bctx.shardManager.DisableSystemShard(name)
	}

	if err := bctx.shardManager.StartSystemShards(bctx.ctx); err != nil {
		return fmt.Errorf("failed to start system shards: %w", err)
	}
	return nil
}

func initFinalExecutors(bctx *bootContext) error {
	worldCfg := bctx.appCfg.GetWorldConfig()
	bctx.scanner = world.NewScannerWithConfig(world.ScannerConfig{
		MaxConcurrency:  worldCfg.FastWorkers,
		IgnorePatterns:  worldCfg.IgnorePatterns,
		MaxASTFileBytes: worldCfg.MaxFastASTBytes,
	})

	sessionKernel := &sessionKernelAdapter{kernel: bctx.kernel}
	sessionVS := &sessionVirtualStoreAdapter{vs: bctx.virtualStore}
	// CLI spawn/create/TaskExecutor use the worker LLM when configured
	// (local Ollama), not the main TUI Grok client.
	taskLLM := bctx.llmClient
	if bctx.shardLLMClient != nil {
		taskLLM = bctx.shardLLMClient
	}
	sessionLLM := &sessionLLMAdapter{client: taskLLM}

	configFactory := prompt.NewDefaultConfigFactory()

	bctx.sessionExecutor = session.NewExecutor(
		sessionKernel,
		sessionVS,
		sessionLLM,
		bctx.jitCompiler,
		configFactory,
		bctx.transducer,
	)
	if bctx.localDB != nil {
		bctx.sessionExecutor.SetSessionPersister(bctx.localDB)
	}

	bctx.sessionSpawner = session.NewSpawner(
		sessionKernel,
		sessionVS,
		sessionLLM,
		bctx.jitCompiler,
		configFactory,
		bctx.transducer,
		session.DefaultSpawnerConfig(),
	)

	bctx.taskExecutor = session.NewJITExecutor(bctx.sessionExecutor, bctx.sessionSpawner, bctx.transducer)
	bctx.virtualStore.SetTaskExecutor(&taskDelegatorAdapter{executor: bctx.taskExecutor})
	logging.Get(logging.CategorySession).Info("JITExecutor wired in BootCortex")
	return nil
}

// BootCortexWithConfig initializes the system with a configuration object.
// This allows for dependency injection during testing.
func BootCortexWithConfig(ctx context.Context, cfg BootConfig) (*Cortex, error) {
	bctx := &bootContext{
		ctx: ctx,
		cfg: cfg,
	}

	if err := initCoreComponents(bctx); err != nil {
		return nil, err
	}
	if err := initPerceptionLayer(bctx); err != nil {
		return nil, err
	}
	if err := initStorageLayer(bctx); err != nil {
		return nil, err
	}
	if err := initKernel(bctx); err != nil {
		return nil, err
	}
	if err := initExecutionLayer(bctx); err != nil {
		return nil, err
	}
	if err := initAutopoiesisAndBrowser(bctx); err != nil {
		return nil, err
	}
	if err := initIntelligenceLayer(bctx); err != nil {
		return nil, err
	}
	if err := initShardManagement(bctx); err != nil {
		return nil, err
	}
	if err := initFinalExecutors(bctx); err != nil {
		return nil, err
	}

	var realKernel *core.RealKernel
	if rk, ok := bctx.kernel.(*core.RealKernel); ok {
		realKernel = rk
	} else if ck, ok := bctx.kernel.(*core.CortexKernel); ok {
		realKernel = ck.GetPrimaryRealKernel()
	}

	return &Cortex{
		Kernel:          bctx.kernel,
		RealKernel:      realKernel,
		LLMClient:       bctx.llmClient,
		ShardManager:    bctx.shardManager,
		TaskExecutor:    bctx.taskExecutor,
		SessionExecutor: bctx.sessionExecutor,
		SessionSpawner:  bctx.sessionSpawner,
		VirtualStore:    bctx.virtualStore,
		Executor:        bctx.executor,
		Transducer:      bctx.transducer,
		Orchestrator:    bctx.poiesis,
		BrowserManager:  bctx.browserMgr,
		Scanner:         bctx.scanner,
		UsageTracker:    bctx.tracker,
		LocalDB:         bctx.localDB,
		LearningStore:   bctx.learningStore,
		EmbeddingEngine: bctx.embeddingEngine,
		Workspace:       bctx.workspace,
		JITCompiler:     bctx.jitCompiler,
		PromptAssembler: bctx.promptAssembler,
	}, nil
}

// IngestHybridPrompts loads PROMPT directives extracted from hybrid .mg files
// into the project prompt corpus database (.nerd/prompts/corpus.db).
// This keeps hybrid files as a readable single source of truth while still
// routing prompt atoms into the JIT system.
func IngestHybridPrompts(ctx context.Context, workspace string, kernel SystemKernel, atomLoader *prompt.AtomLoader) (int, error) {
	if kernel == nil || atomLoader == nil {
		return 0, nil
	}

	hybridPrompts := kernel.ConsumeBootPrompts()
	if len(hybridPrompts) == 0 {
		return 0, nil
	}

	corpusPath := filepath.Join(workspace, ".nerd", "prompts", "corpus.db")
	if wrote, err := prompt.MaterializeDefaultPromptCorpus(corpusPath); err != nil {
		return 0, fmt.Errorf("failed to materialize default prompt corpus for hybrid ingest: %w", err)
	} else if wrote {
		logging.Get(logging.CategoryContext).Info("Materialized default prompt corpus to %s (hybrid ingest)", corpusPath)
	}
	if err := os.MkdirAll(filepath.Dir(corpusPath), 0755); err != nil {
		return 0, fmt.Errorf("failed to create prompts dir for hybrid corpus: %w", err)
	}

	db, err := sql.Open("sqlite3", corpusPath)
	if err != nil {
		return 0, fmt.Errorf("failed to open hybrid prompt corpus DB: %w", err)
	}
	defer db.Close()
	sqlpragmas.ApplyDefaultPragmas(db, sqlpragmas.ProfileBulkBuild)

	if err := atomLoader.EnsureSchema(ctx, db); err != nil {
		return 0, fmt.Errorf("failed to ensure hybrid prompt corpus schema: %w", err)
	}

	stored := 0
	for _, hp := range hybridPrompts {
		catStr := strings.TrimSpace(strings.ToLower(hp.Category))
		if catStr == "" {
			catStr = string(prompt.CategoryDomain)
		}
		atom := prompt.NewPromptAtom(hp.ID, prompt.AtomCategory(catStr), hp.Content)
		if len(hp.Tags) > 1 {
			atom.Subcategory = strings.Join(hp.Tags[1:], ",")
		}

		// Default priority; skeleton categories are always included by selector.
		if err := atomLoader.StoreAtom(ctx, db, atom); err != nil {
			logging.Get(logging.CategoryContext).Warn("Failed to store hybrid prompt atom %s: %v", hp.ID, err)
			continue
		}
		stored++
	}

	return stored, nil
}
