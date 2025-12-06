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

	"codenerd/internal/store"
	"codenerd/internal/tactile"
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

// ToolInfo contains information about a registered tool
type ToolInfo struct {
	Name         string    `json:"name"`
	Description  string    `json:"description"`
	BinaryPath   string    `json:"binary_path"`
	Hash         string    `json:"hash"`
	RegisteredAt time.Time `json:"registered_at"`
	ExecuteCount int64     `json:"execute_count"`
}

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
	kernel Kernel

	// Constitutional logic (safety layer)
	constitution []ConstitutionalRule

	// Working directory
	workingDir string

	// Allowed environment variables
	allowedEnvVars []string

	// Use modern executor for command execution
	useModernExecutor bool

	// Code DOM - semantic code element operations
	codeScope  CodeScope
	fileEditor FileEditor

	// Autopoiesis - tool execution
	toolExecutor ToolExecutor

	// Tool registry - integration with kernel and shards
	toolRegistry *ToolRegistry

	// Knowledge persistence - LocalStore for knowledge.db queries
	// Enables virtual predicates to query learned facts, session history, etc.
	localDB *store.LocalStore
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
	vs := &VirtualStore{
		executor:       executor,
		workingDir:     config.WorkingDir,
		allowedEnvVars: config.AllowedEnvVars,
		shardManager:   NewShardManager(),
		toolRegistry:   NewToolRegistry(config.WorkingDir),
	}

	// Initialize modern executor with audit logging
	vs.initModernExecutor()

	// Initialize constitutional rules (safety layer)
	vs.initConstitution()

	return vs
}

// initModernExecutor sets up the modern tactile executor with audit logging.
// This enables automatic fact generation for all command executions.
func (v *VirtualStore) initModernExecutor() {
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
}

// injectTactileFact converts a tactile.Fact to core.Fact and injects to kernel.
func (v *VirtualStore) injectTactileFact(tf tactile.Fact) {
	v.mu.RLock()
	kernel := v.kernel
	v.mu.RUnlock()

	if kernel == nil {
		return
	}

	// Convert tactile.Fact to core.Fact
	coreFact := Fact{
		Predicate: tf.Predicate,
		Args:      tf.Args,
	}

	_ = kernel.Assert(coreFact)
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

	// Also set kernel on tool registry
	if v.toolRegistry != nil {
		v.toolRegistry.SetKernel(k)
	}
}

// SetShardManager sets the shard manager for delegation.
func (v *VirtualStore) SetShardManager(sm *ShardManager) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.shardManager = sm
}

// SetCodeGraphClient sets the code graph integration client.
func (v *VirtualStore) SetCodeGraphClient(client IntegrationClient) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.codeGraph = client
}

// SetBrowserClient sets the browser integration client.
func (v *VirtualStore) SetBrowserClient(client IntegrationClient) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.browser = client
}

// SetScraperClient sets the scraper integration client.
func (v *VirtualStore) SetScraperClient(client IntegrationClient) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.scraper = client
}

// SetCodeScope sets the Code DOM scope manager.
func (v *VirtualStore) SetCodeScope(scope CodeScope) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.codeScope = scope
}

// SetFileEditor sets the file editor for line-based operations.
func (v *VirtualStore) SetFileEditor(editor FileEditor) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.fileEditor = editor
}

// SetToolExecutor sets the tool executor for generated tool execution.
func (v *VirtualStore) SetToolExecutor(executor ToolExecutor) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.toolExecutor = executor

	// Sync tools from executor to registry
	if v.toolRegistry != nil && executor != nil {
		_ = v.toolRegistry.SyncFromOuroboros(executor)
	}
}

// GetToolExecutor returns the current tool executor.
func (v *VirtualStore) GetToolExecutor() ToolExecutor {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.toolExecutor
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
		return fmt.Errorf("tool registry not initialized")
	}

	return registry.RegisterTool(name, command, shardAffinity)
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

// SetLocalDB sets the knowledge database for virtual predicate queries.
func (v *VirtualStore) SetLocalDB(db *store.LocalStore) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.localDB = db
}

// GetLocalDB returns the current knowledge database.
func (v *VirtualStore) GetLocalDB() *store.LocalStore {
	v.mu.RLock()
	defer v.mu.RUnlock()
	return v.localDB
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
	// Parse the action fact
	req, err := v.parseActionFact(action)
	if err != nil {
		return "", fmt.Errorf("failed to parse action fact: %w", err)
	}

	// Constitutional logic check (defense in depth)
	if err := v.checkConstitution(req); err != nil {
		v.injectFact(Fact{
			Predicate: "security_violation",
			Args:      []interface{}{string(req.Type), req.Target, err.Error()},
		})
		return "", err
	}

	// Route to appropriate handler
	result, err := v.executeAction(ctx, req)
	if err != nil {
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

	default:
		return ActionResult{}, fmt.Errorf("unknown action type: %s", req.Type)
	}
}

