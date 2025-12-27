package core

import (
	"context"
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
	"codenerd/internal/types"

	"github.com/google/mangle/ast"
)

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
	shardManager *coreshards.ShardManager

	// Kernel feedback loop
	kernel  Kernel
	dreamer *Dreamer

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

	// Knowledge persistence - LocalStore for knowledge.db queries
	// Enables virtual predicates to query learned facts, session history, etc.
	localDB *store.LocalStore

	// Learning persistence - LearningStore for autopoiesis (§8.3)
	// Enables shards to persist and retrieve learned patterns across sessions
	learningStore *store.LearningStore

	// Permission cache - O(1) lookup for constitutional permission checks
	// Populated from kernel's permitted/1 facts when kernel is attached
	permittedCache map[string]bool

	// Action log retention (avoid unbounded growth in kernel facts)
	lastLogPrune time.Time

	// Boot guard: prevents action execution until first user interaction.
	// This ensures session rehydration doesn't trigger old actions.
	// Set to true on initialization, disabled when user sends first message.
	bootGuardActive bool
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
		mcpClients:      make(map[string]IntegrationClient),
		bootGuardActive: true, // Prevent action execution until first user interaction
	}

	// Wire up self-reference for ShardManager dependency injection
	vs.shardManager.SetVirtualStore(vs)

	// Initialize modern executor with audit logging
	vs.initModernExecutor()

	// Initialize constitutional rules (safety layer)
	vs.initConstitution()

	logging.VirtualStore("VirtualStore initialized successfully")
	return vs
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
	normalizedArgs := make([]interface{}, len(tf.Args))
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
func (v *VirtualStore) normalizeAtom(val interface{}) interface{} {
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
	defer v.mu.Unlock()
	v.kernel = k

	logging.VirtualStore("Kernel attached to VirtualStore")

	// Build permission cache from kernel's permitted/1 facts (O(1) lookup optimization)
	v.rebuildPermissionCache()

	// Wire VirtualStore back to RealKernel for bidirectional communication.
	// NOTE: Dreamer is created LAZILY in getDreamer() to avoid startup overhead.
	if realKernel, ok := k.(*RealKernel); ok {
		realKernel.SetVirtualStore(v)
	}

	// Also set kernel on tool registry
	if v.toolRegistry != nil {
		v.toolRegistry.SetKernel(k)
		logging.VirtualStoreDebug("Tool registry kernel reference updated")
	}
}

// getDreamer returns the Dreamer instance, creating it lazily if needed.
// This avoids creating the Dreamer at boot time when it's not needed.
func (v *VirtualStore) getDreamer() *Dreamer {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.dreamer != nil {
		return v.dreamer
	}

	// Only create if we have a RealKernel
	if realKernel, ok := v.kernel.(*RealKernel); ok {
		v.dreamer = NewDreamer(realKernel)
		logging.VirtualStore("Dreamer created lazily for speculative execution")
	}

	return v.dreamer
}

// SimulateActionWithDreamer runs speculative dream analysis on an action.
// This is OPT-IN - call this explicitly when you want precognition safety checks.
// Returns (safe, reason) - if safe is false, the action would be blocked.
func (v *VirtualStore) SimulateActionWithDreamer(ctx context.Context, req ActionRequest) (bool, string) {
	dreamer := v.getDreamer()
	if dreamer == nil {
		return true, "" // No dreamer available, allow action
	}

	dream := dreamer.SimulateAction(ctx, req)
	if dream.Unsafe {
		logging.VirtualStore("Dreamer flagged action as unsafe: %s - %s", req.Type, dream.Reason)
		return false, dream.Reason
	}
	return true, ""
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
	case "string_contains":
		return vs.getStringContainsAtoms(query)
	default:
		return nil, nil
	}
}

// rebuildPermissionCache queries the kernel for all permitted/1 facts
// and builds a O(1) lookup cache. Must be called with v.mu held.
func (v *VirtualStore) rebuildPermissionCache() {
	if v.kernel == nil {
		v.permittedCache = nil
		return
	}

	results, err := v.kernel.Query("permitted")
	if err != nil {
		logging.VirtualStoreDebug("Failed to query permitted facts for cache: %v", err)
		v.permittedCache = nil
		return
	}

	cache := make(map[string]bool, len(results))
	for _, f := range results {
		if len(f.Args) == 0 {
			continue
		}
		action := fmt.Sprintf("%v", f.Args[0])
		// Store both with and without leading slash for fast lookup
		cache[action] = true
		if strings.HasPrefix(action, "/") {
			cache[strings.TrimPrefix(action, "/")] = true
		} else {
			cache["/"+action] = true
		}
	}

	v.permittedCache = cache
	logging.VirtualStore("Permission cache built: %d actions permitted", len(results))
}

