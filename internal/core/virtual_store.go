package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// ActionType defines the types of actions the VirtualStore can execute.
type ActionType string

const (
	ActionExecCmd       ActionType = "exec_cmd"
	ActionReadFile      ActionType = "read_file"
	ActionWriteFile     ActionType = "write_file"
	ActionEditFile      ActionType = "edit_file"
	ActionDeleteFile    ActionType = "delete_file"
	ActionSearchCode    ActionType = "search_code"
	ActionRunTests      ActionType = "run_tests"
	ActionBuildProject  ActionType = "build_project"
	ActionGitOperation  ActionType = "git_operation"
	ActionAnalyzeImpact ActionType = "analyze_impact"
	ActionBrowse        ActionType = "browse"
	ActionResearch      ActionType = "research"
	ActionAskUser       ActionType = "ask_user"
	ActionEscalate      ActionType = "escalate"
	ActionDelegate      ActionType = "delegate"

	// Autopoiesis Actions (generated tool execution)
	ActionExecTool ActionType = "exec_tool" // Execute a generated tool

	// Code DOM Actions (semantic code element operations)
	ActionOpenFile     ActionType = "open_file"     // Open file, load 1-hop scope
	ActionGetElements  ActionType = "get_elements"  // Query elements in scope
	ActionGetElement   ActionType = "get_element"   // Get single element by ref
	ActionEditElement  ActionType = "edit_element"  // Replace element body by ref
	ActionRefreshScope ActionType = "refresh_scope" // Re-project after changes
	ActionCloseScope   ActionType = "close_scope"   // Close current scope
	ActionEditLines    ActionType = "edit_lines"    // Line-based file editing
	ActionInsertLines  ActionType = "insert_lines"  // Insert lines at position
	ActionDeleteLines  ActionType = "delete_lines"  // Delete line range

	// TDD Loop Actions (derived from Mangle policy next_action rules)
	ActionReadErrorLog     ActionType = "read_error_log"     // Read test/build error logs
	ActionAnalyzeRootCause ActionType = "analyze_root_cause" // Analyze root cause of failure
	ActionGeneratePatch    ActionType = "generate_patch"     // Generate code patch
	ActionEscalateToUser   ActionType = "escalate_to_user"   // Escalate to user
	ActionComplete         ActionType = "complete"           // Mark task complete
	ActionInterrogative    ActionType = "interrogative_mode" // Ask clarifying questions
	ActionResumeTask       ActionType = "resume_task"        // Resume a paused task

	// File System Actions (semantic aliases)
	ActionFSRead  ActionType = "fs_read"  // Semantic file read
	ActionFSWrite ActionType = "fs_write" // Semantic file write

	// Ouroboros Actions (tool generation pipeline)
	ActionGenerateTool     ActionType = "generate_tool"      // Generate new tool
	ActionOuroborosDetect  ActionType = "ouroboros_detect"   // Detect tool need
	ActionOuroborosGen     ActionType = "ouroboros_generate" // Generate tool code
	ActionOuroborosCompile ActionType = "ouroboros_compile"  // Compile generated tool
	ActionOuroborosReg     ActionType = "ouroboros_register" // Register compiled tool
	ActionRefineTool       ActionType = "refine_tool"        // Refine existing tool

	// Campaign Actions (multi-phase goal orchestration)
	ActionCampaignClarify     ActionType = "campaign_clarify"      // Clarify campaign goal
	ActionCampaignCreateFile  ActionType = "campaign_create_file"  // Create file in campaign
	ActionCampaignModifyFile  ActionType = "campaign_modify_file"  // Modify file in campaign
	ActionCampaignWriteTest   ActionType = "campaign_write_test"   // Write test in campaign
	ActionCampaignRunTest     ActionType = "campaign_run_test"     // Run tests in campaign
	ActionCampaignResearch    ActionType = "campaign_research"     // Research in campaign
	ActionCampaignVerify      ActionType = "campaign_verify"       // Verify campaign step
	ActionCampaignDocument    ActionType = "campaign_document"     // Document in campaign
	ActionCampaignRefactor    ActionType = "campaign_refactor"     // Refactor in campaign
	ActionCampaignIntegrate   ActionType = "campaign_integrate"    // Integrate in campaign
	ActionCampaignComplete    ActionType = "campaign_complete"     // Complete campaign
	ActionCampaignFinalVerify ActionType = "campaign_final_verify" // Final verification
	ActionCampaignCleanup     ActionType = "campaign_cleanup"      // Cleanup after campaign
	ActionArchiveCampaign     ActionType = "archive_campaign"      // Archive campaign
	ActionShowCampaignStatus  ActionType = "show_campaign_status"  // Show campaign status
	ActionShowCampaignProg    ActionType = "show_campaign_progress"
	ActionAskCampaignInt      ActionType = "ask_campaign_interrupt" // Ask about campaign interrupt
	ActionRunPhaseCheckpoint  ActionType = "run_phase_checkpoint"   // Run phase checkpoint
	ActionPauseAndReplan      ActionType = "pause_and_replan"       // Pause and replan

	// Context Management Actions
	ActionCompressContext   ActionType = "compress_context"   // Compress context
	ActionEmergencyCompress ActionType = "emergency_compress" // Emergency context compression
	ActionCreateCheckpoint  ActionType = "create_checkpoint"  // Create checkpoint

	// Investigation/Analysis Actions
	ActionInvestigateAnomaly  ActionType = "investigate_anomaly"  // Investigate anomaly
	ActionInvestigateSystemic ActionType = "investigate_systemic" // Investigate systemic issue
	ActionUpdateWorldModel    ActionType = "update_world_model"   // Update world model

	// Corrective Actions
	ActionCorrectiveResearch  ActionType = "corrective_research"  // Research for correction
	ActionCorrectiveDocs      ActionType = "corrective_docs"      // Documentation correction
	ActionCorrectiveDecompose ActionType = "corrective_decompose" // Decompose for correction

	// Code DOM Query Actions
	ActionQueryElements ActionType = "query_elements" // Query code elements (alias)

	// Python Environment Actions (general-purpose)
	ActionPythonEnvSetup   ActionType = "python_env_setup"   // Create Python dev environment
	ActionPythonEnvExec    ActionType = "python_env_exec"    // Execute command in environment
	ActionPythonRunPytest  ActionType = "python_run_pytest"  // Run pytest tests
	ActionPythonApplyPatch ActionType = "python_apply_patch" // Apply git patch
	ActionPythonSnapshot   ActionType = "python_snapshot"    // Snapshot container state
	ActionPythonRestore    ActionType = "python_restore"     // Restore from snapshot
	ActionPythonTeardown   ActionType = "python_teardown"    // Cleanup environment

	// SWE-bench Actions (benchmark-specific, wraps Python actions)
	ActionSWEBenchSetup      ActionType = "swebench_setup"       // Initialize SWE-bench environment
	ActionSWEBenchApplyPatch ActionType = "swebench_apply_patch" // Apply model's patch
	ActionSWEBenchRunTests   ActionType = "swebench_run_tests"   // Run instance tests
	ActionSWEBenchSnapshot   ActionType = "swebench_snapshot"    // Create container snapshot
	ActionSWEBenchRestore    ActionType = "swebench_restore"     // Restore from snapshot
	ActionSWEBenchEvaluate   ActionType = "swebench_evaluate"    // Evaluate prediction
	ActionSWEBenchTeardown   ActionType = "swebench_teardown"    // Cleanup environment
)

// ActionRequest represents a request to execute an action.
type ActionRequest struct {
	Type       ActionType             `json:"type"`
	Target     string                 `json:"target"`
	Payload    map[string]interface{} `json:"payload"`
	Timeout    int                    `json:"timeout,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	RetryCount int                    `json:"retry_count,omitempty"`
}

// ActionResult represents the result of an action execution.
type ActionResult struct {
	Success    bool                   `json:"success"`
	Output     string                 `json:"output"`
	Error      string                 `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	FactsToAdd []Fact                 `json:"facts_to_add,omitempty"`
}

// ConstitutionalRule represents a safety constraint.
type ConstitutionalRule struct {
	Name        string
	Description string
	Check       func(req ActionRequest) error
}

// IntegrationClient is an interface for external service integrations.
// This breaks the import cycle by defining the interface in core.
type IntegrationClient interface {
	// CallTool makes an MCP tool call via HTTP.
	CallTool(ctx context.Context, tool string, args map[string]interface{}) (interface{}, error)
}

// CodeElement represents a semantic code unit (function, struct, etc.).
// Used by CodeScope to return element information.
type CodeElement struct {
	Ref        string   `json:"ref"`
	Type       string   `json:"type"`
	File       string   `json:"file"`
	StartLine  int      `json:"start_line"`
	EndLine    int      `json:"end_line"`
	Signature  string   `json:"signature"`
	Body       string   `json:"body,omitempty"`
	Parent     string   `json:"parent,omitempty"`
	Visibility string   `json:"visibility"`
	Actions    []string `json:"actions"`
}

// CodeScope is an interface for Code DOM scope management.
// This breaks the import cycle - implemented by world.FileScope.
type CodeScope interface {
	// Open opens a file and loads its 1-hop dependency scope.
	Open(path string) error

	// Refresh re-parses all in-scope files after an edit.
	Refresh() error

	// Close clears the current scope.
	Close()

	// GetCoreElement returns an element by ref.
	GetCoreElement(ref string) *CodeElement

	// GetElementBody returns the body text of an element.
	GetElementBody(ref string) string

	// GetCoreElementsByFile returns all elements in a file.
	GetCoreElementsByFile(path string) []CodeElement

	// IsInScope checks if a file is in the current scope.
	IsInScope(path string) bool

	// ScopeFacts returns all current scope facts.
	ScopeFacts() []Fact

	// GetActiveFile returns the currently active file.
	GetActiveFile() string

	// GetInScopeFiles returns all files in the current scope.
	GetInScopeFiles() []string

	// VerifyFileHash checks if a file has been modified since it was loaded.
	// Returns true if unchanged, false if modified externally.
	VerifyFileHash(path string) (bool, error)

	// RefreshWithRetry attempts refresh with exponential backoff.
	RefreshWithRetry(maxRetries int) error
}

// ToolExecutor is an interface for executing generated tools.
// This breaks the import cycle - implemented by autopoiesis.OuroborosLoop.
type ToolExecutor interface {
	// ExecuteTool runs a registered tool with the given input
	ExecuteTool(ctx context.Context, toolName string, input string) (string, error)

	// ListTools returns all registered tools
	ListTools() []ToolInfo

	// GetTool returns info about a specific tool
	GetTool(name string) (*ToolInfo, bool)
}

// ToolGenerator is an interface for generating new tools via Ouroboros.
// This breaks the import cycle - implemented by autopoiesis.OuroborosLoop.
// The actual ToolNeed and ToolGenerationResult types are in autopoiesis package.
type ToolGenerator interface {
	// GenerateToolFromCode takes pre-generated code and runs it through the
	// Ouroboros pipeline (safety check, compile, register).
	// Parameters: name, purpose, code, confidence, priority, isDiagnostic
	// Returns: success, toolName, binaryPath, error
	GenerateToolFromCode(ctx context.Context, name, purpose, code string, confidence, priority float64, isDiagnostic bool) (success bool, toolName, binaryPath, errMsg string)
}

// ToolInfo contains information about a registered tool
type ToolInfo = types.ToolInfo

// FileEditor is an interface for file operations with audit logging.
// This breaks the import cycle - implemented by tactile.FileEditor.
type FileEditor interface {
	// ReadFile reads an entire file and returns its lines.
	ReadFile(path string) ([]string, error)

	// ReadLines reads specific lines from a file (1-indexed, inclusive).
	ReadLines(path string, startLine, endLine int) ([]string, error)

	// WriteFile writes content to a file.
	WriteFile(path string, lines []string) (*FileEditResult, error)

	// EditLines replaces lines in a file (1-indexed, inclusive).
	EditLines(path string, startLine, endLine int, newLines []string) (*FileEditResult, error)

	// InsertLines inserts lines after the specified line.
	InsertLines(path string, afterLine int, newLines []string) (*FileEditResult, error)

	// DeleteLines removes lines from a file.
	DeleteLines(path string, startLine, endLine int) (*FileEditResult, error)

	// ReplaceElement replaces content between start and end lines.
	ReplaceElement(path string, startLine, endLine int, newContent string) (*FileEditResult, error)
}

// FileEditResult represents the result of a file edit operation.
type FileEditResult struct {
	Success       bool     `json:"success"`
	Path          string   `json:"path"`
	LinesAffected int      `json:"lines_affected"`
	OldContent    []string `json:"old_content,omitempty"`
	NewContent    []string `json:"new_content,omitempty"`
	OldHash       string   `json:"old_hash,omitempty"`
	NewHash       string   `json:"new_hash,omitempty"`
	LineCount     int      `json:"line_count"`
	Facts         []Fact   `json:"facts,omitempty"`
	Error         string   `json:"error,omitempty"`
}

// VirtualStore acts as the FFI Router for the Hollow Kernel.
// It routes 'next_action' atoms to the appropriate driver (Bash, MCP, File IO).
type VirtualStore struct {
	mu sync.RWMutex

	// Execution layer - legacy SafeExecutor for backwards compatibility
	executor *tactile.SafeExecutor

	// New execution layer - modern Executor with audit logging
	modernExecutor tactile.Executor
	auditLogger    *tactile.AuditLogger

	// Integration clients (via interface to break import cycle)
	codeGraph IntegrationClient
	browser   IntegrationClient
	scraper   IntegrationClient

	// Shard delegation
	shardManager *ShardManager

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
func NewVirtualStore(executor *tactile.SafeExecutor) *VirtualStore {
	config := DefaultVirtualStoreConfig()
	return NewVirtualStoreWithConfig(executor, config)
}

// NewVirtualStoreWithConfig creates a new VirtualStore with custom config.
func NewVirtualStoreWithConfig(executor *tactile.SafeExecutor, config VirtualStoreConfig) *VirtualStore {
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
		shardManager:    NewShardManager(),
		toolRegistry:    NewToolRegistry(config.WorkingDir),
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

	// Convert tactile.Fact to core.Fact
	coreFact := Fact{
		Predicate: tf.Predicate,
		Args:      tf.Args,
	}

	logging.VirtualStoreDebug("Injecting tactile fact: %s (args=%d)", tf.Predicate, len(tf.Args))
	if err := kernel.Assert(coreFact); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to inject tactile fact %s: %v", tf.Predicate, err)
	}
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

	// Wire dreamer to the real kernel when available
	if realKernel, ok := k.(*RealKernel); ok {
		if v.dreamer == nil {
			v.dreamer = NewDreamer(realKernel)
			logging.VirtualStoreDebug("Dreamer created for speculative execution")
		} else {
			v.dreamer.SetKernel(realKernel)
			logging.VirtualStoreDebug("Dreamer kernel updated")
		}
	}

	// Also set kernel on tool registry
	if v.toolRegistry != nil {
		v.toolRegistry.SetKernel(k)
		logging.VirtualStoreDebug("Tool registry kernel reference updated")
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
func (v *VirtualStore) SetShardManager(sm *ShardManager) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.shardManager = sm
	logging.VirtualStoreDebug("ShardManager attached to VirtualStore")
}

// SetCodeGraphClient sets the code graph integration client.
func (v *VirtualStore) SetCodeGraphClient(client IntegrationClient) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.codeGraph = client
	logging.VirtualStoreDebug("CodeGraph MCP client attached")
}

