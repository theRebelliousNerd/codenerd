package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/tools"
	"codenerd/internal/transparency"
	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/ast"
)

// TaskDelegator is a minimal interface for task delegation.
// This interface is implemented by session.TaskExecutor to avoid import cycles.
type TaskDelegator interface {
	// Execute runs a task synchronously and returns the result.
	// The intent parameter is an intent verb (e.g., "/fix", "/test", "/review").
	Execute(ctx context.Context, intent string, task string) (string, error)
}

// One-time imports
var _ = types.ShardConfig{}

// VirtualStore acts as the FFI Router for the Hollow Kernel.
// It routes 'next_action' atoms to the appropriate driver (Bash, MCP, File IO).
type VirtualStore struct {
	mu sync.RWMutex

	// Execution layer - Interface-based (modern/direct/safe)
	executor tactile.Executor

	// New execution layer - modern Executor with audit logging
	modernExecutor tactile.Executor
	auditLogger    *tactile.AuditLogger

	// MCP integration clients - dynamic map supports arbitrary servers
	// Key is server ID (e.g., "code_graph", "browser", "my_custom_server")
	mcpClients map[string]IntegrationClient

	// Shard delegation
	shardManager  *coreshards.ShardManager // For bidirectional binding (SetVirtualStore). Use taskDelegator for task execution.
	taskDelegator TaskDelegator            // For task execution (replaces direct shardManager.Spawn calls)

	// Kernel feedback loop
	kernel        Kernel
	dreamer       *Dreamer
	dreamerInitMu sync.Mutex

	// Constitutional logic (safety layer)
	constitution []ConstitutionalRule

	// Working directory
	workingDir string

	// Allowed environment variables
	allowedEnvVars []string

	// Allowed binaries for exec_cmd (defense in depth)
	allowedBinaries []string

	// Use modern executor for command execution
	useModernExecutor bool

	// Code DOM - semantic code element operations
	codeScope  CodeScope
	fileEditor FileEditor
	graphQuery types.GraphQuery // World Model Graph Query Interface

	// Autopoiesis - tool execution and generation
	toolExecutor  ToolExecutor
	toolGenerator ToolGenerator

	// Tool registry - integration with kernel and shards
	toolRegistry *ToolRegistry

	// Modular tools registry - pre-built tools for any agent
	modularTools *tools.Registry

	// Knowledge persistence - LocalStore for knowledge.db queries
	// Enables virtual predicates to query learned facts, session history, etc.
	localDB *store.LocalStore

	// Learning persistence - LearningStore for autopoiesis (§8.3)
	// Enables shards to persist and retrieve learned patterns across sessions
	learningStore *store.LearningStore

	// Permission cache - O(1) lookup for constitutional permission checks
	// Populated from kernel's safe_action/1 facts when kernel is attached
	permittedCache map[string]bool

	// Action log retention (avoid unbounded growth in kernel facts)
	lastLogPrune time.Time

	// Boot guard: prevents action execution until first user interaction.
	// This ensures session rehydration doesn't trigger old actions.
	// Set to true on initialization, disabled when user sends first message.
	bootGuardActive bool

	// Post-action validation registry - verifies actions actually succeeded
	validators *ValidatorRegistry

	// Transaction Manager - atomic multi-file edits with shadow validation (2PC)
	transactionMgr *TransactionManager

	// Glass Box + Tool event buses (optional). When set, RouteAction
	// emits CategoryRouting events on the Glass Box bus and concrete
	// per-tool execution events on the Tool bus so the TUI can show
	// "🛠 exec_cmd ls (12ms)" inline. Nil-safe.
	glassBoxBus  *transparency.GlassBoxEventBus
	toolEventBus *transparency.ToolEventBus
}

// SetGlassBoxBus attaches the Glass Box event bus for routing-layer
// activity. Safe to call before or after actions execute; nil is also
// safe (events become no-ops).
func (v *VirtualStore) SetGlassBoxBus(bus *transparency.GlassBoxEventBus) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.glassBoxBus = bus
}

// SetToolEventBus attaches the always-on tool event bus. Each
// successful RouteAction emits a ToolEvent with name=req.Type so the
// TUI prints a "🔧 actionType (xms) output" milestone in chat.
func (v *VirtualStore) SetToolEventBus(bus *transparency.ToolEventBus) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.toolEventBus = bus
}

