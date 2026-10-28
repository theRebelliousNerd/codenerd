// Package system provides the core initialization and factory logic for the Cortex.
// It acts as the "Motherboard" that wires all components together.
package system

import (
	"codenerd/internal/approval"
	"codenerd/internal/articulation"
	"codenerd/internal/atomjudge"
	"codenerd/internal/autopoiesis"
	promptevolution "codenerd/internal/autopoiesis/prompt_evolution"
	"codenerd/internal/broker"
	"codenerd/internal/browser"
	"codenerd/internal/config"
	ctxlearn "codenerd/internal/context"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/embedding"
	"codenerd/internal/features"
	"codenerd/internal/jsonl"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/mcp"
	"codenerd/internal/observation"
	"codenerd/internal/perception"
	"codenerd/internal/projectdoc"
	"codenerd/internal/prompt"
	prsync "codenerd/internal/prompt/sync"
	"codenerd/internal/retrieval"
	"codenerd/internal/session"
	"codenerd/internal/shards"
	"codenerd/internal/shards/system"
	"codenerd/internal/tools"
	"codenerd/internal/tools/research"
	"codenerd/internal/tools/shell"
	"codenerd/internal/types"
	"codenerd/internal/typesafe"
	"database/sql"
	"encoding/json"
	"errors"
	"sort"
	"strings"

	"codenerd/internal/sessionlog"
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
	"golang.org/x/sync/semaphore"
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

	// SessionID optionally fixes the process-unique session identity minted
	// at boot. Empty means the factory mints `session-<unixnano>`. The TUI
	// passes its loaded (resumed) session ID here so a resumed session keeps
	// its identity instead of starting a forked one.
	SessionID string

	// Overrides for testing
	UserConfigOverride *config.UserConfig
	LLMClientOverride  perception.LLMClient
	KernelOverride     SystemKernel
}

// cortexIdentity contains only non-secret configuration values and digests of
// secret-bearing inputs. ConfigDigest covers the canonical JSON encoding of
// the validated workspace UserConfig, so changing any setting that can shape
// boot produces a new identity without retaining credential text in the
// identity or cache key.
type cortexIdentity struct {
	Workspace           string   `json:"workspace"`
	Engine              string   `json:"engine"`
	Provider            string   `json:"provider"`
	Model               string   `json:"model"`
	ConfigDigest        string   `json:"config_digest"`
	APIKeyDigest        string   `json:"api_key_digest,omitempty"`
	Context7KeyDigest   string   `json:"context7_key_digest,omitempty"`
	DisableSystemShards []string `json:"disable_system_shards,omitempty"`
}

func newCortexIdentity(workspace string, cfg *config.UserConfig, apiKey string, disableSystemShards []string) (cortexIdentity, error) {
	identity := cortexIdentity{
		Workspace:           workspace,
		APIKeyDigest:        secretDigest(apiKey),
		DisableSystemShards: normalizeDisableSystemShards(disableSystemShards),
	}
	if cfg == nil {
		return identity, fmt.Errorf("cannot identify Cortex without validated user config")
	}
	identity.Engine = cfg.GetEngine()
	identity.Provider = cfg.Provider
	identity.Model = cfg.Model
	// Context7 can be configured outside the workspace file through
	// CONTEXT7_API_KEY. Include the effective key as a digest so environment
	// changes cannot reuse a Cortex with stale research credentials.
	identity.Context7KeyDigest = secretDigest(cfg.GetContext7APIKey())

	// encoding/json emits struct fields in declaration order and sorts map
	// keys. This makes equivalent JSON formatting and object-key ordering yield
	// the same digest while preserving the values boot actually receives.
	configBytes, err := json.Marshal(cfg)
	if err != nil {
		return cortexIdentity{}, fmt.Errorf("encode Cortex configuration identity: %w", err)
	}
	poolIdentity := map[string][]config.ResolvedProviderAPIKey{}
	for provider := range cfg.APIKeyPools {
		entries, resolveErr := cfg.ResolveAPIKeyPool(provider)
		if resolveErr != nil {
			return cortexIdentity{}, resolveErr
		}
		poolIdentity[provider] = entries
	}
	poolBytes, _ := json.Marshal(poolIdentity)
	identity.ConfigDigest = digestBytes(configBytes)
	if len(poolIdentity) > 0 {
		identity.ConfigDigest = digestBytes(append(configBytes, poolBytes...))
	}
	return identity, nil
}

func secretDigest(secret string) string {
	if secret == "" {
		return ""
	}
	return digestBytes([]byte(secret))
}

