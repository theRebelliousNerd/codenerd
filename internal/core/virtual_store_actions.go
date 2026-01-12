package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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
	// Provided env overrides allowed env if duplicates exist (tactile executor usually handles last-win or simple append)
	finalEnv := append(v.getAllowedEnv(), env...)

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
				{Predicate: "cmd_failed", Args: []interface{}{binary, err.Error()}},
			},
		}, nil // Return nil error but unsuccessful result
	}

	logging.VirtualStoreDebug("Shell command succeeded: output_len=%d", len(result.Output()))
	return ActionResult{
		Success: true,
		Output:  result.Output(),
		FactsToAdd: []Fact{
			{Predicate: "cmd_succeeded", Args: []interface{}{binary, result.Output()}},
		},
	}, nil
}

// handleExecCmdModern executes using the new tactile.Executor with auto-audit.
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
		}, nil
	}

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

	const MaxFileSize = 100 * 1024 // 100KB limit

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

	if info.IsDir() {
		return v.handleReadDirectory(ctx, path)
	}

	var data []byte
	var truncated bool

	if info.Size() > MaxFileSize {
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

// handleReadDirectory reads a directory and returns a summary.
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

	var dirs, files []string
	for _, entry := range entries {
		if entry.IsDir() {
			dirs = append(dirs, entry.Name()+"/")
		} else {
			files = append(files, entry.Name())
		}
	}

	if len(dirs) > 0 {
		sb.WriteString("Subdirectories:\n")
		for _, d := range dirs {
			sb.WriteString(fmt.Sprintf("  %s\n", d))
		}
		sb.WriteString("\n")
	}

	if len(files) > 0 {
		sb.WriteString("Files:\n")
		for _, f := range files {
			info, err := os.Stat(filepath.Join(dirPath, f))
			if err == nil {
				sb.WriteString(fmt.Sprintf("  %s (%d bytes)\n", f, info.Size()))
			} else {
				sb.WriteString(fmt.Sprintf("  %s\n", f))
			}
		}
	}

	sb.WriteString(fmt.Sprintf("\nTotal: %d directories, %d files\n", len(dirs), len(files)))

	return ActionResult{
		Success: true,
		Output:  sb.String(),
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

	// Extract code block from content (removes LLM reasoning traces and markdown fences)
	originalLen := len(content)
	content = extractCodeBlockForFile(content, path)
	if len(content) != originalLen {
		logging.VirtualStoreDebug("Extracted code block for %s: %d -> %d bytes", path, originalLen, len(content))
	}

	logging.VirtualStoreDebug("Writing file: %s (%d bytes)", path, len(content))

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

	newFileContent := strings.Replace(content, oldContent, newContent, 1)

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

// handleSearchCode searches for code patterns using local filesystem search.
// For semantic/AST-based search, use the internal/world package via shards.
func (v *VirtualStore) handleSearchCode(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleSearchCode")
	defer timer.Stop()

	pattern := req.Target
	facts := make([]Fact, 0)
	var output strings.Builder
	count := 0

	// Local search using filepath.Walk
	err := filepath.Walk(v.workingDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip hidden directories and large files
		if strings.Contains(path, ".git") || strings.Contains(path, ".nerd") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		content := string(data)
		lines := strings.Split(content, "\n")
		relPath, _ := filepath.Rel(v.workingDir, path)

		for i, line := range lines {
			if strings.Contains(line, pattern) {
				count++
				lineNum := i + 1
				facts = append(facts, Fact{
					Predicate: "search_result",
					Args: []interface{}{
						relPath,
						lineNum,
						strings.TrimSpace(line),
					},
				})
				output.WriteString(fmt.Sprintf("%s:%d:%s\n", relPath, lineNum, line))
				if count >= 100 { // Cap results
					return filepath.SkipDir
				}
			}
		}
		return nil
	})

	if err != nil {
		return ActionResult{Success: false, Error: err.Error()}, nil
	}

	logging.VirtualStoreDebug("Local search returned %d results", len(facts))
	return ActionResult{
		Success:    true,
		Output:     output.String(),
		FactsToAdd: facts,
	}, nil
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
	success := err == nil && result.ExitCode == 0
	output := result.Output()

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
	success := err == nil && result.ExitCode == 0
	output := result.Output()

	facts := []Fact{
		{Predicate: "build_result", Args: []interface{}{success, output}},
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

	if extraArgs, ok := req.Payload["args"].([]interface{}); ok {
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
			{Predicate: "git_result", Args: []interface{}{operation, err == nil, output}},
		},
	}, nil
}

func (v *VirtualStore) handleShowDiff(ctx context.Context, req ActionRequest) (ActionResult, error) {
	diffRef := strings.TrimSpace(req.Target)

	payload := make(map[string]interface{})
	for k, val := range req.Payload {
		payload[k] = val
	}
	if _, ok := payload["args"]; !ok && diffRef != "" {
		payload["args"] = []interface{}{diffRef}
	}

	return v.handleGitOperation(ctx, ActionRequest{
		Type:    ActionGitOperation,
		Target:  "diff",
		Payload: payload,
	})
}

// handleAnalyzeImpact analyzes the impact of changes using code graph.
func (v *VirtualStore) handleAnalyzeImpact(ctx context.Context, req ActionRequest) (ActionResult, error) {
	codeGraph := v.GetMCPClient("code_graph")

	if codeGraph == nil {
		logging.Get(logging.CategoryVirtualStore).Warn("Code graph MCP client not configured, skipping deep impact analysis")
		// Fallback: Assume local impact only to satisfy logic requirements without external tool.
		return ActionResult{
			Success: true,
			Output:  "Deep impact analysis skipped (code graph not configured)",
			FactsToAdd: []Fact{
				{Predicate: "impact_radius", Args: []interface{}{req.Target, 0}},
			},
		}, nil
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

// handleBrowse handles browser automation requests.
// Browser automation is provided by internal/browser package via TactileRouterShard.
// VirtualStore provides a routing layer that directs to the appropriate shard.
func (v *VirtualStore) handleBrowse(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleBrowse")
	defer timer.Stop()

	operation := req.Target
	logging.VirtualStore("Browse request: %s (routing to TactileRouterShard)", operation)

	// Browser automation is handled by TactileRouterShard which has the SessionManager wired.
	// VirtualStore cannot directly execute browser operations - it must go through the shard system.
	// This ensures proper session management, sandboxing, and audit trails.
	return ActionResult{
		Success: false,
		Output:  "",
		Error:   "browser operations must be executed via TactileRouterShard - use shard-based browser automation",
		FactsToAdd: []Fact{
			{Predicate: "browser_routing", Args: []interface{}{operation, "/requires_shard"}},
		},
	}, nil
}

// handleResearch handles research requests using modular tools.
// Research functionality is now provided by modular tools (Context7, WebFetch, Browser, etc.)
// that any agent can use via the JIT system.
func (v *VirtualStore) handleResearch(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleResearch")
	defer timer.Stop()

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
				{Predicate: "research_failed", Args: []interface{}{query, err.Error()}},
			},
		}, nil
	}

	return ActionResult{
		Success: true,
		Output:  result.Result,
		FactsToAdd: []Fact{
			{Predicate: "research_completed", Args: []interface{}{query, len(result.Result)}},
		},
	}, nil
}

// handleModularTool handles execution of modular tools via the registry.
func (v *VirtualStore) handleModularTool(ctx context.Context, req ActionRequest) (ActionResult, error) {
	timer := logging.StartTimer(logging.CategoryVirtualStore, "handleModularTool")
	defer timer.Stop()

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
				{Predicate: "tool_failed", Args: []interface{}{toolName, err.Error()}},
			},
		}, nil
	}

	return ActionResult{
		Success: true,
		Output:  result.Result,
		FactsToAdd: []Fact{
			{Predicate: "tool_executed", Args: []interface{}{toolName, result.DurationMs}},
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
		intent := LegacyShardNameToIntent(shardType)
		result, err = delegator.Execute(ctx, intent, task)
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

	payload := make(map[string]interface{})
	for k, val := range req.Payload {
		payload[k] = val
	}
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
	options, _ := req.Payload["options"].([]interface{})

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

	// Second try: Look for first { and match to closing } (for JSON-like or code files)
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

	// Third try: For Go files, look for "package" keyword
	if lang == "go" {
		if pkgIdx := strings.Index(content, "package "); pkgIdx != -1 {
			return strings.TrimSpace(content[pkgIdx:])
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
