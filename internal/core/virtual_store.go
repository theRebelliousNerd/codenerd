package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

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

// VirtualStore acts as the FFI Router for the Hollow Kernel.
// It routes 'next_action' atoms to the appropriate driver (Bash, MCP, File IO).
type VirtualStore struct {
	mu sync.RWMutex

	// Execution layer
	executor *tactile.SafeExecutor

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
	}

	// Initialize constitutional rules (safety layer)
	vs.initConstitution()

	return vs
}

// SetKernel sets the kernel for fact injection feedback.
func (v *VirtualStore) SetKernel(k Kernel) {
	v.mu.Lock()
	defer v.mu.Unlock()
	v.kernel = k
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

// handleReadFile reads a file from disk.
func (v *VirtualStore) handleReadFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
	path := v.resolvePath(req.Target)

	data, err := os.ReadFile(path)
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "file_read_error", Args: []interface{}{path, err.Error()}},
			},
		}, nil
	}

	content := string(data)

	// Add file topology fact
	info, _ := os.Stat(path)
	modTime := int64(0)
	if info != nil {
		modTime = info.ModTime().Unix()
	}

	return ActionResult{
		Success: true,
		Output:  content,
		Metadata: map[string]interface{}{
			"path":     path,
			"size":     len(data),
			"modified": modTime,
		},
		FactsToAdd: []Fact{
			{Predicate: "file_content", Args: []interface{}{path, content}},
			{Predicate: "file_read", Args: []interface{}{path, len(data)}},
		},
	}, nil
}

// handleWriteFile writes content to a file.
func (v *VirtualStore) handleWriteFile(ctx context.Context, req ActionRequest) (ActionResult, error) {
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