func digestBytes(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func (identity cortexIdentity) cacheKey() string {
	h := sha256.New()
	components := append(
		[]string{
			identity.Workspace,
			identity.Engine,
			identity.Provider,
			identity.Model,
			identity.ConfigDigest,
			identity.APIKeyDigest,
			identity.Context7KeyDigest,
		},
		normalizeDisableSystemShards(identity.DisableSystemShards)...,
	)
	for _, component := range components {
		_, _ = fmt.Fprintf(h, "%d:", len(component))
		_, _ = h.Write([]byte(component))
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Keyed cache of Cortex instances. Each entry is keyed by the workspace,
// normalized disabled-shard set, and a digest of the complete validated
// workspace configuration plus caller and effective Context7 credentials.
// Changing any boot-shaping setting therefore cannot reuse a Cortex wired
// under older policy.
//
// Failed boots are NOT cached: returning an error never inserts into the
// map, so a transient initialization failure cannot poison subsequent
// boots for the same key.
var (
	cortexCacheMu = semaphore.NewWeighted(1)
	cortexCache   = make(map[string]*Cortex)
)

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
	if abs, err := tools.CanonicalWorkspaceRoot(root); err == nil {
		root = abs
		_ = os.Setenv("CODENERD_WORKSPACE_ROOT", abs)
		// The env variable is process-global, so two registries in one process
		// cannot be given different roots and anything that reassigns it moves
		// the containment boundary for every tool at once. Bind the registry's
		// own root as well; the guard prefers it and treats the env as a
		// fallback for callers that reach a tool outside a registry.
		tools.SetGlobalWorkspaceRoot(abs)
		// The research cache is otherwise memory-only and dies with the
		// process, so every session re-fetches what the last one already had.
		if err := research.EnableDiskCache(abs); err != nil {
			logging.Get(logging.CategoryTools).Debug(
				"research disk cache unavailable at %s: %v", abs, err)
		}
	}
	return root
}

// GetOrBootCortex returns the Cortex bound to the given workspace and
// provider context, booting it on first use. Subsequent calls with the
// same normalized effective workspace configuration and credentials return
// the cached instance; a changed boot setting or credential boots a fresh
// Cortex.
//
// Failed boots return the error to the caller and are NOT cached, so a
// transient failure (missing config, unreachable embedding service)
// will not poison subsequent attempts.
//
// IMPORTANT: This function should be used instead of BootCortex() in
// all command handlers.
func GetOrBootCortex(ctx context.Context, workspace string, apiKey string, disableSystemShards []string) (*Cortex, error) {
	return getOrBootCortexWithConfig(ctx, workspace, apiKey, disableSystemShards, func(ctx context.Context, workspace string, apiKey string, disabled []string, userConfig *config.UserConfig) (*Cortex, error) {
		if userConfig == nil {
			// Let the ordinary boot path return the detailed config validation
			// error. An unreadable identity is never eligible for cache reuse.
			return BootCortex(ctx, workspace, apiKey, disabled)
		}
		return BootCortexWithConfig(ctx, BootConfig{
			Workspace:           workspace,
			APIKey:              apiKey,
			DisableSystemShards: disabled,
			UserConfigOverride:  userConfig,
		})
	})
}

type cortexBootFunc func(context.Context, string, string, []string) (*Cortex, error)
type cortexBootWithConfigFunc func(context.Context, string, string, []string, *config.UserConfig) (*Cortex, error)

func getOrBootCortexWithConfig(
	ctx context.Context,
	workspace string,
	apiKey string,
	disableSystemShards []string,
	boot cortexBootWithConfigFunc,
) (*Cortex, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if boot == nil {
		return nil, fmt.Errorf("cortex boot function is nil")
	}

	ws := resolveWorkspaceRoot(workspace)
	userCfg, configErr := config.LoadUserConfig(filepath.Join(ws, ".nerd", "config.json"))
	cacheable := configErr == nil && userCfg != nil
	disabled := normalizeDisableSystemShards(disableSystemShards)
	key := ""
	if cacheable {
		identity, identityErr := newCortexIdentity(ws, userCfg, apiKey, disabled)
		if identityErr == nil {
			key = identity.cacheKey()
		} else {
			cacheable = false
		}
	}

	if err := cortexCacheMu.Acquire(ctx, 1); err != nil {
		return nil, err
	}
	lockHeld := true
	defer func() {
		if lockHeld {
			cortexCacheMu.Release(1)
		}
	}()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if cacheable {
		if existing, ok := cortexCache[key]; ok {
			return existing, nil
		}
	}

	cortex, err := boot(ctx, ws, apiKey, disabled, userCfg)
	if cancelErr := ctx.Err(); cancelErr != nil {
		err = errors.Join(cancelErr, err)
	}
	if err != nil {
		// Do NOT cache failures.
		cortexCacheMu.Release(1)
		lockHeld = false
		if cortex != nil {
			err = errors.Join(err, cortex.Close())
		}
		return nil, err
	}
	if cortex == nil {
		return nil, fmt.Errorf("BootCortex returned nil cortex without error")
	}

	if cacheable {
		cortex.cortexKey = key
		cortexCache[key] = cortex
	}
	if cancelErr := ctx.Err(); cancelErr != nil {
		delete(cortexCache, key)
		cortex.cortexKey = ""
		cortexCacheMu.Release(1)
		lockHeld = false
		return nil, errors.Join(cancelErr, cortex.Close())
	}

	// Start background maintenance for archival, cleanup, and logging.
	// Only spawned on a fresh boot so cache hits do not leak goroutines.
	// Cancel is stored on Cortex and invoked from Close() so one-shot CLI
	// (create/spawn) does not hang after Result while the loop holds DB work.
	_ = cortex.StartMaintenanceSchedule(context.Background())
	if cancelErr := ctx.Err(); cancelErr != nil {
		delete(cortexCache, key)
		cortex.cortexKey = ""
		cortexCacheMu.Release(1)
		lockHeld = false
		return nil, errors.Join(cancelErr, cortex.Close())
	}

	return cortex, nil
}

// evictCortexByKey removes the given key from the cache. Used by Cortex.Close
// to keep the cache from holding pointers to torn-down instances.
func evictCortexByKey(key string) {
	if key == "" {
		return
	}
	if err := cortexCacheMu.Acquire(context.Background(), 1); err != nil {
		return
	}
	defer cortexCacheMu.Release(1)
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
	ToolStore       *store.ToolStore
	OuroborosQueue  chan<- core.ToolNeed
	EmbeddingEngine embedding.EmbeddingEngine
	// Retriever is the process's sparse retriever: the session executor's
	// task passes and the chat's issue seed share its keyword cache.
	Retriever       *retrieval.SparseRetriever
	Workspace       string
	JITCompiler     *prompt.JITPromptCompiler
	PromptAssembler *articulation.PromptAssembler
	// RolePrompts compiles the system prompt of every non-persona model call
	// (critic, rule generator, analyzers) from its atoms. Non-nil whenever
	// JITCompiler is.
	RolePrompts types.RolePromptCompiler
	// PromptEvolver is the System Prompt Learning loop. Non-nil on every boot
	// that has a JIT compiler; see factory_learning.go for why it is owned
	// here rather than by the chat TUI that used to build it.
	PromptEvolver *promptevolution.PromptEvolver

	// ContextFeedback stores the model's per-turn rating of the context it was
	// given, which tunes spreading activation. Owned here for the same reason.
	ContextFeedback *ctxlearn.ContextFeedbackStore
	// WorkerLLMClient serves bulk shard/task execution when a worker tier is
	// configured; nil means shards share LLMClient. PlannerLLMClient serves
	// reasoning-intensive turns when a planner tier is configured; nil means
	// those turns stay on the worker/main client.
	WorkerLLMClient  perception.LLMClient
	PlannerLLMClient perception.LLMClient

	// Approvals is the process's human-approval broker
	// (Docs/audits/request-bound-human-approval.md). The interactive UI
	// attaches its host to it; headless runs leave it hostless, so an
	// approvable denial stays a refusal there.
	Approvals        *approval.Broker
	approvalsRelease func() error

	// Boot-owned integration resources. These stay private because callers
	// should release the aggregate Cortex, not individual motherboard parts.
	mcpBridge             *mcp.MCPIntegrationBridge
	mcpCancel             context.CancelFunc
	mcpDone               <-chan struct{}
	ouroborosCancel       context.CancelFunc
	ouroborosDone         <-chan struct{}
	researchBrowser       *research.BrowserRuntime
	generatedExecutor     *orchestratorToolExecutor
	onDemandStop          func()
	perceptionInitialized bool

	// meterSink and couseLog are what THIS boot installed into the two
	// process-wide measurement singletons. Close detaches only these, by
	// identity: several Cortex instances can be live at once (cortexCache is
	// keyed per workspace and provider), and an unconditional detach lets one
	// shutting down close the log another is still writing to.
	meterSink broker.ReceiptSink
	couseLog  *jsonl.Appender
	closeMu   sync.Mutex
	closed    bool

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

	// sessionID is the process-unique session identity minted at boot (or
	// supplied via BootConfig.SessionID). It is the single source of truth
	// for the session executor's turn-persistence identity.
	sessionID string

	// sessionLog holds this boot's session-folder lease (session_log_boot.go).
	sessionLog *sessionlog.Handle
}

// SessionID returns the process-unique session identity minted at boot.
// Headless surfaces (e.g. `nerd chat`) reuse this instead of minting their
// own so all turns persist under one identity.
func (c *Cortex) SessionID() string {
	if c == nil {
		return ""
	}
	return c.sessionID
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

func (c *missingLLMClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan types.StreamResult) {
	return types.FailedStream(c.err)
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
	return c.SpawnTaskWithTarget(ctx, shardType, task, "")
}

func (c *Cortex) SpawnTaskWithTarget(ctx context.Context, shardType string, task, target string) (string, error) {
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
		Target:     target,
	}
	return c.TaskExecutor.Execute(ctx, req)
}

// SpawnTaskObservedWithTarget is SpawnTaskWithTarget returning what the
// executor measured about the run instead of only its prose — in particular
// Outcome, the kernel's verdict for the turn.
//
// It exists so the direct CLI verbs read the SAME verdict the chat, the
// learner and turn_cost read, rather than deciding for themselves from an error
// string whether the work landed. A `nerd fix` that built green and ran its
// tests green is /done; one that wrote and could not show a green gate is
// /unverified, which is not a failure; only /failed is.
//
// An executor that does not implement ObservedTaskExecutor still runs the task;
// the observation then carries the output and nothing else, and Outcome is
// empty — which every consumer must read as "no verdict was taken", never as
// "not done".
func (c *Cortex) SpawnTaskObservedWithTarget(ctx context.Context, shardType string, task, target string) (observation.Return, error) {
	normalized := normalizeShardTypeName(shardType)

	if !config.IsImageShardType(normalized) && c.TaskExecutor != nil {
		systemShard := false
		if c.ShardManager != nil {
			if cfg, ok := c.ShardManager.GetProfile(normalized); ok && cfg.Type == types.ShardTypeSystem {
				systemShard = true
			}
		}
		if !systemShard {
			if observed, ok := c.TaskExecutor.(session.ObservedTaskExecutor); ok {
				return observed.ExecuteObserved(ctx, session.TaskRequest{
					IntentVerb: shardType,
					Task:       task,
					Target:     target,
				})
			}
		}
	}

	out, err := c.SpawnTaskWithTarget(ctx, shardType, task, target)
	return observation.Return{Agent: shardType, Task: task, Output: out}, err
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

// ExecuteObserved forwards the structured return when the wrapped executor can
// produce one, satisfying core.ObservedTaskDelegator.
//
// The adapter has to declare the method unconditionally — Go decides interface
// satisfaction on the adapter's own type, not on what it happens to wrap — so
// an executor that cannot report structure falls back to Execute here rather
// than failing. Returning an error instead would turn "this executor keeps no
// write set" into "the delegation failed", which is a different and much worse
// answer for the parent to act on.
func (a *taskDelegatorAdapter) ExecuteObserved(ctx context.Context, intent string, task string) (observation.Return, error) {
	req := session.TaskRequest{IntentVerb: intent, Task: task}
	if observed, ok := a.executor.(session.ObservedTaskExecutor); ok {
		return observed.ExecuteObserved(ctx, req)
	}
	result, err := a.executor.Execute(ctx, req)
	return observation.Return{Output: result}, err
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
// MCPBridge returns the MCP integration bridge, or nil when no MCP servers are
// configured. The bridge was already retained on the Cortex but had no
// accessor, so the tool compiler it owns was unreachable from outside this
// package and MCP tools could not be compiled into a shard's prompt.
func (c *Cortex) MCPBridge() *mcp.MCPIntegrationBridge {
	if c == nil {
		return nil
	}
	return c.mcpBridge
}

// WorkerLLM returns the worker-tier LLM client for bulk shard/task execution,
// or nil when no worker tier is configured (callers fall back to LLMClient).
func (c *Cortex) WorkerLLM() perception.LLMClient {
	if c == nil {
		return nil
	}
	return c.WorkerLLMClient
}

// PlannerLLM returns the planner-tier LLM client for reasoning-intensive
// turns, or nil when no planner tier is configured.
func (c *Cortex) PlannerLLM() perception.LLMClient {
	if c == nil {
		return nil
	}
	return c.PlannerLLMClient
}

// PlanningLLM returns the client campaign planning runs on: decomposition,
// replanning and assault remediation planning. Task execution stays on the
// worker tier inside TaskExecutor.
func (c *Cortex) PlanningLLM() perception.LLMClient {
	if c == nil {
		return nil
	}
	return planningLLM(c.LLMClient, c.PlannerLLMClient)
}

// planningLLM is the planner slot config.json configures, else the main
// client: the one resolution every planning door shares.
func planningLLM(main, planner perception.LLMClient) perception.LLMClient {
	if planner != nil {
		return planner
	}
	return main
}

// ToolGenerator returns the autopoiesis tool generator held by the boot
// orchestrator, or nil when autopoiesis is disabled/absent.
func (c *Cortex) ToolGenerator() *autopoiesis.ToolGenerator {
	if c == nil || c.Orchestrator == nil {
		return nil
	}
	return c.Orchestrator.GetToolGenerator()
}

// OuroborosLoop returns the concrete autopoiesis loop for campaign tool
// pregeneration, or nil when autopoiesis is disabled or the loop is absent.
func (c *Cortex) OuroborosLoop() *autopoiesis.OuroborosLoop {
	if c == nil || c.Orchestrator == nil {
		return nil
	}
	return c.Orchestrator.GetConcreteOuroborosLoop()
}

// MCPStore returns the MCP tool store owned by the integration bridge, or nil
// when no bridge is wired or the bridge holds no store.
func (c *Cortex) MCPStore() *mcp.MCPToolStore {
	if c == nil || c.mcpBridge == nil {
		return nil
	}
	return c.mcpBridge.GetStore()
}

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
				c.runEvolutionCycle(mCtx)
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
	autoCleanupToolStore(c.ToolStore)
	if c.LocalDB == nil {
		return
	}
	stats, err := c.LocalDB.MaintenanceCleanup(store.MaintenanceConfig{
		ArchiveOlderThanDays:       90,
		MaxAccessCount:             5,
		PurgeArchivedOlderThanDays: 365,
		CleanActivationLogDays:     30,
		VacuumDatabase:             false, // Only vacuum on explicit request
		// Heal the ANN drift a failed vec_index insert leaves behind; until
		// 2026-09-25 only the next engine attach rebuilt the index.
		ReconcileVecIndex: true,
	})
	if err != nil {
		logging.Get(logging.CategoryStore).Warn("Maintenance cycle failed: %v", err)
		return
	}
	if stats.FactsArchived > 0 || stats.FactsPurged > 0 || stats.ActivationLogsDeleted > 0 || stats.VecIndexHealed > 0 {
		logging.Get(logging.CategoryStore).Info(
			"Maintenance complete: archived=%d purged=%d logs_cleaned=%d vec_index_healed=%d",
			stats.FactsArchived, stats.FactsPurged, stats.ActivationLogsDeleted, stats.VecIndexHealed,
		)
	}
}

// autoCleanupToolStore keeps tools.db inside the store's cleanup budget
// (store.DefaultCleanupConfig: 336 runtime hours of tool executions, cleaned
// from 80%). ToolStore.AutoCleanup existed with no caller, so the journal grew
// until someone ran /cleanup-tools by hand -- and a CLI-only user, who has no
// /cleanup-tools, never could. Run at boot, which every entry path passes
// through, and on each maintenance cycle.
func autoCleanupToolStore(ts *store.ToolStore) {
	if ts == nil {
		return
	}
	stats, err := ts.AutoCleanup(store.DefaultCleanupConfig())
	if err != nil {
		logging.Get(logging.CategoryStore).Warn("tools.db auto-cleanup failed: %v", err)
		return
	}
	if stats != nil && stats.ExecutionsDeleted > 0 {
		logging.Get(logging.CategoryStore).Info("tools.db auto-cleanup (%s): %d executions deleted, %d bytes freed",
			stats.Method, stats.ExecutionsDeleted, stats.BytesFreed)
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
	plannerLLMClient             perception.LLMClient        // high-reasoning tier for planning/analysis intents
	shardProfileContext          session.ShardProfileContext // persona profile -> call context, for executor and spawner
	imageLLMClient               perception.LLMClient        // Gemini Nano Banana 2 for image_generator only
	providerCfgForClassification *perception.ProviderConfig
	// classificationClient is the one resolved per-turn classifier (worker
	// tier failing over to the main fast tier): the direct transducer and
	// the registered perception firewall both classify through it.
	classificationClient  perception.LLMClient
	perceptionInitialized bool
	meterSink             broker.ReceiptSink
	couseLog              *jsonl.Appender
	localDB               *store.LocalStore
	learningStore         *store.LearningStore
	kernel                SystemKernel
	transducer            perception.Transducer
	virtualStore          *core.VirtualStore
	embeddingEngine       embedding.EmbeddingEngine
	retriever             *retrieval.SparseRetriever
	mcpBridge             *mcp.MCPIntegrationBridge
	mcpCancel             context.CancelFunc
	mcpDone               <-chan struct{}
	approvals             *approval.Broker
	approvalsRelease      func() error
	toolStore             *store.ToolStore
	ouroborosCtx          context.Context
	ouroborosCancel       context.CancelFunc
	ouroborosDone         <-chan struct{}
	generatedExecutor     *orchestratorToolExecutor
	ouroborosQueue        chan core.ToolNeed
	projectDB             *sql.DB
	atomLoader            *prompt.AtomLoader
	jitCompiler           *prompt.JITPromptCompiler
	rolePrompts           types.RolePromptCompiler
	promptAssembler       *articulation.PromptAssembler
	shardManager          *coreshards.ShardManager
	executor              tactile.Executor
	sessionExecutor       *session.Executor
	sessionSpawner        *session.Spawner
	taskExecutor          session.TaskExecutor
	poiesis               *autopoiesis.Orchestrator
	promptEvolver         *promptevolution.PromptEvolver
	contextFeedback       *ctxlearn.ContextFeedbackStore
	browserMgr            *browser.SessionManager
	researchBrowser       *research.BrowserRuntime
	scanner               *world.Scanner
	tracker               *usage.Tracker
	projectDoc            *projectdoc.Document // nerd.md, nil when absent or invalid
	// sessionID is the process-unique session identity minted in
	// initFinalExecutors (or taken from BootConfig.SessionID). It is the
	// single source of truth propagated to the session executor, the
	// spawner, and the Cortex accessor.
	sessionID  string
	sessionLog *sessionlog.Handle
	// onDemandStop halts the runtime activate_shard watcher. It is set
	// where system shards boot and transferred to the Cortex aggregate.
	onDemandStop func()
}

func initCoreComponents(bctx *bootContext) error {
	// resolveWorkspaceRoot, not a second copy of its logic.
	//
	// This used to inline the same "cfg.Workspace, else FindWorkspaceRoot, else
	// Getwd" cascade — everything resolveWorkspaceRoot does except the part that
	// matters: binding the containment boundary for the modular file tools. The
	// binding therefore only happened on the GetOrBootCortex path (the one-shot
	// CLI), and never on BootCortexWithConfig, which is the path interactive
	// chat and every shard actually boot through. `nerd -w <dir> chat` booted
	// Cortex against <dir> while its tools still resolved relative paths under
	// the process CWD, so a glob in the chat session listed the wrong tree.
	//
	// Two copies of one resolution is how that gap opened, so there is now one.
	// It also returns an absolute path, which keeps the boot's effective
	// workspace identical to the string used for cortex cache keying.
	bctx.workspace = resolveWorkspaceRoot(bctx.cfg.Workspace)
	if _, err := tools.CanonicalWorkspaceRoot(bctx.workspace); err != nil {
		return fmt.Errorf("canonical workspace identity: %w", err)
	}
	bctx.apiKey = bctx.cfg.APIKey
	if perception.SharedTaxonomy != nil {
		perception.SharedTaxonomy.SetWorkspace(bctx.workspace)
	}

	userCfgPath := filepath.Join(bctx.workspace, ".nerd", "config.json")
	var appCfg *config.UserConfig
	if bctx.cfg.UserConfigOverride != nil {
		appCfg = bctx.cfg.UserConfigOverride
	} else {
		var err error
		appCfg, err = config.LoadUserConfig(userCfgPath)
		if err != nil {
			return fmt.Errorf("load user config: %w", err)
		}
	}
	if appCfg == nil {
		appCfg = config.DefaultUserConfig()
	}

	// Shared so a host that already owns a tracker for this workspace (the
	// interactive chat model does) meters into the same one instead of racing
	// it for the file. Each owner Closes its own handle; the last one flushes.
	// Acquired after the config loads: price overrides and the event log come
	// from its usage section.
	tracker, err := usage.Shared(bctx.workspace, UsageOptions(appCfg)...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to initialize usage tracker: %v\n", err)
	}
	bctx.tracker = tracker

	// Boot has already parsed .nerd/config.json into appCfg, so hand those
	// logging settings to internal/logging instead of letting it re-read and
	// re-parse the same file. The injected config is pinned so a later
	// ReloadConfig cannot silently revert to disk.
	config.ApplyLoggingConfig(appCfg)
	// The secret-path patterns every tool and the constitution's measurement
	// consult (tools/secret_paths.go), from execution.secret_paths.
	execCfg := appCfg.GetExecution()
	tools.SetSecretPathPatterns(execCfg.ResolvedSecretPaths())
	if err := logging.Initialize(bctx.workspace); err != nil {
		return fmt.Errorf("initialize workspace logging: %w", err)
	}
	bctx.appCfg = appCfg
	// The fact ceilings are process-wide: every kernel this process builds
	// (session, system shards, trial and dream clones) reads them, and until
	// 2026-09-18 none did -- core_limits.max_facts_in_kernel and
	// max_derived_facts_limit bound nothing and the kernels ran on constants.
	bootLimits := appCfg.GetCoreLimits()
	core.ConfigureFactLimits(bootLimits.MaxFactsInKernel, bootLimits.MaxDerivedFactsLimit)
	// Before any store opens: pragmas are applied at open, sized by the class.
	configureSQLPragmas(bootLimits.SQLHostClass)
	// Before the kernels allocate: GC target and soft memory limit.
	configureGoRuntime(bootLimits)
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
	// Configure the process meter before any client exists. The broker counts
	// and records regardless, but until it knows the window it cannot enforce
	// one — and an unconfigured window is reported as zero headroom on every
	// receipt rather than passing silently, so doing this late would be visible
	// but wrong.
	bctx.meterSink = configureBrokerMeter(bctx.appCfg, bctx.workspace)
	bctx.couseLog = configureCoUseLog(bctx.workspace)

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
	// A bare --api-key names no provider or model, so it builds no client of
	// its own: it overrides the key of the provider config.json names.
	if baseLLMClient == nil {
		err := fmt.Errorf("no LLM client configured (missing config or env keys)")
		if strings.TrimSpace(bctx.apiKey) != "" {
			err = fmt.Errorf("no LLM client configured: --api-key overrides the key of the provider set in .nerd/config.json, and none is set")
		}
		if configuredLLMErr != nil {
			err = fmt.Errorf("no LLM client configured (configured backend unusable): %w", configuredLLMErr)
		}
		logging.Get(logging.CategoryContext).Error("%v", err)
		baseLLMClient = &missingLLMClient{err: err}
	}

	localDBPath := filepath.Join(bctx.workspace, ".nerd", "knowledge.db")
	var localDB *store.LocalStore
	rawLLMClient := baseLLMClient
	if db, err := store.NewLocalStore(localDBPath); err == nil {
		localDB = db
		traceStore := createTraceStoreAdapter(db)
		rawLLMClient = perception.NewTracingLLMClient(baseLLMClient, traceStore)
	}
	bctx.localDB = localDB

	// A call whose context names a provider (a shard profile's provider,
	// config.ShardProfileContext) runs on that provider's client, traced like
	// the slots are. Each slot routes with its own router, because the route
	// carries the reasoning effort of the slot whose call it took
	// (config.SecondaryLLMConfig.RouteTo): the planner's /review keeps the
	// planner's effort on the reviewer's route.
	mainScheduled := core.NewScheduledLLMCall("main", rawLLMClient)
	mainScheduled.Router = profileRouter(bctx.appCfg, bctx.appCfg.GetMainLLMConfig(), localDB)
	bctx.llmClient = mainScheduled

	// Optional worker LLM for shards (e.g. local Ollama for cheap testing).
	// When unset, shards share the main client (SuperGrok xai-oauth, API keys, etc.).
	shardRaw := rawLLMClient
	shardServing := bctx.appCfg.GetMainLLMConfig()
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
			shardServing = bctx.appCfg.GetWorkerLLMConfig()
			logging.Get(logging.CategoryPerception).Info("Worker LLM enabled for shards/spawn/create")
		}
	}
	shardScheduled := core.NewScheduledLLMCall("shards", shardRaw)
	shardScheduled.Router = profileRouter(bctx.appCfg, shardServing, localDB)
	bctx.shardLLMClient = shardScheduled

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
			plannerScheduled := core.NewScheduledLLMCall("planner", plannerRaw)
			plannerScheduled.Router = profileRouter(bctx.appCfg, bctx.appCfg.GetPlannerLLMConfig(), localDB)
			bctx.plannerLLMClient = plannerScheduled
			logging.Get(logging.CategoryPerception).Info("Planner LLM enabled for reasoning-intensive intents")
		}
	}

	// Image generation stays on Gemini Nano Banana 2 (gemini-3.1-flash-image) —
	// never the Ollama worker. Attached to ShardManager in initShardManagement.
	// Only when an image block was configured: warning every boot that an
	// optional, unconfigured feature lacks an API key is noise in the
	// problems log, not a defect.
	if bctx.appCfg != nil && bctx.appCfg.Image != nil {
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
	bctx.classificationClient = classificationClientFor(bctx)
	bctx.transducer = perception.NewUnderstandingTransducer(bctx.classificationClient)
	return nil
}

// classificationClientFor resolves the cheapest capable client for per-turn
// intent classification, preferring, in order:
//
//  1. the worker slot's provider — the cheap tier by definition, with thinking
//     disabled (an explanation in front of a labelling task is pure latency);
//  2. the main provider's fast tier (Haiku / Flash-Lite / gpt-4o-mini, or an
//     explicit classification_model);
//  3. the main client, when neither of the above can be built.
//
// Tiers 1 and 2 combine into a call-time failover pair, not just a
// construction-time preference. A worker client builds fine with a valid key
// and then 403s on every call (observed live: OpenRouter attestation gate),
// and without call-time failover that silently degrades every turn's intent
// understanding to a non-LLM guess — policy then derives the wrong
// next_action and the run reports success on work it never did.
func classificationClientFor(bctx *bootContext) perception.LLMClient {
	// The main provider's fast-tier config is the failover secondary, not
	// the worker config.
	mainCfg := bctx.providerCfgForClassification
	var primary perception.LLMClient
	if bctx.appCfg != nil {
		if w := bctx.appCfg.GetWorkerLLMConfig(); w != nil {
			if key := bctx.appCfg.APIKeyForProvider(w.Provider); key != "" {
				poolKeys, poolErr := bctx.appCfg.ResolveAPIKeyPool(w.Provider)
				if poolErr != nil {
					logging.Get(logging.CategoryPerception).Warn("Worker classification credentials unavailable: %v", poolErr)
					return bctx.llmClient
				}
				workerCfg := &perception.ProviderConfig{
					Engine:   "api",
					Provider: perception.Provider(strings.ToLower(strings.TrimSpace(w.Provider))),
					APIKey:   key,
					APIKeys:  poolKeys,
					BaseURL:  firstNonEmpty(w.Endpoint, bctx.appCfg.BaseURL),
					Model:    w.Model,
					// ClassificationModel stays empty here on purpose: it names
					// a model on the MAIN provider, and the classification
					// factory prefers it over Model — so copying it into the
					// worker-tier config sent the main-tier model name to the
					// worker's provider (observed live: Meta's Spark name sent
					// to OpenRouter, 403 on every classification). Empty falls
					// back to the worker's own model.
					ClassificationModel: "",
					Gemini:              bctx.appCfg.GetGeminiConfig(),
					Anthropic:           bctx.appCfg.Anthropic,
					ReasoningEffort:     w.ReasoningEffort,
					// The classifier is sent the config's classification
					// effort for this tier's provider
					// (classification_reasoning_effort, else the provider's
					// lowest), not the worker's working effort.
					ClassificationReasoningEffort: bctx.appCfg.GetClassificationReasoningEffort(w.Provider),
				}
				if w.Anthropic != nil {
					workerCfg.Anthropic = w.Anthropic
				}
				if client, err := perception.NewClassificationClientFromConfig(workerCfg); err == nil && client != nil {
					logging.Get(logging.CategoryPerception).Info(
						"Classification on the worker tier: provider=%s", workerCfg.Provider)
					primary = client
				}
			}
		}
	}

	var secondary perception.LLMClient
	secondaryIsFastTier := false
	if mainCfg != nil {
		if client, err := perception.NewClassificationClientFromConfig(mainCfg); err == nil && client != nil {
			secondary = client
			secondaryIsFastTier = true
		}
	}
	if secondary == nil {
		secondary = bctx.llmClient
	}

	switch {
	case primary != nil && secondary != nil:
		logging.Get(logging.CategoryPerception).Info(
			"Classification failover armed: worker tier primary, main tier secondary")
		return core.NewScheduledLLMCall("classification",
			perception.NewFallbackClient("classification", primary, secondary))
	case primary != nil:
		return core.NewScheduledLLMCall("classification", primary)
	case secondary != nil && secondaryIsFastTier:
		logging.Get(logging.CategoryPerception).Info(
			"Classification on the main provider's fast tier: provider=%s", mainCfg.Provider)
		return core.NewScheduledLLMCall("classification", secondary)
	case secondary != nil:
		// secondary here is the already-scheduled main client: return it
		// directly rather than double-wrapping it in a second scheduler.
		logging.Get(logging.CategoryPerception).Warn(
			"No cheap classification tier available; intent classification runs on the main model (slow on every turn)")
		return secondary
	default:
		logging.Get(logging.CategoryPerception).Warn(
			"No classification client available at all; transducer runs without LLM understanding")
		return bctx.llmClient
	}
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

// NewDomainCortex builds the production kernel: the domain shards of
// defaultKernelShardConfigs, the shared per-turn predicates, and the
// derivation map, evaluated once. initKernel boots it; a test that must see
// what production sees (split joins, facts in their owners' shards) builds
// it too, instead of a single-store RealKernel.
func NewDomainCortex(workspace string) (*core.CortexKernel, error) {
	cortex := core.NewCortexKernel("cortex")

	// Always the domain manifests. per_shard_facts=false was honored for
	// one build on 2026-09-04 (a single catch-all shard) and measured live:
	// every kernel evaluation went from ~0.25 s to 33–40 s and hydrating
	// the 9.4K learned facts from 1.2 s to 70 s, because one store puts
	// every large fact family under every rule that joins it. The domain
	// split is what keeps evaluation fast, so it stays on regardless of the
	// flag; the flag is logged, not honored, until single-store evaluation
	// is made affordable. Correctness under the split is guaranteed by the
	// manifests owning every predicate their rules join (see
	// internal/shards/registration.go and the campaign contract test).
	shardConfigs := defaultKernelShardConfigs(workspace)
	if err := cortex.SetSharedPredicates(shards.SharedPredicates()); err != nil {
		return nil, fmt.Errorf("configure shared kernel predicates: %w", err)
	}

	// Each shard is an independent kernel that parses the schemas, policy and
	// workspace projections on its own, so the seven are built concurrently
	// (measured 2026-10-27: 4.9 s of a 28.6 s boot built one at a time).
	// Registration stays in config order.
	built := make([]*core.KernelShard, len(shardConfigs))
	buildErrs := make([]error, len(shardConfigs))
	var wg sync.WaitGroup
	for i, scfg := range shardConfigs {
		wg.Add(1)
		go func(i int, scfg core.KernelShardConfig) {
			defer wg.Done()
			built[i], buildErrs[i] = core.NewKernelShard(scfg)
		}(i, scfg)
	}
	wg.Wait()
	for i, scfg := range shardConfigs {
		if buildErrs[i] != nil {
			return nil, fmt.Errorf("failed to create shard %s: %w", scfg.Domain, buildErrs[i])
		}
		if err := cortex.RegisterShard(built[i]); err != nil {
			return nil, fmt.Errorf("failed to register shard %s: %w", scfg.Domain, err)
		}
	}
	consumerRoutes := shards.PredicateConsumerRoutes()
	if err := cortex.SetPredicateConsumerRoutes(consumerRoutes); err != nil {
		return nil, fmt.Errorf("configure kernel predicate consumer routes: %w", err)
	}

	logging.Boot("kernel: %d domain shards (per_shard_facts=%v recorded, not honored: single-store evaluation measured 35 s/eval vs 0.25 s)",
		len(shardConfigs), features.IsPerShardFactsEnabled())

	// The static derivation map narrows shared replication to the shards
	// whose rules read each per-turn fact and fan-out queries to the
	// shards that can derive the predicate (item 56: without it every
	// intent change dirtied all seven shards and every fan-out query
	// evaluated all of them). A build failure is logged and leaves the
	// kernel on the replicate-everywhere path, which is correct but slow.
	if dm, err := buildKernelDerivationMap(shardConfigs); err != nil {
		logging.Get(logging.CategoryBoot).Warn("kernel: derivation map unavailable, replicating shared facts everywhere: %v", err)
	} else {
		cortex.SetDerivationMap(dm)
	}

	// features.provenance is documented as turning the derivation recorder
	// on so /explain can answer; until 2026-09-25 nothing read it, and /explain
	// only ever switched recording on itself, mid-session, with a forced
	// re-evaluation. Installed here, after the shards exist (EnableProvenance
	// fans out to each) and before the first evaluation, so the first pass is
	// recorded. Measured on the default corpus: 79 ms per evaluation off, 81 ms
	// on; the buffer holds only the latest pass.
	if features.IsProvenanceEnabled() {
		cortex.EnableProvenance()
		logging.Boot("kernel: provenance recording on from boot (features.provenance)")
	}

	if err := cortex.Evaluate(); err != nil {
		return nil, fmt.Errorf("failed to boot cortex kernel: %w", err)
	}
	return cortex, nil
}

func initKernel(bctx *bootContext) error {
	if bctx.cfg.KernelOverride != nil {
		bctx.kernel = bctx.cfg.KernelOverride
	} else {
		cortex, err := NewDomainCortex(bctx.workspace)
		if err != nil {
			return err
		}
		bctx.kernel = cortex
		go refreshOrientationInBackground(bctx.workspace, cortex)
	}
	if err := loadCodeFamilies(bctx.ctx, bctx.workspace, bctx.kernel); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: code families not loaded: %v\n", err)
	}

	if err := perception.InitPerceptionLayerWithContext(bctx.ctx, bctx.kernel, bctx.appCfg); err != nil {
		if bctx.ctx.Err() != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Warning: Perception init failed: %v\n", err)
	} else {
		bctx.perceptionInitialized = true
	}
	if err := bctx.ctx.Err(); err != nil {
		return err
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

// refreshWorldModelAtBoot runs an incremental world scan after the cache is
// loaded and applies the delta, exactly as the TUI post-boot scan does
// (cmd/nerd/chat/helpers_scan.go), minus the KG-link persistence block which
// is TUI-only bookkeeping. It runs on every boot path — nerd chat, nerd fix,
// campaigns, spawned shards — not just the TUI and `nerd run`, which were the
// only callers that rescanned. When the cache is empty the scan.mg fallback in
// initKernel stays as is; this scan still runs so a fresh workspace gets a
// world model without `nerd init`. A scan error is logged at Warn and never
// fails boot. Bound by the boot context.
func refreshWorldModelAtBoot(bctx *bootContext) {
	if bctx == nil || bctx.scanner == nil || bctx.kernel == nil {
		return
	}
	if bctx.workspace == "" {
		return
	}
	ctx := bctx.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	res, err := bctx.scanner.ScanWorkspaceIncremental(ctx, bctx.workspace, bctx.localDB, world.IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		logging.Get(logging.CategoryBoot).Warn("world model refresh at boot failed: %v", err)
		return
	}
	if res == nil {
		logging.World("world model unchanged at boot")
		return
	}
	if applyErr := world.ApplyIncrementalResult(bctx.kernel, res); applyErr != nil {
		logging.Get(logging.CategoryBoot).Warn("world model refresh at boot: apply failed: %v", applyErr)
		return
	}
	if res.Unchanged {
		logging.World("world model unchanged at boot: files=%d in %v", res.FileCount, res.Duration)
		return
	}
	changed := len(res.ChangedFiles) + len(res.NewFiles) + len(res.DeletedFiles)
	logging.World("world model refreshed at boot: files=%d changed=%d facts=%d in %v", res.FileCount, changed, len(res.NewFacts), res.Duration)
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
	docs, err := projectdoc.LoadAll(bctx.workspace)
	if err != nil {
		fmt.Fprintf(os.Stderr,
			"Warning: %s is invalid and NONE of its rules are in force (including any write protection): %v\n",
			projectdoc.FileName, err)
		logging.Get(logging.CategoryBoot).Warn("%s rejected, no project rules active: %v", projectdoc.FileName, err)
		return
	}
	if len(docs) == 0 {
		return
	}

	var coreFacts []core.Fact
	for _, doc := range docs {
		for _, f := range doc.Facts() {
			coreFacts = append(coreFacts, core.Fact(f))
		}
	}
	if err := bctx.kernel.LoadFacts(coreFacts); err != nil {
		fmt.Fprintf(os.Stderr,
			"Warning: %d %s document(s) parsed but their rules could not be asserted; write protection is NOT active: %v\n",
			len(docs), projectdoc.FileName, err)
		logging.Get(logging.CategoryBoot).Warn("%d %s document(s) facts rejected by kernel: %v", len(docs), projectdoc.FileName, err)
		return
	}

	// Only the workspace-root document is the project document. Module-level
	// nerd.md files contribute facts but must not become the SYSTEM PROMPT's
	// project instructions — otherwise one module's prose would silently become
	// project-wide. Pre-LoadAll, a workspace with only module docs had no
	// project doc at all (Load returned nil), so we preserve that by leaving
	// bctx.projectDoc nil when no root doc is present. Do not assume position in
	// the slice: identify the root by its Path being the bare file name
	// (projectdoc.FileName) or equivalently its directory resolving to ".".
	var rootDoc *projectdoc.Document
	for _, doc := range docs {
		if doc.Path == projectdoc.FileName || filepath.ToSlash(filepath.Dir(doc.Path)) == "." {
			rootDoc = doc
			break
		}
	}
	bctx.projectDoc = rootDoc

	totalForbid := 0
	totalCommands := 0
	for _, d := range docs {
		totalForbid += len(d.Spec.Forbid)
		totalCommands += d.CommandCount()
	}
	if rootDoc != nil {
		logging.Boot("Loaded %d %s document(s) (%s: %d facts, %d write-protected path(s), %d command(s))",
			len(docs), projectdoc.FileName, rootDoc.Path, len(coreFacts), totalForbid, totalCommands)
	} else {
		logging.Boot("Loaded %d %s document(s) (%d facts, %d write-protected path(s), %d command(s)) [no workspace root document]",
			len(docs), projectdoc.FileName, len(coreFacts), totalForbid, totalCommands)
	}
}

// firstNonEmpty returns the first argument that is not blank.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

// buildKernelDerivationMap runs the static cross-shard analysis over the
// embedded corpus with the manifests' ownership and the shared per-turn set.
// The same analysis gates the test suite (internal/shards/shard_join_audit_test.go).
func buildKernelDerivationMap(shardConfigs []core.KernelShardConfig) (*core.DerivationMap, error) {
	schemas, policy, err := core.DefaultCorpusText()
	if err != nil {
		return nil, err
	}
	owners := make(map[string]string)
	for _, scfg := range shardConfigs {
		for _, p := range scfg.OwnedPredicates {
			owners[p] = scfg.Domain
		}
	}
	shared := make(map[string]struct{})
	for _, p := range shards.SharedPredicates() {
		shared[p] = struct{}{}
	}
	return core.BuildDerivationMapWithConsumerRoutes(schemas+"\n"+policy, nil, owners, shared, shards.PredicateConsumerRoutes(), "cortex")
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

	var groundedSearcher types.GroundedWebSearcher
	if gws, ok := bctx.llmClient.(types.GroundedWebSearcher); ok {
		groundedSearcher = gws
	}
	if err := bctx.virtualStore.HydrateModularTools(groundedSearcher); err != nil {
		logging.Get(logging.CategorySession).Warn("Failed to hydrate modular tools: %v", err)
	}

	// Static tools from available_tools.json plus generated tools from disk.
	// Warnings are logged, not fatal, so a corrupt tools dir never fails boot.
	nerdDir := filepath.Join(bctx.workspace, ".nerd")
	if staticDefs, err := WorkspaceStaticToolDefs(nerdDir); err != nil {
		logging.Get(logging.CategorySession).Warn("Failed to load available_tools.json: %v", err)
	} else if len(staticDefs) > 0 {
		if err := bctx.virtualStore.HydrateStaticTools(staticDefs); err != nil {
			logging.Get(logging.CategorySession).Warn("Failed to hydrate static tools: %v", err)
		}
	}
	if err := bctx.virtualStore.HydrateToolsFromDisk(nerdDir); err != nil {
		logging.Get(logging.CategorySession).Warn("Failed to hydrate tools from disk: %v", err)
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
	engineCfg := embedCfg.EngineConfig()
	if engineCfg.Provider == "genai" && engineCfg.GenAIAPIKey == "" && bctx.apiKey != "" {
		engineCfg.GenAIAPIKey = bctx.apiKey
	}
	if engine, err := embedding.NewEngineWithContext(bctx.ctx, engineCfg); err == nil {
		if checker, ok := engine.(embedding.HealthChecker); ok {
			if err := checker.HealthCheck(bctx.ctx); err != nil {
				logging.Get(logging.CategoryEmbedding).Warn("Embedding engine health check failed: %v", err)
				fmt.Fprintf(os.Stderr, "Warning: Embedding engine unavailable: %v\n", err)
				if closer, ok := engine.(interface{ Close() error }); ok {
					if closeErr := closer.Close(); closeErr != nil {
						if bctx.ctx.Err() != nil {
							return errors.Join(bctx.ctx.Err(), err, closeErr)
						}
						logging.Get(logging.CategoryEmbedding).Warn("Failed to close unhealthy embedding engine: %v", closeErr)
					}
				}
				if bctx.ctx.Err() != nil {
					return errors.Join(bctx.ctx.Err(), err)
				}
			} else {
				bctx.embeddingEngine = engine
			}
		} else {
			bctx.embeddingEngine = engine
		}
	} else {
		if bctx.ctx.Err() != nil {
			return errors.Join(bctx.ctx.Err(), err)
		}
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
	// The kernel-injection bounds come from jit.* in config; without these
	// lines a user-set value would never reach a compile.
	compilerCfg.KernelContextRows = bctx.jitCfg.KernelContextRows
	compilerCfg.KernelContextRowChars = bctx.jitCfg.KernelContextRowChars
	compilerCfg.KernelInjectedAtomChars = bctx.jitCfg.KernelInjectedAtomChars
	compilerCfg.SpecialistKnowledgeBlocks = bctx.jitCfg.SpecialistKnowledgeBlocks
	compilerCfg.SpecialistTopicChars = bctx.jitCfg.SpecialistTopicChars
	compilerCfg.SpecialistBlockChars = bctx.jitCfg.SpecialistBlockChars
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
					if counts, err := prompt.ReconcilePromptCorpus(bctx.ctx, projectDB, embeddedCorpus.All()); err != nil {
						logging.Get(logging.CategoryContext).Warn("Failed to reconcile project corpus: %v", err)
					} else {
						logging.Get(logging.CategoryContext).Info("Reconciled project corpus: upserted=%d deleted=%d retained_embeddings=%d cleared_embeddings=%d", counts.Upserted, counts.Deleted, counts.RetainedEmbeddings, counts.ClearedEmbeddings)
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
	rolePrompts, err := prompt.NewRoleCompiler(jitCompiler, bctx.jitCfg)
	if err != nil {
		return fmt.Errorf("failed to init role prompt compiler: %w", err)
	}
	bctx.rolePrompts = rolePrompts
	// Ownership of the project DB transfers to JITCompiler on success.
	bctx.projectDB = nil
	// The kernel's rule generator and the MCP tool analyzer compile their
	// system prompts from role atoms, so both are wired after the compiler.
	if rk, ok := bctx.kernel.(*core.RealKernel); ok {
		rk.SetRolePrompts(bctx.rolePrompts)
	}
	// The learning critic (perception.LearnFromInteraction) compiles its
	// system prompt from the learning_critic role atoms.
	if perception.SharedTaxonomy != nil {
		perception.SharedTaxonomy.SetRolePrompts(bctx.rolePrompts)
	}
	initMCPBridge(bctx)

	if bctx.localDB != nil {
		bctx.jitCompiler.SetLocalDB(bctx.localDB)
	}
	if bctx.learningStore != nil {
		bctx.jitCompiler.SetLearningStore(bctx.learningStore)
	}
	// Per-turn Jev relevance of flesh atoms (typesafe.atom_relevance):
	// vector search nominates, Jev judges, jit_compiler.mg admits.
	if jev, err := typesafe.FromConfig(bctx.appCfg.TypeSafe); err != nil {
		logging.Get(logging.CategoryBoot).Warn("Jev atom relevance off: %v", err)
	} else if jev != nil {
		if use := atomjudge.RelevanceFor(jev, bctx.appCfg.TypeSafe); use.Judge != nil {
			bctx.jitCompiler.SetAtomRelevance(use)
			logging.Get(logging.CategoryBoot).Info("Jev atom relevance on: pool %d, timeout %s, yes %d%% / no %d%%", use.Pool, use.Timeout, use.YesPercent, use.NoPercent)
		}
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

	// Wire the kernel into the transducer so routing derives from Mangle
	// tables (mode/shard/context/tool affinities) instead of passing the
	// LLM's suggestions through verbatim. Without this, deriveRouting takes
	// its no-kernel fallback on every turn and the routing corpus plus the
	// per-turn current_understanding/derived_* facts are dead paths.
	// Extraction mirrors initExecutionLayer: a bare RealKernel passes
	// through, a CortexKernel contributes its primary shard's kernel.
	if bctx.transducer != nil {
		tk, ok := bctx.transducer.(perception.TransducerWithKernel)
		if !ok {
			logging.Get(logging.CategoryPerception).Warn(
				"transducer %T has no kernel port; routing falls back to LLM suggestions",
				bctx.transducer)
		} else {
			var routingKernel *core.RealKernel
			if rk, ok := bctx.kernel.(*core.RealKernel); ok {
				routingKernel = rk
			} else if ck, ok := bctx.kernel.(*core.CortexKernel); ok {
				routingKernel = ck.GetPrimaryRealKernel()
			}
			if routingKernel != nil {
				tk.SetKernel(routingKernel)
			} else {
				logging.Get(logging.CategoryPerception).Warn(
					"transducer routing kernel unavailable; routing falls back to LLM suggestions")
			}
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
	configuredBrowser := bctx.appCfg.GetBrowserConfig()
	if err := ensureBrowserPolicyParams(bctx.kernel, configuredBrowser); err != nil {
		return err
	}

	autopoiesisConfig := autopoiesis.DefaultConfig(bctx.workspace)
	bctx.poiesis = autopoiesis.NewOrchestrator(bctx.llmClient, autopoiesisConfig)
	// An agent autopoiesis authors is written by the one agent-definition
	// writer: init's atoms, its system prompt admitted as methodology.
	bctx.poiesis.SetAgentDefinitionWriter(func(ctx context.Context, workspace, name, role, topics, systemPrompt string) (string, error) {
		return WriteAgentDefinition(ctx, workspace, name, role, topics, systemPrompt, "")
	})
	// Orchestrator now takes types.Kernel directly, so the AutopoiesisBridge adapter is no longer needed here - step 2 of the KernelInterface deprecation plan documented on types.KernelInterface.
	bctx.poiesis.SetKernel(bctx.kernel)

	if ouroborosLoop := bctx.poiesis.GetOuroborosLoop(); ouroborosLoop != nil {
		bctx.virtualStore.SetToolGenerator(ouroborosLoop)
	}
	// Executions go through the orchestrator's evaluate-and-refine path, not
	// the bare loop, so tool quality is learned on every boot path.
	// Refinement outlives the tool call and stops when Cortex.Close cancels
	// this context. Created here so the executor and the dream-queue loop
	// share one lifetime.
	bctx.generatedExecutor = newOrchestratorToolExecutor(bctx.poiesis, ensureOuroborosLifetime(bctx))
	bctx.virtualStore.SetToolExecutor(bctx.generatedExecutor)

	browserCfg := browser.DefaultConfig()
	browserCfg.DebuggerURL = configuredBrowser.DebuggerURL
	browserCfg.Launch = append([]string(nil), configuredBrowser.Launch...)
	browserCfg.Reaper = configuredBrowser.Reaper
	browserCfg.Headless = configuredBrowser.Headless
	browserCfg.ViewportWidth = configuredBrowser.ViewportWidth
	browserCfg.ViewportHeight = configuredBrowser.ViewportHeight
	browserCfg.NavigationTimeoutMs = configuredBrowser.NavigationTimeoutMs
	browserCfg.MultiTabDefault = configuredBrowser.MultiTabDefault
	browserCfg.MaxTabs = configuredBrowser.MaxTabs
	browserCfg.MaxBrowsers = configuredBrowser.MaxBrowsers
	browserCfg.IdleTabTimeoutMs = configuredBrowser.IdleTabTimeoutMs
	browserCfg.ExtraSensitiveKeys = append([]string(nil), configuredBrowser.ExtraSensitiveKeys...)
	browserCfg.WorkspaceRoot = bctx.workspace
	browserCfg.WritableRoots = append([]string(nil), configuredBrowser.WritableRoots...)
	browserCfg.CorrelationContainers = append([]string(nil), configuredBrowser.CorrelationContainers...)
	browserCfg.EvidenceEnabled = configuredBrowser.EvidenceEnabled
	browserCfg.EvidenceDir = configuredBrowser.EvidenceDir
	browserCfg.MaxEvidenceFiles = configuredBrowser.MaxEvidenceFiles
	browserCfg.MaxEvidenceFileBytes = configuredBrowser.MaxEvidenceFileBytes
	browserCfg.Specs = configuredBrowser.Specs
	var allowedBinaries []string
	if bctx.appCfg != nil {
		allowedBinaries = bctx.appCfg.GetExecution().AllowedBinaries
	}
	browserCfg.DockerPath = browser.LookupDockerBinary(allowedBinaries)
	browserCfg.SessionStore = filepath.Join(bctx.workspace, ".nerd", "browser", "sessions.json")
	bctx.browserMgr = browser.NewSessionManagerWithSink(browserCfg, browserKernelSink{kernel: bctx.kernel})

	// Close the read loop. browserKernelSink is write-only, so the manager could
	// assert element evidence and never read the is_honeypot verdict derived
	// from it — the honeypot gate failed open in the Cortex path, which is the
	// path shards actually use. The retractor is what makes epoch GC do more
	// than publish a watermark.
	bctx.browserMgr.SetFactQuerier(browser.NewKernelFactQuerier(bctx.kernel))
	bctx.browserMgr.SetFactRetractor(browser.NewKernelFactRetractor(bctx.kernel))

	// The research runtime owns the headless browser page reads launch lazily.
	// It inherits the operator's browser binary, cleanup bounds, redaction keys
	// and limits, and Cortex.Close (boot rollback included) closes it.
	bctx.researchBrowser = research.SetBrowserRuntime(bctx.browserMgr, bctx.kernel, research.BrowserRuntimeOptions{
		Launch:             append([]string(nil), configuredBrowser.Launch...),
		Reaper:             configuredBrowser.Reaper,
		ExtraSensitiveKeys: append([]string(nil), configuredBrowser.ExtraSensitiveKeys...),
		MaxTabs:            configuredBrowser.MaxTabs,
		MaxBrowsers:        configuredBrowser.MaxBrowsers,
	})
	// run_check runs a campaign's declared acceptance argv through the same
	// audited executor the orchestrator's acceptance round uses (Cortex.Executor).
	shell.SetCheckExecutor(bctx.virtualStore.AuditedExecutor())
	return nil
}

func ensureBrowserPolicyParams(kernel SystemKernel, configured config.BrowserAutomationConfig) error {
	if invalid := config.Errors(configured.Policy.Check("browser.policy")); len(invalid) > 0 {
		problems := make([]error, 0, len(invalid))
		for _, problem := range invalid {
			problems = append(problems, errors.New(problem.String()))
		}
		return fmt.Errorf("validate browser policy configuration: %w", errors.Join(problems...))
	}
	if kernel == nil {
		return fmt.Errorf("publish browser policy parameters: no system kernel")
	}
	if err := config.EnsureParams(kernel, configured.Policy.Params()); err != nil {
		return fmt.Errorf("publish browser policy parameters: %w", err)
	}
	return nil
}

// browserKernelSink keeps browser evidence in the Cortex's live kernel. The
// standalone browser mangle.Engine used before this adapter created a private
// fact island that no planner, shard, or policy query could observe.
type browserKernelSink struct {
	kernel SystemKernel
}

func (s browserKernelSink) AddFacts(facts []mangle.Fact) error {
	if s.kernel == nil || len(facts) == 0 {
		return nil
	}
	converted := make([]core.Fact, 0, len(facts))
	for _, fact := range facts {
		converted = append(converted, core.Fact{
			Predicate: fact.Predicate,
			Args:      append([]any(nil), fact.Args...),
		})
	}
	return s.kernel.AssertBatch(converted)
}

// shardRegistryContext is what every registered shard factory is built with.
func shardRegistryContext(bctx *bootContext) shards.RegistryContext {
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
		RolePrompts:  bctx.rolePrompts,
		World:        bctx.appCfg.GetWorldConfig(),
	}
	// The firewall classifies through the same resolved client as the
	// direct transducer. Until 2026-10-28 it was rebuilt here from the worker
	// config alone, so a worker that failed at call time degraded the
	// firewall's intent (nil error, chat treats perception as handled) and
	// the main-tier secondary the direct transducer would have used was never
	// asked (Docs/audits/stream-completion-contract.md, appendix N3).
	regCtx.ClassificationClient = bctx.classificationClient
	return regCtx
}

func initShardManagement(bctx *bootContext) error {
	coreLimits := bctx.appCfg.GetCoreLimits()
	limitsEnforcer := core.NewLimitsEnforcer(core.LimitsConfig{
		MaxTotalMemoryMB:     coreLimits.MaxTotalMemoryMB,
		MaxConcurrentShards:  coreLimits.MaxConcurrentShards,
		MaxFactsInKernel:     coreLimits.MaxFactsInKernel,
		MaxDerivedFactsLimit: coreLimits.MaxDerivedFactsLimit,
	})
	bctx.shardManager.SetLimitsEnforcer(limitsEnforcer)

	spawnQueue := coreshards.NewSpawnQueue(bctx.shardManager, limitsEnforcer, coreshards.DefaultSpawnQueueConfig())
	bctx.shardManager.SetSpawnQueue(spawnQueue)
	_ = spawnQueue.Start()

	regCtx := shardRegistryContext(bctx)
	shards.RegisterAllShardFactories(bctx.shardManager, regCtx)

	nerdDir := filepath.Join(bctx.workspace, ".nerd")
	bctx.shardManager.SetNerdDir(nerdDir)
	bctx.shardManager.SetPromptLoader(func(ctx context.Context, agentName, nerdDir string) (int, error) {
		return prompt.LoadAgentPrompts(ctx, agentName, nerdDir, bctx.embeddingEngine)
	})
	bctx.shardManager.SetJITRegistrar(prompt.CreateJITDBRegistrar(bctx.jitCompiler))
	bctx.shardManager.SetJITUnregistrar(prompt.CreateJITDBUnregistrar(bctx.jitCompiler))

	// Sync all agent prompts.yaml -> knowledge DBs at boot (upsert semantics).
	// This must run AFTER the agent knowledge DBs exist and AFTER the JIT
	// registrar is wired so the freshly synced atoms are visible to the
	// compiler in the same boot; otherwise the JIT would serve stale prompts
	// until the next TUI/init run.
	if promptCount, syncErr := prompt.ReloadAllPrompts(bctx.ctx, nerdDir, bctx.embeddingEngine); syncErr != nil {
		logging.BootWarn("Failed to sync agent prompts: %v", syncErr)
	} else if promptCount > 0 {
		logging.Boot("Synced %d prompt atoms from YAML to knowledge DBs", promptCount)
	}

	bctx.shardManager.RegisterShard("tactile_router", func(id string, _ types.ShardConfig) types.ShardAgent {
		shard := system.NewTactileRouterShard()
		shard.SetParentKernel(bctx.kernel)
		shard.SetVirtualStore(bctx.virtualStore)
		shard.SetLLMClient(regCtx.LLMClient)
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
		shard.SetLLMClient(regCtx.LLMClient)
		if setter, ok := any(shard).(interface{ SetJITConfig(config.JITConfig) }); ok {
			setter.SetJITConfig(bctx.jitCfg)
		}
		shard.SetWorkspaceRoot(bctx.workspace)
		shard.SetShardManager(bctx.shardManager)
		// Read at spawn time, not registration time: this closure runs on
		// demand, so the executor built later in boot is already in place.
		// Without it the orchestrator's verification checkpoints cannot run.
		shard.SetTaskExecutor(bctx.taskExecutor)
		// The supervised campaigns replan on the planning tier (Cortex.
		// PlanningLLM), not on regLLM, the worker the shard itself runs on.
		shard.SetPlanning(planningLLM(bctx.llmClient, bctx.plannerLLMClient), bctx.promptAssembler)
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

	// features.IsSystemShardsEnabled is documented as "the master switch for
	// booting the autopoiesis/observer background shards" and is reported by
	// `nerd features`, but it had no non-test caller anywhere: setting
	// CODENERD_SYSTEM_SHARDS=0 did nothing at all. A flag an operator can see,
	// read a description of, and set with no effect is worse than an absent
	// one — it makes them believe they have turned something off.
	//
	// Gated here rather than inside ShardManager so the per-shard
	// --disable-system-shard flag above and this master switch stay in one
	// place, and so internal/core/shards keeps its independence from
	// internal/features.
	if !features.IsSystemShardsEnabled() {
		logging.Boot("System shards disabled by CODENERD_SYSTEM_SHARDS; skipping StartSystemShards")
		return nil
	}

	if err := bctx.shardManager.StartSystemShards(bctx.ctx); err != nil {
		return fmt.Errorf("failed to start system shards: %w", err)
	}
	bctx.onDemandStop = startOnDemandWatcher(bctx)
	return nil
}

// startOnDemandWatcher subscribes the shard manager to kernel fact events so
// on-demand system shards (world_model_ingestor, tactile_router,
// session_planner) actually start when their activate_shard rules derive at
// runtime. StartSystemShards queries activate_shard once at boot, before any
// trigger fact exists; without this watcher those derivations fire into a
// void and the shards never run.
func startOnDemandWatcher(bctx *bootContext) func() {
	noop := func() {}
	if bctx == nil || bctx.shardManager == nil || bctx.kernel == nil {
		return noop
	}
	provider, ok := bctx.kernel.(interface{ GetEventBus() *core.FactEventBus })
	if !ok || provider == nil {
		logging.Boot("On-demand shard activation disabled: kernel has no fact event bus")
		return noop
	}
	bus := provider.GetEventBus()
	if bus == nil {
		logging.Boot("On-demand shard activation disabled: fact event bus is nil")
		return noop
	}
	mgr := bctx.shardManager
	return core.StartOnDemandWatcher(bctx.ctx, bus, func(ctx context.Context) []string {
		return mgr.EnsureOnDemandShards(ctx)
	})
}

// registerUserAgentConfigAtoms gives every user-defined agent in
// .nerd/agents.json a config atom under both verb shapes the executor can see:
// "/<name>" (any bare agent name, lower-cased by the task executor's
// intentFor) and "/consult/<name>" (SpawnConsultation and campaign
// specialists).
//
// The catalog a turn enforces is turn_tool_allowed. compileConfig overwrites
// whatever this atom lists, so the declared tools are also asserted as
// user_agent_declared_tool facts (policy/intent_routing_rules.mg). That rule
// unions them with the /general floor: DeriveTurnTools returns the first
// non-empty derivation, and a specialist that declared go_build would
// otherwise lose read_file. A declared name that is neither
// is_tool_registered nor already inside a persona envelope refuses that
// agent only: no atom and no facts for it, a warning names the agent and
// the tool, and every other agent is still registered. One agent's tools
// are all checked before any of that agent's atoms or facts are written,
// so a later bad name cannot leave a partial grant for the same agent.
// A kernel failure still returns an error and writes nothing.
//
// Every mutation still has to derive permitted(...) in the kernel.
func registerUserAgentConfigAtoms(provider *prompt.DefaultConfigAtomProvider, kernel core.Kernel, workspace string) error {
	if provider == nil {
		return nil
	}
	base, ok := provider.GetAtom("/general")
	if !ok {
		return nil
	}

	// Hard errors (a name that is not a verb, a kernel query failure) still
	// write nothing: grants are applied only after this loop. A tool the
	// host has not registered is not one of those errors. It drops that
	// agent and the loop continues, which is what keeps one optional agent
	// from stopping boot.
	grants := make([]userAgentGrant, 0)
	for _, def := range LoadUserAgentDefinitions(workspace) {
		name := strings.ToLower(strings.TrimSpace(def.Name))
		if name == "" {
			continue
		}
		verb, ok := userAgentCanonicalVerb(name)
		if !ok {
			return fmt.Errorf("user agent %q has a name that is not a queryable intent verb", def.Name)
		}
		declared, refusals, err := userAgentDeclaredTools(kernel, def.Name, def.Tools)
		if err != nil {
			return err
		}
		if len(refusals) > 0 {
			for _, refused := range refusals {
				warnRefusedUserAgent(refused.agent, refused.tool)
			}
			continue
		}

		seen := make(map[string]struct{}, len(base.Tools)+len(declared))
		merged := make([]string, 0, len(base.Tools)+len(declared))
		for _, tool := range append(append([]string(nil), base.Tools...), declared...) {
			if _, dup := seen[tool]; dup {
				continue
			}
			seen[tool] = struct{}{}
			merged = append(merged, tool)
		}
		grants = append(grants, userAgentGrant{
			name: name,
			verb: verb,
			atom: prompt.ConfigAtom{
				Tools:    merged,
				Policies: append([]string(nil), base.Policies...),
				// Above /general so an explicit agent atom wins a merge, below the
				// built-in personas so a name collision cannot quietly demote /fix.
				Priority: base.Priority + 5,
			},
			declared: declared,
		})
	}

	var facts []types.Fact
	for _, g := range grants {
		provider.RegisterAtom("/"+g.name, g.atom)
		provider.RegisterAtom("/consult/"+g.name, g.atom)
		logging.Get(logging.CategoryContext).Info(
			"Registered config atom for user agent %q (%d tools)", g.name, len(g.atom.Tools))
		for _, tool := range g.declared {
			facts = append(facts, types.Fact{
				Predicate: "user_agent_declared_tool",
				Args:      []any{g.verb, "/" + tool},
			})
		}
	}
	if len(facts) == 0 {
		return nil
	}
	if kernel == nil {
		return fmt.Errorf("user agent tools cannot be recorded: no kernel")
	}
	if err := kernel.AssertBatch(facts); err != nil {
		return fmt.Errorf("record user agent tools: %w", err)
	}
	return nil
}

// userAgentGrant is one validated agent, held until every agent in the
// registry has been checked.
type userAgentGrant struct {
	name     string
	verb     string
	atom     prompt.ConfigAtom
	declared []string
}

// userAgentCanonicalVerb is the atom DeriveTurnTools queries. The name is
// already lowercased. Hyphens become underscores because a Mangle atom
// cannot spell one, which is the same rewrite canonicalTurnVerb applies
// (that function is unexported in prompt, and this package already imports
// prompt). Anything else is refused by the caller, named.
func userAgentCanonicalVerb(name string) (string, bool) {
	name = strings.ReplaceAll(name, "-", "_")
	if name == "" {
		return "", false
	}
	for i := 0; i < len(name); i++ {
		c := name[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return "", false
		}
	}
	return "/" + name, true
}

// refusedUserAgentTool is one declared name the host does not know.
// The agent that named it is skipped whole; the warning prints this pair.
type refusedUserAgentTool struct {
	agent string
	tool  string
}

// warnRefusedUserAgent is how a bad optional agent stays visible without
// stopping boot. loadProjectDoc does the same for a malformed nerd.md:
// stderr is what the operator sees (the boot logger is a no-op unless
// debug_mode is on), and the boot warning is mirrored into the problems
// log when file logging is on.
func warnRefusedUserAgent(agent, tool string) {
	msg := fmt.Sprintf("user agent %q declares tool %q, which is not a registered tool", agent, tool)
	fmt.Fprintf(os.Stderr, "Warning: %s\n", msg)
	logging.Get(logging.CategoryBoot).Warn("%s", msg)
}

// userAgentDeclaredTools keeps the declared names that the host can see,
// in order, dropping blanks and duplicates. A name that is not a Mangle
// tool atom, or that neither is_tool_registered nor persona_tool_allowed
// derives, is a refusal of the whole agent: the declared slice comes back
// empty so a caller cannot grant the prefix of a list that also named a
// bad tool. The tool string is interpolated into a query only after the
// atom-shape check. A query failure is an error, not a refusal.
func userAgentDeclaredTools(kernel core.Kernel, agent string, tools []string) ([]string, []refusedUserAgentTool, error) {
	seen := make(map[string]struct{}, len(tools))
	pending := make([]string, 0, len(tools))
	for _, tool := range tools {
		tool = strings.TrimSpace(tool)
		if tool == "" {
			continue
		}
		if _, dup := seen[tool]; dup {
			continue
		}
		seen[tool] = struct{}{}
		pending = append(pending, tool)
	}
	if len(pending) == 0 {
		return nil, nil, nil
	}
	if kernel == nil {
		return nil, nil, fmt.Errorf("user agent tools cannot be recorded: no kernel")
	}
	declared := make([]string, 0, len(pending))
	var refusals []refusedUserAgentTool
	for _, tool := range pending {
		ok, err := kernelToolIsRegistered(kernel, agent, tool)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			refusals = append(refusals, refusedUserAgentTool{agent: agent, tool: tool})
			continue
		}
		declared = append(declared, tool)
	}
	if len(refusals) > 0 {
		return nil, refusals, nil
	}
	return declared, nil, nil
}

func kernelToolIsRegistered(kernel core.Kernel, agent, tool string) (bool, error) {
	if kernel == nil {
		return false, fmt.Errorf("user agent tools cannot be recorded: no kernel")
	}
	if !mangleToolName(tool) {
		return false, nil
	}
	reg, err := kernel.Query(fmt.Sprintf("is_tool_registered(%q)", tool))
	if err != nil {
		return false, fmt.Errorf("user agent %q: is_tool_registered(%q): %w", agent, tool, err)
	}
	if len(reg) > 0 {
		return true, nil
	}
	env, err := kernel.Query(fmt.Sprintf("persona_tool_allowed(Persona, /%s)", tool))
	if err != nil {
		return false, fmt.Errorf("user agent %q: persona_tool_allowed for %q: %w", agent, tool, err)
	}
	return len(env) > 0, nil
}

// mangleToolName is the tool-atom shape persona_tool_allowed uses: a
// lowercase name constant with no slash. The registry is case-sensitive,
// so this does not lowercase a declared name into a different tool.
func mangleToolName(tool string) bool {
	if tool == "" || tool[0] < 'a' || tool[0] > 'z' {
		return false
	}
	for i := 1; i < len(tool); i++ {
		c := tool[i]
		if (c < 'a' || c > 'z') && (c < '0' || c > '9') && c != '_' {
			return false
		}
	}
	return true
}

// installContextProviders wires the holographic file-context provider and the
// impacted-test tools to the kernel they read.
//
// Both used to be built on GetPrimaryRealKernel, the catch-all shard.
// code_calls, dependency_link, code_element and element_modified
// are owned by the world shard, and context_priority_file is derived there
// from those facts. Measured on NewDomainCortex (2026-09-28): each of those
// predicates had 0 rows in the catch-all and was present on CortexKernel.Query,
// so every production turn rendered unordered callers and run_impacted_tests
// reported that no edited refs were found. Neither provider asserts; Query is the
// whole surface, and CortexKernel already implements it. A non-cortex kernel
// (a test override) gets no provider, which is what the old type switch did.
func installContextProviders(kernel core.Kernel, workspace string) session.FileContextProvider {
	ck, ok := kernel.(*core.CortexKernel)
	if !ok || ck == nil {
		return nil
	}
	provider := world.NewHolographicProvider(ck, workspace)
	wireTestImpactProvider(ck, workspace)
	return provider
}

func initFinalExecutors(bctx *bootContext) error {
	worldCfg := bctx.appCfg.GetWorldConfig()
	bctx.scanner = world.NewScannerWithConfig(world.ScannerConfig{
		MaxConcurrency:  worldCfg.FastWorkers,
		IgnorePatterns:  worldCfg.IgnorePatterns,
		MaxASTFileBytes: worldCfg.MaxFastASTBytes,
	})

	// Every boot path refreshes the world model: the cache may hold facts from
	// weeks ago (only the TUI and `nerd run` rescanned), so loading it without
	// a delta leaves every CodeDOM/world rule joining stale, half mis-keyed
	// facts. A scan error is logged and never fails boot.
	refreshWorldModelAtBoot(bctx)

	sessionVS := &sessionVirtualStoreAdapter{vs: bctx.virtualStore}
	// CLI spawn/create/TaskExecutor use the worker LLM when configured
	// (local Ollama), not the main TUI Grok client.
	taskLLM := bctx.llmClient
	if bctx.shardLLMClient != nil {
		taskLLM = bctx.shardLLMClient
	}
	sessionLLM := &sessionLLMAdapter{client: taskLLM, tracker: bctx.tracker}

	// User-defined agents come from .nerd/agents.json. Registration asserts
	// user_agent_declared_tool so turn_tool_allowed grants the declared
	// tools. An agent that names a tool the host has not registered is
	// refused on its own (no atom, no facts, warning on stderr and in the
	// boot log) and is not a boot error. A kernel failure still is.
	atomProvider := prompt.NewDefaultConfigAtomProvider()
	if err := registerUserAgentConfigAtoms(atomProvider, bctx.kernel, bctx.workspace); err != nil {
		return err
	}
	configFactory := prompt.NewConfigFactory(atomProvider)

	bctx.sessionExecutor = session.NewExecutor(
		bctx.kernel,
		sessionVS,
		sessionLLM,
		bctx.jitCompiler,
		configFactory,
		bctx.transducer,
	)
	// Executor config for every boot. It carries no tool-call or tool-round
	// ceiling: the tool loop runs while the working policy derives no stop
	// (internal/context/working_set.mg), and until 2026-09-18 this block
	// forwarded six core_limits keys that only ever narrowed that.
	//
	// Applied unconditionally, because it also carries WorkspaceRoot for
	// post-edit build verification. Gating the whole config behind "did the user
	// set a core limit?" left verification without a tree to compile on any
	// workspace that took the defaults — a guard that is off by default for most
	// users is not a guard.
	//
	// Its thresholds are the `session` section of .nerd/config.json. The
	// loader has already refused a file whose section does not check, so a
	// Resolve error here is a section the checker let through.
	sessionPolicy, err := bctx.appCfg.GetSessionConfig().Resolve()
	if err != nil {
		return fmt.Errorf("session config: %w", err)
	}
	execCfg := session.ExecutorConfigFrom(sessionPolicy, bctx.appCfg.GetWorkingConfig())
	execCfg.WorkspaceRoot = bctx.workspace
	bctx.sessionExecutor.SetConfig(execCfg)
	// Working-context archives whose owner process is gone, or retired them,
	// can never be redeemed; the retention policy releases them here, at the
	// start of every session (nothing else removes one from a crashed run).
	if report, err := bctx.sessionExecutor.PruneWorkingArchives(); err != nil {
		logging.Get(logging.CategoryBoot).Warn("working-context archives were not pruned: %v", err)
	} else if report.Total > 0 {
		logging.Boot("working context: %s", report)
	}
	// The shard profile's enable_learning gates what the executor records
	// for prompt evolution, per persona.
	if appCfg := bctx.appCfg; appCfg != nil {
		bctx.sessionExecutor.SetLearningPolicy(func(shardType string) bool {
			return appCfg.GetShardProfile(shardType).EnableLearning
		})
		// The persona's model, provider and sampling, on every path that runs a
		// persona -- not only the TUI's delegation.
		profileContext := func(ctx context.Context, shardType string) context.Context {
			return config.ShardProfileContext(ctx, appCfg.GetShardProfile(shardType))
		}
		bctx.sessionExecutor.SetShardProfileContext(profileContext)
		// The spawner does not exist yet; it receives the same hook where it is
		// built, below.
		bctx.shardProfileContext = profileContext
	}
	logging.Boot("Tool loop: continuation derived by the working policy, no call or round ceiling; build verification after edits: %v (workspace %s)",
		execCfg.VerifyBuildAfterEdits, execCfg.WorkspaceRoot)

	// Every boot mints one process-unique session identity in the same form
	// the headless chat path uses (`session-<unixnano>`), unless the caller
	// supplied one via BootConfig (the TUI passes its loaded session ID so a
	// resumed session keeps its identity). Previously only cmd_chat set an ID,
	// so every campaign task, run/fix/create/review turn, and sub-agent
	// persisted under "default", interleaving unrelated runs.
	if sid := strings.TrimSpace(bctx.cfg.SessionID); sid != "" {
		bctx.sessionID = sid
	} else {
		bctx.sessionID = fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	bctx.sessionExecutor.SetSessionID(bctx.sessionID)
	startSessionLog(bctx)
	// An approvable denial asks the person through the process's broker when
	// a host (the chat TUI) is attached, and stays a refusal otherwise.
	bctx.sessionExecutor.SetApprovalBroker(bctx.approvals)
	// Hand the executor its usage meter so ProcessWithIntent tags the turn
	// context and snapshotTurnUsage reads spend instead of zeros. Nil-safe:
	// when metering failed to boot, turns keep the previous behaviour.
	bctx.sessionExecutor.SetUsageTracker(bctx.tracker)
	// The build critic's system prompt is a role compile (session/role_prompts.go).
	bctx.sessionExecutor.SetRolePrompts(bctx.rolePrompts)
	// Every tool offer and call, on every channel, goes to the tool ledger the
	// autopoiesis profile store keeps (policy/tool_value.mg classifies from it).
	bctx.sessionExecutor.SetToolMeter(bctx.poiesis.ToolMeter())
	// Wire the VirtualStore's generated-tool registry into the session
	// executor so non-TUI paths see the same Ouroboros tools the TUI wires in
	// session_shared_boot/session_boot. SetOuroborosRegistry is nil-safe: a
	// nil store or registry clears the slot instead of panicking.
	if bctx.virtualStore != nil {
		bctx.sessionExecutor.SetOuroborosRegistry(bctx.virtualStore.GetToolRegistry())
	} else {
		bctx.sessionExecutor.SetOuroborosRegistry(nil)
	}

	if bctx.localDB != nil {
		bctx.sessionExecutor.SetSessionPersister(bctx.localDB)
	}
	// nerd.md instructions reach the prompt here. Its write protection does not
	// depend on this call — that is enforced from the kernel facts asserted in
	// loadProjectDoc, so a construction site that forgets this line loses the
	// prose, not the guarantee.
	bctx.sessionExecutor.SetProjectDoc(bctx.projectDoc)
	// The holographic provider and the impacted-test tools read world-owned
	// predicates. GetPrimaryRealKernel is the catch-all and does not hold
	// them; installContextProviders passes the cortex. See that function.
	fileContextProvider := installContextProviders(bctx.kernel, bctx.workspace)
	if fileContextProvider != nil {
		bctx.sessionExecutor.SetFileContextProvider(fileContextProvider)
	}
	// The structural query tools (find_symbol, package_outline, callers_of,
	// callees_of, unreferenced_symbols) answer from the workspace-wide structure
	// index. It needs no kernel, so it is wired for every boot with a workspace.
	wireStructureProvider(bctx.workspace)
	// The working set above queries code_element for every entity it selects.
	// Nothing populated that predicate in a headless run -- only the CodeDOM
	// scope does, and only VirtualStore.handleOpenFile opens one, which the
	// session path never dispatches. The executor now parses the file each turn
	// is looking at into the layer itself. See session/codedom_scope.go.
	codeElements := world.NewCodeElementFacts(bctx.workspace)
	bctx.sessionExecutor.SetCodeElementSource(codeElements)

	bctx.sessionSpawner = session.NewSpawner(
		bctx.kernel,
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
	// The shard is what does the work: `nerd fix` delegates, so the CodeDOM
	// fact layer has to reach the spawned executor and not only the session's.
	bctx.sessionSpawner.SetCodeElementSource(codeElements)
	// The issue-driven retrieval pass, for every turn the session executor
	// runs: the kernel decides whether the turn retrieves and which of the
	// files it finds the model is handed (schemas_knowledge.mg 52.5). Until
	// 2026-09-25 only the chat TUI built a retriever, and only for its
	// compressor, so `nerd fix` and every delegated task started blind.
	bctx.retriever = retrieval.NewSparseRetriever(retrieval.DefaultSparseRetrieverConfig(bctx.workspace))
	taskRetrieverCfg := taskRetrieverConfig(bctx.appCfg, bctx.workspace)
	taskRetrieverCfg.Retriever = bctx.retriever
	taskRetrieverCfg.EmbeddingEngine = bctx.embeddingEngine
	if taskRetriever := retrieval.NewTaskRetriever(bctx.kernel, taskRetrieverCfg); taskRetriever != nil {
		bctx.sessionExecutor.SetIssueRetriever(taskRetriever)
		bctx.sessionSpawner.SetIssueRetriever(taskRetriever)
	}
	bctx.sessionSpawner.SetExecutorConfig(&execCfg)
	bctx.sessionSpawner.SetSessionID(bctx.sessionID)
	bctx.sessionSpawner.SetApprovalBroker(bctx.approvals)
	bctx.sessionSpawner.SetRolePrompts(bctx.rolePrompts)
	bctx.sessionSpawner.SetToolMeter(bctx.poiesis.ToolMeter())
	// Same meter for spawned subagents: their executors are fresh builds, not
	// clones, so without this their turn_cost deltas read zero. Nil-safe.
	bctx.sessionSpawner.SetUsageTracker(bctx.tracker)
	// Spawned subagents are fresh executors: without this they would run every
	// persona on the serving client's model whatever shard_profiles says.
	if bctx.shardProfileContext != nil {
		bctx.sessionSpawner.SetShardProfileContext(bctx.shardProfileContext)
	}
	// Mirror the executor wiring so subagent executors see the same
	// generated tools. Nil-safe on both ends.
	if bctx.virtualStore != nil {
		bctx.sessionSpawner.SetOuroborosRegistry(bctx.virtualStore.GetToolRegistry())
	} else {
		bctx.sessionSpawner.SetOuroborosRegistry(nil)
	}

	// Reasoning-intensive intents (/review, /audit, /campaign, ...) escape the
	// worker tier onto the planner client. The kernel decides which those are
	// via intent_requires_reasoning_model/1; this only supplies the client.
	if bctx.plannerLLMClient != nil {
		plannerSessionLLM := &sessionLLMAdapter{client: bctx.plannerLLMClient, tracker: bctx.tracker}
		bctx.sessionExecutor.SetPlannerClient(plannerSessionLLM)
		bctx.sessionSpawner.SetPlannerClient(plannerSessionLLM)
	}
	bctx.taskExecutor = session.NewJITExecutor(bctx.sessionExecutor, bctx.sessionSpawner, bctx.transducer)
	bctx.virtualStore.SetTaskExecutor(&taskDelegatorAdapter{executor: bctx.taskExecutor})
	// Domain personas (coder/tester/reviewer/researcher) and user-defined agents
	// have no ShardManager factory — the JIT migration deleted those shards. Give
	// the manager the same clean-loop executor so ShardManager.Spawn on such a
	// type does real work instead of falling through to a BaseShardAgent that
	// reported success without doing any.
	if bctx.shardManager != nil {
		bctx.shardManager.SetTaskDelegator(&taskDelegatorAdapter{executor: bctx.taskExecutor})
	}
	logging.Get(logging.CategorySession).Info("JITExecutor wired in BootCortex")
	initFactoryToolStore(bctx)
	initFactoryOuroborosWiring(bctx)
	return nil
}

// initFactoryToolStore opens the ToolStore and wires it into every shard.
// It mirrors the TUI boot (cmd/nerd/chat/session_shared_boot.go lines 217-289)
// for the SetToolStore setter only; glass-box and tool-event-bus setters belong
// to a later run.
func initFactoryToolStore(bctx *bootContext) {
	if bctx == nil {
		return
	}
	nerdDir := filepath.Join(bctx.workspace, ".nerd")
	toolsDBPath := filepath.Join(nerdDir, "tools.db")
	ts, err := store.NewToolStore(toolsDBPath)
	if err != nil {
		logging.Get(logging.CategoryBoot).Warn("Failed to initialize ToolStore: %v", err)
		return
	}
	bctx.toolStore = ts
	autoCleanupToolStore(ts)
	if bctx.shardManager == nil {
		return
	}
	for _, agent := range bctx.shardManager.GetActiveShards() {
		if setter, ok := agent.(interface{ SetToolStore(*store.ToolStore) }); ok {
			setter.SetToolStore(ts)
		}
	}
	toolStore := ts
	bctx.shardManager.SetPostSpawnHook(func(agent types.ShardAgent) {
		if setter, ok := agent.(interface{ SetToolStore(*store.ToolStore) }); ok {
			setter.SetToolStore(toolStore)
		}
	})
}

// ensureOuroborosLifetime is the autopoiesis context Cortex.Close cancels.
// It is not the boot caller's context: GetOrBootCortex caches the cortex
// under the first caller's ctx, which may already be done while the process
// is still serving later commands.
func ensureOuroborosLifetime(bctx *bootContext) context.Context {
	if bctx == nil {
		return context.Background()
	}
	if bctx.ouroborosCtx != nil {
		return bctx.ouroborosCtx
	}
	ctx, cancel := context.WithCancel(context.Background())
	bctx.ouroborosCtx = ctx
	bctx.ouroborosCancel = cancel
	return ctx
}

// serveDreamToolNeeds runs one Ouroboros generation per dream-router need.
// Generation is a multi-stage job (propose, audit, simulate, commit, and
// the retries between them). It stops when lifetime is cancelled
// (Cortex.Close) or the queue closes. There is no wall clock around a
// generation: each LLM call inside ExecuteOuroborosLoop keeps the client's
// own request bound.
func serveDreamToolNeeds(lifetime context.Context, needs <-chan core.ToolNeed, run func(context.Context, core.ToolNeed)) {
	if lifetime == nil {
		lifetime = context.Background()
	}
	if run == nil {
		return
	}
	for {
		select {
		case <-lifetime.Done():
			return
		case need, ok := <-needs:
			if !ok {
				return
			}
			run(lifetime, need)
		}
	}
}

// initFactoryOuroborosWiring starts the kernel listener that reacts to
// missing_tool_for facts and connects the DreamRouter to the Ouroboros tool
// generation pipeline. It mirrors the TUI boot
// (cmd/nerd/chat/session_shared_boot.go lines 153-206) so every path (nerd
// chat, fix runs, campaigns, spawned shards) gets it.
func initFactoryOuroborosWiring(bctx *bootContext) {
	if bctx == nil || bctx.poiesis == nil {
		return
	}
	autoCtx := ensureOuroborosLifetime(bctx)
	listenerDone := bctx.poiesis.StartKernelListener(autoCtx, 2*time.Second)
	ouroborosDone := make(chan struct{})
	bctx.ouroborosDone = ouroborosDone

	dreamToolCh := make(chan core.ToolNeed, 16)
	bctx.ouroborosQueue = dreamToolCh
	if bctx.virtualStore != nil {
		if dreamer := bctx.virtualStore.GetDreamer(); dreamer != nil {
			if dreamRouter := dreamer.GetDreamRouter(); dreamRouter != nil {
				dreamRouter.SetOuroborosQueue(dreamToolCh)
			}
		}
	}

	poiesis := bctx.poiesis
	go func() {
		defer func() {
			<-listenerDone
			close(ouroborosDone)
		}()
		// dreamToolCh is fed by the DreamRouter; we don't own its close.
		// Without the lifetime.Done arm this goroutine would block forever
		// on the receive after Close.
		serveDreamToolNeeds(autoCtx, dreamToolCh, func(ctx context.Context, need core.ToolNeed) {
			autoNeed := &autopoiesis.ToolNeed{
				Name:     need.Name,
				Purpose:  need.Description,
				Priority: need.Priority,
			}
			recordToolGeneration(need.Name, poiesis.ExecuteOuroborosLoop(ctx, autoNeed))
		})
	}()
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
		// Before the intelligence layer: the MCP remote authority it builds
		// asks the person through this broker.
		{name: "approval broker", run: initApprovalBroker},
		{name: "intelligence layer", run: initIntelligenceLayer},
		{name: "shard management", run: initShardManagement},
		{name: "final executors", run: initFinalExecutors},
		// Last: the learning loop binds the session executor to the evolver
		// and the evolver to the JIT compiler, so both must already exist.
		{name: "learning loop", run: initLearningLoop},
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
		stepErr := ctx.Err()
		if stepErr == nil {
			stepErr = step.run(bctx)
			if cancelErr := ctx.Err(); cancelErr != nil {
				stepErr = errors.Join(cancelErr, stepErr)
			}
		}
		if stepErr != nil {
			bootErr := fmt.Errorf("boot %s: %w", step.name, stepErr)
			if rollbackErr := rollbackBootContext(bctx); rollbackErr != nil {
				return nil, errors.Join(bootErr, fmt.Errorf("boot rollback: %w", rollbackErr))
			}
			return nil, bootErr
		}
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, rollbackBootContext(bctx))
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
		Executor:              bctx.virtualStore.AuditedExecutor(),
		Transducer:            bctx.transducer,
		Orchestrator:          bctx.poiesis,
		BrowserManager:        bctx.browserMgr,
		researchBrowser:       bctx.researchBrowser,
		Scanner:               bctx.scanner,
		UsageTracker:          bctx.tracker,
		LocalDB:               bctx.localDB,
		LearningStore:         bctx.learningStore,
		EmbeddingEngine:       bctx.embeddingEngine,
		Workspace:             bctx.workspace,
		JITCompiler:           bctx.jitCompiler,
		RolePrompts:           bctx.rolePrompts,
		PromptAssembler:       bctx.promptAssembler,
		PromptEvolver:         bctx.promptEvolver,
		ContextFeedback:       bctx.contextFeedback,
		WorkerLLMClient:       bctx.shardLLMClient,
		PlannerLLMClient:      bctx.plannerLLMClient,
		ToolStore:             bctx.toolStore,
		Retriever:             bctx.retriever,
		OuroborosQueue:        bctx.ouroborosQueue,
		mcpBridge:             bctx.mcpBridge,
		Approvals:             bctx.approvals,
		approvalsRelease:      bctx.approvalsRelease,
		mcpCancel:             bctx.mcpCancel,
		mcpDone:               bctx.mcpDone,
		ouroborosCancel:       bctx.ouroborosCancel,
		ouroborosDone:         bctx.ouroborosDone,
		generatedExecutor:     bctx.generatedExecutor,
		onDemandStop:          bctx.onDemandStop,
		perceptionInitialized: bctx.perceptionInitialized,
		sessionID:             bctx.sessionID,
		sessionLog:            bctx.sessionLog,
		meterSink:             bctx.meterSink,
		couseLog:              bctx.couseLog,
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

// profileRouter builds the clients shard profiles route serving's calls to:
// serving is the slot whose calls this router takes, and its routes carry that
// slot's reasoning effort (config.SecondaryLLMConfig.RouteTo). A nil config
// routes nowhere, and a routed call then fails by name.
func profileRouter(appCfg *config.UserConfig, serving *config.SecondaryLLMConfig, localDB *store.LocalStore) core.ProviderRouter {
	if appCfg == nil {
		return nil
	}
	return func(provider, model string) (core.LLMClient, error) {
		client, err := perception.NewRoutedClientFromUserConfig(appCfg, serving, provider, model)
		if err != nil {
			return nil, err
		}
		logging.Get(logging.CategoryPerception).Info("Routed LLM client built: provider=%s model=%s", provider, model)
		if localDB != nil {
			return perception.NewTracingLLMClient(client, createTraceStoreAdapter(localDB)), nil
		}
		return client, nil
	}
}