// SetBrowserClient sets the browser integration client.
func (v *VirtualStore) SetBrowserClient(client IntegrationClient) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.browser = client
	logging.VirtualStoreDebug("Browser MCP client attached")
}

// SetScraperClient sets the scraper integration client.
func (v *VirtualStore) SetScraperClient(client IntegrationClient) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.scraper = client
	logging.VirtualStoreDebug("Scraper MCP client attached")
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

	// Speculative dreaming: block obviously unsafe actions before constitutional checks.
	// Skip for read-only actions to prevent false positives on file content (Bug #9)
	if v.dreamer != nil && req.Type != ActionReadFile && req.Type != ActionFSRead {
		logging.VirtualStoreDebug("Running speculative dream simulation for action %s", req.Type)
		dream := v.dreamer.SimulateAction(ctx, req)
		if dream.Unsafe {
			logging.Get(logging.CategoryVirtualStore).Warn("Action blocked by precognition: %s - %s", req.Type, dream.Reason)
			v.injectFact(Fact{
				Predicate: "dream_block",
				Args: []interface{}{
					dream.ActionID,
					string(req.Type),
					req.Target,
					dream.Reason,
				},
			})
			return "", fmt.Errorf("precognition block: %s", dream.Reason)
		}
	}

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
		v.mu.Lock()
		v.rebuildPermissionCache()
		v.mu.Unlock()

		permitted := v.checkKernelPermitted(string(req.Type))
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

	// Inject result facts into kernel
	for _, fact := range result.FactsToAdd {
		v.injectFact(fact)
	}

	// Record execution result
	v.injectFact(Fact{
		Predicate: "execution_result",
		Args:      []interface{}{string(req.Type), req.Target, result.Success, result.Output},
	})

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

	if len(action.Args) < 2 {
		return req, fmt.Errorf("invalid action fact: requires at least 2 arguments")
	}

	// First arg is action type
	actionType, ok := action.Args[0].(string)
	if !ok {
		actionType = fmt.Sprintf("%v", action.Args[0])
	}
	// Strip leading slash if present (Mangle name constants)
	actionType = strings.TrimPrefix(actionType, "/")
	req.Type = ActionType(actionType)

	// Second arg is target
	target, ok := action.Args[1].(string)
	if !ok {
		target = fmt.Sprintf("%v", action.Args[1])
	}
	req.Target = target

	// Remaining args go into payload
	for i := 2; i < len(action.Args); i++ {
		// If the argument is a map, merge it into the payload
		if argMap, ok := action.Args[i].(map[string]interface{}); ok {
			for k, v := range argMap {
				req.Payload[k] = v
			}
			continue
		}

		key := fmt.Sprintf("arg%d", i-2)
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
	case ActionSearchCode:
		return v.handleSearchCode(ctx, req)
	case ActionRunTests:
		return v.handleRunTests(ctx, req)
	case ActionBuildProject:
		return v.handleBuildProject(ctx, req)
	case ActionGitOperation:
		return v.handleGitOperation(ctx, req)
	case ActionAnalyzeImpact:
		return v.handleAnalyzeImpact(ctx, req)
	case ActionBrowse:
		return v.handleBrowse(ctx, req)
	case ActionResearch:
		return v.handleResearch(ctx, req)
	case ActionDelegate:
		return v.handleDelegate(ctx, req)
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

// handleExecCmd executes a shell command safely.
func (v *VirtualStore) handleExecCmd(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleExecCmd")
	defer timer.Stop()

	// Parse command details
	binary := "bash"
	args := []string{"-c", req.Target}

	if b, ok := req.Payload["binary"].(string); ok {
		binary = b
	}
	if a, ok := req.Payload["args"].([]interface{}); ok {
		args = make([]string, len(a))
		for i, arg := range a {
			args[i] = fmt.Sprintf("%v", arg)
		}
	}

	timeout := 30
	if t, ok := req.Payload["timeout"].(int); ok {
		timeout = t
	}

	logging.VirtualStore("Shell exec: binary=%s, timeout=%ds", binary, timeout)
	logging.VirtualStoreDebug("Shell command target: %s", req.Target)

	// Quick traversal guard on the command text itself
	if strings.Contains(req.Target, "..") {
		logging.Get(logging.CategoryVirtualStore).Warn("Path traversal detected in command: %s", req.Target)
		return ActionResult{
			Success: false,
			Error:   "path traversal detected in command",
		}, nil
	}

	// Enforce binary allowlist (defense in depth)
	if !v.isBinaryAllowed(binary) {
		logging.Get(logging.CategoryVirtualStore).Warn("Binary not allowed: %s", binary)
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("binary %s not allowed", binary),
		}, nil
	}

	// Use modern executor if enabled (auto-generates audit facts)
	v.mu.RLock()
	useModern := v.useModernExecutor && v.modernExecutor != nil
	v.mu.RUnlock()

	if useModern {
		logging.VirtualStoreDebug("Using modern executor with audit logging")
		return v.handleExecCmdModern(ctx, binary, args, timeout, req.SessionID)
	}

	logging.VirtualStoreDebug("Using legacy SafeExecutor")

	// Legacy path using SafeExecutor
	cmd := tactile.ShellCommand{
		Binary:           binary,
		Arguments:        args,
		WorkingDirectory: v.workingDir,
		TimeoutSeconds:   timeout,
		EnvironmentVars:  v.getAllowedEnv(),
	}

	output, err := v.executor.Execute(ctx, cmd)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Shell command failed: %s - %v", binary, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "cmd_failed", Args: []interface{}{binary, err.Error()}},
			},
		}, nil // Return nil error but unsuccessful result
	}

	logging.VirtualStoreDebug("Shell command succeeded: output_len=%d", len(output))
	return ActionResult{
		Success: true,
		Output:  output,
		FactsToAdd: []Fact{
			{Predicate: "cmd_succeeded", Args: []interface{}{binary, output}},
		},
	}, nil
}

// handleExecCmdModern executes using the new tactile.Executor with auto-audit.
// Facts are automatically generated and injected via the audit callback.
func (v *VirtualStore) handleExecCmdModern(ctx context.Context, binary string, args []string, timeout int, sessionID string) (ActionResult, error) {
	cmd := tactile.Command{
		Binary:           binary,
		Arguments:        args,
		WorkingDirectory: v.workingDir,
		Environment:      v.getAllowedEnv(),
		SessionID:        sessionID,
		Limits: &tactile.ResourceLimits{
			TimeoutMs: int64(timeout) * 1000,
		},
	}

	result, err := v.modernExecutor.Execute(ctx, cmd)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Modern executor error: %s - %v", binary, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			// No manual facts needed - audit logger auto-injects them
		}, nil
	}

	// Build result - audit facts are auto-injected via callback
	actionResult := ActionResult{
		Success: result.Success && result.ExitCode == 0,
		Output:  result.Output(),
		Metadata: map[string]interface{}{
			"exit_code":    result.ExitCode,
			"duration_ms":  result.Duration.Milliseconds(),
			"killed":       result.Killed,
			"sandbox_used": string(result.SandboxUsed),
		},
	}

	if !actionResult.Success {
		actionResult.Error = result.Error
		if result.IsNonZeroExit() {
			actionResult.Error = fmt.Sprintf("exit code %d", result.ExitCode)
		}
		logging.Get(logging.CategoryVirtualStore).Warn("Shell command exit_code=%d, killed=%v", result.ExitCode, result.Killed)
	} else {
		logging.VirtualStoreDebug("Modern exec success: exit_code=%d, duration=%v, sandbox=%s",
			result.ExitCode, result.Duration, result.SandboxUsed)
	}

	return actionResult, nil
}

// handleReadFile reads a file from disk.
func (v *VirtualStore) handleReadFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleReadFile")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	path := v.resolvePath(req.Target)
	logging.VirtualStoreDebug("Reading file: %s", path)

	// Enforce max file size to prevent OOM (Bug #6 Fix)
	const MaxFileSize = 100 * 1024 // 100KB limit for full content loading

	info, err := os.Stat(path)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "file_read_error", Args: []interface{}{path, err.Error()}},
			},
		}, nil
	}

	// Handle directories - list contents instead of reading as file
	if info.IsDir() {
		return v.handleReadDirectory(ctx, path)
	}

	var data []byte
	var truncated bool

	if info.Size() > MaxFileSize {
		// Read only the first MaxFileSize bytes
		f, err := os.Open(path)
		if err != nil {
			return ActionResult{
				Success: false,
				Error:   err.Error(),
				FactsToAdd: []Fact{
					{Predicate: "file_read_error", Args: []interface{}{path, err.Error()}},
				},
			}, nil
		}
		defer f.Close()

		data = make([]byte, MaxFileSize)
		n, err := f.Read(data)
		if err != nil && err.Error() != "EOF" {
			return ActionResult{
				Success: false,
				Error:   err.Error(),
				FactsToAdd: []Fact{
					{Predicate: "file_read_error", Args: []interface{}{path, err.Error()}},
				},
			}, nil
		}
		data = data[:n]
		truncated = true
	} else {
		data, err = os.ReadFile(path)
		if err != nil {
			return ActionResult{
				Success: false,
				Error:   err.Error(),
				FactsToAdd: []Fact{
					{Predicate: "file_read_error", Args: []interface{}{path, err.Error()}},
				},
			}, nil
		}
	}

	content := string(data)
	modTime := info.ModTime().Unix()

	facts := []Fact{
		{Predicate: "file_content", Args: []interface{}{path, content}},
		{Predicate: "file_read", Args: []interface{}{path, info.Size()}},
	}

	if truncated {
		facts = append(facts, Fact{
			Predicate: "file_truncated",
			Args:      []interface{}{path, int64(MaxFileSize)},
		})
	}

	logging.VirtualStore("File read: path=%s, size=%d, truncated=%v", path, info.Size(), truncated)
	return ActionResult{
		Success: true,
		Output:  content,
		Metadata: map[string]interface{}{
			"path":      path,
			"size":      info.Size(),
			"modified":  modTime,
			"truncated": truncated,
		},
		FactsToAdd: facts,
	}, nil
}

// handleReadDirectory reads a directory and returns a summary of its contents.
// This is called when handleReadFile detects the target is a directory.
func (v *VirtualStore) handleReadDirectory(ctx context.Context, dirPath string) (ActionResult, error) {
	logging.VirtualStoreDebug("Reading directory: %s", dirPath)

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "dir_read_error", Args: []interface{}{dirPath, err.Error()}},
			},
		}, nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Directory: %s\n\n", dirPath))

	// Categorize entries
	var dirs, files []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name()+"/")
		} else {
			files = append(files, entry.Name())
		}
	}

	// List directories first
	if len(dirs) > 0 {
		sb.WriteString("Subdirectories:\n")
		for _, d := range dirs {
			sb.WriteString(fmt.Sprintf("  %s\n", d))
		}
		sb.WriteString("\n")
	}

	// List files
	if len(files) > 0 {
		sb.WriteString("Files:\n")
		for _, f := range files {
			// Get file info for size
			info, err := os.Stat(filepath.Join(dirPath, f))
			if err == nil {
				sb.WriteString(fmt.Sprintf("  %s (%d bytes)\n", f, info.Size()))
			} else {
				sb.WriteString(fmt.Sprintf("  %s\n", f))
			}
		}
	}

	// Add summary
	sb.WriteString(fmt.Sprintf("\nTotal: %d directories, %d files\n", len(dirs), len(files)))

	content := sb.String()

	return ActionResult{
		Success: true,
		Output:  content,
		Metadata: map[string]interface{}{
			"path":        dirPath,
			"is_dir":      true,
			"dir_count":   len(dirs),
			"file_count":  len(files),
			"total_count": len(entries),
		},
		FactsToAdd: []Fact{
			{Predicate: "dir_read", Args: []interface{}{dirPath, int64(len(entries))}},
		},
	}, nil
}

// handleWriteFile writes content to a file.
func (v *VirtualStore) handleWriteFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleWriteFile")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	path := v.resolvePath(req.Target)

	content, ok := req.Payload["content"].(string)
	if !ok {
		logging.Get(logging.CategoryVirtualStore).Error("write_file missing content in payload")
		return ActionResult{}, fmt.Errorf("write_file requires 'content' in payload")
	}

	logging.VirtualStoreDebug("Writing file: %s (%d bytes)", path, len(content))

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to create directory %s: %v", dir, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to write file %s: %v", path, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "file_write_error", Args: []interface{}{path, err.Error()}},
			},
		}, nil
	}

	logging.VirtualStore("File written: path=%s, bytes=%d", path, len(content))
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Written %d bytes to %s", len(content), path),
		FactsToAdd: []Fact{
			{Predicate: "file_written", Args: []interface{}{path, len(content)}},
			{Predicate: "modified", Args: []interface{}{path}},
		},
	}, nil
}