// VirtualStoreConfig holds configuration for the VirtualStore.
type VirtualStoreConfig struct {
	WorkingDir      string
	AllowedEnvVars  []string
	AllowedBinaries []string
}

// DefaultVirtualStoreConfig returns sensible defaults.
func DefaultVirtualStoreConfig() VirtualStoreConfig {
	return VirtualStoreConfig{
		WorkingDir:     ".",
		AllowedEnvVars: []string{"PATH", "HOME", "GOPATH", "GOROOT"},
		AllowedBinaries: []string{
			"bash", "sh", "pwsh", "powershell", "cmd",
			"go", "git", "grep", "ls", "mkdir", "cp", "mv",
			"npm", "npx", "node", "python", "python3", "pip",
			"cargo", "rustc", "make", "cmake",
		},
	}
}

// NewVirtualStore creates a new VirtualStore with the given executor.
func NewVirtualStore(executor tactile.Executor) *VirtualStore {
	config := DefaultVirtualStoreConfig()
	return NewVirtualStoreWithConfig(executor, config)
}

// NewVirtualStoreWithConfig creates a new VirtualStore with custom config.
func NewVirtualStoreWithConfig(executor tactile.Executor, config VirtualStoreConfig) *VirtualStore {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "NewVirtualStoreWithConfig")
	defer timer.Stop()

	logging.VirtualStore("Initializing VirtualStore with workingDir=%s", config.WorkingDir)
	logging.VirtualStoreDebug("Config: allowedEnvVars=%v, allowedBinaries=%d",
		config.AllowedEnvVars, len(config.AllowedBinaries))

	vs := &VirtualStore{
		executor:        executor,
		workingDir:      config.WorkingDir,
		allowedEnvVars:  config.AllowedEnvVars,
		allowedBinaries: config.AllowedBinaries,
		shardManager:    coreshards.NewShardManager(),
		toolRegistry:    NewToolRegistry(config.WorkingDir),
		modularTools:    tools.NewRegistry(),
		mcpClients:      make(map[string]IntegrationClient),
		bootGuardActive: true, // Prevent action execution until first user interaction
	}

	// Wire up self-reference for ShardManager dependency injection
	vs.shardManager.SetVirtualStore(vs)

	// Initialize modern executor with audit logging
	vs.initModernExecutor()

	// Initialize constitutional rules (safety layer)
	vs.initConstitution()

	// Initialize post-action validator registry
	vs.initValidators()

	logging.VirtualStore("VirtualStore initialized successfully")
	return vs
}

// Close releases all resources held by the VirtualStore, verifying FFI memory cleanup.
func (v *VirtualStore) Close() error {
	v.mu.Lock()
	defer v.mu.Unlock()

	// Verify FFI memory cleanup for Transaction Manager
	if v.transactionMgr != nil {
		if v.transactionMgr.IsTransactionActive() {
			v.transactionMgr.Abort(context.Background(), "virtual_store_closed")
		}
	}

	// Close learning store
	if v.learningStore != nil {
		if closer, ok := any(v.learningStore).(interface{ Close() error }); ok {
			closer.Close()
		}
	}

	// Close local DB
	if v.localDB != nil {
		if closer, ok := any(v.localDB).(interface{ Close() error }); ok {
			closer.Close()
		}
	}

	// Disconnect MCP clients if they support closing
	for _, client := range v.mcpClients {
		if closer, ok := client.(interface{ Close() error }); ok {
			closer.Close()
		}
	}

	// Clean up Modern Executor
	if v.modernExecutor != nil {
		if closer, ok := v.modernExecutor.(interface{ Close() error }); ok {
			closer.Close()
		}
	}

	// Clear memory caches
	v.permittedCache = nil

	logging.VirtualStore("VirtualStore closed and FFI memory cleaned up")
	return nil
}

// initModernExecutor sets up the modern tactile executor with audit logging.
// This enables automatic fact generation for all command executions.
func (v *VirtualStore) initModernExecutor() {
	logging.VirtualStoreDebug("Initializing modern executor with audit logging")

	// Create executor config
	execConfig := tactile.DefaultExecutorConfig()
	execConfig.DefaultWorkingDir = v.workingDir
	execConfig.AllowedEnvironment = v.allowedEnvVars

	// Create composite executor (supports multiple sandbox modes)
	composite := tactile.NewCompositeExecutorWithConfig(execConfig)

	// Create audit logger
	v.auditLogger = tactile.NewAuditLogger()

	// Wire audit events to emit facts to kernel
	v.auditLogger.SetFactCallback(func(fact tactile.Fact) {
		v.injectTactileFact(fact)
	})

	// Connect audit logger to executor
	composite.SetAuditCallback(v.auditLogger.Log)

	v.modernExecutor = composite
	v.useModernExecutor = true

	logging.VirtualStoreDebug("Modern executor initialized, audit logging enabled")
}

