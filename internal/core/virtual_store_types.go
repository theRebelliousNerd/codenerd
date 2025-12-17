package core

import (
	"context"

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
	ActionSearchFiles   ActionType = "search_files" // Back-compat alias for ActionSearchCode
	ActionAnalyzeCode   ActionType = "analyze_code" // Alias for ActionSearchCode (policy emits /analyze_code)
	ActionRunTests      ActionType = "run_tests"
	ActionBuildProject  ActionType = "build_project"
	ActionGitOperation  ActionType = "git_operation"
	ActionAnalyzeImpact ActionType = "analyze_impact"
	ActionBrowse        ActionType = "browse"
	ActionResearch      ActionType = "research"
	ActionAskUser       ActionType = "ask_user"
	ActionEscalate      ActionType = "escalate"
	ActionDelegate      ActionType = "delegate"

	// Delegation aliases emitted by policy (map to ActionDelegate)
	ActionDelegateReviewer      ActionType = "delegate_reviewer"
	ActionDelegateCoder         ActionType = "delegate_coder"
	ActionDelegateResearcher    ActionType = "delegate_researcher"
	ActionDelegateToolGenerator ActionType = "delegate_tool_generator"

	// Diff alias emitted by policy (maps to git_operation diff)
	ActionShowDiff ActionType = "show_diff"

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
	ActionReadErrorLog     ActionType = "read_error_log"        // Read test/build error logs
	ActionAnalyzeRootCause ActionType = "analyze_root_cause"    // Analyze root cause of failure
	ActionGeneratePatch    ActionType = "generate_patch"        // Generate code patch
	ActionEscalateToUser   ActionType = "escalate_to_user"      // Escalate to user
	ActionComplete         ActionType = "complete"              // Mark task complete
	ActionInterrogative    ActionType = "interrogative_mode"    // Ask clarifying questions
	ActionResumeTask       ActionType = "resume_task"           // Resume a paused task
	ActionRefreshShardCtx  ActionType = "refresh_shard_context" // Refresh stale shard context

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
	ActionID   string                 `json:"action_id"`
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

// StaticToolDef is defined in tool_registry.go