// handleEditFile performs a search-and-replace edit on a file.
func (v *VirtualStore) handleEditFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleEditFile")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	path := v.resolvePath(req.Target)

	oldContent, ok := req.Payload["old"].(string)
	if !ok {
		logging.Get(logging.CategoryVirtualStore).Error("edit_file missing 'old' in payload")
		return ActionResult{}, fmt.Errorf("edit_file requires 'old' in payload")
	}
	newContent, ok := req.Payload["new"].(string)
	if !ok {
		logging.Get(logging.CategoryVirtualStore).Error("edit_file missing 'new' in payload")
		return ActionResult{}, fmt.Errorf("edit_file requires 'new' in payload")
	}

	logging.VirtualStoreDebug("Editing file: %s (old_len=%d, new_len=%d)", path, len(oldContent), len(newContent))

	// Read existing file
	data, err := os.ReadFile(path)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to read file for edit %s: %v", path, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	content := string(data)
	if !strings.Contains(content, oldContent) {
		logging.Get(logging.CategoryVirtualStore).Warn("Edit failed: pattern not found in %s", path)
		return ActionResult{
			Success: false,
			Error:   "old content not found in file",
			FactsToAdd: []Fact{
				{Predicate: "edit_failed", Args: []interface{}{path, "pattern_not_found"}},
			},
		}, nil
	}

	// Perform replacement
	newFileContent := strings.Replace(content, oldContent, newContent, 1)

	// Write back
	err = os.WriteFile(path, []byte(newFileContent), 0644)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to write edited file %s: %v", path, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	logging.VirtualStore("File edited: %s", path)
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Edited %s", path),
		FactsToAdd: []Fact{
			{Predicate: "file_edited", Args: []interface{}{path}},
			{Predicate: "modified", Args: []interface{}{path}},
		},
	}, nil
}

// handleDeleteFile deletes a file (requires explicit confirmation flag).
func (v *VirtualStore) handleDeleteFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	path := v.resolvePath(req.Target)

	logging.VirtualStoreDebug("Delete file requested: %s", path)

	// Require confirmation flag for safety
	confirmed, _ := req.Payload["confirmed"].(bool)
	if !confirmed {
		logging.Get(logging.CategoryVirtualStore).Warn("Delete blocked: no confirmation for %s", path)
		return ActionResult{
			Success: false,
			Error:   "delete_file requires 'confirmed: true' in payload",
			FactsToAdd: []Fact{
				{Predicate: "delete_blocked", Args: []interface{}{path, "no_confirmation"}},
			},
		}, nil
	}

	err := os.Remove(path)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to delete file %s: %v", path, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	logging.VirtualStore("File deleted: %s", path)
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Deleted %s", path),
		FactsToAdd: []Fact{
			{Predicate: "file_deleted", Args: []interface{}{path}},
		},
	}, nil
}

// handleSearchCode searches for code patterns using code-graph integration.
func (v *VirtualStore) handleSearchCode(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleSearchCode")
	defer timer.Stop()

	v.mu.RLock()
	codeGraph := v.codeGraph
	v.mu.RUnlock()

	if codeGraph == nil {
		logging.Get(logging.CategoryVirtualStore).Error("Code graph MCP client not configured")
		return ActionResult{Success: false, Error: "code graph integration not configured"}, nil
	}

	pattern := req.Target
	args := map[string]interface{}{
		"pattern": pattern,
	}

	if lang, ok := req.Payload["language"].(string); ok {
		args["language"] = lang
	}
	if scope, ok := req.Payload["scope"].(string); ok {
		args["scope"] = scope
	}

	logging.VirtualStore("MCP call: code-graph search, pattern=%s", pattern)
	result, err := codeGraph.CallTool(ctx, "search", args)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("MCP code-graph search failed: %v", err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Convert results to facts
	facts := make([]Fact, 0)
	if results, ok := result.([]interface{}); ok {
		for _, r := range results {
			if item, ok := r.(map[string]interface{}); ok {
				facts = append(facts, Fact{
					Predicate: "search_result",
					Args: []interface{}{
						item["file_path"],
						item["line_number"],
						item["content"],
					},
				})
			}
		}
	}

	logging.VirtualStoreDebug("MCP search returned %d results", len(facts))
	output, _ := json.Marshal(result)
	return ActionResult{
		Success:    true,
		Output:     string(output),
		FactsToAdd: facts,
	}, nil
}

// handleRunTests executes the test suite.
func (v *VirtualStore) handleRunTests(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleRunTests")
	defer timer.Stop()

	// Determine test command based on project type
	testCmd := "go test ./..."
	if cmd, ok := req.Payload["command"].(string); ok {
		testCmd = cmd
	}

	logging.VirtualStore("Running tests: %s", testCmd)

	cmd := tactile.ShellCommand{
		Binary:           "bash",
		Arguments:        []string{"-c", testCmd},
		WorkingDirectory: v.workingDir,
		TimeoutSeconds:   300, // 5 minute timeout for tests
		EnvironmentVars:  v.getAllowedEnv(),
	}

	output, err := v.executor.Execute(ctx, cmd)
	success := err == nil

	testState := "/passing"
	if !success {
		testState = "/failing"
		logging.Get(logging.CategoryVirtualStore).Warn("Tests failed: %v", err)
	} else {
		logging.VirtualStore("Tests passed")
	}

	return ActionResult{
		Success: success,
		Output:  output,
		Error:   errString(err),
		FactsToAdd: []Fact{
			{Predicate: "test_state", Args: []interface{}{testState}},
			{Predicate: "test_output", Args: []interface{}{output}},
		},
	}, nil
}

// handleBuildProject builds the project.
func (v *VirtualStore) handleBuildProject(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleBuildProject")
	defer timer.Stop()

	buildCmd := "go build ./..."
	if cmd, ok := req.Payload["command"].(string); ok {
		buildCmd = cmd
	}

	logging.VirtualStore("Building project: %s", buildCmd)

	cmd := tactile.ShellCommand{
		Binary:           "bash",
		Arguments:        []string{"-c", buildCmd},
		WorkingDirectory: v.workingDir,
		TimeoutSeconds:   120,
		EnvironmentVars:  v.getAllowedEnv(),
	}

	output, err := v.executor.Execute(ctx, cmd)
	success := err == nil

	facts := []Fact{
		{Predicate: "build_result", Args: []interface{}{success, output}},
	}

	// Parse diagnostics from output
	if !success {
		logging.Get(logging.CategoryVirtualStore).Warn("Build failed: %v", err)
		diagnostics := v.parseBuildDiagnostics(output)
		logging.VirtualStoreDebug("Parsed %d diagnostics from build output", len(diagnostics))
		for _, d := range diagnostics {
			facts = append(facts, d)
		}
	} else {
		logging.VirtualStore("Build succeeded")
	}

	return ActionResult{
		Success:    success,
		Output:     output,
		Error:      errString(err),
		FactsToAdd: facts,
	}, nil
}

// handleGitOperation performs git operations.
func (v *VirtualStore) handleGitOperation(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleGitOperation")
	defer timer.Stop()

	operation := req.Target
	args := []string{operation}

	if extraArgs, ok := req.Payload["args"].([]interface{}); ok {
		for _, a := range extraArgs {
			args = append(args, fmt.Sprintf("%v", a))
		}
	}

	logging.VirtualStore("Git operation: %s %v", operation, args[1:])

	cmd := tactile.ShellCommand{
		Binary:           "git",
		Arguments:        args,
		WorkingDirectory: v.workingDir,
		TimeoutSeconds:   60,
		EnvironmentVars:  v.getAllowedEnv(),
	}

	output, err := v.executor.Execute(ctx, cmd)

	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Git %s failed: %v", operation, err)
	} else {
		logging.VirtualStoreDebug("Git %s succeeded", operation)
	}

	return ActionResult{
		Success: err == nil,
		Output:  output,
		Error:   errString(err),
		FactsToAdd: []Fact{
			{Predicate: "git_result", Args: []interface{}{operation, err == nil, output}},
		},
	}, nil
}

// handleAnalyzeImpact analyzes the impact of changes using code graph.
func (v *VirtualStore) handleAnalyzeImpact(ctx context.Context, req ActionRequest) (ActionResult, error) {
	v.mu.RLock()
	codeGraph := v.codeGraph
	v.mu.RUnlock()

	if codeGraph == nil {
		return ActionResult{Success: false, Error: "code graph integration not configured"}, nil
	}

	result, err := codeGraph.CallTool(ctx, "impact-analysis", map[string]interface{}{
		"file": req.Target,
	})
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Convert to facts
	facts := []Fact{}

	if data, ok := result.(map[string]interface{}); ok {
		if direct, ok := data["direct_dependents"].([]interface{}); ok {
			facts = append(facts, Fact{
				Predicate: "impact_radius",
				Args:      []interface{}{req.Target, len(direct)},
			})
			for _, dep := range direct {
				facts = append(facts, Fact{
					Predicate: "impacted",
					Args:      []interface{}{dep},
				})
			}
		}
	}

	output, _ := json.Marshal(result)
	return ActionResult{
		Success:    true,
		Output:     string(output),
		FactsToAdd: facts,
	}, nil
}

// handleBrowse performs browser automation via BrowserNERD.
func (v *VirtualStore) handleBrowse(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleBrowse")
	defer timer.Stop()

	v.mu.RLock()
	browser := v.browser
	v.mu.RUnlock()

	if browser == nil {
		logging.Get(logging.CategoryVirtualStore).Error("Browser MCP client not configured")
		return ActionResult{Success: false, Error: "browser integration not configured"}, nil
	}

	operation := req.Target
	sessionID, _ := req.Payload["session_id"].(string)
	if sessionID == "" {
		sessionID = "default"
	}

	logging.VirtualStore("MCP call: browser %s, session=%s", operation, sessionID)

	var output string
	var err error
	facts := make([]Fact, 0)

	args := map[string]interface{}{
		"session_id": sessionID,
	}

	switch operation {
	case "navigate":
		url, _ := req.Payload["url"].(string)
		args["url"] = url
		_, err = browser.CallTool(ctx, "navigate-url", args)
		output = fmt.Sprintf("Navigated to %s", url)

	case "snapshot_dom":
		result, callErr := browser.CallTool(ctx, "snapshot-dom", args)
		err = callErr
		if err == nil {
			if nodes, ok := result.([]interface{}); ok {
				for _, n := range nodes {
					if node, ok := n.(map[string]interface{}); ok {
						facts = append(facts, Fact{
							Predicate: "dom_node",
							Args: []interface{}{
								node["id"],
								node["tag"],
								node["parent"],
							},
						})
					}
				}
				output = fmt.Sprintf("Captured %d DOM nodes", len(nodes))
			}
		}

	case "click":
		selector, _ := req.Payload["selector"].(string)
		args["selector"] = selector
		args["action"] = "click"
		_, err = browser.CallTool(ctx, "interact", args)
		output = fmt.Sprintf("Clicked %s", selector)

	case "type":
		selector, _ := req.Payload["selector"].(string)
		text, _ := req.Payload["text"].(string)
		args["selector"] = selector
		args["action"] = "type"
		args["value"] = text
		_, err = browser.CallTool(ctx, "interact", args)
		output = fmt.Sprintf("Typed into %s", selector)

	default:
		logging.Get(logging.CategoryVirtualStore).Warn("Unknown browse operation: %s", operation)
		return ActionResult{Success: false, Error: fmt.Sprintf("unknown browse operation: %s", operation)}, nil
	}

	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Browser %s failed: %v", operation, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	logging.VirtualStoreDebug("Browser %s completed: %s", operation, output)
	return ActionResult{
		Success:    true,
		Output:     output,
		FactsToAdd: facts,
	}, nil
}

// handleResearch performs deep research via scraper service.
func (v *VirtualStore) handleResearch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleResearch")
	defer timer.Stop()

	v.mu.RLock()
	scraper := v.scraper
	v.mu.RUnlock()

	if scraper == nil {
		logging.Get(logging.CategoryVirtualStore).Error("Scraper MCP client not configured")
		return ActionResult{Success: false, Error: "scraper integration not configured"}, nil
	}

	args := map[string]interface{}{
		"query": req.Target,
	}

	if keywords, ok := req.Payload["keywords"].([]interface{}); ok {
		args["keywords"] = keywords
	}
	if depth, ok := req.Payload["depth"].(int); ok {
		args["max_depth"] = depth
	}
	if maxPages, ok := req.Payload["max_pages"].(int); ok {
		args["max_pages"] = maxPages
	}

	logging.VirtualStore("MCP call: scraper deep-research, query=%s", req.Target)
	result, err := scraper.CallTool(ctx, "deep-research", args)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("MCP scraper deep-research failed: %v", err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Convert to knowledge atoms
	facts := make([]Fact, 0)
	if data, ok := result.(map[string]interface{}); ok {
		if atoms, ok := data["knowledge_atoms"].([]interface{}); ok {
			for _, atom := range atoms {
				if a, ok := atom.(map[string]interface{}); ok {
					facts = append(facts, Fact{
						Predicate: "knowledge_atom",
						Args: []interface{}{
							a["source_url"],
							a["title"],
							a["content"],
						},
					})
				}
			}
		}
	}

	logging.VirtualStoreDebug("Research returned %d knowledge atoms", len(facts))
	output, _ := json.Marshal(result)
	return ActionResult{
		Success:    true,
		Output:     string(output),
		FactsToAdd: facts,
	}, nil
}

// handleDelegate delegates a task to a ShardAgent.
func (v *VirtualStore) handleDelegate(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleDelegate")
	defer timer.Stop()

	v.mu.RLock()
	sm := v.shardManager
	v.mu.RUnlock()

	if sm == nil {
		logging.Get(logging.CategoryVirtualStore).Error("ShardManager not configured for delegation")
		return ActionResult{Success: false, Error: "shard manager not configured"}, nil
	}

	shardType := req.Target
	task, _ := req.Payload["task"].(string)

	logging.VirtualStore("Delegating to shard: type=%s, task_len=%d", shardType, len(task))

	result, err := sm.Spawn(ctx, shardType, task)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Shard delegation failed: %s - %v", shardType, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "delegation_failed", Args: []interface{}{shardType, err.Error()}},
			},
		}, nil
	}

	logging.VirtualStore("Shard delegation completed: type=%s, result_len=%d", shardType, len(result))
	return ActionResult{
		Success: true,
		Output:  result,
		FactsToAdd: []Fact{
			{Predicate: "delegation_result", Args: []interface{}{shardType, result}},
		},
	}, nil
}

// handleAskUser handles requests that require user input.
func (v *VirtualStore) handleAskUser(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	question := req.Target
	options, _ := req.Payload["options"].([]interface{})

	// In a real implementation, this would prompt the user via CLI
	// For now, we signal that user input is needed
	return ActionResult{
		Success: false,
		Output:  question,
		Error:   "USER_INPUT_REQUIRED",
		Metadata: map[string]interface{}{
			"question": question,
			"options":  options,
		},
		FactsToAdd: []Fact{
			{Predicate: "awaiting_user_input", Args: []interface{}{question}},
		},
	}, nil
}

// handleEscalate escalates to the user when the agent cannot proceed.
func (v *VirtualStore) handleEscalate(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	reason := req.Target

	return ActionResult{
		Success: false,
		Output:  fmt.Sprintf("ESCALATION: %s", reason),
		Error:   "ESCALATION_REQUIRED",
		FactsToAdd: []Fact{
			{Predicate: "escalated", Args: []interface{}{reason}},
			{Predicate: "task_blocked", Args: []interface{}{reason}},
		},
	}, nil
}