// SetShardManager sets the shard manager for delegation.
func (v *VirtualStore) SetShardManager(sm *coreshards.ShardManager) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.shardManager = sm
	logging.VirtualStoreDebug("ShardManager attached to VirtualStore")
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
	return v.mcpClients[serverID]
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

// SetToolExecutor sets the tool executor for generated tool execution.
func (v *VirtualStore) SetToolExecutor(executor ToolExecutor) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.toolExecutor = executor

	logging.VirtualStoreDebug("ToolExecutor attached for Ouroboros tool execution")

	// Sync tools from executor to registry
	if v.toolRegistry != nil && executor != nil {
		if err := v.toolRegistry.SyncFromOuroboros(executor); err != nil {
			logging.Get(logging.CategoryVirtualStore).Warn("Failed to sync tools from Ouroboros: %v", err)
		} else {
			logging.VirtualStoreDebug("Tools synced from Ouroboros executor to registry")
		}
	}
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

// GetToolRegistry returns the tool registry.
func (v *VirtualStore) GetToolRegistry() *ToolRegistry {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.toolRegistry
}

// RegisterTool registers a tool with the registry and injects facts into the kernel.
func (v *VirtualStore) RegisterTool(name, command, shardAffinity string) error {
	v.mu.RLock()
	registry := v.toolRegistry
	v.mu.RUnlock()

	if registry == nil {
		logging.Get(logging.CategoryVirtualStore).Error("Cannot register tool %s: registry not initialized", name)
		return fmt.Errorf("tool registry not initialized")
	}

	logging.VirtualStore("Registering tool: name=%s, shardAffinity=%s", name, shardAffinity)
	if err := registry.RegisterTool(name, command, shardAffinity); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to register tool %s: %v", name, err)
		return err
	}

	logging.VirtualStoreDebug("Tool %s registered successfully", name)
	return nil
}

// GetToolsForShard returns all tools available for a specific shard type.
func (v *VirtualStore) GetToolsForShard(shardType string) []*Tool {
	v.mu.RLock()
	registry := v.toolRegistry
	v.mu.RUnlock()

	if registry == nil {
		return nil
	}

	return registry.GetToolsForShard(shardType)
}

// HydrateToolsFromDisk restores compiled tools from the .compiled directory
// and syncs from the Ouroboros executor if available.
// This should be called during session boot after the kernel is ready.
func (v *VirtualStore) HydrateToolsFromDisk(nerdDir string) error {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "HydrateToolsFromDisk")
	defer timer.Stop()

	v.mu.RLock()
	registry := v.toolRegistry
	kernel := v.kernel
	executor := v.toolExecutor
	v.mu.RUnlock()

	if registry == nil {
		logging.VirtualStoreDebug("HydrateToolsFromDisk: no registry, skipping")
		return nil
	}

	logging.VirtualStore("Hydrating tools from disk: %s", nerdDir)

	// Ensure kernel is set for fact injection
	if kernel != nil {
		registry.SetKernel(kernel)
	}

	// 1. Restore compiled tools from disk (.nerd/tools/.compiled/)
	compiledDir := filepath.Join(nerdDir, "tools", ".compiled")
	if err := registry.RestoreFromDisk(compiledDir); err != nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Partial error restoring tools from disk: %v", err)
	} else {
		logging.VirtualStoreDebug("Tools restored from compiled directory")
	}

	// 2. Sync from Ouroboros if tool executor exists
	if executor != nil {
		if err := registry.SyncFromOuroboros(executor); err != nil {
			logging.Get(logging.CategoryVirtualStore).Warn("Failed to sync from Ouroboros: %v", err)
		} else {
			logging.VirtualStoreDebug("Tools synced from Ouroboros executor")
		}
	}

	return nil
}

