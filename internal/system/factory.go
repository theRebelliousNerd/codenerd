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
	"codenerd/internal/projectdoc"
	"codenerd/internal/prompt"
	prsync "codenerd/internal/prompt/sync"
	"codenerd/internal/session"
	"codenerd/internal/shards"
	"codenerd/internal/shards/system"
	"codenerd/internal/types"
	"database/sql"
	"errors"
	"sort"
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
// (workspace + provider + apiKey + model + disabled system-shard set) so that
// switching workspace, provider, API key, model, or system-shard topology
// mid-process yields the correct instance
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
// model, and the normalized disabled system-shard set. The components are
// length-delimited before hashing so embedded separators cannot collide. The
// SHA-256 digest is safe to use as a map key without exposing the API key.
func cortexKey(workspace, provider, apiKey, model string, disableSystemShards []string) string {
	h := sha256.New()
	components := append(
		[]string{workspace, provider, apiKey, model},
		normalizeDisableSystemShards(disableSystemShards)...,
	)
	for _, component := range components {
		_, _ = fmt.Fprintf(h, "%d:", len(component))
		_, _ = h.Write([]byte(component))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// normalizeDisableSystemShards canonicalizes a caller-provided set for both
// cache identity and boot behavior. Empty entries are ignored; names are
// trimmed, deduplicated, and sorted so order cannot create duplicate Cortexes.
func normalizeDisableSystemShards(names []string) []string {
	if len(names) == 0 {
		return nil
	}

	set := make(map[string]struct{}, len(names))
	for _, name := range names {
		if normalized := strings.TrimSpace(name); normalized != "" {
			set[normalized] = struct{}{}
		}
	}

	normalized := make([]string, 0, len(set))
	for name := range set {
		normalized = append(normalized, name)
	}
	sort.Strings(normalized)
	return normalized
}

// resolveWorkspaceRoot mirrors BootCortexWithConfig's workspace resolution
// so cache keying uses the same effective workspace path as boot.
//
// Also binds CODENERD_WORKSPACE_ROOT so modular file tools (write_file,
// read_file, edit_file, …) resolve relative paths inside the same workspace
// as VirtualStore. Without this, `nerd -w <dir> create …` boots Cortex against
// <dir> but tools still write under process CWD (often the monorepo root).
func resolveWorkspaceRoot(workspace string) string {
	var root string
	if workspace != "" {
		root = workspace
	} else if found, err := config.FindWorkspaceRoot(); err == nil && found != "" {
		root = found
	} else {
		root, _ = os.Getwd()
	}
	if root == "" {
		return root
	}
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
		_ = os.Setenv("CODENERD_WORKSPACE_ROOT", abs)
	}
	return root
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
	return getOrBootCortex(ctx, workspace, apiKey, disableSystemShards, BootCortex)
}

type cortexBootFunc func(context.Context, string, string, []string) (*Cortex, error)

// getOrBootCortex contains the cache transaction independently of the heavy
// production bootstrap. Keeping the boot function injectable lets tests prove
// failure and identity behavior without starting the entire runtime.
func getOrBootCortex(
	ctx context.Context,
	workspace string,
	apiKey string,
	disableSystemShards []string,
	boot cortexBootFunc,
) (*Cortex, error) {
	if boot == nil {
		return nil, fmt.Errorf("cortex boot function is nil")
	}

	ws := resolveWorkspaceRoot(workspace)
	provider, model := resolveProviderModelForKey(ws)
	disabled := normalizeDisableSystemShards(disableSystemShards)
	key := cortexKey(ws, provider, apiKey, model, disabled)

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

	cortex, err := boot(ctx, ws, apiKey, disabled)
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
	// Cancel is stored on Cortex and invoked from Close() so one-shot CLI
	// (create/spawn) does not hang after Result while the loop holds DB work.
	_ = cortex.StartMaintenanceSchedule(context.Background())

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

	// Boot-owned integration resources. These stay private because callers
	// should release the aggregate Cortex, not individual motherboard parts.
	mcpBridge             *mcp.MCPIntegrationBridge
	mcpCancel             context.CancelFunc
	mcpDone               <-chan struct{}
	perceptionInitialized bool
	closeMu               sync.Mutex
	closed                bool

	// cortexKey is the cache key under which this Cortex is registered
	// in cortexCache (set by GetOrBootCortex). Direct BootCortex callers
	// leave it empty; Close() then becomes a no-op against the cache.
	cortexKey string

	// maintenanceCancel stops StartMaintenanceSchedule's background loop.
	// Must be invoked in Close() before LocalDB.Close() or one-shot CLI
	// commands hang after printing Result (DB close blocked / process stuck).
	maintenanceCancel context.CancelFunc
	// maintenanceDone is closed when the maintenance goroutine exits.
	// Close waits on it (briefly) so LocalDB.Close does not race an in-flight
	// MaintenanceCleanup against open SQLite statements on Windows.
	maintenanceDone <-chan struct{}
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

func (c *missingLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string)
	errCh := make(chan error, 1)
	close(ch)
	errCh <- c.err
	close(errCh)
	return ch, errCh
}

func (c *missingLLMClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return nil, c.err
}

// SpawnTask is the unified entry point for task execution.
// System shards (Type S) are routed to ShardManager for lifecycle management.
// Image shards (Nano Banana 2) also use ShardManager so SetImageLLMClient applies
// — TaskExecutor is wired to the worker/main client and must not handle image gen.
// All other tasks go through TaskExecutor.
func (c *Cortex) SpawnTask(ctx context.Context, shardType string, task string) (string, error) {
	normalized := normalizeShardTypeName(shardType)

	// Image generation (Gemini Nano Banana 2) is isolated from the worker LLM.
	// TaskExecutor maps image_generator → /create and would hit Ollama when
	// worker is configured; always route through ShardManager's image client.
	if config.IsImageShardType(normalized) {
		if c.ShardManager == nil {
			return "", fmt.Errorf("image generation requires ShardManager with Nano Banana 2 client")
		}
		return c.ShardManager.Spawn(ctx, normalized, task)
	}

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

	// Image generation stays on ShardManager image client (never TaskExecutor/worker).
	if config.IsImageShardType(normalized) {
		if c.ShardManager == nil {
			return "", fmt.Errorf("image generation requires ShardManager with Nano Banana 2 client")
		}
		return c.ShardManager.SpawnWithPriority(ctx, normalized, task, sessionCtx, priority)
	}

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

// maintenanceInterval is the period between LocalDB maintenance cycles.
// First cycle waits a full interval (no immediate run) so one-shot CLI
// create/spawn can exit without racing MaintenanceCleanup against LocalDB.Close.
// Overridable in tests.
var maintenanceInterval = 30 * time.Minute

// maintenanceStopWait bounds how long StartMaintenanceSchedule / Close wait
// for the prior maintenance goroutine after cancel.
const maintenanceStopWait = 2 * time.Second

// StartMaintenanceSchedule launches a background goroutine that periodically
// runs cold storage archival and cleanup. Call this once after boot.
// Returns a cancel function to stop the maintenance loop.
//
// Does NOT run maintenance immediately: GetOrBootCortex starts this for every
// fresh boot, including `nerd create` / `nerd spawn`. An immediate cycle
// holds SQLite while Close tears down LocalDB and historically stalled
// Windows process exit for many seconds after Result was printed.
func (c *Cortex) StartMaintenanceSchedule(ctx context.Context) context.CancelFunc {
	if c.LocalDB == nil {
		logging.Get(logging.CategorySession).Warn("Maintenance schedule skipped: no LocalDB")
		return func() {}
	}

	// Stop any prior schedule (idempotent re-entry).
	c.stopMaintenanceSchedule(maintenanceStopWait)

	mCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	c.maintenanceCancel = cancel
	c.maintenanceDone = done
	interval := maintenanceInterval
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	go func() {
		defer close(done)
		// No immediate runMaintenance — see StartMaintenanceSchedule doc.
		ticker := time.NewTicker(interval)
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

	logging.Get(logging.CategorySession).Info("Maintenance schedule started (every %v)", interval)
	return cancel
}

// stopMaintenanceSchedule cancels the background loop and waits briefly for
// the goroutine to exit so callers can safely close LocalDB afterward.
func (c *Cortex) stopMaintenanceSchedule(wait time.Duration) {
	if c == nil {
		return
	}
	if c.maintenanceCancel != nil {
		c.maintenanceCancel()
		c.maintenanceCancel = nil
	}
	if c.maintenanceDone == nil {
		return
	}
	if wait <= 0 {
		<-c.maintenanceDone
		c.maintenanceDone = nil
		return
	}
	select {
	case <-c.maintenanceDone:
	case <-time.After(wait):
		logging.Get(logging.CategorySession).Warn(
			"Maintenance goroutine did not exit within %v; continuing shutdown", wait,
		)
	}
	c.maintenanceDone = nil
}

// maintenanceTestHook, when non-nil, is invoked instead of real LocalDB work.
// Tests use this to assert that the schedule does not fire immediately.
var maintenanceTestHook func()

// runMaintenance performs a single maintenance cycle.
func (c *Cortex) runMaintenance() {
	if maintenanceTestHook != nil {
		maintenanceTestHook()
		return
	}
	if c.LocalDB == nil {
		return
	}
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
	plannerLLMClient             perception.LLMClient // high-reasoning tier for planning/analysis intents
	imageLLMClient               perception.LLMClient // Gemini Nano Banana 2 for image_generator only
	providerCfgForClassification *perception.ProviderConfig
	perceptionInitialized        bool
	localDB                      *store.LocalStore
	learningStore                *store.LearningStore
	kernel                       SystemKernel
	transducer                   perception.Transducer
	virtualStore                 *core.VirtualStore
	embeddingEngine              embedding.EmbeddingEngine
	mcpBridge                    *mcp.MCPIntegrationBridge
	mcpCancel                    context.CancelFunc
	mcpDone                      <-chan struct{}
	projectDB                    *sql.DB
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
	projectDoc                   *projectdoc.Document // nerd.md, nil when absent or invalid
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
		appCfg, err = config.LoadUserConfig(userCfgPath)
		if err != nil {
			return fmt.Errorf("load user config: %w", err)
		}
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
	var baseLLMClient perception.LLMClient
	explicitLLM := bctx.appCfg.HasExplicitLLMSelection()
	var configuredLLMErr error
	if bctx.cfg.LLMClientOverride != nil {
		baseLLMClient = bctx.cfg.LLMClientOverride
	}
	// Prefer workspace config first so engine selection wins over a ambient
	// ZAI_API_KEY (or --api-key). Previously any non-empty apiKey forced
	// NewZAIClient and bypassed engine=xai-oauth / claude-cli / codex-cli.
	if baseLLMClient == nil {
		if providerCfg, err := perception.ProviderConfigFromUserConfig(bctx.appCfg); err == nil {
			if client, err2 := perception.NewClientFromConfig(providerCfg); err2 == nil {
				baseLLMClient = client
				bctx.providerCfgForClassification = providerCfg
				logging.Get(logging.CategoryPerception).Info(
					"LLM client from config: engine=%s provider=%s",
					providerCfg.Engine, providerCfg.Provider,
				)
			} else if explicitLLM {
				configuredLLMErr = fmt.Errorf("initialize configured LLM client: %w", err2)
			}
		} else if explicitLLM {
			configuredLLMErr = fmt.Errorf("resolve configured LLM client: %w", err)
		}
	}
	if baseLLMClient == nil && !explicitLLM {
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
		if configuredLLMErr != nil {
			err = fmt.Errorf("no LLM client configured (configured backend unusable): %w", configuredLLMErr)
		}
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

	// Optional worker LLM for shards (e.g. local Ollama for cheap testing).
	// When unset, shards share the main client (SuperGrok xai-oauth, API keys, etc.).
	shardRaw := rawLLMClient
	if bctx.appCfg != nil {
		if worker, werr := perception.NewWorkerClientFromUserConfig(bctx.appCfg); werr != nil {
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

	// Optional planner LLM for reasoning-intensive turns. Without this slot a
	// cheap worker would also serve /review, /audit and campaign planning —
	// exactly the turns whose quality is the product. Nil leaves everything on
	// the worker, which is the pre-existing single-tier behaviour.
	if bctx.appCfg != nil {
		if planner, perr := perception.NewPlannerClientFromUserConfig(bctx.appCfg); perr != nil {
			logging.Get(logging.CategoryPerception).Warn(
				"Planner LLM init failed: %v (reasoning turns stay on the worker client)", perr)
		} else if planner != nil {
			plannerRaw := planner
			if localDB != nil {
				plannerRaw = perception.NewTracingLLMClient(planner, createTraceStoreAdapter(localDB))
			}
			bctx.plannerLLMClient = core.NewScheduledLLMCall("planner", plannerRaw)
			logging.Get(logging.CategoryPerception).Info("Planner LLM enabled for reasoning-intensive intents")
		}
	}

	// Image generation stays on Gemini Nano Banana 2 (gemini-3.1-flash-image) —
	// never the Ollama worker. Attached to ShardManager in initShardManagement.
	if bctx.appCfg != nil {
		if imgClient, ierr := perception.NewImageClientFromUserConfig(bctx.appCfg); ierr != nil {
			logging.Get(logging.CategoryPerception).Warn("Image LLM (Nano Banana 2) unavailable: %v", ierr)
		} else if imgClient != nil {
			bctx.imageLLMClient = core.NewScheduledLLMCall("image_generator", imgClient)
			logging.Get(logging.CategoryPerception).Info("Image LLM enabled: Nano Banana 2 family")
		}
	}

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

	// Intent classification runs on EVERY interactive turn before anything else
	// can happen, so it must not sit on the main reasoning model. Measured on a
	// two-tier stack (main=qwen3.8-max, worker=muse-spark-1.2): a single
	// `nerd run` spent 91 seconds in classification alone.
	//
	// NewClassificationClientFromConfig has existed for this since the P2
	// model-tiering work, and its own doc comment calls routing classification
	// to the main model "a bug" — but it was only ever wired into shard
	// registration, never into the transducer that does the classifying. This
	// is that wiring.
	bctx.transducer = perception.NewUnderstandingTransducer(classificationClientFor(bctx))
	return nil
}

// classificationClientFor resolves the cheapest capable client for per-turn
// intent classification, preferring, in order:
//
//  1. the worker slot's provider — the cheap tier by definition, with thinking
//     disabled (a reasoning trace in front of a labelling task is pure latency);
//  2. the main provider's fast tier (Haiku / Flash-Lite / gpt-4o-mini, or an
//     explicit classification_model);
//  3. the main client, when neither of the above can be built.
func classificationClientFor(bctx *bootContext) perception.LLMClient {
	if bctx.appCfg != nil {
		if w := bctx.appCfg.GetWorkerLLMConfig(); w != nil {
			if key := bctx.appCfg.APIKeyForProvider(w.Provider); key != "" {
				workerCfg := &perception.ProviderConfig{
					Engine:              "api",
					Provider:            perception.Provider(strings.ToLower(strings.TrimSpace(w.Provider))),
					APIKey:              key,
					BaseURL:             w.Endpoint,
					Model:               w.Model,
					ClassificationModel: bctx.appCfg.ClassificationModel,
					Gemini:              bctx.appCfg.GetGeminiConfig(),
				}
				if client, err := perception.NewClassificationClientFromConfig(workerCfg); err == nil && client != nil {
					logging.Get(logging.CategoryPerception).Info(
						"Classification on the worker tier: provider=%s", workerCfg.Provider)
					// Reuse the classification client for shard registration too,
					// so both paths agree on the cheap tier.
					bctx.providerCfgForClassification = workerCfg
					return core.NewScheduledLLMCall("classification", client)
				}
			}
		}
	}

	if bctx.providerCfgForClassification != nil {
		if client, err := perception.NewClassificationClientFromConfig(bctx.providerCfgForClassification); err == nil && client != nil {
			logging.Get(logging.CategoryPerception).Info(
				"Classification on the main provider's fast tier: provider=%s",
				bctx.providerCfgForClassification.Provider)
			return core.NewScheduledLLMCall("classification", client)
		}
	}

	logging.Get(logging.CategoryPerception).Warn(
		"No cheap classification tier available; intent classification runs on the main model (slow on every turn)")
	return bctx.llmClient
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

		for _, scfg := range defaultKernelShardConfigs(bctx.workspace) {
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
	} else {
		bctx.perceptionInitialized = true
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

	loadProjectDoc(bctx)
	return nil
}

// loadProjectDoc reads nerd.md and asserts its frontmatter into the kernel.
//
// It runs after world facts so that a project rule is in place before the first
// turn can act. nerd.md is optional; a missing file is silent.
//
// A malformed file is NOT silent and is NOT fatal. Refusing to boot would strand
// the user with no way to run the agent that could fix the file, but degrading
// quietly would leave them believing a write-protection rule is in force when it
// is not — which is the more dangerous of the two. So: boot, and say loudly
// which directives are not being enforced.
func loadProjectDoc(bctx *bootContext) {
	doc, err := projectdoc.Load(bctx.workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Warning: %s is invalid and NONE of its rules are in force (including any write protection): %v\n",
			projectdoc.FileName, err)
		logging.Get(logging.CategoryBoot).Warn("%s rejected, no project rules active: %v", projectdoc.FileName, err)
		return
	}
	if doc == nil {
		return
	}

	facts := doc.Facts()
	coreFacts := make([]core.Fact, 0, len(facts))
	for _, f := range facts {
		coreFacts = append(coreFacts, core.Fact{Predicate: f.Predicate, Args: f.Args})
	}
	if err := bctx.kernel.LoadFacts(coreFacts); err != nil {
		fmt.Fprintf(os.Stderr,
			"Warning: %s parsed but its rules could not be asserted; write protection is NOT active: %v\n",
			doc.Path, err)
		logging.Get(logging.CategoryBoot).Warn("%s facts rejected by kernel: %v", doc.Path, err)
		return
	}

	bctx.projectDoc = doc
	logging.Boot("Loaded %s: %d facts, %d write-protected path(s), %d command(s)",
		doc.Path, len(coreFacts), len(doc.Spec.Forbid), doc.CommandCount())
}

func defaultKernelShardConfigs(workspace string) []core.KernelShardConfig {
	manifests := shards.DefaultShardPredicateManifests()
	configs := make([]core.KernelShardConfig, 0, len(manifests))
	for _, manifest := range manifests {
		configs = append(configs, core.KernelShardConfig{
			Domain:          manifest.Domain,
			WorkspaceRoot:   workspace,
			OwnedPredicates: append([]string(nil), manifest.OwnedPredicates...),
		})
	}
	return configs
}

func initExecutionLayer(bctx *bootContext) error {
	executorCfg, vsCfg, err := executionLayerConfigs(bctx.appCfg, bctx.workspace)
	if err != nil {
		return fmt.Errorf("configure execution layer: %w", err)
	}
	bctx.executor = tactile.NewDirectExecutorWithConfig(executorCfg)
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
				if closer, ok := engine.(interface{ Close() error }); ok {
					if closeErr := closer.Close(); closeErr != nil {
						logging.Get(logging.CategoryEmbedding).Warn("Failed to close unhealthy embedding engine: %v", closeErr)
					}
				}
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
			mcpCtx, mcpCancel := context.WithCancel(bctx.ctx)
			mcpDone := make(chan struct{})
			bctx.mcpBridge = mcpBridge
			bctx.mcpCancel = mcpCancel
			bctx.mcpDone = mcpDone
			for serverID := range serverConfigs {
				bctx.virtualStore.SetMCPClient(serverID, mcpBridge.GetAdapter(serverID))
				logging.Get(logging.CategoryTools).Info("Wired MCP integration: %s", serverID)
			}
			go func() {
				defer close(mcpDone)
				if err := mcpBridge.ConnectAll(mcpCtx); err != nil && !errors.Is(err, context.Canceled) {
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
				bctx.projectDB = projectDB
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
	// Ownership of the project DB transfers to JITCompiler on success.
	bctx.projectDB = nil

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
	if bctx.imageLLMClient != nil {
		bctx.shardManager.SetImageLLMClient(bctx.imageLLMClient)
	}
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
	// Tool-loop budget from core_limits. Both ceilings used to be hardcoded in
	// DefaultExecutorConfig with no way to raise them, and 8 iterations is low
	// for research-heavy work: a `nerd create <architecture doc>` turn spent its
	// whole budget reading source and reached the ceiling before writing
	// anything.
	if limits := bctx.appCfg.GetCoreLimits(); limits.MaxToolCalls > 0 || limits.MaxToolIterations > 0 {
		execCfg := session.DefaultExecutorConfig()
		if limits.MaxToolCalls > 0 {
			execCfg.MaxToolCalls = limits.MaxToolCalls
		}
		if limits.MaxToolIterations > 0 {
			execCfg.MaxToolIterations = limits.MaxToolIterations
		}
		bctx.sessionExecutor.SetConfig(execCfg)
		logging.Boot("Tool loop budget: %d calls / %d iterations per turn",
			execCfg.MaxToolCalls, execCfg.MaxToolIterations)
	}

	if bctx.localDB != nil {
		bctx.sessionExecutor.SetSessionPersister(bctx.localDB)
	}
	// nerd.md instructions reach the prompt here. Its write protection does not
	// depend on this call — that is enforced from the kernel facts asserted in
	// loadProjectDoc, so a construction site that forgets this line loses the
	// prose, not the guarantee.
	bctx.sessionExecutor.SetProjectDoc(bctx.projectDoc)
	var fileContextProvider session.FileContextProvider
	if ck, ok := bctx.kernel.(*core.CortexKernel); ok {
		if rk := ck.GetPrimaryRealKernel(); rk != nil {
			fileContextProvider = world.NewHolographicProvider(rk, bctx.workspace)
		}
	}
	if fileContextProvider != nil {
		bctx.sessionExecutor.SetFileContextProvider(fileContextProvider)
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
	bctx.sessionSpawner.SetProjectDoc(bctx.projectDoc)
	if fileContextProvider != nil {
		bctx.sessionSpawner.SetFileContextProvider(fileContextProvider)
	}

	// Reasoning-intensive intents (/review, /audit, /campaign, ...) escape the
	// worker tier onto the planner client. The kernel decides which those are
	// via intent_requires_reasoning_model/1; this only supplies the client.
	if bctx.plannerLLMClient != nil {
		plannerSessionLLM := &sessionLLMAdapter{client: bctx.plannerLLMClient}
		bctx.sessionExecutor.SetPlannerClient(plannerSessionLLM)
		bctx.sessionSpawner.SetPlannerClient(plannerSessionLLM)
	}

	bctx.taskExecutor = session.NewJITExecutor(bctx.sessionExecutor, bctx.sessionSpawner, bctx.transducer)
	bctx.virtualStore.SetTaskExecutor(&taskDelegatorAdapter{executor: bctx.taskExecutor})
	logging.Get(logging.CategorySession).Info("JITExecutor wired in BootCortex")
	return nil
}

// BootCortexWithConfig initializes the system with a configuration object.
// This allows for dependency injection during testing.
func BootCortexWithConfig(ctx context.Context, cfg BootConfig) (*Cortex, error) {
	return bootCortexWithSteps(ctx, cfg, defaultBootSteps())
}

type bootStep struct {
	name string
	run  func(*bootContext) error
}

func defaultBootSteps() []bootStep {
	return []bootStep{
		{name: "core components", run: initCoreComponents},
		{name: "perception layer", run: initPerceptionLayer},
		{name: "storage layer", run: initStorageLayer},
		{name: "kernel", run: initKernel},
		{name: "execution layer", run: initExecutionLayer},
		{name: "autopoiesis and browser", run: initAutopoiesisAndBrowser},
		{name: "intelligence layer", run: initIntelligenceLayer},
		{name: "shard management", run: initShardManagement},
		{name: "final executors", run: initFinalExecutors},
	}
}

// bootCortexWithSteps treats bootstrap as a resource transaction: every error
// flows through one rollback path that tears down the partial Cortex. Tests can
// append a failing late step to exercise rollback without production hooks.
func bootCortexWithSteps(ctx context.Context, cfg BootConfig, steps []bootStep) (*Cortex, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	cfg.DisableSystemShards = normalizeDisableSystemShards(cfg.DisableSystemShards)
	bctx := &bootContext{
		ctx: ctx,
		cfg: cfg,
	}

	for _, step := range steps {
		if step.run == nil {
			continue
		}
		if err := step.run(bctx); err != nil {
			bootErr := fmt.Errorf("boot %s: %w", step.name, err)
			if rollbackErr := rollbackBootContext(bctx); rollbackErr != nil {
				return nil, errors.Join(bootErr, fmt.Errorf("boot rollback: %w", rollbackErr))
			}
			return nil, bootErr
		}
	}

	return cortexFromBootContext(bctx), nil
}

func rollbackBootContext(bctx *bootContext) error {
	if bctx == nil {
		return nil
	}

	var errs []error
	// A project DB opened before compiler construction has not transferred to
	// JITCompiler yet, so the aggregate Cortex cannot own it.
	if bctx.projectDB != nil {
		if err := bctx.projectDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("project prompt DB: %w", err))
		}
		bctx.projectDB = nil
	}
	if err := cortexFromBootContext(bctx).Close(); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}

func cortexFromBootContext(bctx *bootContext) *Cortex {
	var realKernel *core.RealKernel
	if rk, ok := bctx.kernel.(*core.RealKernel); ok {
		realKernel = rk
	} else if ck, ok := bctx.kernel.(*core.CortexKernel); ok {
		realKernel = ck.GetPrimaryRealKernel()
	}

	return &Cortex{
		Kernel:                bctx.kernel,
		RealKernel:            realKernel,
		LLMClient:             bctx.llmClient,
		ShardManager:          bctx.shardManager,
		TaskExecutor:          bctx.taskExecutor,
		SessionExecutor:       bctx.sessionExecutor,
		SessionSpawner:        bctx.sessionSpawner,
		VirtualStore:          bctx.virtualStore,
		Executor:              bctx.executor,
		Transducer:            bctx.transducer,
		Orchestrator:          bctx.poiesis,
		BrowserManager:        bctx.browserMgr,
		Scanner:               bctx.scanner,
		UsageTracker:          bctx.tracker,
		LocalDB:               bctx.localDB,
		LearningStore:         bctx.learningStore,
		EmbeddingEngine:       bctx.embeddingEngine,
		Workspace:             bctx.workspace,
		JITCompiler:           bctx.jitCompiler,
		PromptAssembler:       bctx.promptAssembler,
		mcpBridge:             bctx.mcpBridge,
		mcpCancel:             bctx.mcpCancel,
		mcpDone:               bctx.mcpDone,
		perceptionInitialized: bctx.perceptionInitialized,
	}
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
