package core

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/tactile"
)

// Exec executes a command directly, bypassing the ActionRequest routing but maintaining safety checks.
// It returns stdout, stderr, and error.
// This is used by the session package to execute tools directly via VirtualStore.
func (v *VirtualStore) Exec(ctx context.Context, cmd string, env []string) (string, string, error) {
	// 1. Safety Checks
	if strings.Contains(cmd, "..") {
		return "", "", fmt.Errorf("path traversal detected in command: %s", cmd)
	}

	// Default to bash execution for consistency with handleExecCmd
	binary := "bash"
	args := []string{"-c", cmd}

	// Enforce binary allowlist
	if !v.isBinaryAllowed(binary) {
		return "", "", fmt.Errorf("binary %s not allowed", binary)
	}

	// Merge allowed env with provided env
	// SECURITY: Filter provided env vars against the allowlist to prevent
	// PATH/LD_PRELOAD injection. In Go's os/exec, the last duplicate key wins,
	// so an attacker could override critical vars by appending them.
	filteredEnv := v.filterCallerEnv(env)
	finalEnv := append(v.getAllowedEnv(), filteredEnv...)

	// Construct Command
	command := tactile.Command{
		Binary:           binary,
		Arguments:        args,
		WorkingDirectory: v.workingDir,
		Environment:      finalEnv,
		Limits: &tactile.ResourceLimits{
			TimeoutMs: 30000, // Default 30s
		},
	}

	// Choose executor
	v.mu.RLock()
	useModern := v.useModernExecutor && v.modernExecutor != nil
	executor := v.executor
	if useModern {
		executor = v.modernExecutor
	}
	v.mu.RUnlock()

	result, err := executor.Execute(ctx, command)
	if err != nil {
		// Infrastructure error
		return "", "", err
	}

	if result.ExitCode != 0 {
		// Command failed
		errMsg := fmt.Sprintf("command failed with exit code %d", result.ExitCode)
		if result.Error != "" {
			errMsg += ": " + result.Error
		}
		return result.Stdout, result.Stderr, fmt.Errorf("%s", errMsg)
	}

	return result.Stdout, result.Stderr, nil
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
	if a, ok := req.Payload["args"].([]any); ok {
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
		return v.handleExecCmdModern(ctx, binary, args, timeout, req.SessionID, req.ActionID)
	}

	logging.VirtualStoreDebug("Using legacy SafeExecutor")

	// Legacy path using SafeExecutor (or whatever implementation is injected)
	cmd := tactile.Command{
		Binary:           binary,
		Arguments:        args,
		WorkingDirectory: v.workingDir,
		Environment:      v.getAllowedEnv(),
		Limits: &tactile.ResourceLimits{
			TimeoutMs: int64(timeout) * 1000,
		},
	}

	result, err := v.executor.Execute(ctx, cmd)
	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Shell command failed: %s - %v", binary, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "cmd_failed", Args: []any{binary, err.Error()}},
			},
		}, nil // Return nil error but unsuccessful result
	}

	logging.VirtualStoreDebug("Shell command succeeded: output_len=%d", len(result.Output()))
	return ActionResult{
		Success: true,
		Output:  result.Output(),
		FactsToAdd: []Fact{
			{Predicate: "cmd_succeeded", Args: []any{binary, result.Output()}},
		},
	}, nil
}