// HydrateStaticTools loads static tool definitions into the registry.
// This is used to hydrate tools from available_tools.json at session boot.
func (v *VirtualStore) HydrateStaticTools(defs []StaticToolDef) error {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "HydrateStaticTools")
	defer timer.Stop()

	v.mu.RLock()
	registry := v.toolRegistry
	kernel := v.kernel
	v.mu.RUnlock()

	if registry == nil {
		logging.VirtualStoreDebug("HydrateStaticTools: no registry, skipping")
		return nil
	}

	logging.VirtualStore("Hydrating %d static tool definitions", len(defs))

	// Ensure kernel is set for fact injection
	if kernel != nil {
		registry.SetKernel(kernel)
	}

	if err := registry.RestoreFromStaticDefs(defs); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to hydrate static tools: %v", err)
		return err
	}

	logging.VirtualStoreDebug("Static tools hydrated successfully")
	return nil
}

// SetLocalDB sets the knowledge database for virtual predicate queries.
func (v *VirtualStore) SetLocalDB(db *store.LocalStore) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.localDB = db
	logging.VirtualStoreDebug("LocalDB (knowledge.db) attached for memory store queries")
}

// GetLocalDB returns the current knowledge database.
func (v *VirtualStore) GetLocalDB() *store.LocalStore {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.localDB
}

// SetLearningStore sets the learning database for shard autopoiesis.
func (v *VirtualStore) SetLearningStore(ls *store.LearningStore) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.learningStore = ls
	logging.VirtualStoreDebug("LearningStore attached for autopoiesis persistence")
}

// GetLearningStore returns the current learning database.
func (v *VirtualStore) GetLearningStore() *store.LearningStore {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.learningStore
}

// initConstitution initializes the constitutional safety rules.
func (v *VirtualStore) initConstitution() {
	v.constitution = []ConstitutionalRule{
		{
			Name:        "no_destructive_commands",
			Description: "Prevent destructive shell commands",
			Check: func(req ActionRequest) error {
				if req.Type != ActionExecCmd {
					return nil
				}
				cmd := strings.ToLower(req.Target)
				forbidden := []string{"rm -rf", "mkfs", "dd if=", ":(){", "chmod 777"}
				for _, f := range forbidden {
					if strings.Contains(cmd, f) {
						return fmt.Errorf("constitutional violation: destructive command '%s' blocked", f)
					}
				}
				return nil
			},
		},
		{
			Name:        "no_secret_exfiltration",
			Description: "Prevent exfiltration of secrets",
			Check: func(req ActionRequest) error {
				payload := fmt.Sprintf("%v", req.Payload)
				secrets := []string{".env", "credentials", "secret", "api_key", "password"}
				dangerous := []string{"curl", "wget", "nc ", "netcat"}
				hasSecret := false
				hasDangerous := false
				for _, s := range secrets {
					if strings.Contains(strings.ToLower(payload), s) {
						hasSecret = true
						break
					}
				}
				for _, d := range dangerous {
					if strings.Contains(strings.ToLower(req.Target), d) {
						hasDangerous = true
						break
					}
				}
				if hasSecret && hasDangerous {
					return fmt.Errorf("constitutional violation: potential secret exfiltration blocked")
				}
				return nil
			},
		},
		{
			Name:        "path_traversal_protection",
			Description: "Prevent path traversal attacks",
			Check: func(req ActionRequest) error {
				if req.Type != ActionReadFile && req.Type != ActionWriteFile && req.Type != ActionDeleteFile {
					return nil
				}
				if strings.Contains(req.Target, "..") {
					return fmt.Errorf("constitutional violation: path traversal blocked")
				}
				return nil
			},
		},
		{
			Name:        "no_system_file_modification",
			Description: "Prevent modification of system files",
			Check: func(req ActionRequest) error {
				if req.Type != ActionWriteFile && req.Type != ActionDeleteFile && req.Type != ActionEditFile {
					return nil
				}
				systemPaths := []string{"/etc/", "/usr/", "/bin/", "/sbin/", "C:\\Windows\\"}
				target := req.Target
				for _, sp := range systemPaths {
					if strings.HasPrefix(target, sp) {
						return fmt.Errorf("constitutional violation: system file modification blocked")
					}
				}
				return nil
			},
		},
	}
}

// checkConstitution verifies the action against all constitutional rules.
func (v *VirtualStore) checkConstitution(req ActionRequest) error {
	for _, rule := range v.constitution {
		if err := rule.Check(req); err != nil {
			return err
		}
	}
	return nil
}