// initValidators sets up the post-action validation registry.
// Validators verify that actions actually succeeded after execution.
// NOTE: All standard validators are registered via RegisterAllValidators (see validator_registry.go).
func (v *VirtualStore) initValidators() {
	logging.VirtualStoreDebug("Initializing post-action validator registry")

	v.validators = NewValidatorRegistry()

	// Register all standard validators
	RegisterAllValidators(v.validators)

	logging.VirtualStoreDebug("Validator registry initialized with %d validators", len(v.validators.validators))
}

// processValidationResults handles the outcomes of post-action validation.
// It injects validation facts into the kernel for policy reasoning.
func (v *VirtualStore) processValidationResults(req ActionRequest, result ActionResult, validations []ValidationResult) {
	if v.kernel == nil {
		return
	}

	// Logging result status to wire the parameter
	if !result.Success {
		logging.VirtualStoreDebug("Validating failed action %s", req.ActionID)
	}

	for _, vr := range validations {
		// Convert validation result to Mangle facts
		facts := vr.ToFacts()
		for _, fact := range facts {
			if err := v.kernel.Assert(fact); err != nil {
				logging.Get(logging.CategoryVirtualStore).Error(
					"Failed to inject validation fact %s: %v", fact.Predicate, err)
			}
		}

		// Log validation outcome
		if vr.Verified {
			logging.VirtualStoreDebug("Validation passed: action=%s method=%s confidence=%.2f",
				req.ActionID, vr.Method, vr.Confidence)
		} else {
			logging.Get(logging.CategoryVirtualStore).Warn(
				"Validation failed: action=%s method=%s error=%s",
				req.ActionID, vr.Method, vr.Error)
		}
	}
}

// injectTactileFact converts a tactile.Fact to core.Fact and injects to kernel.
func (v *VirtualStore) injectTactileFact(tf tactile.Fact) {
	v.mu.RLock()
	kernel := v.kernel
	v.mu.RUnlock()

	if kernel == nil {
		logging.VirtualStoreDebug("Cannot inject tactile fact %s: no kernel configured", tf.Predicate)
		return
	}

	// Normalize args to Mangle atoms where appropriate (Fix 11.11)
	normalizedArgs := make([]any, len(tf.Args))
	for i, arg := range tf.Args {
		normalizedArgs[i] = v.normalizeAtom(arg)
	}

	// Convert tactile.Fact to core.Fact
	coreFact := Fact{
		Predicate: tf.Predicate,
		Args:      normalizedArgs,
	}

	logging.VirtualStoreDebug("Injecting tactile fact: %s (args=%d)", tf.Predicate, len(tf.Args))
	if err := kernel.Assert(coreFact); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to inject tactile fact %s: %v", tf.Predicate, err)
	}
}

// normalizeAtom converts known status strings to Mangle atoms.
func (v *VirtualStore) normalizeAtom(val any) any {
	s, ok := val.(string)
	if !ok {
		return val
	}
	// List of keywords that should be treated as atoms in Mangle policies
	switch s {
	case "success", "failure", "strict", "permissive", "none", "running", "completed", "failed", "pending", "blocked":
		return MangleAtom("/" + s)
	}
	return val
}

// EnableModernExecutor switches to the modern tactile executor.
func (v *VirtualStore) EnableModernExecutor() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.useModernExecutor = true
}

// DisableModernExecutor switches back to the legacy executor.
func (v *VirtualStore) DisableModernExecutor() {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.useModernExecutor = false
}

// DisableBootGuard disables the boot guard, allowing action routing.
// This should be called when the first user message is received.
// Until this is called, ALL action routing through RouteAction is blocked,
// preventing session rehydration from replaying old actions.
func (v *VirtualStore) DisableBootGuard() {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.bootGuardActive {
		v.bootGuardActive = false
		logging.VirtualStore("Boot guard disabled: action routing now enabled")
	}
}

// IsBootGuardActive returns whether the boot guard is currently active.
func (v *VirtualStore) IsBootGuardActive() bool {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.bootGuardActive
}