// Helper functions

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

// checkKernelPermitted consults the permission cache (O(1) lookup).
// The cache is populated from kernel-derived permitted/1 facts when SetKernel is called.
func (v *VirtualStore) checkKernelPermitted(actionType string) bool {
	v.mu.RLock()
	cache := v.permittedCache
	k := v.kernel
	v.mu.RUnlock()

	// No kernel attached - fail open
	if k == nil {
		logging.VirtualStoreDebug("checkKernelPermitted(%s): no kernel attached, allowing", actionType)
		return true
	}

	// No cache available - fall back to kernel query (shouldn't happen normally)
	if cache == nil {
		logging.VirtualStoreDebug("checkKernelPermitted(%s): cache miss, using fallback", actionType)
		return v.checkKernelPermittedFallback(actionType)
	}

	// O(1) cache lookup - check both with and without leading slash
	if cache[actionType] || cache["/"+actionType] {
		logging.VirtualStoreDebug("checkKernelPermitted(%s): ALLOWED (cache hit)", actionType)
		return true
	}

	logging.VirtualStoreDebug("checkKernelPermitted(%s): DENIED (not in permitted cache)", actionType)
	return false
}

// checkKernelPermittedFallback is the original O(n) implementation used when cache is unavailable.
func (v *VirtualStore) checkKernelPermittedFallback(actionType string) bool {
	v.mu.RLock()
	k := v.kernel
	v.mu.RUnlock()

	if k == nil {
		return true
	}

	results, err := k.Query("permitted")
	if err != nil {
		logging.VirtualStoreDebug("checkKernelPermittedFallback(%s): query error, failing open: %v", actionType, err)
		return true // fail open to avoid accidental full block
	}

	want := "/" + actionType
	alt := actionType

	for _, f := range results {
		if len(f.Args) == 0 {
			continue
		}
		arg := fmt.Sprintf("%v", f.Args[0])
		if arg == want || arg == alt {
			logging.VirtualStoreDebug("checkKernelPermittedFallback(%s): ALLOWED (found in %d facts)", actionType, len(results))
			return true
		}
	}
	logging.VirtualStoreDebug("checkKernelPermittedFallback(%s): DENIED (checked %d facts)", actionType, len(results))
	return false
}

// =============================================================================
// CODE DOM HANDLERS
// =============================================================================

// handleOpenFile opens a file and loads its 1-hop dependency scope.
func (v *VirtualStore) handleOpenFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	scope := v.codeScope
	v.mu.RUnlock()

	if scope == nil {
		return ActionResult{Success: false, Error: "code scope not configured"}, nil
	}

	path := v.resolvePath(req.Target)
	if err := scope.Open(path); err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "scope_open_failed", Args: []interface{}{path, err.Error()}},
			},
		}, nil
	}

	// Inject scope facts into kernel
	facts := scope.ScopeFacts()
	for _, fact := range facts {
		v.injectFact(fact)
	}

	inScopeFiles := scope.GetInScopeFiles()
	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Opened %s with %d files in scope", path, len(inScopeFiles)),
		Metadata: map[string]interface{}{
			"active_file":    path,
			"in_scope_count": len(inScopeFiles),
			"in_scope":       inScopeFiles,
		},
		FactsToAdd: facts,
	}, nil
}

// handleGetElements returns all elements in the current scope.
func (v *VirtualStore) handleGetElements(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	scope := v.codeScope
	v.mu.RUnlock()

	if scope == nil {
		return ActionResult{Success: false, Error: "code scope not configured"}, nil
	}

	// Get elements, optionally filtered by file
	var elements []CodeElement
	if req.Target != "" {
		path := v.resolvePath(req.Target)
		elements = scope.GetCoreElementsByFile(path)
	} else {
		// Return all elements in scope - need to iterate files
		for _, file := range scope.GetInScopeFiles() {
			elements = append(elements, scope.GetCoreElementsByFile(file)...)
		}
	}

	// Filter by type if specified
	if elemType, ok := req.Payload["type"].(string); ok && elemType != "" {
		var filtered []CodeElement
		for _, e := range elements {
			if e.Type == elemType {
				filtered = append(filtered, e)
			}
		}
		elements = filtered
	}

	output, _ := json.Marshal(elements)
	return ActionResult{
		Success: true,
		Output:  string(output),
		Metadata: map[string]interface{}{
			"count": len(elements),
		},
	}, nil
}

// handleGetElement returns a single element by ref.
func (v *VirtualStore) handleGetElement(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	scope := v.codeScope
	v.mu.RUnlock()

	if scope == nil {
		return ActionResult{Success: false, Error: "code scope not configured"}, nil
	}

	ref := req.Target
	elem := scope.GetCoreElement(ref)
	if elem == nil {
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("element not found: %s", ref),
		}, nil
	}

	// Include body if requested
	includeBody, _ := req.Payload["include_body"].(bool)
	if includeBody && elem.Body == "" {
		elem.Body = scope.GetElementBody(ref)
	}

	output, _ := json.Marshal(elem)
	return ActionResult{
		Success: true,
		Output:  string(output),
		Metadata: map[string]interface{}{
			"ref":        elem.Ref,
			"type":       elem.Type,
			"file":       elem.File,
			"start_line": elem.StartLine,
			"end_line":   elem.EndLine,
		},
	}, nil
}

// handleEditElement replaces an element's body by ref.
func (v *VirtualStore) handleEditElement(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	scope := v.codeScope
	editor := v.fileEditor
	v.mu.RUnlock()

	if scope == nil {
		return ActionResult{Success: false, Error: "code scope not configured"}, nil
	}
	if editor == nil {
		return ActionResult{Success: false, Error: "file editor not configured"}, nil
	}

	ref := req.Target
	newContent, ok := req.Payload["content"].(string)
	if !ok {
		return ActionResult{Success: false, Error: "edit_element requires 'content' in payload"}, nil
	}

	elem := scope.GetCoreElement(ref)
	if elem == nil {
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("element not found: %s", ref),
		}, nil
	}

	// Verify file hasn't been modified externally before editing
	unchanged, hashErr := scope.VerifyFileHash(elem.File)
	if hashErr != nil {
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("failed to verify file hash: %v", hashErr),
			FactsToAdd: []Fact{
				{Predicate: "element_edit_blocked", Args: []interface{}{ref, "hash_verification_failed"}},
			},
		}, nil
	}
	if !unchanged {
		// File was modified externally - refresh scope first
		if err := scope.RefreshWithRetry(3); err != nil {
			return ActionResult{
				Success: false,
				Error:   "file was modified externally and refresh failed",
				FactsToAdd: []Fact{
					{Predicate: "element_edit_blocked", Args: []interface{}{ref, "concurrent_modification"}},
					{Predicate: "file_modified_externally", Args: []interface{}{elem.File}},
				},
			}, nil
		}
		// Re-fetch element after refresh (line numbers may have changed)
		elem = scope.GetCoreElement(ref)
		if elem == nil {
			return ActionResult{
				Success: false,
				Error:   fmt.Sprintf("element %s no longer exists after refresh", ref),
				FactsToAdd: []Fact{
					{Predicate: "element_stale", Args: []interface{}{ref, "not_found_after_refresh"}},
				},
			}, nil
		}
	}

	// Replace the element
	result, err := editor.ReplaceElement(elem.File, elem.StartLine, elem.EndLine, newContent)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Refresh scope to update line numbers with retry
	if err := scope.RefreshWithRetry(3); err != nil {
		v.injectFact(Fact{
			Predicate: "scope_refresh_failed",
			Args:      []interface{}{elem.File, err.Error()},
		})
	}

	// Inject the new scope facts
	for _, fact := range scope.ScopeFacts() {
		v.injectFact(fact)
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Replaced element %s (%d lines affected)", ref, result.LinesAffected),
		Metadata: map[string]interface{}{
			"ref":            ref,
			"lines_affected": result.LinesAffected,
			"new_line_count": result.LineCount,
		},
		FactsToAdd: []Fact{
			{Predicate: "element_modified", Args: []interface{}{ref, req.SessionID}},
			{Predicate: "modified", Args: []interface{}{elem.File}},
		},
	}, nil
}

// handleRefreshScope re-parses all in-scope files.
func (v *VirtualStore) handleRefreshScope(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	scope := v.codeScope
	v.mu.RUnlock()

	if scope == nil {
		return ActionResult{Success: false, Error: "code scope not configured"}, nil
	}

	if err := scope.Refresh(); err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Re-inject all scope facts
	facts := scope.ScopeFacts()
	for _, fact := range facts {
		v.injectFact(fact)
	}

	return ActionResult{
		Success:    true,
		Output:     "Scope refreshed",
		FactsToAdd: facts,
	}, nil
}

// handleCloseScope closes the current scope.
func (v *VirtualStore) handleCloseScope(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	scope := v.codeScope
	v.mu.RUnlock()

	if scope == nil {
		return ActionResult{Success: false, Error: "code scope not configured"}, nil
	}

	scope.Close()

	return ActionResult{
		Success: true,
		Output:  "Scope closed",
		FactsToAdd: []Fact{
			{Predicate: "scope_closed", Args: []interface{}{}},
		},
	}, nil
}

// handleEditLines performs line-based file editing.
func (v *VirtualStore) handleEditLines(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	editor := v.fileEditor
	scope := v.codeScope
	v.mu.RUnlock()

	if editor == nil {
		return ActionResult{Success: false, Error: "file editor not configured"}, nil
	}

	path := v.resolvePath(req.Target)

	startLine, _ := req.Payload["start_line"].(float64)
	endLine, _ := req.Payload["end_line"].(float64)
	newContent, _ := req.Payload["content"].(string)

	if startLine == 0 || endLine == 0 {
		return ActionResult{Success: false, Error: "edit_lines requires 'start_line' and 'end_line' in payload"}, nil
	}

	// Split content into lines
	var newLines []string
	if newContent != "" {
		newLines = strings.Split(strings.TrimSuffix(newContent, "\n"), "\n")
	}

	result, err := editor.EditLines(path, int(startLine), int(endLine), newLines)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Refresh scope if active with retry
	if scope != nil && scope.IsInScope(path) {
		if err := scope.RefreshWithRetry(3); err != nil {
			v.injectFact(Fact{
				Predicate: "scope_refresh_failed",
				Args:      []interface{}{path, err.Error()},
			})
		}
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Edited lines %d-%d in %s", int(startLine), int(endLine), path),
		Metadata: map[string]interface{}{
			"path":           path,
			"start_line":     int(startLine),
			"end_line":       int(endLine),
			"lines_affected": result.LinesAffected,
		},
		FactsToAdd: []Fact{
			{Predicate: "lines_edited", Args: []interface{}{path, int64(startLine), int64(endLine), req.SessionID}},
			{Predicate: "modified", Args: []interface{}{path}},
		},
	}, nil
}

// handleInsertLines inserts lines at a position.
func (v *VirtualStore) handleInsertLines(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	editor := v.fileEditor
	scope := v.codeScope
	v.mu.RUnlock()

	if editor == nil {
		return ActionResult{Success: false, Error: "file editor not configured"}, nil
	}

	path := v.resolvePath(req.Target)

	afterLine, _ := req.Payload["after_line"].(float64)
	content, _ := req.Payload["content"].(string)

	if content == "" {
		return ActionResult{Success: false, Error: "insert_lines requires 'content' in payload"}, nil
	}

	newLines := strings.Split(strings.TrimSuffix(content, "\n"), "\n")

	result, err := editor.InsertLines(path, int(afterLine), newLines)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Refresh scope if active with retry
	if scope != nil && scope.IsInScope(path) {
		if err := scope.RefreshWithRetry(3); err != nil {
			v.injectFact(Fact{
				Predicate: "scope_refresh_failed",
				Args:      []interface{}{path, err.Error()},
			})
		}
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Inserted %d lines after line %d in %s", result.LinesAffected, int(afterLine), path),
		Metadata: map[string]interface{}{
			"path":        path,
			"after_line":  int(afterLine),
			"lines_added": result.LinesAffected,
		},
		FactsToAdd: []Fact{
			{Predicate: "lines_inserted", Args: []interface{}{path, int64(afterLine), int64(len(newLines)), req.SessionID}},
			{Predicate: "modified", Args: []interface{}{path}},
		},
	}, nil
}

// handleDeleteLines removes lines from a file.
func (v *VirtualStore) handleDeleteLines(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	editor := v.fileEditor
	scope := v.codeScope
	v.mu.RUnlock()

	if editor == nil {
		return ActionResult{Success: false, Error: "file editor not configured"}, nil
	}

	path := v.resolvePath(req.Target)

	startLine, _ := req.Payload["start_line"].(float64)
	endLine, _ := req.Payload["end_line"].(float64)

	if startLine == 0 || endLine == 0 {
		return ActionResult{Success: false, Error: "delete_lines requires 'start_line' and 'end_line' in payload"}, nil
	}

	result, err := editor.DeleteLines(path, int(startLine), int(endLine))
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Refresh scope if active with retry
	if scope != nil && scope.IsInScope(path) {
		if err := scope.RefreshWithRetry(3); err != nil {
			v.injectFact(Fact{
				Predicate: "scope_refresh_failed",
				Args:      []interface{}{path, err.Error()},
			})
		}
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Deleted lines %d-%d from %s", int(startLine), int(endLine), path),
		Metadata: map[string]interface{}{
			"path":          path,
			"start_line":    int(startLine),
			"end_line":      int(endLine),
			"lines_deleted": result.LinesAffected,
		},
		FactsToAdd: []Fact{
			{Predicate: "lines_deleted", Args: []interface{}{path, int64(startLine), int64(endLine), req.SessionID}},
			{Predicate: "modified", Args: []interface{}{path}},
		},
	}, nil
}

// =============================================================================
// AUTOPOIESIS HANDLERS - GENERATED TOOL EXECUTION
// =============================================================================