// handleExecCmdModern executes using the new tactile.Executor with auto-audit.
func (v *VirtualStore) handleExecCmdModern(ctx context.Context, binary string, args []string, timeout int, sessionID, requestID string) (ActionResult, error) {
	cmd := tactile.Command{
		Binary:           binary,
		Arguments:        args,
		WorkingDirectory: v.workingDir,
		Environment:      v.getAllowedEnv(),
		SessionID:        sessionID,
		RequestID:        requestID,
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
		}, nil
	}

	actionResult := ActionResult{
		Success: result.Success && result.ExitCode == 0,
		Output:  result.Output(),
		Metadata: map[string]any{
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

func commandFromActionRequest(req ActionRequest, defaultCommand string) string {
	if cmd, ok := req.Payload["command"].(string); ok && strings.TrimSpace(cmd) != "" {
		return cmd
	}
	if strings.TrimSpace(req.Target) != "" {
		return req.Target
	}
	return defaultCommand
}

func timeoutSecondsFromActionRequest(req ActionRequest, defaultSeconds int) int {
	if req.Timeout > 0 {
		return req.Timeout
	}
	if v, ok := payloadInt(req.Payload["timeout_seconds"]); ok && v > 0 {
		return v
	}
	if v, ok := payloadInt(req.Payload["timeout"]); ok && v > 0 {
		return v
	}
	if defaultSeconds <= 0 {
		return 30
	}
	return defaultSeconds
}

func payloadInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return 0, false
		}
		return int(x), true
	case json.Number:
		i, err := x.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(x))
		if err != nil {
			return 0, false
		}
		return i, true
	default:
		return 0, false
	}
}

// handleRunTests executes the test suite.
func (v *VirtualStore) handleRunTests(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleRunTests")
	defer timer.Stop()

	testCmd := commandFromActionRequest(req, "go test ./...")
	timeoutSeconds := timeoutSecondsFromActionRequest(req, 300)

	logging.VirtualStore("Running tests: %s", testCmd)

	cmd := tactile.Command{
		Binary:           "bash",
		Arguments:        []string{"-c", testCmd},
		WorkingDirectory: v.workingDir,
		Environment:      v.getAllowedEnv(),
		Limits: &tactile.ResourceLimits{
			TimeoutMs: int64(timeoutSeconds) * 1000,
		},
	}

	result, err := v.executor.Execute(ctx, cmd)
	var output string
	var success bool
	if result != nil {
		output = result.Output()
		success = err == nil && result.ExitCode == 0
	} else if err != nil {
		output = err.Error()
	}

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
			{Predicate: "test_state", Args: []any{testState}},
			{Predicate: "test_output", Args: []any{output}},
		},
	}, nil
}

// handleBuildProject builds the project.
func (v *VirtualStore) handleBuildProject(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleBuildProject")
	defer timer.Stop()

	buildCmd := commandFromActionRequest(req, "go build ./...")
	timeoutSeconds := timeoutSecondsFromActionRequest(req, 120)

	logging.VirtualStore("Building project: %s", buildCmd)

	cmd := tactile.Command{
		Binary:           "bash",
		Arguments:        []string{"-c", buildCmd},
		WorkingDirectory: v.workingDir,
		Environment:      v.getAllowedEnv(),
		Limits: &tactile.ResourceLimits{
			TimeoutMs: int64(timeoutSeconds) * 1000,
		},
	}

	result, err := v.executor.Execute(ctx, cmd)
	var output string
	var success bool
	if result != nil {
		output = result.Output()
		success = err == nil && result.ExitCode == 0
	} else if err != nil {
		output = err.Error()
	}

	facts := []Fact{
		{Predicate: "build_result", Args: []any{success, output}},
	}

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

	if extraArgs, ok := req.Payload["args"].([]any); ok {
		for _, a := range extraArgs {
			args = append(args, fmt.Sprintf("%v", a))
		}
	}

	logging.VirtualStore("Git operation: %s %v", operation, args[1:])

	cmd := tactile.Command{
		Binary:           "git",
		Arguments:        args,
		WorkingDirectory: v.workingDir,
		Environment:      v.getAllowedEnv(),
		Limits: &tactile.ResourceLimits{
			TimeoutMs: 60 * 1000,
		},
	}

	result, err := v.executor.Execute(ctx, cmd)
	output := ""
	if result != nil {
		output = result.Output()
	}

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
			{Predicate: "git_result", Args: []any{operation, err == nil, output}},
		},
	}, nil
}