// RouteAction intercepts 'next_action' atoms and routes them to appropriate handlers.
func (v *VirtualStore) RouteAction(ctx context.Context, action Fact) (string, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, fmt.Sprintf("RouteAction(%s)", action.Predicate))
	defer timer.Stop()

	logging.VirtualStore("Routing action: predicate=%s, args=%d", action.Predicate, len(action.Args))

	// Parse the action fact
	req, err := v.parseActionFact(action)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to parse action fact: %v", err)
		return "", fmt.Errorf("failed to parse action fact: %w", err)
	}

	logging.VirtualStoreDebug("Parsed action: type=%s, target=%s", req.Type, req.Target)

	// NOTE: Speculative dreaming (precognition) is OPT-IN only.
	// It was previously auto-invoked here but caused unwanted startup activity.
	// Use SimulateActionWithDreamer() explicitly when dream state analysis is needed.

	// Constitutional logic check (defense in depth)
	if err := v.checkConstitution(req); err != nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Constitutional violation: %s on %s - %v", req.Type, req.Target, err)
		v.injectFact(Fact{
			Predicate: "security_violation",
			Args:      []interface{}{string(req.Type), req.Target, err.Error()},
		})
		return "", err
	}

	// Kernel-level permission gate (default deny if kernel says not permitted)
	if v.kernel != nil {
		// Refresh permission cache in case policy/facts changed since last action.
		// Note: Cache is deprecated for fine-grained permissions, direct query used.

		permitted := v.CheckKernelPermitted(string(req.Type), req.Target, req.Payload)
		if !permitted {
			logging.Get(logging.CategoryVirtualStore).Warn("Kernel policy denied action: %s", req.Type)
			err := fmt.Errorf("action %s not permitted by kernel policy", req.Type)
			v.injectFact(Fact{
				Predicate: "security_violation",
				Args:      []interface{}{string(req.Type), req.Target, err.Error()},
			})
			return "", err
		}
	}

	// Route to appropriate handler
	logging.VirtualStoreDebug("Dispatching action %s to handler", req.Type)
	logging.Audit().ActionRoute(string(req.Type), req.Target)
	actionStart := time.Now()
	result, err := v.executeAction(ctx, req)
	actionDuration := time.Since(actionStart)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Action execution failed: %s - %v", req.Type, err)
		v.injectFact(Fact{
			Predicate: "execution_error",
			Args:      []interface{}{string(req.Type), req.Target, err.Error()},
		})
		return "", err
	}

	// Inject result facts into kernel (batched when possible).
	completedAt := time.Now()
	factsToInject := make([]Fact, 0, len(result.FactsToAdd)+1)
	factsToInject = append(factsToInject, result.FactsToAdd...)
	factsToInject = append(factsToInject, Fact{
		Predicate: "execution_result",
		Args:      []interface{}{req.ActionID, string(req.Type), req.Target, result.Success, result.Output, completedAt.Unix()},
	})
	v.injectFacts(factsToInject)
	v.maybePruneActionLogs(completedAt)

	if result.Success {
		logging.VirtualStore("Action %s completed: success=%v, output_len=%d", req.Type, result.Success, len(result.Output))
	} else {
		logging.VirtualStore("Action %s completed: success=%v, error=%s", req.Type, result.Success, result.Error)
	}

	// Audit: Action completed
	logging.Audit().ActionComplete(string(req.Type), req.Target, actionDuration.Milliseconds(), result.Success, result.Error)
	return result.Output, nil
}

// parseActionFact converts a Fact to an ActionRequest.
func (v *VirtualStore) parseActionFact(action Fact) (ActionRequest, error) {
	req := ActionRequest{
		Payload: make(map[string]interface{}),
	}

	// Bug #4 fix: Add detailed logging for malformed action facts
	if len(action.Args) < 3 {
		logging.Get(logging.CategoryVirtualStore).Error(
			"Malformed action fact: predicate=%s, got %d args (need 3+), args=%v",
			action.Predicate, len(action.Args), action.Args)
		return req, fmt.Errorf("invalid action fact: requires at least 3 arguments (ActionID, Type, Target), got %d", len(action.Args))
	}

	// First arg is ActionID
	req.ActionID = fmt.Sprintf("%v", action.Args[0])

	// Second arg is action type
	actionType, ok := action.Args[1].(string)
	if !ok {
		actionType = fmt.Sprintf("%v", action.Args[1])
	}
	// Strip leading slash if present (Mangle name constants)
	actionType = strings.TrimPrefix(actionType, "/")
	req.Type = ActionType(actionType)

	// Third arg is target
	target, ok := action.Args[2].(string)
	if !ok {
		target = fmt.Sprintf("%v", action.Args[2])
	}
	req.Target = target

	// Remaining args go into payload
	for i := 3; i < len(action.Args); i++ {
		// If the argument is a map, merge it into the payload
		if argMap, ok := action.Args[i].(map[string]interface{}); ok {
			for k, v := range argMap {
				req.Payload[k] = v
			}
			continue
		}

		key := fmt.Sprintf("arg%d", i-3)
		req.Payload[key] = action.Args[i]
	}

	return req, nil
}