// handleExecCmd executes a shell command safely.
func (v *VirtualStore) handleExecCmd(ctx context.Context, req ActionRequest) (ActionResult, error) {
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

	// Use modern executor if enabled (auto-generates audit facts)
	v.mu.RLock()
	useModern := v.useModernExecutor && v.modernExecutor != nil
	v.mu.RUnlock()

	if useModern {
		return v.handleExecCmdModern(ctx, binary, args, timeout, req.SessionID)
	}

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
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "cmd_failed", Args: []interface{}{binary, err.Error()}},
			},
		}, nil // Return nil error but unsuccessful result
	}

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
	}

	return actionResult, nil
}

// handleReadFile reads a file from disk.
func (v *VirtualStore) handleReadFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	path := v.resolvePath(req.Target)

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
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	path := v.resolvePath(req.Target)

	content, ok := req.Payload["content"].(string)
	if !ok {
		return ActionResult{}, fmt.Errorf("write_file requires 'content' in payload")
	}

	// Ensure directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	err := os.WriteFile(path, []byte(content), 0644)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "file_write_error", Args: []interface{}{path, err.Error()}},
			},
		}, nil
	}

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
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	path := v.resolvePath(req.Target)

	oldContent, ok := req.Payload["old"].(string)
	if !ok {
		return ActionResult{}, fmt.Errorf("edit_file requires 'old' in payload")
	}
	newContent, ok := req.Payload["new"].(string)
	if !ok {
		return ActionResult{}, fmt.Errorf("edit_file requires 'new' in payload")
	}

	// Read existing file
	data, err := os.ReadFile(path)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	content := string(data)
	if !strings.Contains(content, oldContent) {
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
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

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

	// Require confirmation flag for safety
	confirmed, _ := req.Payload["confirmed"].(bool)
	if !confirmed {
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
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

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
	v.mu.RLock()
	codeGraph := v.codeGraph
	v.mu.RUnlock()

	if codeGraph == nil {
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

	result, err := codeGraph.CallTool(ctx, "search", args)
	if err != nil {
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

	output, _ := json.Marshal(result)
	return ActionResult{
		Success:    true,
		Output:     string(output),
		FactsToAdd: facts,
	}, nil
}

// handleRunTests executes the test suite.
func (v *VirtualStore) handleRunTests(ctx context.Context, req ActionRequest) (ActionResult, error) {
	// Determine test command based on project type
	testCmd := "go test ./..."
	if cmd, ok := req.Payload["command"].(string); ok {
		testCmd = cmd
	}

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
	buildCmd := "go build ./..."
	if cmd, ok := req.Payload["command"].(string); ok {
		buildCmd = cmd
	}

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
		diagnostics := v.parseBuildDiagnostics(output)
		for _, d := range diagnostics {
			facts = append(facts, d)
		}
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
	operation := req.Target
	args := []string{operation}

	if extraArgs, ok := req.Payload["args"].([]interface{}); ok {
		for _, a := range extraArgs {
			args = append(args, fmt.Sprintf("%v", a))
		}
	}

	cmd := tactile.ShellCommand{
		Binary:           "git",
		Arguments:        args,
		WorkingDirectory: v.workingDir,
		TimeoutSeconds:   60,
		EnvironmentVars:  v.getAllowedEnv(),
	}

	output, err := v.executor.Execute(ctx, cmd)

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
	v.mu.RLock()
	browser := v.browser
	v.mu.RUnlock()

	if browser == nil {
		return ActionResult{Success: false, Error: "browser integration not configured"}, nil
	}

	operation := req.Target
	sessionID, _ := req.Payload["session_id"].(string)
	if sessionID == "" {
		sessionID = "default"
	}

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
		return ActionResult{Success: false, Error: fmt.Sprintf("unknown browse operation: %s", operation)}, nil
	}

	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	return ActionResult{
		Success:    true,
		Output:     output,
		FactsToAdd: facts,
	}, nil
}

// handleResearch performs deep research via scraper service.
func (v *VirtualStore) handleResearch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	v.mu.RLock()
	scraper := v.scraper
	v.mu.RUnlock()

	if scraper == nil {
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

	result, err := scraper.CallTool(ctx, "deep-research", args)
	if err != nil {
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

	output, _ := json.Marshal(result)
	return ActionResult{
		Success:    true,
		Output:     string(output),
		FactsToAdd: facts,
	}, nil
}

// handleDelegate delegates a task to a ShardAgent.
func (v *VirtualStore) handleDelegate(ctx context.Context, req ActionRequest) (ActionResult, error) {
	v.mu.RLock()
	sm := v.shardManager
	v.mu.RUnlock()

	if sm == nil {
		return ActionResult{Success: false, Error: "shard manager not configured"}, nil
	}

	shardType := req.Target
	task, _ := req.Payload["task"].(string)

	result, err := sm.Spawn(ctx, shardType, task)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "delegation_failed", Args: []interface{}{shardType, err.Error()}},
			},
		}, nil
	}

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
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	v.mu.RLock()
	toolExec := v.toolExecutor
	registry := v.toolRegistry
	v.mu.RUnlock()

	if toolExec == nil {
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

	// Check if tool exists in registry first
	var registeredTool *Tool
	if registry != nil {
		registeredTool, _ = registry.GetTool(toolName)
	}

	// Check if tool exists in executor
	toolInfo, exists := toolExec.GetTool(toolName)
	if !exists {
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
	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("no knowledge database configured")
	}

	storedFacts, err := db.LoadFacts(predicate)
	if err != nil {
		return nil, fmt.Errorf("failed to query learned facts: %w", err)
	}

	facts := make([]Fact, 0, len(storedFacts))
	for _, sf := range storedFacts {
		facts = append(facts, Fact{
			Predicate: sf.Predicate,
			Args:      sf.Args,
		})
	}
	return facts, nil
}