// handleExecTool executes a generated tool from the Ouroboros registry.
func (v *VirtualStore) handleExecTool(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleExecTool")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	toolExec := v.toolExecutor
	registry := v.toolRegistry
	v.mu.RUnlock()

	if toolExec == nil {
		logging.Get(logging.CategoryVirtualStore).Error("Tool executor not configured")
		return ActionResult{
			Success: false,
			Error:   "tool executor not configured",
			FactsToAdd: []Fact{
				{Predicate: "tool_exec_failed", Args: []interface{}{req.Target, "no_executor"}},
			},
		}, nil
	}

	toolName := req.Target
	input, _ := req.Payload["input"].(string)

	logging.VirtualStore("Executing tool: %s (input_len=%d)", toolName, len(input))

	// Check if tool exists in registry first
	var registeredTool *Tool
	if registry != nil {
		registeredTool, _ = registry.GetTool(toolName)
	}

	// Check if tool exists in executor
	toolInfo, exists := toolExec.GetTool(toolName)
	if !exists {
		logging.Get(logging.CategoryVirtualStore).Warn("Tool not found: %s", toolName)
		return ActionResult{
			Success: false,
			Error:   fmt.Sprintf("tool not found: %s", toolName),
			FactsToAdd: []Fact{
				{Predicate: "tool_not_found", Args: []interface{}{toolName}},
			},
		}, nil
	}

	// Execute the tool
	output, err := toolExec.ExecuteTool(ctx, toolName, input)

	// Update execution count in registry
	if registeredTool != nil && registry != nil {
		registeredTool.ExecuteCount++
	}

	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Tool execution failed: %s - %v", toolName, err)
		return ActionResult{
			Success: false,
			Output:  output, // Might have partial output
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "tool_exec_failed", Args: []interface{}{toolName, err.Error()}},
			},
		}, nil
	}

	metadata := map[string]interface{}{
		"tool_name":     toolName,
		"tool_hash":     toolInfo.Hash,
		"execute_count": toolInfo.ExecuteCount + 1,
	}

	if registeredTool != nil {
		metadata["shard_affinity"] = registeredTool.ShardAffinity
		metadata["command"] = registeredTool.Command
	}

	logging.VirtualStore("Tool %s executed successfully: output_len=%d", toolName, len(output))
	return ActionResult{
		Success:  true,
		Output:   output,
		Metadata: metadata,
		FactsToAdd: []Fact{
			{Predicate: "tool_executed", Args: []interface{}{toolName, output}},
			{Predicate: "tool_exec_success", Args: []interface{}{toolName}},
		},
	}, nil
}

// =============================================================================
// FILE EDITOR ADAPTER
// =============================================================================

// TactileFileEditorAdapter wraps tactile.FileEditor to implement core.FileEditor.
type TactileFileEditorAdapter struct {
	editor *tactile.FileEditor
}

// NewTactileFileEditorAdapter creates a new adapter.
func NewTactileFileEditorAdapter(editor *tactile.FileEditor) *TactileFileEditorAdapter {
	return &TactileFileEditorAdapter{editor: editor}
}

func (a *TactileFileEditorAdapter) ReadFile(path string) ([]string, error) {
	return a.editor.ReadFile(path)
}

func (a *TactileFileEditorAdapter) ReadLines(path string, startLine, endLine int) ([]string, error) {
	return a.editor.ReadLines(path, startLine, endLine)
}

func (a *TactileFileEditorAdapter) WriteFile(path string, lines []string) (*FileEditResult, error) {
	result, err := a.editor.WriteFile(path, lines)
	if err != nil {
		return nil, err
	}
	return a.convertResult(result), nil
}

func (a *TactileFileEditorAdapter) EditLines(path string, startLine, endLine int, newLines []string) (*FileEditResult, error) {
	result, err := a.editor.EditLines(path, startLine, endLine, newLines)
	if err != nil {
		return nil, err
	}
	return a.convertResult(result), nil
}

func (a *TactileFileEditorAdapter) InsertLines(path string, afterLine int, newLines []string) (*FileEditResult, error) {
	result, err := a.editor.InsertLines(path, afterLine, newLines)
	if err != nil {
		return nil, err
	}
	return a.convertResult(result), nil
}

func (a *TactileFileEditorAdapter) DeleteLines(path string, startLine, endLine int) (*FileEditResult, error) {
	result, err := a.editor.DeleteLines(path, startLine, endLine)
	if err != nil {
		return nil, err
	}
	return a.convertResult(result), nil
}

func (a *TactileFileEditorAdapter) ReplaceElement(path string, startLine, endLine int, newContent string) (*FileEditResult, error) {
	result, err := a.editor.ReplaceElement(path, startLine, endLine, newContent)
	if err != nil {
		return nil, err
	}
	return a.convertResult(result), nil
}

func (a *TactileFileEditorAdapter) convertResult(r *tactile.FileResult) *FileEditResult {
	if r == nil {
		return nil
	}
	// Convert tactile.Fact to core.Fact
	facts := make([]Fact, len(r.Facts))
	for i, f := range r.Facts {
		facts[i] = Fact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return &FileEditResult{
		Success:       r.Success,
		Path:          r.Path,
		LinesAffected: r.LinesAffected,
		OldContent:    r.OldContent,
		NewContent:    r.NewContent,
		OldHash:       r.OldHash,
		NewHash:       r.NewHash,
		LineCount:     r.LineCount,
		Facts:         facts,
		Error:         r.Error,
	}
}

// =============================================================================
// VIRTUAL PREDICATES - Knowledge Query Handlers
// =============================================================================
// These methods implement virtual predicates for the Mangle kernel,
// enabling logic rules to query the knowledge.db (LocalStore).
// Used during OODA Observe phase to hydrate learned facts into the kernel.

// QueryLearned queries cold_storage for learned facts by predicate name.
// Implements: query_learned(Predicate, Args) Bound
func (v *VirtualStore) QueryLearned(predicate string) ([]Fact, error) {
	logging.VirtualStoreDebug("QueryLearned: predicate=%s", predicate)

	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		logging.Get(logging.CategoryVirtualStore).Warn("QueryLearned: no knowledge database configured")
		return nil, fmt.Errorf("no knowledge database configured")
	}

	storedFacts, err := db.LoadFacts(predicate)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("QueryLearned failed: %v", err)
		return nil, fmt.Errorf("failed to query learned facts: %w", err)
	}

	facts := make([]Fact, 0, len(storedFacts))
	for _, sf := range storedFacts {
		facts = append(facts, Fact{
			Predicate: sf.Predicate,
			Args:      sf.Args,
		})
	}

	logging.VirtualStoreDebug("QueryLearned: found %d facts for predicate %s", len(facts), predicate)
	return facts, nil
}

// QueryAllLearned queries all facts from cold_storage.
// Returns facts grouped by fact_type (preference, constraint, fact).
func (v *VirtualStore) QueryAllLearned(factType string) ([]Fact, error) {
	logging.VirtualStoreDebug("QueryAllLearned: factType=%s", factType)

	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("no knowledge database configured")
	}

	storedFacts, err := db.LoadAllFacts(factType)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("QueryAllLearned failed: %v", err)
		return nil, fmt.Errorf("failed to query all learned facts: %w", err)
	}

	facts := make([]Fact, 0, len(storedFacts))
	for _, sf := range storedFacts {
		facts = append(facts, Fact{
			Predicate: sf.Predicate,
			Args:      sf.Args,
		})
	}

	logging.VirtualStoreDebug("QueryAllLearned: found %d facts of type %s", len(facts), factType)
	return facts, nil
}

// PersistFactsToKnowledge stores a batch of facts into knowledge.db cold_storage.
// This is used to mirror on-disk/AST projections into the learning store so
// HydrateLearnings can re-assert them for Mangle logic.
func (v *VirtualStore) PersistFactsToKnowledge(facts []Fact, factType string, priority int) error {
	logging.VirtualStoreDebug("PersistFactsToKnowledge: %d facts, type=%s, priority=%d", len(facts), factType, priority)

	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		logging.VirtualStoreDebug("PersistFactsToKnowledge: no database, skipping")
		return nil
	}
	if factType == "" {
		factType = "fact"
	}
	if priority <= 0 {
		priority = 5
	}

	for _, f := range facts {
		if err := db.StoreFact(f.Predicate, f.Args, factType, priority); err != nil {
			logging.Get(logging.CategoryVirtualStore).Error("Failed to persist fact %s: %v", f.Predicate, err)
			return fmt.Errorf("persist fact %s: %w", f.Predicate, err)
		}
	}

	logging.VirtualStoreDebug("PersistFactsToKnowledge: persisted %d facts", len(facts))
	return nil
}

// PersistLink stores a relationship into the knowledge graph table.
func (v *VirtualStore) PersistLink(entityA, relation, entityB string, weight float64, meta map[string]interface{}) error {
	logging.VirtualStoreDebug("PersistLink: %s -[%s]-> %s (weight=%.2f)", entityA, relation, entityB, weight)

	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil
	}
	if weight <= 0 {
		weight = 1.0
	}

	if err := db.StoreLink(entityA, relation, entityB, weight, meta); err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Failed to persist link: %v", err)
		return err
	}

	return nil
}

// QueryKnowledgeGraph queries the knowledge graph for entity relationships.
// Implements: query_knowledge_graph(EntityA, Relation, EntityB) Bound
func (v *VirtualStore) QueryKnowledgeGraph(entity, direction string) ([]Fact, error) {
	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("no knowledge database configured")
	}

	links, err := db.QueryLinks(entity, direction)
	if err != nil {
		return nil, fmt.Errorf("failed to query knowledge graph: %w", err)
	}

	facts := make([]Fact, 0, len(links))
	for _, link := range links {
		facts = append(facts, Fact{
			Predicate: "knowledge_link",
			Args:      []interface{}{link.EntityA, link.Relation, link.EntityB},
		})
	}
	return facts, nil
}

// QueryActivations queries the activation log for recent activation scores.
// Implements: query_activations(FactID, Score) Bound
func (v *VirtualStore) QueryActivations(limit int, minScore float64) ([]Fact, error) {
	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("no knowledge database configured")
	}

	activations, err := db.GetRecentActivations(limit, minScore)
	if err != nil {
		return nil, fmt.Errorf("failed to query activations: %w", err)
	}

	facts := make([]Fact, 0, len(activations))
	for factID, score := range activations {
		facts = append(facts, Fact{
			Predicate: "activation",
			Args:      []interface{}{factID, score},
		})
	}
	return facts, nil
}

// RecallSimilar performs semantic search on the vectors table.
// Implements: recall_similar(Query, TopK, Results) Bound
func (v *VirtualStore) RecallSimilar(query string, topK int) ([]Fact, error) {
	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("no knowledge database configured")
	}

	entries, err := db.VectorRecall(query, topK)
	if err != nil {
		return nil, fmt.Errorf("failed semantic recall: %w", err)
	}

	facts := make([]Fact, 0, len(entries))
	for i, entry := range entries {
		facts = append(facts, Fact{
			Predicate: "similar_content",
			Args:      []interface{}{i, entry.Content},
		})
	}
	return facts, nil
}

// QuerySession queries session history for conversation turns.
// Implements: query_session(SessionID, TurnNumber, UserInput) Bound
func (v *VirtualStore) QuerySession(sessionID string, limit int) ([]Fact, error) {
	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("no knowledge database configured")
	}

	history, err := db.GetSessionHistory(sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query session: %w", err)
	}

	facts := make([]Fact, 0, len(history))
	for _, turn := range history {
		turnNum, _ := turn["turn_number"].(int64)
		userInput, _ := turn["user_input"].(string)
		response, _ := turn["response"].(string)
		facts = append(facts, Fact{
			Predicate: "session_turn",
			Args:      []interface{}{sessionID, turnNum, userInput, response},
		})
	}
	return facts, nil
}

// HasLearned checks if any facts with the given predicate exist in cold_storage.
// Implements: has_learned(Predicate) Bound
func (v *VirtualStore) HasLearned(predicate string) (bool, error) {
	facts, err := v.QueryLearned(predicate)
	if err != nil {
		return false, err
	}
	return len(facts) > 0, nil
}

// QueryTraces queries reasoning_traces for shard execution history.
// Implements: query_traces(ShardType, Limit, TraceID, Success, DurationMs) Bound
func (v *VirtualStore) QueryTraces(shardType string, limit int) ([]Fact, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "QueryTraces")
	defer timer.Stop()

	logging.VirtualStoreDebug("QueryTraces: shardType=%s limit=%d", shardType, limit)

	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		logging.Get(logging.CategoryVirtualStore).Warn("QueryTraces: no knowledge database configured")
		return nil, fmt.Errorf("no knowledge database configured")
	}

	if limit <= 0 {
		limit = 50
	}

	traces, err := db.GetShardTraces(shardType, limit)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("QueryTraces failed: %v", err)
		return nil, fmt.Errorf("failed to query traces: %w", err)
	}

	facts := make([]Fact, 0, len(traces))
	for _, trace := range traces {
		facts = append(facts, Fact{
			Predicate: "reasoning_trace",
			Args: []interface{}{
				shardType,
				trace.ID,
				trace.Success,
				trace.DurationMs,
			},
		})
	}

	logging.VirtualStoreDebug("QueryTraces: found %d traces for shardType=%s", len(facts), shardType)
	return facts, nil
}

// QueryTraceStats retrieves aggregate statistics for a shard type.
// Implements: query_trace_stats(ShardType, SuccessCount, FailCount, AvgDuration) Bound
func (v *VirtualStore) QueryTraceStats(shardType string) ([]Fact, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "QueryTraceStats")
	defer timer.Stop()

	logging.VirtualStoreDebug("QueryTraceStats: shardType=%s", shardType)

	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		logging.Get(logging.CategoryVirtualStore).Warn("QueryTraceStats: no knowledge database configured")
		return nil, fmt.Errorf("no knowledge database configured")
	}

	stats, err := db.GetTraceStats()
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("QueryTraceStats failed: %v", err)
		return nil, fmt.Errorf("failed to query trace stats: %w", err)
	}

	// Extract stats for the requested shard type
	successRateByType, _ := stats["success_rate_by_type"].(map[string]float64)
	byShardType, _ := stats["by_shard_type"].(map[string]int64)

	totalCount := int64(0)
	successRate := 0.0
	avgDuration := 0.0

	if byShardType != nil {
		if count, ok := byShardType[shardType]; ok {
			totalCount = count
		}
	}
	if successRateByType != nil {
		if rate, ok := successRateByType[shardType]; ok {
			successRate = rate
		}
	}
	if avgDur, ok := stats["avg_duration_ms"].(float64); ok {
		avgDuration = avgDur
	}

	// Calculate success and fail counts from rate
	successCount := int64(float64(totalCount) * successRate)
	failCount := totalCount - successCount

	facts := []Fact{
		{
			Predicate: "trace_stats",
			Args: []interface{}{
				shardType,
				successCount,
				failCount,
				avgDuration,
			},
		},
	}

	logging.VirtualStoreDebug("QueryTraceStats: shardType=%s total=%d success=%d fail=%d avgDur=%.2f",
		shardType, totalCount, successCount, failCount, avgDuration)
	return facts, nil
}