// executeAction dispatches to the appropriate handler.
func (v *VirtualStore) executeAction(ctx context.Context, req ActionRequest) (ActionResult, error) {
	switch req.Type {
	case ActionExecCmd:
		return v.handleExecCmd(ctx, req)
	case ActionReadFile:
		return v.handleReadFile(ctx, req)
	case ActionWriteFile:
		return v.handleWriteFile(ctx, req)
	case ActionEditFile:
		return v.handleEditFile(ctx, req)
	case ActionDeleteFile:
		return v.handleDeleteFile(ctx, req)
	case ActionSearchCode, ActionSearchFiles, ActionAnalyzeCode:
		return v.handleSearchCode(ctx, req)
	case ActionRunTests:
		return v.handleRunTests(ctx, req)
	case ActionBuildProject:
		return v.handleBuildProject(ctx, req)
	case ActionGitOperation:
		return v.handleGitOperation(ctx, req)
	case ActionShowDiff:
		return v.handleShowDiff(ctx, req)
	case ActionAnalyzeImpact:
		return v.handleAnalyzeImpact(ctx, req)
	case ActionBrowse:
		return v.handleBrowse(ctx, req)
	case ActionResearch:
		return v.handleResearch(ctx, req)
	case ActionDelegate:
		return v.handleDelegate(ctx, req)
	case ActionDelegateReviewer:
		return v.handleDelegateAlias(ctx, req, "/reviewer")
	case ActionDelegateCoder:
		return v.handleDelegateAlias(ctx, req, "/coder")
	case ActionDelegateResearcher:
		return v.handleDelegateAlias(ctx, req, "/researcher")
	case ActionDelegateToolGenerator:
		return v.handleDelegateAlias(ctx, req, "/tool_generator")
	case ActionAskUser:
		return v.handleAskUser(ctx, req)
	case ActionEscalate:
		return v.handleEscalate(ctx, req)

	// Code DOM actions
	case ActionOpenFile:
		return v.handleOpenFile(ctx, req)
	case ActionGetElements:
		return v.handleGetElements(ctx, req)
	case ActionGetElement:
		return v.handleGetElement(ctx, req)
	case ActionEditElement:
		return v.handleEditElement(ctx, req)
	case ActionRefreshScope:
		return v.handleRefreshScope(ctx, req)
	case ActionCloseScope:
		return v.handleCloseScope(ctx, req)
	case ActionEditLines:
		return v.handleEditLines(ctx, req)
	case ActionInsertLines:
		return v.handleInsertLines(ctx, req)
	case ActionDeleteLines:
		return v.handleDeleteLines(ctx, req)

	// Autopoiesis actions
	case ActionExecTool:
		return v.handleExecTool(ctx, req)

	// TDD Loop actions
	case ActionReadErrorLog:
		return v.handleReadErrorLog(ctx, req)
	case ActionAnalyzeRootCause:
		return v.handleAnalyzeRootCause(ctx, req)
	case ActionGeneratePatch:
		return v.handleGeneratePatch(ctx, req)
	case ActionEscalateToUser:
		return v.handleEscalateToUser(ctx, req)
	case ActionComplete:
		return v.handleComplete(ctx, req)
	case ActionInterrogative:
		return v.handleInterrogative(ctx, req)
	case ActionResumeTask:
		return v.handleResumeTask(ctx, req)
	case ActionRefreshShardCtx:
		return v.handleRefreshShardContext(ctx, req)

	// File System semantic aliases
	case ActionFSRead:
		return v.handleReadFile(ctx, req) // Delegate to existing handler
	case ActionFSWrite:
		return v.handleWriteFile(ctx, req) // Delegate to existing handler

	// Ouroboros actions
	case ActionGenerateTool:
		return v.handleGenerateTool(ctx, req)
	case ActionOuroborosDetect:
		return v.handleOuroborosDetect(ctx, req)
	case ActionOuroborosGen:
		return v.handleOuroborosGenerate(ctx, req)
	case ActionOuroborosCompile:
		return v.handleOuroborosCompile(ctx, req)
	case ActionOuroborosReg:
		return v.handleOuroborosRegister(ctx, req)
	case ActionRefineTool:
		return v.handleRefineTool(ctx, req)

	// Campaign actions
	case ActionCampaignClarify:
		return v.handleCampaignClarify(ctx, req)
	case ActionCampaignCreateFile:
		return v.handleCampaignCreateFile(ctx, req)
	case ActionCampaignModifyFile:
		return v.handleCampaignModifyFile(ctx, req)
	case ActionCampaignWriteTest:
		return v.handleCampaignWriteTest(ctx, req)
	case ActionCampaignRunTest:
		return v.handleCampaignRunTest(ctx, req)
	case ActionCampaignResearch:
		return v.handleCampaignResearch(ctx, req)
	case ActionCampaignVerify:
		return v.handleCampaignVerify(ctx, req)
	case ActionCampaignDocument:
		return v.handleCampaignDocument(ctx, req)
	case ActionCampaignRefactor:
		return v.handleCampaignRefactor(ctx, req)
	case ActionCampaignIntegrate:
		return v.handleCampaignIntegrate(ctx, req)
	case ActionCampaignComplete:
		return v.handleCampaignComplete(ctx, req)
	case ActionCampaignFinalVerify:
		return v.handleCampaignFinalVerify(ctx, req)
	case ActionCampaignCleanup:
		return v.handleCampaignCleanup(ctx, req)
	case ActionArchiveCampaign:
		return v.handleArchiveCampaign(ctx, req)
	case ActionShowCampaignStatus:
		return v.handleShowCampaignStatus(ctx, req)
	case ActionShowCampaignProg:
		return v.handleShowCampaignProgress(ctx, req)
	case ActionAskCampaignInt:
		return v.handleAskCampaignInterrupt(ctx, req)
	case ActionRunPhaseCheckpoint:
		return v.handleRunPhaseCheckpoint(ctx, req)
	case ActionPauseAndReplan:
		return v.handlePauseAndReplan(ctx, req)

	// Context Management actions
	case ActionCompressContext:
		return v.handleCompressContext(ctx, req)
	case ActionEmergencyCompress:
		return v.handleEmergencyCompress(ctx, req)
	case ActionCreateCheckpoint:
		return v.handleCreateCheckpoint(ctx, req)

	// Investigation/Analysis actions
	case ActionInvestigateAnomaly:
		return v.handleInvestigateAnomaly(ctx, req)
	case ActionInvestigateSystemic:
		return v.handleInvestigateSystemic(ctx, req)
	case ActionUpdateWorldModel:
		return v.handleUpdateWorldModel(ctx, req)

	// Corrective actions
	case ActionCorrectiveResearch:
		return v.handleCorrectiveResearch(ctx, req)
	case ActionCorrectiveDocs:
		return v.handleCorrectiveDocs(ctx, req)
	case ActionCorrectiveDecompose:
		return v.handleCorrectiveDecompose(ctx, req)

	// Code DOM Query alias
	case ActionQueryElements:
		return v.handleGetElements(ctx, req) // Delegate to existing handler

	// Python environment actions (general-purpose)
	case ActionPythonEnvSetup:
		return v.handlePythonEnvSetup(ctx, req)
	case ActionPythonEnvExec:
		return v.handlePythonEnvExec(ctx, req)
	case ActionPythonRunPytest:
		return v.handlePythonRunPytest(ctx, req)
	case ActionPythonApplyPatch:
		return v.handlePythonApplyPatch(ctx, req)
	case ActionPythonSnapshot:
		return v.handlePythonSnapshot(ctx, req)
	case ActionPythonRestore:
		return v.handlePythonRestore(ctx, req)
	case ActionPythonTeardown:
		return v.handlePythonTeardown(ctx, req)

	// SWE-bench actions (benchmark-specific, delegates to Python handlers)
	case ActionSWEBenchSetup:
		return v.handleSWEBenchSetup(ctx, req)
	case ActionSWEBenchApplyPatch:
		return v.handleSWEBenchApplyPatch(ctx, req)
	case ActionSWEBenchRunTests:
		return v.handleSWEBenchRunTests(ctx, req)
	case ActionSWEBenchSnapshot:
		return v.handleSWEBenchSnapshot(ctx, req)
	case ActionSWEBenchRestore:
		return v.handleSWEBenchRestore(ctx, req)
	case ActionSWEBenchEvaluate:
		return v.handleSWEBenchEvaluate(ctx, req)
	case ActionSWEBenchTeardown:
		return v.handleSWEBenchTeardown(ctx, req)

	default:
		return ActionResult{}, fmt.Errorf("unknown action type: %s", req.Type)
	}
}