// GetAuditMetrics returns execution metrics from the audit logger.
func (v *VirtualStore) GetAuditMetrics() tactile.ExecutionMetricsSnapshot {
	if v.auditLogger == nil {
		return tactile.ExecutionMetricsSnapshot{}
	}
	return v.auditLogger.GetMetrics()
}

// SetKernel sets the kernel for fact injection feedback.
func (v *VirtualStore) SetKernel(k Kernel) {
	v.mu.Lock()
	v.kernel = k
	// A Dreamer is bound to one concrete RealKernel. Clear it whenever the
	// executive kernel changes so a later route cannot simulate stale state.
	v.dreamer = nil
	v.mu.Unlock()

	logging.VirtualStore("Kernel attached to VirtualStore")

	// Build permission cache from kernel's safe_action/1 facts (O(1) lookup optimization)
	// NOTE: rebuildPermissionCache manages its own locking to avoid deadlock
	v.rebuildPermissionCache()

	// Wire VirtualStore back to RealKernel for bidirectional communication.
	// NOTE: Dreamer is created LAZILY in getDreamer() to avoid startup overhead.
	if realKernel := realKernelForDreamer(k); realKernel != nil {
		realKernel.SetVirtualStore(v)
	}

	// Also set kernel on tool registry
	v.mu.RLock()
	tr := v.toolRegistry
	v.mu.RUnlock()
	if tr != nil {
		tr.SetKernel(k)
		logging.VirtualStoreDebug("Tool registry kernel reference updated")
	}
}

// getDreamer returns the Dreamer instance, creating it lazily if needed.
// This avoids creating the Dreamer at boot time when it's not needed.
func (v *VirtualStore) getDreamer() *Dreamer {
	v.dreamerInitMu.Lock()
	defer v.dreamerInitMu.Unlock()

	for {
		v.mu.RLock()
		if v.dreamer != nil {
			dreamer := v.dreamer
			v.mu.RUnlock()
			return dreamer
		}
		kernel := v.kernel
		v.mu.RUnlock()

		// CortexKernel is the default production executive. Its catch-all shard
		// owns the concrete RealKernel used for isolated speculative evaluation.
		realKernel := realKernelForDreamer(kernel)
		if realKernel == nil {
			return nil
		}

		// NewDreamer asserts boot facts and can evaluate Mangle. Never perform
		// that work while holding VirtualStore.mu: virtual predicates may route
		// back into this store during evaluation.
		candidate := NewDreamer(realKernel)

		v.mu.Lock()
		if v.dreamer != nil {
			dreamer := v.dreamer
			v.mu.Unlock()
			return dreamer
		}
		if realKernelForDreamer(v.kernel) == realKernel {
			v.dreamer = candidate
			v.mu.Unlock()
			logging.VirtualStore("Dreamer created lazily for speculative execution")
			return candidate
		}
		v.mu.Unlock()
		// SetKernel raced with construction. Retry against the current kernel;
		// never publish a Dreamer bound to stale executive state.
	}
}

func realKernelForDreamer(kernel Kernel) *RealKernel {
	switch k := kernel.(type) {
	case *RealKernel:
		return k
	case *CortexKernel:
		return k.GetPrimaryRealKernel()
	default:
		return nil
	}
}

// GetDreamer returns the Dreamer instance, creating it lazily if needed.
// Exported for external wiring (e.g., connecting DreamRouter to Ouroboros).
func (v *VirtualStore) GetDreamer() *Dreamer {
	return v.getDreamer()
}

// SetDreamRouter connects a DreamRouter to the Dreamer for learning persistence.
// If the Dreamer hasn't been created yet, it will be created lazily.
func (v *VirtualStore) SetDreamRouter(router *DreamRouter) {
	dreamer := v.getDreamer()
	if dreamer != nil {
		dreamer.SetDreamRouter(router)
	}
}

// SetDreamPlanManager connects a DreamPlanManager to the Dreamer for plan lifecycle.
// If the Dreamer hasn't been created yet, it will be created lazily.
func (v *VirtualStore) SetDreamPlanManager(pm *DreamPlanManager) {
	dreamer := v.getDreamer()
	if dreamer != nil {
		dreamer.SetDreamPlanManager(pm)
	}
}

// SetTransactionManager sets the atomic multi-file edit manager.
func (v *VirtualStore) SetTransactionManager(tm *TransactionManager) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.transactionMgr = tm
	logging.VirtualStore("TransactionManager connected for atomic multi-file edits")
}