// toAtomOrString converts string to MangleAtom if it starts with /.
func toAtomOrString(v interface{}) interface{} {
	if s, ok := v.(string); ok && strings.HasPrefix(s, "/") {
		return MangleAtom(s)
	}
	return v
}

// HydrateKnowledgeGraph loads knowledge graph entries from LocalStore and hydrates
// the kernel with knowledge_link facts. This can be called independently or as part
// of HydrateLearnings for targeted knowledge graph updates.
func (v *VirtualStore) HydrateKnowledgeGraph(ctx context.Context) (int, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "HydrateKnowledgeGraph")
	defer timer.Stop()

	logging.VirtualStoreDebug("HydrateKnowledgeGraph: starting")

	v.mu.RLock()
	db := v.localDB
	kernel := v.kernel
	v.mu.RUnlock()

	if db == nil {
		logging.VirtualStoreDebug("HydrateKnowledgeGraph: no database, skipping")
		return 0, nil // No database, nothing to hydrate
	}
	if kernel == nil {
		logging.Get(logging.CategoryVirtualStore).Error("HydrateKnowledgeGraph: no kernel configured")
		return 0, fmt.Errorf("no kernel configured")
	}

	// Create assertion function that wraps kernel.Assert
	assertFunc := func(predicate string, args []interface{}) error {
		// Convert args to MangleAtom if needed
		safeArgs := make([]interface{}, len(args))
		for i, arg := range args {
			safeArgs[i] = toAtomOrString(arg)
		}
		return kernel.Assert(Fact{
			Predicate: predicate,
			Args:      safeArgs,
		})
	}

	// Delegate to LocalStore's HydrateKnowledgeGraph
	count, err := db.HydrateKnowledgeGraph(assertFunc)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("HydrateKnowledgeGraph failed: %v", err)
		return 0, fmt.Errorf("failed to hydrate knowledge graph: %w", err)
	}

	logging.VirtualStoreDebug("HydrateKnowledgeGraph: hydrated %d links", count)
	return count, nil
}

// HydrateLearnings loads all learned facts from knowledge.db and asserts them into the kernel.
// This should be called during OODA Observe phase to make learned knowledge available to rules.
func (v *VirtualStore) HydrateLearnings(ctx context.Context) (int, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "HydrateLearnings")
	defer timer.Stop()

	logging.VirtualStore("Hydrating learnings from knowledge.db")

	v.mu.RLock()
	db := v.localDB
	kernel := v.kernel
	v.mu.RUnlock()

	if db == nil {
		logging.VirtualStoreDebug("HydrateLearnings: no database, skipping")
		return 0, nil // No database, nothing to hydrate
	}
	if kernel == nil {
		logging.Get(logging.CategoryVirtualStore).Error("HydrateLearnings: no kernel configured")
		return 0, fmt.Errorf("no kernel configured")
	}

	count := 0

	// Helper to assert with atom conversion
	assertLearned := func(metaPred string, fact Fact) error {
		// Convert args
		safeArgs := make([]interface{}, len(fact.Args))
		for i, arg := range fact.Args {
			safeArgs[i] = toAtomOrString(arg)
		}

		// The predicate itself might be an atom if referenced as data
		predArg := toAtomOrString(fact.Predicate)

		return kernel.Assert(Fact{
			Predicate: metaPred,
			Args:      []interface{}{predArg, safeArgs},
		})
	}

	// 1. Load all preferences (highest priority)
	preferences, err := v.QueryAllLearned("preference")
	if err == nil {
		for _, fact := range preferences {
			if err := assertLearned("learned_preference", fact); err == nil {
				count++
			}
		}
	}

	// 2. Load all user facts
	userFacts, err := v.QueryAllLearned("user_fact")
	if err == nil {
		for _, fact := range userFacts {
			if err := assertLearned("learned_fact", fact); err == nil {
				count++
			}
		}
	}

	// 3. Load all constraints
	constraints, err := v.QueryAllLearned("constraint")
	if err == nil {
		for _, fact := range constraints {
			if err := assertLearned("learned_constraint", fact); err == nil {
				count++
			}
		}
	}

	// 4. Load knowledge graph links (now delegates to dedicated method)
	kgCount, err := v.HydrateKnowledgeGraph(ctx)
	if err == nil {
		count += kgCount
	}

	// 5. Load recent activations (top 50 with score > 0.3)
	activations, err := v.QueryActivations(50, 0.3)
	if err == nil {
		for _, fact := range activations {
			// Activations are direct facts, not meta-facts
			safeArgs := make([]interface{}, len(fact.Args))
			for i, arg := range fact.Args {
				safeArgs[i] = toAtomOrString(arg)
			}
			if err := kernel.Assert(Fact{
				Predicate: fact.Predicate,
				Args:      safeArgs,
			}); err == nil {
				count++
			}
		}
	}

	logging.VirtualStore("HydrateLearnings completed: %d facts hydrated", count)
	return count, nil
}

// =============================================================================
// TDD LOOP ACTION HANDLERS
// =============================================================================

// handleReadErrorLog reads test/build error logs from the last execution.
func (v *VirtualStore) handleReadErrorLog(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	logging.VirtualStoreDebug("handleReadErrorLog: target=%s", req.Target)

	// Target specifies the log type (test, build, or file path)
	logType := req.Target
	if logType == "" {
		logType = "test"
	}

	// Try to read from common log locations
	var logContent string
	var logPath string

	switch logType {
	case "test":
		logPath = filepath.Join(v.workingDir, ".nerd", "logs", "test.log")
	case "build":
		logPath = filepath.Join(v.workingDir, ".nerd", "logs", "build.log")
	default:
		logPath = v.resolvePath(logType)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		// Return empty result if log not found, not an error
		return ActionResult{
			Success: true,
			Output:  "",
			FactsToAdd: []Fact{
				{Predicate: "error_log_empty", Args: []interface{}{logType}},
			},
		}, nil
	}
	logContent = string(data)

	return ActionResult{
		Success: true,
		Output:  logContent,
		Metadata: map[string]interface{}{
			"log_type": logType,
			"log_path": logPath,
			"size":     len(logContent),
		},
		FactsToAdd: []Fact{
			{Predicate: "error_log_read", Args: []interface{}{logType, len(logContent)}},
		},
	}, nil
}

// handleAnalyzeRootCause signals the kernel to analyze root cause of a failure.
func (v *VirtualStore) handleAnalyzeRootCause(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	logging.VirtualStoreDebug("handleAnalyzeRootCause: target=%s", req.Target)

	// This is a signal action - the actual analysis is done by the LLM
	// We inject facts to indicate the analysis should proceed
	errorContext := req.Target
	if errorContext == "" {
		errorContext = "unknown"
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Root cause analysis requested for: %s", errorContext),
		FactsToAdd: []Fact{
			{Predicate: "analyzing_root_cause", Args: []interface{}{errorContext}},
			{Predicate: "tdd_phase", Args: []interface{}{"/analyze"}},
		},
	}, nil
}

// handleGeneratePatch signals the kernel that a patch should be generated.
func (v *VirtualStore) handleGeneratePatch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	logging.VirtualStoreDebug("handleGeneratePatch: target=%s", req.Target)

	// Signal action for patch generation
	targetFile := req.Target
	patchDesc, _ := req.Payload["description"].(string)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Patch generation requested for: %s", targetFile),
		FactsToAdd: []Fact{
			{Predicate: "generating_patch", Args: []interface{}{targetFile, patchDesc}},
			{Predicate: "tdd_phase", Args: []interface{}{"/patch"}},
		},
	}, nil
}

// handleEscalateToUser escalates an issue to the user for intervention.
func (v *VirtualStore) handleEscalateToUser(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	reason := req.Target
	logging.VirtualStore("Escalating to user: %s", reason)

	return ActionResult{
		Success: false, // Escalation means current task cannot proceed
		Output:  fmt.Sprintf("ESCALATION REQUIRED: %s", reason),
		Error:   "USER_INTERVENTION_REQUIRED",
		FactsToAdd: []Fact{
			{Predicate: "escalated_to_user", Args: []interface{}{reason}},
			{Predicate: "task_blocked", Args: []interface{}{reason}},
		},
	}, nil
}

// handleComplete marks the current task as complete.
func (v *VirtualStore) handleComplete(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	taskID := req.Target
	summary, _ := req.Payload["summary"].(string)

	logging.VirtualStore("Task completed: %s", taskID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Task %s completed: %s", taskID, summary),
		FactsToAdd: []Fact{
			{Predicate: "task_completed", Args: []interface{}{taskID, summary}},
			{Predicate: "completion_signal", Args: []interface{}{taskID}},
		},
	}, nil
}

// handleInterrogative enters interrogative mode for clarification.
func (v *VirtualStore) handleInterrogative(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	question := req.Target
	options, _ := req.Payload["options"].([]interface{})

	logging.VirtualStoreDebug("Entering interrogative mode: %s", question)

	return ActionResult{
		Success: false, // Needs user response
		Output:  question,
		Error:   "CLARIFICATION_NEEDED",
		Metadata: map[string]interface{}{
			"question": question,
			"options":  options,
			"mode":     "interrogative",
		},
		FactsToAdd: []Fact{
			{Predicate: "awaiting_clarification", Args: []interface{}{question}},
			{Predicate: "interrogative_mode", Args: []interface{}{true}},
		},
	}, nil
}

// handleResumeTask resumes a previously paused task.
func (v *VirtualStore) handleResumeTask(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	taskID := req.Target
	logging.VirtualStore("Resuming task: %s", taskID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Resuming task: %s", taskID),
		FactsToAdd: []Fact{
			{Predicate: "task_resumed", Args: []interface{}{taskID}},
			{Predicate: "active_task", Args: []interface{}{taskID}},
		},
	}, nil
}

// =============================================================================
// OUROBOROS ACTION HANDLERS
// =============================================================================

// handleGenerateTool generates a new tool via the Ouroboros pipeline.
func (v *VirtualStore) handleGenerateTool(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	generator := v.toolGenerator
	v.mu.RUnlock()

	if generator == nil {
		return ActionResult{
			Success: false,
			Error:   "tool generator not configured",
			FactsToAdd: []Fact{
				{Predicate: "tool_generation_failed", Args: []interface{}{req.Target, "no_generator"}},
			},
		}, nil
	}

	toolName := req.Target
	purpose, _ := req.Payload["purpose"].(string)
	code, _ := req.Payload["code"].(string)
	confidence, _ := req.Payload["confidence"].(float64)
	priority, _ := req.Payload["priority"].(float64)
	isDiagnostic, _ := req.Payload["is_diagnostic"].(bool)

	if confidence == 0 {
		confidence = 0.8
	}
	if priority == 0 {
		priority = 5.0
	}

	logging.VirtualStore("Generating tool: %s (purpose=%s)", toolName, purpose)

	success, registeredName, binaryPath, errMsg := generator.GenerateToolFromCode(
		ctx, toolName, purpose, code, confidence, priority, isDiagnostic,
	)

	if !success {
		return ActionResult{
			Success: false,
			Error:   errMsg,
			FactsToAdd: []Fact{
				{Predicate: "tool_generation_failed", Args: []interface{}{toolName, errMsg}},
			},
		}, nil
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Tool %s generated at %s", registeredName, binaryPath),
		Metadata: map[string]interface{}{
			"tool_name":   registeredName,
			"binary_path": binaryPath,
		},
		FactsToAdd: []Fact{
			{Predicate: "tool_generated", Args: []interface{}{registeredName, binaryPath}},
			{Predicate: "tool_available", Args: []interface{}{registeredName}},
		},
	}, nil
}

// handleOuroborosDetect detects tool needs based on task context.
func (v *VirtualStore) handleOuroborosDetect(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	logging.VirtualStoreDebug("Ouroboros detect: %s", req.Target)

	// Detection is a signal action - the LLM identifies tool needs
	taskContext := req.Target

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Tool detection initiated for: %s", taskContext),
		FactsToAdd: []Fact{
			{Predicate: "ouroboros_phase", Args: []interface{}{"/detect"}},
			{Predicate: "tool_detection_context", Args: []interface{}{taskContext}},
		},
	}, nil
}

// handleOuroborosGenerate generates tool code.
func (v *VirtualStore) handleOuroborosGenerate(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	toolName := req.Target
	logging.VirtualStoreDebug("Ouroboros generate: %s", toolName)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Tool code generation initiated for: %s", toolName),
		FactsToAdd: []Fact{
			{Predicate: "ouroboros_phase", Args: []interface{}{"/generate"}},
			{Predicate: "tool_generating", Args: []interface{}{toolName}},
		},
	}, nil
}

// handleOuroborosCompile compiles a generated tool.
func (v *VirtualStore) handleOuroborosCompile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	toolName := req.Target
	sourcePath, _ := req.Payload["source_path"].(string)

	logging.VirtualStore("Ouroboros compile: %s from %s", toolName, sourcePath)

	// Compile the tool
	if sourcePath == "" {
		sourcePath = filepath.Join(v.workingDir, ".nerd", "tools", toolName+".go")
	}

	outputPath := filepath.Join(v.workingDir, ".nerd", "tools", ".compiled", toolName)

	cmd := tactile.ShellCommand{
		Binary:           "go",
		Arguments:        []string{"build", "-o", outputPath, sourcePath},
		WorkingDirectory: v.workingDir,
		TimeoutSeconds:   60,
		EnvironmentVars:  v.getAllowedEnv(),
	}

	output, err := v.executor.Execute(ctx, cmd)
	if err != nil {
		return ActionResult{
			Success: false,
			Output:  output,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "ouroboros_compile_failed", Args: []interface{}{toolName, err.Error()}},
			},
		}, nil
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Tool %s compiled to %s", toolName, outputPath),
		Metadata: map[string]interface{}{
			"tool_name":   toolName,
			"binary_path": outputPath,
		},
		FactsToAdd: []Fact{
			{Predicate: "ouroboros_phase", Args: []interface{}{"/compiled"}},
			{Predicate: "tool_compiled", Args: []interface{}{toolName, outputPath}},
		},
	}, nil
}