// QueryAllLearned queries all facts from cold_storage.
// Returns facts grouped by fact_type (preference, constraint, fact).
func (v *VirtualStore) QueryAllLearned(factType string) ([]Fact, error) {
	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return nil, fmt.Errorf("no knowledge database configured")
	}

	storedFacts, err := db.LoadAllFacts(factType)
	if err != nil {
		return nil, fmt.Errorf("failed to query all learned facts: %w", err)
	}

	facts := make([]Fact, 0, len(storedFacts))
	for _, sf := range storedFacts {
		facts = append(facts, Fact{
			Predicate: sf.Predicate,
			Args:      sf.Args,
		})
	}
	return facts, nil
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

// HydrateKnowledgeGraph loads knowledge graph entries from LocalStore and hydrates
// the kernel with knowledge_link facts. This can be called independently or as part
// of HydrateLearnings for targeted knowledge graph updates.
func (v *VirtualStore) HydrateKnowledgeGraph(ctx context.Context) (int, error) {
	v.mu.RLock()
	db := v.localDB
	kernel := v.kernel
	v.mu.RUnlock()

	if db == nil {
		return 0, nil // No database, nothing to hydrate
	}
	if kernel == nil {
		return 0, fmt.Errorf("no kernel configured")
	}

	// Create assertion function that wraps kernel.Assert
	assertFunc := func(predicate string, args []interface{}) error {
		return kernel.Assert(Fact{
			Predicate: predicate,
			Args:      args,
		})
	}

	// Delegate to LocalStore's HydrateKnowledgeGraph
	count, err := db.HydrateKnowledgeGraph(assertFunc)
	if err != nil {
		return 0, fmt.Errorf("failed to hydrate knowledge graph: %w", err)
	}

	return count, nil
}

// HydrateLearnings loads all learned facts from knowledge.db and asserts them into the kernel.
// This should be called during OODA Observe phase to make learned knowledge available to rules.
func (v *VirtualStore) HydrateLearnings(ctx context.Context) (int, error) {
	v.mu.RLock()
	db := v.localDB
	kernel := v.kernel
	v.mu.RUnlock()

	if db == nil {
		return 0, nil // No database, nothing to hydrate
	}
	if kernel == nil {
		return 0, fmt.Errorf("no kernel configured")
	}

	count := 0

	// 1. Load all preferences (highest priority)
	preferences, err := v.QueryAllLearned("preference")
	if err == nil {
		for _, fact := range preferences {
			if err := kernel.Assert(Fact{
				Predicate: "learned_preference",
				Args:      []interface{}{fact.Predicate, fmt.Sprintf("%v", fact.Args)},
			}); err == nil {
				count++
			}
		}
	}

	// 2. Load all user facts
	userFacts, err := v.QueryAllLearned("user_fact")
	if err == nil {
		for _, fact := range userFacts {
			if err := kernel.Assert(Fact{
				Predicate: "learned_fact",
				Args:      []interface{}{fact.Predicate, fmt.Sprintf("%v", fact.Args)},
			}); err == nil {
				count++
			}
		}
	}

	// 3. Load all constraints
	constraints, err := v.QueryAllLearned("constraint")
	if err == nil {
		for _, fact := range constraints {
			if err := kernel.Assert(Fact{
				Predicate: "learned_constraint",
				Args:      []interface{}{fact.Predicate, fmt.Sprintf("%v", fact.Args)},
			}); err == nil {
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
			if err := kernel.Assert(fact); err == nil {
				count++
			}
		}
	}

	return count, nil
}