func (v *VirtualStore) handleShowDiff(ctx context.Context, req ActionRequest) (ActionResult, error) {
	diffRef := strings.TrimSpace(req.Target)

	payload := make(map[string]any)
	maps.Copy(payload, req.Payload)
	if _, ok := payload["args"]; !ok && diffRef != "" {
		payload["args"] = []any{diffRef}
	}

	return v.handleGitOperation(ctx, ActionRequest{
		Type:    ActionGitOperation,
		Target:  "diff",
		Payload: payload,
	})
}

// handleAnalyzeImpact analyzes the impact of changes using code graph.
func (v *VirtualStore) handleAnalyzeImpact(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	codeGraph := v.GetMCPClient("code_graph")

	if codeGraph == nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Code graph MCP client not configured, skipping deep impact analysis")
		// Fallback: Assume local impact only to satisfy logic requirements without external tool.
		return ActionResult{
			Success: true,
			Output:  "Deep impact analysis skipped (code graph not configured)",
			FactsToAdd: []Fact{
				{Predicate: "impact_radius", Args: []any{req.Target, 0}},
			},
		}, nil
	}

	result, err := codeGraph.CallTool(ctx, "impact-analysis", map[string]any{
		"file": req.Target,
	})
	if err != nil {
		return ActionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	facts := []Fact{}

	if data, ok := result.(map[string]any); ok {
		if direct, ok := data["direct_dependents"].([]any); ok {
			facts = append(facts, Fact{
				Predicate: "impact_radius",
				Args:      []any{req.Target, len(direct)},
			})
			for _, dep := range direct {
				facts = append(facts, Fact{
					Predicate: "impacted",
					Args:      []any{dep},
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

// handleBrowse handles browser automation requests.
// Browser automation is provided by internal/browser package via TactileRouterShard.
// VirtualStore provides a routing layer that directs to the appropriate shard.
func (v *VirtualStore) handleBrowse(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleBrowse")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	operation := req.Target
	logging.VirtualStore("Browse request: %s (routing to TactileRouterShard)", operation)

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	// Browser automation is handled by TactileRouterShard which has the SessionManager wired.
	// VirtualStore cannot directly execute browser operations - it must go through the shard system.
	// This ensures proper session management, sandboxing, and audit trails.
	return ActionResult{
		Success: false,
		Output:  "",
		Error:   "browser operations must be executed via TactileRouterShard - use shard-based browser automation",
		FactsToAdd: []Fact{
			{Predicate: "browser_routing", Args: []any{operation, "/requires_shard"}},
		},
	}, nil
}

// handleResearch handles research requests using modular tools.
// Research functionality is now provided by modular tools (Context7, WebFetch, Browser, etc.)
// that any agent can use via the JIT system.
func (v *VirtualStore) handleResearch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleResearch")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	query := req.Target
	logging.VirtualStore("Research request: %s", query)

	// Try Context7 first for library/framework documentation
	v.mu.RLock()
	registry := v.modularTools
	v.mu.RUnlock()

	if registry == nil {
		return ActionResult{
			Success: false,
			Error:   "modular tools registry not initialized",
		}, nil
	}

	// Execute context7_fetch tool
	tool := registry.Get("context7_fetch")
	if tool == nil {
		return ActionResult{
			Success: false,
			Error:   "context7_fetch tool not registered",
		}, nil
	}

	result, err := registry.Execute(ctx, "context7_fetch", map[string]any{"topic": query})
	if err != nil {
		logging.VirtualStoreDebug("Context7 fetch failed: %v", err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "research_failed", Args: []any{query, err.Error()}},
			},
		}, nil
	}

	return ActionResult{
		Success: true,
		Output:  result.Result,
		FactsToAdd: []Fact{
			{Predicate: "research_completed", Args: []any{query, len(result.Result)}},
		},
	}, nil
}

// handleModularTool handles execution of modular tools via the registry.
func (v *VirtualStore) handleModularTool(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleModularTool")
	defer timer.Stop()

	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	toolName := string(req.Type)
	logging.VirtualStore("Executing modular tool: %s", toolName)

	v.mu.RLock()
	registry := v.modularTools
	v.mu.RUnlock()

	if registry == nil {
		return ActionResult{
			Success: false,
			Error:   "modular tools registry not initialized",
		}, nil
	}

	// Build args from request
	args := make(map[string]any)

	// Add target as primary arg based on tool type
	switch req.Type {
	case ActionContext7Fetch:
		args["topic"] = req.Target
	case ActionWebFetch:
		args["url"] = req.Target
	case ActionBrowserNavigate:
		args["url"] = req.Target
		if sid, ok := req.Payload["session_id"].(string); ok {
			args["session_id"] = sid
		}
	case ActionBrowserExtract, ActionBrowserScreenshot, ActionBrowserClose:
		args["session_id"] = req.Target
		if sel, ok := req.Payload["selector"].(string); ok {
			args["selector"] = sel
		}
	case ActionBrowserClick:
		args["session_id"] = req.Target
		args["selector"], _ = req.Payload["selector"].(string)
	case ActionBrowserType:
		args["session_id"] = req.Target
		args["selector"], _ = req.Payload["selector"].(string)
		args["text"], _ = req.Payload["text"].(string)
	case ActionResearchCacheGet:
		args["key"] = req.Target
	case ActionResearchCacheSet:
		args["key"] = req.Target
		args["value"], _ = req.Payload["value"].(string)
		args["source"], _ = req.Payload["source"].(string)
	}

	// Merge any additional payload args
	for k, v := range req.Payload {
		if _, exists := args[k]; !exists {
			args[k] = v
		}
	}

	result, err := registry.Execute(ctx, toolName, args)
	if err != nil {
		logging.VirtualStoreDebug("Modular tool %s failed: %v", toolName, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "tool_failed", Args: []any{toolName, err.Error()}},
			},
		}, nil
	}

	return ActionResult{
		Success: true,
		Output:  result.Result,
		FactsToAdd: []Fact{
			{Predicate: "tool_executed", Args: []any{toolName, result.DurationMs}},
		},
	}, nil
}

// handleDelegate delegates a task to a ShardAgent.
func (v *VirtualStore) handleDelegate(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleDelegate")
	defer timer.Stop()

	v.mu.RLock()
	delegator := v.taskDelegator
	sm := v.shardManager
	v.mu.RUnlock()

	if delegator == nil && sm == nil {
		logging.Get(logging.CategoryVirtualStore).Error("No executor configured for delegation")
		return ActionResult{Success: false, Error: "no executor configured (taskDelegator and shardManager are nil)"}, nil
	}

	shardType := req.Target
	task, _ := req.Payload["task"].(string)

	logging.VirtualStore("Delegating to shard: type=%s, task_len=%d", shardType, len(task))

	var result string
	var err error
	if delegator != nil {
		// Use new TaskDelegator (JIT architecture)
		result, err = delegator.Execute(ctx, shardType, task)
	} else {
		// Fall back to legacy ShardManager
		result, err = sm.Spawn(ctx, shardType, task)
	}

	if err != nil {
		logging.Get(logging.CategoryVirtualStore).Error("Shard delegation failed: %s - %v", shardType, err)
		return ActionResult{
			Success: false,
			Error:   err.Error(),
			FactsToAdd: []Fact{
				{Predicate: "delegation_failed", Args: []any{shardType, err.Error()}},
			},
		}, nil
	}

	logging.VirtualStore("Shard delegation completed: type=%s, result_len=%d", shardType, len(result))
	return ActionResult{
		Success: true,
		Output:  result,
		FactsToAdd: []Fact{
			{Predicate: "delegation_result", Args: []any{shardType, result}},
		},
	}, nil
}

func (v *VirtualStore) handleDelegateAlias(ctx context.Context, req ActionRequest, shardType string) (ActionResult, error) {
	task := ""
	if t, ok := req.Payload["task"].(string); ok {
		task = strings.TrimSpace(t)
	}
	if task == "" {
		task = strings.TrimSpace(req.Target)
	}
	if task == "" {
		return ActionResult{Success: false, Error: "delegate task is empty"}, nil
	}

	payload := make(map[string]any)
	maps.Copy(payload, req.Payload)
	payload["task"] = task

	return v.handleDelegate(ctx, ActionRequest{
		Type:    ActionDelegate,
		Target:  shardType,
		Payload: payload,
	})
}

// handleAskUser handles requests that require user input.
func (v *VirtualStore) handleAskUser(ctx context.Context, req ActionRequest) (ActionResult, error) {
	if err := ctx.Err(); err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}
	question := req.Target
	options, _ := req.Payload["options"].([]any)

	return ActionResult{
		Success: false,
		Output:  question,
		Error:   "USER_INPUT_REQUIRED",
		Metadata: map[string]any{
			"question": question,
			"options":  options,
		},
		FactsToAdd: []Fact{
			{Predicate: "awaiting_user_input", Args: []any{question}},
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
			{Predicate: "escalated", Args: []any{reason}},
			{Predicate: "task_blocked", Args: []any{reason}},
		},
	}, nil
}