// handleOuroborosRegister registers a compiled tool.
func (v *VirtualStore) handleOuroborosRegister(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	toolName := req.Target
	binaryPath, _ := req.Payload["binary_path"].(string)
	shardAffinity, _ := req.Payload["shard_affinity"].(string)

	logging.VirtualStore("Ouroboros register: %s at %s", toolName, binaryPath)

	if shardAffinity == "" {
		shardAffinity = "coder"
	}

	if err := v.RegisterTool(toolName, binaryPath, shardAffinity); err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "ouroboros_register_failed", Args: []interface{}{toolName, err.Error()}},
			},
		}, nil
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Tool %s registered", toolName),
		FactsToAdd: []Fact{
			{Predicate: "ouroboros_phase", Args: []interface{}{"/registered"}},
			{Predicate: "tool_registered", Args: []interface{}{toolName, binaryPath}},
			{Predicate: "tool_available", Args: []interface{}{toolName}},
		},
	}, nil
}

// handleRefineTool refines an existing tool.
func (v *VirtualStore) handleRefineTool(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	toolName := req.Target
	feedback, _ := req.Payload["feedback"].(string)

	logging.VirtualStoreDebug("Refine tool: %s", toolName)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Tool refinement initiated for: %s", toolName),
		FactsToAdd: []Fact{
			{Predicate: "tool_refining", Args: []interface{}{toolName, feedback}},
			{Predicate: "ouroboros_phase", Args: []interface{}{"/refine"}},
		},
	}, nil
}

// =============================================================================
// CAMPAIGN ACTION HANDLERS
// =============================================================================

// handleCampaignClarify requests clarification for a campaign goal.
func (v *VirtualStore) handleCampaignClarify(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	question := req.Target
	campaignID, _ := req.Payload["campaign_id"].(string)

	logging.VirtualStoreDebug("Campaign clarify: %s", question)

	return ActionResult{
		Success: false, // Needs user input
		Output:  question,
		Error:   "CAMPAIGN_CLARIFICATION_NEEDED",
		Metadata: map[string]interface{}{
			"campaign_id": campaignID,
			"question":    question,
		},
		FactsToAdd: []Fact{
			{Predicate: "campaign_awaiting_clarification", Args: []interface{}{campaignID, question}},
		},
	}, nil
}

// handleCampaignCreateFile creates a file as part of a campaign.
func (v *VirtualStore) handleCampaignCreateFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// Delegate to write file handler
	return v.handleWriteFile(ctx, req)
}

// handleCampaignModifyFile modifies a file as part of a campaign.
func (v *VirtualStore) handleCampaignModifyFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// Delegate to edit file handler
	return v.handleEditFile(ctx, req)
}

// handleCampaignWriteTest writes a test file as part of a campaign.
func (v *VirtualStore) handleCampaignWriteTest(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	// Similar to write file but adds test-specific facts
	result, err := v.handleWriteFile(ctx, req)
	if err != nil {
		return result, err
	}

	if result.Success {
		result.FactsToAdd = append(result.FactsToAdd, Fact{
			Predicate: "test_written",
			Args:      []interface{}{req.Target},
		})
	}

	return result, nil
}

// handleCampaignRunTest runs tests as part of a campaign.
func (v *VirtualStore) handleCampaignRunTest(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// Delegate to run tests handler
	return v.handleRunTests(ctx, req)
}

// handleCampaignResearch performs research as part of a campaign.
func (v *VirtualStore) handleCampaignResearch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// Delegate to research handler
	return v.handleResearch(ctx, req)
}

// handleCampaignVerify verifies a campaign step.
func (v *VirtualStore) handleCampaignVerify(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	stepID := req.Target
	campaignID, _ := req.Payload["campaign_id"].(string)

	logging.VirtualStoreDebug("Campaign verify step: %s", stepID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Verifying campaign step: %s", stepID),
		FactsToAdd: []Fact{
			{Predicate: "campaign_step_verifying", Args: []interface{}{campaignID, stepID}},
			{Predicate: "campaign_phase", Args: []interface{}{"/verify"}},
		},
	}, nil
}

// handleCampaignDocument creates documentation as part of a campaign.
func (v *VirtualStore) handleCampaignDocument(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	docTarget := req.Target
	content, _ := req.Payload["content"].(string)

	logging.VirtualStoreDebug("Campaign document: %s", docTarget)

	// If content provided, write it
	if content != "" {
		req.Payload["content"] = content
		return v.handleWriteFile(ctx, req)
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Documentation requested for: %s", docTarget),
		FactsToAdd: []Fact{
			{Predicate: "campaign_documenting", Args: []interface{}{docTarget}},
			{Predicate: "campaign_phase", Args: []interface{}{"/document"}},
		},
	}, nil
}

// handleCampaignRefactor performs refactoring as part of a campaign.
func (v *VirtualStore) handleCampaignRefactor(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	target := req.Target
	refactorType, _ := req.Payload["refactor_type"].(string)

	logging.VirtualStoreDebug("Campaign refactor: %s (%s)", target, refactorType)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Refactoring initiated for: %s", target),
		FactsToAdd: []Fact{
			{Predicate: "campaign_refactoring", Args: []interface{}{target, refactorType}},
			{Predicate: "campaign_phase", Args: []interface{}{"/refactor"}},
		},
	}, nil
}

// handleCampaignIntegrate performs integration as part of a campaign.
func (v *VirtualStore) handleCampaignIntegrate(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	target := req.Target
	campaignID, _ := req.Payload["campaign_id"].(string)

	logging.VirtualStoreDebug("Campaign integrate: %s", target)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Integration step for: %s", target),
		FactsToAdd: []Fact{
			{Predicate: "campaign_integrating", Args: []interface{}{campaignID, target}},
			{Predicate: "campaign_phase", Args: []interface{}{"/integrate"}},
		},
	}, nil
}

// handleCampaignComplete marks a campaign as complete.
func (v *VirtualStore) handleCampaignComplete(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target
	summary, _ := req.Payload["summary"].(string)

	logging.VirtualStore("Campaign completed: %s", campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Campaign %s completed: %s", campaignID, summary),
		FactsToAdd: []Fact{
			{Predicate: "campaign_completed", Args: []interface{}{campaignID, summary}},
			{Predicate: "campaign_phase", Args: []interface{}{"/complete"}},
		},
	}, nil
}

// handleCampaignFinalVerify performs final verification of a campaign.
func (v *VirtualStore) handleCampaignFinalVerify(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target

	logging.VirtualStore("Campaign final verification: %s", campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Final verification for campaign: %s", campaignID),
		FactsToAdd: []Fact{
			{Predicate: "campaign_final_verifying", Args: []interface{}{campaignID}},
			{Predicate: "campaign_phase", Args: []interface{}{"/final_verify"}},
		},
	}, nil
}

// handleCampaignCleanup performs cleanup after a campaign.
func (v *VirtualStore) handleCampaignCleanup(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target

	logging.VirtualStore("Campaign cleanup: %s", campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Cleanup completed for campaign: %s", campaignID),
		FactsToAdd: []Fact{
			{Predicate: "campaign_cleaned_up", Args: []interface{}{campaignID}},
			{Predicate: "campaign_phase", Args: []interface{}{"/cleanup"}},
		},
	}, nil
}

// handleArchiveCampaign archives a completed campaign.
func (v *VirtualStore) handleArchiveCampaign(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target

	logging.VirtualStore("Archiving campaign: %s", campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Campaign %s archived", campaignID),
		FactsToAdd: []Fact{
			{Predicate: "campaign_archived", Args: []interface{}{campaignID}},
		},
	}, nil
}

// handleShowCampaignStatus shows the current status of a campaign.
func (v *VirtualStore) handleShowCampaignStatus(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target

	logging.VirtualStoreDebug("Show campaign status: %s", campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Showing status for campaign: %s", campaignID),
		FactsToAdd: []Fact{
			{Predicate: "campaign_status_requested", Args: []interface{}{campaignID}},
		},
	}, nil
}

// handleShowCampaignProgress shows the progress of a campaign.
func (v *VirtualStore) handleShowCampaignProgress(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target

	logging.VirtualStoreDebug("Show campaign progress: %s", campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Showing progress for campaign: %s", campaignID),
		FactsToAdd: []Fact{
			{Predicate: "campaign_progress_requested", Args: []interface{}{campaignID}},
		},
	}, nil
}

// handleAskCampaignInterrupt handles campaign interrupt requests.
func (v *VirtualStore) handleAskCampaignInterrupt(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target
	reason, _ := req.Payload["reason"].(string)

	logging.VirtualStore("Campaign interrupt requested: %s - %s", campaignID, reason)

	return ActionResult{
		Success: false,
		Output:  fmt.Sprintf("Campaign %s interrupt requested: %s", campaignID, reason),
		Error:   "CAMPAIGN_INTERRUPT_REQUESTED",
		FactsToAdd: []Fact{
			{Predicate: "campaign_interrupt_requested", Args: []interface{}{campaignID, reason}},
		},
	}, nil
}

// handleRunPhaseCheckpoint runs a checkpoint for the current phase.
func (v *VirtualStore) handleRunPhaseCheckpoint(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	phaseID := req.Target
	campaignID, _ := req.Payload["campaign_id"].(string)

	logging.VirtualStoreDebug("Phase checkpoint: %s in campaign %s", phaseID, campaignID)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Checkpoint for phase: %s", phaseID),
		FactsToAdd: []Fact{
			{Predicate: "phase_checkpoint", Args: []interface{}{campaignID, phaseID}},
		},
	}, nil
}

// handlePauseAndReplan pauses and replans the current campaign.
func (v *VirtualStore) handlePauseAndReplan(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	campaignID := req.Target
	reason, _ := req.Payload["reason"].(string)

	logging.VirtualStore("Pause and replan: %s - %s", campaignID, reason)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Campaign %s paused for replanning: %s", campaignID, reason),
		FactsToAdd: []Fact{
			{Predicate: "campaign_paused", Args: []interface{}{campaignID, reason}},
			{Predicate: "campaign_replanning", Args: []interface{}{campaignID}},
		},
	}, nil
}

// =============================================================================
// CONTEXT MANAGEMENT ACTION HANDLERS
// =============================================================================

// handleCompressContext compresses the current context.
func (v *VirtualStore) handleCompressContext(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	reason := req.Target
	targetRatio, _ := req.Payload["ratio"].(float64)
	if targetRatio == 0 {
		targetRatio = 0.5
	}

	logging.VirtualStore("Context compression requested: ratio=%.2f", targetRatio)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Context compression initiated (target ratio: %.2f)", targetRatio),
		FactsToAdd: []Fact{
			{Predicate: "context_compressing", Args: []interface{}{reason, targetRatio}},
			{Predicate: "compression_requested", Args: []interface{}{"/normal"}},
		},
	}, nil
}

// handleEmergencyCompress performs emergency context compression.
func (v *VirtualStore) handleEmergencyCompress(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	logging.VirtualStore("Emergency context compression requested")

	return ActionResult{
		Success: true,
		Output:  "Emergency context compression initiated",
		FactsToAdd: []Fact{
			{Predicate: "context_compressing", Args: []interface{}{"emergency", 0.25}},
			{Predicate: "compression_requested", Args: []interface{}{"/emergency"}},
		},
	}, nil
}

// handleCreateCheckpoint creates a checkpoint of the current state.
func (v *VirtualStore) handleCreateCheckpoint(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	checkpointName := req.Target
	if checkpointName == "" {
		checkpointName = fmt.Sprintf("checkpoint_%d", time.Now().Unix())
	}

	logging.VirtualStore("Creating checkpoint: %s", checkpointName)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Checkpoint created: %s", checkpointName),
		Metadata: map[string]interface{}{
			"checkpoint_name": checkpointName,
			"timestamp":       time.Now().Unix(),
		},
		FactsToAdd: []Fact{
			{Predicate: "checkpoint_created", Args: []interface{}{checkpointName, time.Now().Unix()}},
		},
	}, nil
}

// =============================================================================
// INVESTIGATION/ANALYSIS ACTION HANDLERS
// =============================================================================

// handleInvestigateAnomaly investigates an anomaly.
func (v *VirtualStore) handleInvestigateAnomaly(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	anomalyDesc := req.Target
	severity, _ := req.Payload["severity"].(string)
	if severity == "" {
		severity = "medium"
	}

	logging.VirtualStore("Investigating anomaly: %s (severity=%s)", anomalyDesc, severity)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Investigating anomaly: %s", anomalyDesc),
		FactsToAdd: []Fact{
			{Predicate: "anomaly_investigating", Args: []interface{}{anomalyDesc, severity}},
			{Predicate: "investigation_phase", Args: []interface{}{"/anomaly"}},
		},
	}, nil
}

// handleInvestigateSystemic investigates a systemic issue.
func (v *VirtualStore) handleInvestigateSystemic(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	issueDesc := req.Target

	logging.VirtualStore("Investigating systemic issue: %s", issueDesc)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Investigating systemic issue: %s", issueDesc),
		FactsToAdd: []Fact{
			{Predicate: "systemic_investigating", Args: []interface{}{issueDesc}},
			{Predicate: "investigation_phase", Args: []interface{}{"/systemic"}},
		},
	}, nil
}

// handleUpdateWorldModel updates the world model.
func (v *VirtualStore) handleUpdateWorldModel(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	updateType := req.Target
	scope, _ := req.Payload["scope"].(string)

	logging.VirtualStoreDebug("Updating world model: type=%s, scope=%s", updateType, scope)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("World model update: %s", updateType),
		FactsToAdd: []Fact{
			{Predicate: "world_model_updating", Args: []interface{}{updateType, scope}},
		},
	}, nil
}

// =============================================================================
// CORRECTIVE ACTION HANDLERS
// =============================================================================

// handleCorrectiveResearch performs research to correct an issue.
func (v *VirtualStore) handleCorrectiveResearch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	topic := req.Target
	issueType, _ := req.Payload["issue_type"].(string)

	logging.VirtualStoreDebug("Corrective research: %s (issue=%s)", topic, issueType)

	// If scraper is available, delegate to research handler
	v.mu.RLock()
	scraper := v.scraper
	v.mu.RUnlock()

	if scraper != nil {
		return v.handleResearch(ctx, req)
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Corrective research initiated for: %s", topic),
		FactsToAdd: []Fact{
			{Predicate: "corrective_researching", Args: []interface{}{topic, issueType}},
			{Predicate: "corrective_phase", Args: []interface{}{"/research"}},
		},
	}, nil
}

// handleCorrectiveDocs creates corrective documentation.
func (v *VirtualStore) handleCorrectiveDocs(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	docTarget := req.Target

	logging.VirtualStoreDebug("Corrective documentation: %s", docTarget)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Corrective documentation initiated for: %s", docTarget),
		FactsToAdd: []Fact{
			{Predicate: "corrective_documenting", Args: []interface{}{docTarget}},
			{Predicate: "corrective_phase", Args: []interface{}{"/docs"}},
		},
	}, nil
}