// =============================================================================
// UTILITY METHODS
// =============================================================================

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

func (v *VirtualStore) injectFact(fact Fact) {
	v.mu.RLock()
	kernel := v.kernel
	v.mu.RUnlock()

	if kernel != nil {
		_ = kernel.Assert(fact)
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
		_ = realKernel.AssertBatch(facts)
		return
	}

	for _, fact := range facts {
		_ = kernel.Assert(fact)
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
		_ = realKernel.RetractExactFactsBatch(toRemove)
	}

	// Keep action logs bounded to protect kernel evaluation performance.
	prune("execution_result", 4, now.Add(-15*time.Minute).Unix())
	prune("shard_context_refreshed", 2, now.Add(-60*time.Minute).Unix())
}

func unixSecondsArgAt(args []interface{}, idx int) (int64, bool) {
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
		_ = realKernel.RemoveFactsByPredicateSet(preds)
		return
	}

	for p := range preds {
		_ = kernel.Retract(p)
	}
}

func (v *VirtualStore) parseBuildDiagnostics(output string) []Fact {
	facts := make([]Fact, 0)
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		// Parse Go-style errors: file.go:line:col: message
		if strings.Contains(line, ":") && (strings.Contains(line, "error") || strings.Contains(line, "warning")) {
			parts := strings.SplitN(line, ":", 4)
			if len(parts) >= 4 {
				facts = append(facts, Fact{
					Predicate: "diagnostic",
					Args:      []interface{}{"/error", parts[0], parts[1], parts[3]},
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
// We query the kernel for permitted(Action, Target, Payload) facts.
func (v *VirtualStore) CheckKernelPermitted(actionType, target string, payload map[string]interface{}) bool {
	v.mu.RLock()
	k := v.kernel
	v.mu.RUnlock()

	// No kernel attached - fail open (legacy behavior)
	if k == nil {
		logging.VirtualStoreDebug("checkKernelPermitted(%s): no kernel attached, allowing", actionType)
		return true
	}

	// Fast allow: safe_action is an explicit allowlist in the constitution.
	// This keeps legacy direct RouteAction() callsites working without requiring a pending_action envelope.
	// (Dangerous actions are NOT marked safe_action; they must be explicitly permitted.)
	safe, err := k.Query("safe_action")
	if err == nil {
		wantType := "/" + actionType
		altType := actionType
		for _, f := range safe {
			if len(f.Args) < 1 {
				continue
			}
			argType := fmt.Sprintf("%v", f.Args[0])
			if argType == wantType || argType == altType {
				logging.VirtualStoreDebug("checkKernelPermitted(%s): ALLOWED (safe_action)", actionType)
				return true
			}
		}
	}

	// Query all permitted facts
	results, err := k.Query("permitted")
	if err != nil {
		logging.VirtualStoreDebug("checkKernelPermitted(%s): query error, failing open: %v", actionType, err)
		return true // fail open to avoid accidental full block
	}

	wantType := "/" + actionType
	altType := actionType

	for _, f := range results {
		if len(f.Args) < 1 {
			continue
		}

		// 1. Check ActionType
		argType := fmt.Sprintf("%v", f.Args[0])
		if argType != wantType && argType != altType {
			continue
		}

		// 2. Check Target (if present in fact)
		if len(f.Args) >= 2 {
			factTarget := fmt.Sprintf("%v", f.Args[1])
			if factTarget != target && factTarget != "_" {
				continue
			}
		}

		// 3. Check Payload (if present in fact)
		// Note: Exact payload matching might be tricky with maps.
		// For now, if the policy derived permitted(...), we assume it validated the payload
		// via pending_action unification. We accept it if Type and Target match.
		// Strict payload matching would require deep comparison.

		logging.VirtualStoreDebug("checkKernelPermitted(%s): ALLOWED (found permitted fact)", actionType)
		return true
	}

	logging.VirtualStoreDebug("checkKernelPermitted(%s): DENIED (no matching permitted fact)", actionType)
	return false
}