// GetStrategicSummary retrieves a formatted summary of strategic knowledge
// for injection into prompts when handling conceptual queries about the codebase.
// Extended by Semantic Knowledge Bridge to include high-value doc/ atoms.
// Returns empty string if no strategic knowledge is available.
func (v *VirtualStore) GetStrategicSummary() string {
	v.mu.RLock()
	db := v.localDB
	v.mu.RUnlock()

	if db == nil {
		return ""
	}

	// Query all strategic knowledge atoms
	atoms, err := db.GetKnowledgeAtomsByPrefix("strategic/")
	if err != nil {
		atoms = nil // Continue with empty if error
	}

	var sb strings.Builder
	sb.WriteString("## Project Strategic Knowledge\n\n")

	// Group by category for organized output
	categories := map[string][]string{
		"vision":       {},
		"philosophy":   {},
		"architecture": {},
		"pattern":      {},
		"component":    {},
		"capability":   {},
		"constraint":   {},
	}

	for _, atom := range atoms {
		category := strings.TrimPrefix(atom.Concept, "strategic/")
		// Skip the full_knowledge blob - it's too verbose for context injection
		if category == "full_knowledge" {
			continue
		}
		if _, ok := categories[category]; ok {
			categories[category] = append(categories[category], atom.Content)
		}
	}

	// Semantic Knowledge Bridge: Also query high-confidence doc atoms
	// These provide architecture/pattern/philosophy insights from documentation
	docAtoms, err := db.GetKnowledgeAtomsByPrefix("doc/")
	if err == nil {
		for _, atom := range docAtoms {
			// Only include high-confidence atoms
			if atom.Confidence < 0.85 {
				continue
			}
			// Categorize based on concept path
			if strings.Contains(atom.Concept, "/architecture/") {
				categories["architecture"] = append(categories["architecture"], atom.Content)
			} else if strings.Contains(atom.Concept, "/pattern/") {
				categories["pattern"] = append(categories["pattern"], atom.Content)
			} else if strings.Contains(atom.Concept, "/philosophy/") {
				categories["philosophy"] = append(categories["philosophy"], atom.Content)
			} else if strings.Contains(atom.Concept, "/capability/") {
				categories["capability"] = append(categories["capability"], atom.Content)
			} else if strings.Contains(atom.Concept, "/constraint/") {
				categories["constraint"] = append(categories["constraint"], atom.Content)
			}
		}
	}

	// Output in structured order
	if len(categories["vision"]) > 0 {
		sb.WriteString("**Vision:** ")
		sb.WriteString(categories["vision"][0])
		sb.WriteString("\n\n")
	}

	if len(categories["philosophy"]) > 0 {
		sb.WriteString("**Philosophy:** ")
		sb.WriteString(categories["philosophy"][0])
		sb.WriteString("\n\n")
	}

	if len(categories["architecture"]) > 0 {
		sb.WriteString("**Architecture:** ")
		sb.WriteString(categories["architecture"][0])
		sb.WriteString("\n\n")
	}

	if len(categories["component"]) > 0 {
		sb.WriteString("**Key Components:**\n")
		for _, c := range categories["component"] {
			sb.WriteString("- ")
			sb.WriteString(c)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(categories["pattern"]) > 0 {
		sb.WriteString("**Core Patterns:**\n")
		for _, p := range categories["pattern"] {
			sb.WriteString("- ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(categories["capability"]) > 0 {
		sb.WriteString("**Capabilities:**\n")
		for _, c := range categories["capability"] {
			sb.WriteString("- ")
			sb.WriteString(c)
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	if len(categories["constraint"]) > 0 {
		sb.WriteString("**Safety Constraints:**\n")
		for _, c := range categories["constraint"] {
			sb.WriteString("- ")
			sb.WriteString(c)
			sb.WriteString("\n")
		}
	}

	result := sb.String()
	if result == "## Project Strategic Knowledge\n\n" {
		return "" // No meaningful content
	}

	logging.VirtualStoreDebug("GetStrategicSummary: generated %d chars", len(result))
	return result
}

// extractCodeBlockForFile extracts code from content that may contain LLM reasoning traces.
// It looks for code blocks matching the file extension's language.
func extractCodeBlockForFile(content, path string) string {
	ext := strings.TrimPrefix(filepath.Ext(path), ".")
	lang := extToLang(ext)

	// First try: Look for ```lang block
	patterns := []string{
		"```" + lang + "\n",
		"```" + lang + "\r\n",
		"```\n",
		"```\r\n",
	}

	for _, pattern := range patterns {
		if idx := strings.Index(content, pattern); idx != -1 {
			start := idx + len(pattern)
			end := strings.Index(content[start:], "```")
			if end != -1 {
				extracted := strings.TrimSpace(content[start : start+end])
				if len(extracted) > 0 {
					return extracted
				}
			}
		}
	}

	// Second try: For Go files, look for "package" keyword
	if lang == "go" {
		if pkgIdx := strings.Index(content, "package "); pkgIdx != -1 {
			return strings.TrimSpace(content[pkgIdx:])
		}
	}

	// Third try: Look for first { and match to closing } (for JSON-like or code files)
	if braceStart := strings.Index(content, "{"); braceStart != -1 && (lang == "json" || lang == "go" || lang == "kotlin" || lang == "typescript" || lang == "javascript") {
		depth := 0
		inString := false
		escape := false
		for i := braceStart; i < len(content); i++ {
			c := content[i]
			if escape {
				escape = false
				continue
			}
			if c == '\\' && inString {
				escape = true
				continue
			}
			if c == '"' {
				inString = !inString
				continue
			}
			if inString {
				continue
			}
			if c == '{' {
				depth++
			} else if c == '}' {
				depth--
				if depth == 0 {
					return strings.TrimSpace(content[braceStart : i+1])
				}
			}
		}
	}

	// Fallback: return original (might already be clean)
	return strings.TrimSpace(content)
}

// extToLang maps file extensions to language identifiers.
func extToLang(ext string) string {
	switch ext {
	case "go":
		return "go"
	case "ts", "tsx":
		return "typescript"
	case "js", "jsx":
		return "javascript"
	case "kt":
		return "kotlin"
	case "py":
		return "python"
	case "sql":
		return "sql"
	case "yaml", "yml":
		return "yaml"
	case "json":
		return "json"
	case "md":
		return "markdown"
	default:
		return ext
	}
}