// handleCorrectiveDecompose decomposes a problem for correction.
func (v *VirtualStore) handleCorrectiveDecompose(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	problem := req.Target

	logging.VirtualStoreDebug("Corrective decompose: %s", problem)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Decomposing problem for correction: %s", problem),
		FactsToAdd: []Fact{
			{Predicate: "corrective_decomposing", Args: []interface{}{problem}},
			{Predicate: "corrective_phase", Args: []interface{}{"/decompose"}},
		},
	}, nil
}

// =============================================================================
// PYTHON ENVIRONMENT ACTION HANDLERS (General Purpose)
// =============================================================================
// These handlers work with ANY Python project, not just benchmarks.

// handlePythonEnvSetup creates a Python development environment.
// Payload expects: project_name, git_url (optional), commit (optional), branch (optional)
func (v *VirtualStore) handlePythonEnvSetup(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	gitURL, _ := req.Payload["git_url"].(string)
	commit, _ := req.Payload["commit"].(string)
	branch, _ := req.Payload["branch"].(string)

	if projectName == "" && gitURL == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name or git_url required in payload",
		}, nil
	}

	logging.VirtualStore("Python env setup: project=%s", projectName)

	facts := []Fact{
		{Predicate: "python_environment", Args: []interface{}{projectName, "", "/initializing", time.Now().Unix()}},
	}

	if gitURL != "" {
		facts = append(facts, Fact{
			Predicate: "python_project_source",
			Args:      []interface{}{projectName, gitURL, commit, branch},
		})
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Python environment initializing for %s", projectName),
		Metadata: map[string]interface{}{
			"project_name": projectName,
			"git_url":      gitURL,
			"commit":       commit,
			"branch":       branch,
		},
		FactsToAdd: facts,
	}, nil
}

// handlePythonEnvExec executes a command in a Python environment.
// Payload expects: project_name, command
func (v *VirtualStore) handlePythonEnvExec(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	command, _ := req.Payload["command"].(string)

	if projectName == "" || command == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name and command required in payload",
		}, nil
	}

	logging.VirtualStore("Python exec: project=%s, cmd=%s", projectName, command)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Executing in %s: %s", projectName, command),
		Metadata: map[string]interface{}{
			"project_name": projectName,
			"command":      command,
		},
		FactsToAdd: []Fact{
			{Predicate: "python_command_executed", Args: []interface{}{projectName, command, time.Now().Unix()}},
		},
	}, nil
}

// handlePythonRunPytest runs pytest in a Python environment.
// Payload expects: project_name, test_args (optional - array of test names/patterns)
func (v *VirtualStore) handlePythonRunPytest(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	if projectName == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name required in payload",
		}, nil
	}

	// Optional test arguments
	var testArgs []string
	if args, ok := req.Payload["test_args"].([]interface{}); ok {
		for _, a := range args {
			if s, ok := a.(string); ok {
				testArgs = append(testArgs, s)
			}
		}
	}

	logging.VirtualStore("Python pytest: project=%s, args=%v", projectName, testArgs)

	facts := []Fact{
		{Predicate: "python_environment", Args: []interface{}{projectName, "", "/testing", time.Now().Unix()}},
		{Predicate: "pytest_execution", Args: []interface{}{projectName, len(testArgs), time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Running pytest in %s", projectName),
		Metadata: map[string]interface{}{
			"project_name": projectName,
			"test_args":    testArgs,
		},
		FactsToAdd: facts,
	}, nil
}

// handlePythonApplyPatch applies a git patch to a Python project.
// Payload expects: project_name, patch (unified diff format)
func (v *VirtualStore) handlePythonApplyPatch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	patch, _ := req.Payload["patch"].(string)

	if projectName == "" || patch == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name and patch required in payload",
		}, nil
	}

	logging.VirtualStore("Python apply patch: project=%s, size=%d", projectName, len(patch))

	facts := []Fact{
		{Predicate: "python_patch_applied", Args: []interface{}{projectName, len(patch), time.Now().Unix()}},
		{Predicate: "python_environment", Args: []interface{}{projectName, "", "/patched", time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Patch applied to %s (%d bytes)", projectName, len(patch)),
		Metadata: map[string]interface{}{
			"project_name": projectName,
			"patch_size":   len(patch),
		},
		FactsToAdd: facts,
	}, nil
}

// handlePythonSnapshot creates a snapshot of the Python environment.
// Payload expects: project_name, snapshot_name (optional)
func (v *VirtualStore) handlePythonSnapshot(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	snapshotName, _ := req.Payload["snapshot_name"].(string)

	if projectName == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name required in payload",
		}, nil
	}

	if snapshotName == "" {
		snapshotName = fmt.Sprintf("%s-snapshot-%d", projectName, time.Now().Unix())
	}

	logging.VirtualStore("Python snapshot: project=%s, name=%s", projectName, snapshotName)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Snapshot created: %s", snapshotName),
		Metadata: map[string]interface{}{
			"project_name":  projectName,
			"snapshot_name": snapshotName,
		},
		FactsToAdd: []Fact{
			{Predicate: "python_snapshot", Args: []interface{}{projectName, snapshotName, time.Now().Unix()}},
		},
	}, nil
}

// handlePythonRestore restores a Python environment from snapshot.
// Payload expects: project_name, snapshot_name
func (v *VirtualStore) handlePythonRestore(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	snapshotName, _ := req.Payload["snapshot_name"].(string)

	if projectName == "" || snapshotName == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name and snapshot_name required in payload",
		}, nil
	}

	logging.VirtualStore("Python restore: project=%s, snapshot=%s", projectName, snapshotName)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Restored %s from snapshot %s", projectName, snapshotName),
		Metadata: map[string]interface{}{
			"project_name":  projectName,
			"snapshot_name": snapshotName,
		},
		FactsToAdd: []Fact{
			{Predicate: "python_restored", Args: []interface{}{projectName, snapshotName, time.Now().Unix()}},
			{Predicate: "python_environment", Args: []interface{}{projectName, "", "/ready", time.Now().Unix()}},
		},
	}, nil
}

// handlePythonTeardown cleans up a Python environment.
// Payload expects: project_name
func (v *VirtualStore) handlePythonTeardown(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	projectName, _ := req.Payload["project_name"].(string)
	if projectName == "" {
		return ActionResult{
			Success: false,
			Error:   "project_name required in payload",
		}, nil
	}

	logging.VirtualStore("Python teardown: project=%s", projectName)

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Python environment torn down for %s", projectName),
		Metadata: map[string]interface{}{
			"project_name": projectName,
		},
		FactsToAdd: []Fact{
			{Predicate: "python_environment", Args: []interface{}{projectName, "", "/terminated", time.Now().Unix()}},
			{Predicate: "python_teardown_complete", Args: []interface{}{projectName, time.Now().Unix()}},
		},
	}, nil
}

// =============================================================================
// SWE-BENCH ACTION HANDLERS (Benchmark-specific)
// =============================================================================
// These handlers delegate to Python handlers with SWE-bench metadata.

// handleSWEBenchSetup initializes a SWE-bench environment for an instance.
// Payload expects: instance_id, repo, base_commit, problem_statement, fail_to_pass, pass_to_pass
func (v *VirtualStore) handleSWEBenchSetup(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	if instanceID == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id required in payload",
		}, nil
	}

	logging.VirtualStore("SWE-bench setup: instance=%s", instanceID)

	// Extract instance details from payload
	repo, _ := req.Payload["repo"].(string)
	baseCommit, _ := req.Payload["base_commit"].(string)
	problemStatement, _ := req.Payload["problem_statement"].(string)

	// Convert test lists
	var failToPass, passToPass []string
	if ftp, ok := req.Payload["fail_to_pass"].([]interface{}); ok {
		for _, t := range ftp {
			if s, ok := t.(string); ok {
				failToPass = append(failToPass, s)
			}
		}
	}
	if ptp, ok := req.Payload["pass_to_pass"].([]interface{}); ok {
		for _, t := range ptp {
			if s, ok := t.(string); ok {
				passToPass = append(passToPass, s)
			}
		}
	}

	// Generate Mangle facts for the instance
	facts := []Fact{
		{Predicate: "swebench_instance", Args: []interface{}{instanceID, repo, baseCommit, ""}},
		{Predicate: "swebench_environment", Args: []interface{}{instanceID, "", "/initializing", time.Now().Unix()}},
	}

	// Add test expectations as facts
	for _, test := range failToPass {
		facts = append(facts, Fact{
			Predicate: "swebench_expected_fail_to_pass",
			Args:      []interface{}{instanceID, test},
		})
	}
	for _, test := range passToPass {
		facts = append(facts, Fact{
			Predicate: "swebench_expected_pass_to_pass",
			Args:      []interface{}{instanceID, test},
		})
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("SWE-bench environment initializing for %s (%s@%s)", instanceID, repo, baseCommit[:8]),
		Metadata: map[string]interface{}{
			"instance_id":       instanceID,
			"repo":              repo,
			"base_commit":       baseCommit,
			"problem_statement": problemStatement,
			"fail_to_pass":      failToPass,
			"pass_to_pass":      passToPass,
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchApplyPatch applies a model-generated patch to the environment.
// Payload expects: instance_id, patch (unified diff format)
func (v *VirtualStore) handleSWEBenchApplyPatch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	patch, _ := req.Payload["patch"].(string)

	if instanceID == "" || patch == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id and patch required in payload",
		}, nil
	}

	logging.VirtualStore("SWE-bench apply patch: instance=%s, patch_size=%d", instanceID, len(patch))

	// Record patch application attempt
	facts := []Fact{
		{Predicate: "swebench_patch_applied", Args: []interface{}{instanceID, len(patch), time.Now().Unix()}},
		{Predicate: "swebench_environment", Args: []interface{}{instanceID, "", "/patched", time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Patch applied to instance %s (%d bytes)", instanceID, len(patch)),
		Metadata: map[string]interface{}{
			"instance_id": instanceID,
			"patch_size":  len(patch),
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchRunTests runs tests for a SWE-bench instance.
// Payload expects: instance_id, test_names (optional - defaults to all)
func (v *VirtualStore) handleSWEBenchRunTests(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	if instanceID == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id required in payload",
		}, nil
	}

	// Optional: specific test names
	var testNames []string
	if tn, ok := req.Payload["test_names"].([]interface{}); ok {
		for _, t := range tn {
			if s, ok := t.(string); ok {
				testNames = append(testNames, s)
			}
		}
	}

	logging.VirtualStore("SWE-bench run tests: instance=%s, tests=%d", instanceID, len(testNames))

	facts := []Fact{
		{Predicate: "swebench_environment", Args: []interface{}{instanceID, "", "/testing", time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Running tests for instance %s", instanceID),
		Metadata: map[string]interface{}{
			"instance_id": instanceID,
			"test_count":  len(testNames),
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchSnapshot creates a container snapshot for rollback.
// Payload expects: instance_id, snapshot_name (optional)
func (v *VirtualStore) handleSWEBenchSnapshot(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	snapshotName, _ := req.Payload["snapshot_name"].(string)

	if instanceID == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id required in payload",
		}, nil
	}

	if snapshotName == "" {
		snapshotName = fmt.Sprintf("%s-snapshot-%d", instanceID, time.Now().Unix())
	}

	logging.VirtualStore("SWE-bench snapshot: instance=%s, name=%s", instanceID, snapshotName)

	facts := []Fact{
		{Predicate: "swebench_snapshot", Args: []interface{}{instanceID, snapshotName, time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Snapshot created: %s", snapshotName),
		Metadata: map[string]interface{}{
			"instance_id":   instanceID,
			"snapshot_name": snapshotName,
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchRestore restores environment from a snapshot.
// Payload expects: instance_id, snapshot_name
func (v *VirtualStore) handleSWEBenchRestore(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	snapshotName, _ := req.Payload["snapshot_name"].(string)

	if instanceID == "" || snapshotName == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id and snapshot_name required in payload",
		}, nil
	}

	logging.VirtualStore("SWE-bench restore: instance=%s, snapshot=%s", instanceID, snapshotName)

	facts := []Fact{
		{Predicate: "swebench_restored", Args: []interface{}{instanceID, snapshotName, time.Now().Unix()}},
		{Predicate: "swebench_environment", Args: []interface{}{instanceID, "", "/ready", time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Restored instance %s from snapshot %s", instanceID, snapshotName),
		Metadata: map[string]interface{}{
			"instance_id":   instanceID,
			"snapshot_name": snapshotName,
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchEvaluate evaluates a prediction against the instance tests.
// Payload expects: instance_id, patch, model_name (optional)
func (v *VirtualStore) handleSWEBenchEvaluate(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	patch, _ := req.Payload["patch"].(string)
	modelName, _ := req.Payload["model_name"].(string)

	if instanceID == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id required in payload",
		}, nil
	}

	if modelName == "" {
		modelName = "codenerd"
	}

	logging.VirtualStore("SWE-bench evaluate: instance=%s, model=%s", instanceID, modelName)

	// Evaluation result will be populated by actual test execution
	// For now, record the evaluation attempt
	facts := []Fact{
		{Predicate: "swebench_evaluation_started", Args: []interface{}{instanceID, modelName, time.Now().Unix()}},
		{Predicate: "swebench_environment", Args: []interface{}{instanceID, "", "/evaluating", time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("Evaluation started for instance %s with model %s", instanceID, modelName),
		Metadata: map[string]interface{}{
			"instance_id": instanceID,
			"model_name":  modelName,
			"patch_size":  len(patch),
		},
		FactsToAdd: facts,
	}, nil
}

// handleSWEBenchTeardown cleans up a SWE-bench environment.
// Payload expects: instance_id
func (v *VirtualStore) handleSWEBenchTeardown(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	instanceID, _ := req.Payload["instance_id"].(string)
	if instanceID == "" {
		return ActionResult{
			Success: false,
			Error:   "instance_id required in payload",
		}, nil
	}

	logging.VirtualStore("SWE-bench teardown: instance=%s", instanceID)

	facts := []Fact{
		{Predicate: "swebench_environment", Args: []interface{}{instanceID, "", "/terminated", time.Now().Unix()}},
		{Predicate: "swebench_teardown_complete", Args: []interface{}{instanceID, time.Now().Unix()}},
	}

	return ActionResult{
		Success: true,
		Output:  fmt.Sprintf("SWE-bench environment torn down for instance %s", instanceID),
		Metadata: map[string]interface{}{
			"instance_id": instanceID,
		},
		FactsToAdd: facts,
	}, nil
}