// GetTransactionManager returns the transaction manager for atomic edits.
func (v *VirtualStore) GetTransactionManager() *TransactionManager {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.transactionMgr
}

// Get resolves virtual predicates for the Mangle kernel on demand.
func (vs *VirtualStore) Get(query ast.Atom) ([]ast.Atom, error) {
	switch query.Predicate.Symbol {
	case "query_learned":
		return vs.getQueryLearnedAtoms(query)
	case "query_session":
		return vs.getQuerySessionAtoms(query)
	case "recall_similar":
		return vs.getRecallSimilarAtoms(query)
	case "query_knowledge_graph":
		return vs.getQueryKnowledgeGraphAtoms(query)
	case "query_activations":
		return vs.getQueryActivationsAtoms(query)
	case "has_learned":
		return vs.getHasLearnedAtoms(query)
	case "query_traces":
		return vs.getQueryTracesAtoms(query)
	case "query_trace_stats":
		return vs.getQueryTraceStatsAtoms(query)
	case "query_strategic":
		return vs.getQueryStrategicAtoms(query)
	case "query_graph":
		return vs.getQueryGraphAtoms(query)
	// string_contains is commented out as we transitioned to native :string:contains.
	// case "string_contains":
	// 	return vs.getStringContainsAtoms(query)
	default:
		return nil, nil
	}
}

// rebuildPermissionCache queries the kernel for all safe_action/1 facts
// and builds a O(1) lookup cache.
//
// DEADLOCK FIX: This method does NOT hold v.mu while querying the kernel.
// The previous implementation held v.mu (write lock) during kernel.Query(),
// which could deadlock if the kernel's virtual predicate handlers tried to
// read-lock v.mu. Now we query first, then lock only to write the cache.
func (v *VirtualStore) rebuildPermissionCache() {
	if v.kernel == nil {
		v.mu.Lock()
		v.permittedCache = nil
		v.mu.Unlock()
		return
	}

	// Query kernel WITHOUT holding v.mu to prevent deadlock
	results, err := v.kernel.Query("safe_action")
	if err != nil {
		logging.VirtualStoreDebug("Failed to query safe_action facts for cache: %v", err)
		v.mu.Lock()
		v.permittedCache = nil
		v.mu.Unlock()
		return
	}

	cache := make(map[string]bool, len(results))
	for _, f := range results {
		if len(f.Args) == 0 {
			continue
		}
		action := types.ExtractString(f.Args[0])
		// Store both with and without leading slash for fast lookup
		cache[action] = true
		if after, ok := strings.CutPrefix(action, "/"); ok {
			cache[after] = true
		} else {
			cache["/"+action] = true
		}
	}

	// Lock only to write the cache
	v.mu.Lock()
	v.permittedCache = cache
	v.mu.Unlock()
	logging.VirtualStore("Permission cache built: %d safe_action entries", len(results))
}

// SetShardManager sets the shard manager for delegation.
// DEPRECATED: Use SetTaskExecutor instead.
// NOTE: Still in use by older components (ShardManager, CampaignRunner). Refactor pending.
func (v *VirtualStore) SetShardManager(sm *coreshards.ShardManager) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.shardManager = sm
	logging.VirtualStoreDebug("ShardManager attached to VirtualStore")
}

// SetTaskExecutor sets the task delegator for delegation.
// This is the preferred method - uses the new JIT architecture.
// The delegator parameter must implement TaskDelegator (e.g., session.TaskExecutor).
func (v *VirtualStore) SetTaskExecutor(delegator TaskDelegator) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.taskDelegator = delegator
	logging.VirtualStoreDebug("TaskDelegator attached to VirtualStore")
}

// SetMCPClient registers an MCP integration client for the given server ID.
// Server IDs are arbitrary strings (e.g., "code_graph", "browser", "my_custom_server").
func (v *VirtualStore) SetMCPClient(serverID string, client IntegrationClient) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.mcpClients == nil {
		v.mcpClients = make(map[string]IntegrationClient)
	}
	v.mcpClients[serverID] = client
	logging.VirtualStoreDebug("MCP client attached: %s", serverID)
}

// GetMCPClient returns the MCP integration client for the given server ID.
// Returns nil if no client is registered for that server.
func (v *VirtualStore) GetMCPClient(serverID string) IntegrationClient {
	v.mu.RLock()
	defer v.mu.RUnlock()
	if v.mcpClients == nil {
		return nil
	}
	client := v.mcpClients[serverID]
	if client == nil {
		return nil
	}
	return &mcpClientProxy{vs: v, client: client}
}

// GetMCPClientNames returns all registered MCP client server IDs.
func (v *VirtualStore) GetMCPClientNames() []string {
	v.mu.RLock()
	defer v.mu.RUnlock()
	names := make([]string, 0, len(v.mcpClients))
	for name := range v.mcpClients {
		names = append(names, name)
	}
	return names
}

// SetCodeScope sets the Code DOM scope manager.
func (v *VirtualStore) SetCodeScope(scope CodeScope) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.codeScope = scope
	logging.VirtualStoreDebug("CodeScope attached for Code DOM operations")
}

// SetFileEditor sets the file editor for line-based operations.
func (v *VirtualStore) SetFileEditor(editor FileEditor) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.fileEditor = editor
	logging.VirtualStoreDebug("FileEditor attached for line-based file operations")
}

// GetFileEditor returns the file editor for line-based operations.
func (v *VirtualStore) GetFileEditor() FileEditor {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.fileEditor
}

// SetToolExecutor sets the tool executor for generated tool execution.
func (v *VirtualStore) SetToolExecutor(executor ToolExecutor) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.toolExecutor = executor

	logging.VirtualStoreDebug("ToolExecutor attached for Ouroboros tool execution")
	// NOTE: Tool sync is intentionally NOT done here. It is handled by
	// HydrateToolsFromDisk() during boot to avoid a duplicate
	// SyncFromOuroboros pass that wastes ~36s of Mangle evaluation.
}

// GetToolExecutor returns the current tool executor.
func (v *VirtualStore) GetToolExecutor() ToolExecutor {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.toolExecutor
}

// SetToolGenerator sets the tool generator for creating new tools via Ouroboros.
func (v *VirtualStore) SetToolGenerator(generator ToolGenerator) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.toolGenerator = generator
	logging.VirtualStoreDebug("ToolGenerator attached for Ouroboros tool generation")
}

// GetToolGenerator returns the current tool generator.
func (v *VirtualStore) GetToolGenerator() ToolGenerator {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.toolGenerator
}

func (v *VirtualStore) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(v.workingDir, path)
}

func (v *VirtualStore) isBinaryAllowed(binary string) bool {
	if binary == "" {
		return false
	}
	for _, b := range v.allowedBinaries {
		if strings.EqualFold(b, binary) {
			return true
		}
	}
	return false
}

func (v *VirtualStore) getAllowedEnv() []string {
	env := make([]string, 0)
	for _, key := range v.allowedEnvVars {
		if val := os.Getenv(key); val != "" {
			env = append(env, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return env
}

// filterCallerEnv filters caller-provided env vars against the allowlist.
// This prevents attackers from injecting PATH, LD_PRELOAD, or other dangerous
// environment variables via the env parameter of Exec(). In Go's os/exec,
// the last duplicate key wins, so unfiltered appending is exploitable.
func (v *VirtualStore) filterCallerEnv(env []string) []string {
	if len(env) == 0 {
		return nil
	}
	// Build a set from the allowlist for O(1) lookup
	allowed := make(map[string]bool, len(v.allowedEnvVars))
	for _, key := range v.allowedEnvVars {
		allowed[strings.ToUpper(key)] = true
	}

	filtered := make([]string, 0, len(env))
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 0 {
			continue
		}
		key := strings.ToUpper(parts[0])
		if allowed[key] {
			filtered = append(filtered, e)
		} else {
			logging.VirtualStoreDebug("Rejected caller env var not in allowlist: %s", parts[0])
		}
	}
	return filtered
}

func (v *VirtualStore) injectFact(fact Fact) {
	v.mu.RLock()
	kernel := v.kernel
	v.mu.RUnlock()

	if kernel != nil {
		if err := kernel.Assert(fact); err != nil {
			logging.Get(logging.CategoryVirtualStore).Error("Failed to inject fact %s: %v", fact.Predicate, err)
		}
	}
}

func (v *VirtualStore) injectFacts(facts []Fact) {
	if len(facts) == 0 {
		return
	}

	v.mu.RLock()
	kernel := v.kernel
	v.mu.RUnlock()

	if kernel == nil {
		return
	}

	// Fast path: RealKernel supports batch assertion (single evaluation pass).
	if realKernel, ok := kernel.(*RealKernel); ok {
		if err := realKernel.AssertBatch(facts); err != nil {
			logging.Get(logging.CategoryVirtualStore).Error("Failed to inject batch facts: %v", err)
		}
		return
	}

	for _, fact := range facts {
		if err := kernel.Assert(fact); err != nil {
			logging.Get(logging.CategoryVirtualStore).Error("Failed to inject fact %s: %v", fact.Predicate, err)
		}
	}
}

func (v *VirtualStore) maybePruneActionLogs(now time.Time) {
	v.mu.Lock()
	if !v.lastLogPrune.IsZero() && now.Sub(v.lastLogPrune) < 10*time.Second {
		v.mu.Unlock()
		return
	}
	v.lastLogPrune = now
	kernel := v.kernel
	v.mu.Unlock()

	realKernel, ok := kernel.(*RealKernel)
	if !ok || realKernel == nil {
		return
	}

	prune := func(predicate string, tsIndex int, cutoffUnix int64) {
		facts, err := realKernel.Query(predicate)
		if err != nil || len(facts) == 0 {
			return
		}
		toRemove := make([]Fact, 0, len(facts)/4)
		for _, f := range facts {
			ts, ok := unixSecondsArgAt(f.Args, tsIndex)
			if !ok {
				continue
			}
			if ts < cutoffUnix {
				toRemove = append(toRemove, f)
			}
		}
		if len(toRemove) == 0 {
			return
		}
		if err := realKernel.RetractExactFactsBatch(toRemove); err != nil {
			logging.Get(logging.CategoryKernel).Warn("failed to retract stale facts batch: %v", err)
		}
	}

	// Keep action logs bounded to protect kernel evaluation performance.
	prune("execution_result", 5, now.Add(-15*time.Minute).Unix())
	prune("shard_context_refreshed", 2, now.Add(-60*time.Minute).Unix())
	prune("system_heartbeat", 1, now.Add(-5*time.Minute).Unix())

	// Cap diagnostics by count (no timestamp field).
	pruneByCount := func(predicate string, maxFacts int) {
		facts, err := realKernel.Query(predicate)
		if err != nil || len(facts) <= maxFacts {
			return
		}
		// Remove oldest first (they appear earlier in the slice).
		excess := facts[:len(facts)-maxFacts]
		if err := realKernel.RetractExactFactsBatch(excess); err != nil {
			logging.Get(logging.CategoryKernel).Warn("failed to cap %s facts: %v", predicate, err)
		}
	}
	pruneByCount("diagnostic", 200)
	pruneByCount("code_diagnostic", 200)
	pruneByCount("lsp_diagnostic", 200)
}

func unixSecondsArgAt(args []any, idx int) (int64, bool) {
	if idx < 0 || len(args) <= idx {
		return 0, false
	}
	switch v := args[idx].(type) {
	case int64:
		return v, true
	case int:
		return int64(v), true
	case float64:
		return int64(v), true
	case string:
		parsed, err := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if err != nil {
			return 0, false
		}
		return parsed, true
	default:
		return 0, false
	}
}

func (v *VirtualStore) clearCodeDOMFacts() {
	v.mu.RLock()
	kernel := v.kernel
	v.mu.RUnlock()

	if kernel == nil {
		return
	}

	preds := map[string]struct{}{
		// Scope state
		"active_file":        {},
		"file_in_scope":      {},
		"code_element":       {},
		"element_signature":  {},
		"element_visibility": {},
		"element_parent":     {},
		"code_interactable":  {},

		// Scope diagnostics/meta (emitted by world.FileScope)
		"parse_error":          {},
		"file_not_found":       {},
		"scope_refresh_failed": {},
		"file_hash_mismatch":   {},
		"element_stale":        {},
		"encoding_issue":       {},
		"large_file_warning":   {},
		"generated_code":       {},
		"cgo_code":             {},
		"build_tag":            {},
		"embed_directive":      {},
		"api_client_function":  {},
		"api_handler_function": {},
		"edit_unsafe":          {},
	}

	// Fast path: RealKernel can remove a predicate set with a single rebuild pass.
	if realKernel, ok := kernel.(*RealKernel); ok {
		if err := realKernel.RemoveFactsByPredicateSet(preds); err != nil {
			logging.Get(logging.CategoryKernel).Warn("failed to remove facts by predicate set: %v", err)
		}
		return
	}

	for p := range preds {
		if err := kernel.Retract(p); err != nil {
			logging.Get(logging.CategoryKernel).Warn("failed to retract predicate %q: %v", p, err)
		}
	}
}

func (v *VirtualStore) parseBuildDiagnostics(output string) []Fact {
	facts := make([]Fact, 0)
	lines := strings.SplitSeq(output, "\n")

	for line := range lines {
		// Parse Go-style errors: file.go:line:col: message
		if strings.Contains(line, ":") && (strings.Contains(line, "error") || strings.Contains(line, "warning")) {
			parts := strings.SplitN(line, ":", 4)
			if len(parts) >= 4 {
				facts = append(facts, Fact{
					Predicate: "diagnostic",
					Args:      []any{"/error", parts[0], parts[1], parts[3]},
				})
			}
		}
	}

	return facts
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// QueryPermitted checks if an action is permitted by the constitutional logic.
func (v *VirtualStore) QueryPermitted(req ActionRequest) bool {
	return v.checkConstitution(req) == nil
}

// CheckKernelPermitted consults the kernel to verify if the specific action is permitted.
// safe_action/1 is only a classification hint; authorization always requires an
// exact permitted(Action, Target, Payload) fact derived for this request.
// Default deny when the kernel is unavailable or queries fail.
func (v *VirtualStore) CheckKernelPermitted(actionType, target string, payload map[string]any) bool {
	v.mu.RLock()
	k := v.kernel
	cache := v.permittedCache
	v.mu.RUnlock()

	// No kernel attached - fail closed
	if k == nil {
		logging.VirtualStoreDebug("checkKernelPermitted(%s): no kernel attached, denying", actionType)
		return false
	}

	wantType := actionType
	altType := actionType
	if after, ok := strings.CutPrefix(actionType, "/"); ok {
		altType = after
	} else {
		wantType = "/" + actionType
	}

	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		logging.VirtualStoreDebug("checkKernelPermitted(%s): payload encoding failed, denying: %v", actionType, err)
		return false
	}
	if payload == nil {
		payloadJSON = []byte("{}")
	}

	// Query all permitted facts
	results, err := k.Query("permitted")
	if err != nil {
		logging.VirtualStoreDebug("checkKernelPermitted(%s): query error, denying: %v", actionType, err)
		return false
	}

	for _, f := range results {
		if len(f.Args) != 3 {
			continue
		}

		// 1. Check ActionType
		argType := types.ExtractString(f.Args[0])
		if argType != wantType && argType != altType {
			continue
		}

		// 2. Match the concrete target. A query wildcard is never an
		// authorization result and must not be treated as one.
		if factTarget := types.ExtractString(f.Args[1]); factTarget != target {
			continue
		}

		// 3. Match the canonical JSON payload used by pending_action/5.
		if factPayload := types.ExtractString(f.Args[2]); factPayload != string(payloadJSON) {
			continue
		}

		safeClassified := cache != nil && (cache[wantType] || cache[altType])
		logging.VirtualStoreDebug(
			"checkKernelPermitted(%s): ALLOWED (exact permitted fact; safe_classified=%v)",
			actionType, safeClassified)
		return true
	}

	logging.VirtualStoreDebug("checkKernelPermitted(%s): DENIED (no matching permitted fact)", actionType)
	return false
}

// =============================================================================
// TYPES.VIRTUALSTORE INTERFACE IMPLEMENTATION
// =============================================================================

// ReadFile reads a file and returns its lines.
// Implements types.VirtualStore interface.
func (v *VirtualStore) ReadFile(path string) ([]string, error) {
	v.mu.RLock()
	editor := v.fileEditor
	v.mu.RUnlock()

	if editor != nil {
		return editor.ReadFile(path)
	}
	// Fallback: direct read
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

// WriteFile writes lines to a file.
// Implements types.VirtualStore interface.
func (v *VirtualStore) WriteFile(path string, content []string) error {
	v.mu.RLock()
	editor := v.fileEditor
	v.mu.RUnlock()

	if editor != nil {
		_, err := editor.WriteFile(path, content)
		return err
	}
	// Fallback: direct write
	return os.WriteFile(path, []byte(strings.Join(content, "\n")), 0644)
}

// ReadRaw reads a file and returns its raw bytes.
// Implements types.VirtualStore interface.
func (v *VirtualStore) ReadRaw(path string) ([]byte, error) {
	return os.ReadFile(path)
}
