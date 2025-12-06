// Package shards implements specialized ShardAgent types for the Cortex 1.5.0 architecture.
// This file implements the Coder ShardAgent (Type B: Persistent Specialist).
// The Coder Shard is responsible for code writing, modification, and refactoring.
// It is LANGUAGE-AGNOSTIC - language detection is automatic based on file extensions.
// For language-specific expertise, create Type 3 specialists via: nerd define-agent
package shards

import (
	"codenerd/internal/core"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// CoderConfig holds configuration for the coder shard.
type CoderConfig struct {
	MaxEdits     int    // Max edits per task (default: 10)
	StyleProfile string // Code style rules or path to style guide
	SafetyMode   bool   // Block risky edits without confirmation
	MaxRetries   int    // Retry limit for failed edits
	WorkingDir   string // Workspace path
	ImpactCheck  bool   // Check dependency impact before write (default: true)
}

// DefaultCoderConfig returns sensible defaults for coding.
func DefaultCoderConfig() CoderConfig {
	return CoderConfig{
		MaxEdits:     10,
		StyleProfile: "default",
		SafetyMode:   true,
		MaxRetries:   3,
		WorkingDir:   ".",
		ImpactCheck:  true,
	}
}

// CodeEdit represents a proposed change to a file.
type CodeEdit struct {
	File       string `json:"file"`
	OldContent string `json:"old_content,omitempty"` // For modifications
	NewContent string `json:"new_content"`           // New content
	Type       string `json:"type"`                  // "create", "modify", "delete"
	Language   string `json:"language"`              // Detected language
	Rationale  string `json:"rationale"`             // Why this change
}

// CoderResult represents the output of a coding task.
type CoderResult struct {
	Summary     string          `json:"summary"`
	Edits       []CodeEdit      `json:"edits"`
	BuildPassed bool            `json:"build_passed"`
	TestsPassed bool            `json:"tests_passed"`
	Diagnostics []core.Diagnostic `json:"diagnostics,omitempty"`
	Facts       []core.Fact     `json:"facts,omitempty"`
	Duration    time.Duration   `json:"duration"`
}

// CoderTask represents a parsed coding task.
type CoderTask struct {
	Action      string            // create, modify, refactor, fix, implement
	Target      string            // File path or symbol
	Instruction string            // What to do
	Context     map[string]string // Additional context
}

// CoderShard is specialized for code writing and modification.
// It is language-agnostic and auto-detects languages from file extensions.
type CoderShard struct {
	mu sync.RWMutex

	// Identity
	id     string
	config core.ShardConfig
	state  core.ShardState

	// Coder-specific
	coderConfig CoderConfig

	// Components - each shard has its own kernel
	kernel       *core.RealKernel
	llmClient    core.LLMClient
	virtualStore *core.VirtualStore

	// State tracking
	startTime   time.Time
	editHistory []CodeEdit
	diagnostics []core.Diagnostic

	// Learnings for autopoiesis
	rejectionCount map[string]int
	acceptanceCount map[string]int
}

// NewCoderShard creates a new Coder shard.
func NewCoderShard() *CoderShard {
	return NewCoderShardWithConfig(DefaultCoderConfig())
}

// NewCoderShardWithConfig creates a coder shard with custom config.
func NewCoderShardWithConfig(coderConfig CoderConfig) *CoderShard {
	shard := &CoderShard{
		id:              fmt.Sprintf("coder-%d", time.Now().UnixNano()),
		config:          core.DefaultSpecialistConfig("coder", ""),
		state:           core.ShardStateIdle,
		coderConfig:     coderConfig,
		editHistory:     make([]CodeEdit, 0),
		diagnostics:     make([]core.Diagnostic, 0),
		rejectionCount:  make(map[string]int),
		acceptanceCount: make(map[string]int),
	}
	return shard
}

// SetLLMClient sets the LLM client for code generation.
func (c *CoderShard) SetLLMClient(client core.LLMClient) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.llmClient = client
}

// SetParentKernel sets the Mangle kernel for fact storage and policy evaluation.
func (c *CoderShard) SetParentKernel(k core.Kernel) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if rk, ok := k.(*core.RealKernel); ok {
		c.kernel = rk
	} else {
		panic("CoderShard requires *core.RealKernel")
	}
}

// SetVirtualStore sets the virtual store for action routing.
func (c *CoderShard) SetVirtualStore(vs *core.VirtualStore) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.virtualStore = vs
}

// GetID returns the shard ID.
func (c *CoderShard) GetID() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.id
}

// GetState returns the current state.
func (c *CoderShard) GetState() core.ShardState {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// GetConfig returns the shard configuration.
func (c *CoderShard) GetConfig() core.ShardConfig {
	return c.config
}

// GetKernel returns the shard's kernel (for fact propagation).
func (c *CoderShard) GetKernel() *core.RealKernel {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.kernel
}

// Stop stops the shard.
func (c *CoderShard) Stop() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.state = core.ShardStateCompleted
	return nil
}

// Execute performs the coding task.
// Supported formats:
//   - create file:path/to/file.go spec:description
//   - modify file:path/to/file.go instruction:what to change
//   - refactor file:path/to/file.go target:functionName instruction:how
//   - fix file:path/to/file.go error:error message
//   - implement interface:InterfaceName in:file.go
func (c *CoderShard) Execute(ctx context.Context, task string) (string, error) {
	c.mu.Lock()
	c.state = core.ShardStateRunning
	c.startTime = time.Now()
	c.editHistory = make([]CodeEdit, 0)
	c.diagnostics = make([]core.Diagnostic, 0)
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.state = core.ShardStateCompleted
		c.mu.Unlock()
	}()

	// Initialize kernel if not set
	if c.kernel == nil {
		c.kernel = core.NewRealKernel()
	}
	// Load coder-specific policy
	_ = c.kernel.LoadPolicyFile("coder.gl")

	// Parse the task
	coderTask := c.parseTask(task)

	// Assert task facts to kernel
	c.assertTaskFacts(coderTask)

	// Read file context if modifying existing file
	fileContext, err := c.readFileContext(ctx, coderTask.Target)
	if err != nil && coderTask.Action != "create" {
		return "", fmt.Errorf("failed to read file context: %w", err)
	}

	// Check impact if enabled
	if c.coderConfig.ImpactCheck && coderTask.Action != "create" {
		blocked, reason := c.checkImpact(coderTask.Target)
		if blocked {
			return "", fmt.Errorf("edit blocked by impact analysis: %s", reason)
		}
	}

	// Generate code with LLM
	result, err := c.generateCode(ctx, coderTask, fileContext)
	if err != nil {
		return "", fmt.Errorf("code generation failed: %w", err)
	}

	// Apply edits via VirtualStore
	if err := c.applyEdits(ctx, result.Edits); err != nil {
		// Track rejection for autopoiesis
		c.trackRejection(coderTask.Action, err.Error())
		return "", fmt.Errorf("failed to apply edits: %w", err)
	}

	// Track success for autopoiesis
	c.trackAcceptance(coderTask.Action)

	// Run build check if available
	result.BuildPassed = c.runBuildCheck(ctx)

	// Generate facts for propagation
	result.Facts = c.generateFacts(result)
	result.Duration = time.Since(c.startTime)

	// Build response
	return c.buildResponse(result), nil
}

// parseTask extracts structured information from a task string.
func (c *CoderShard) parseTask(task string) CoderTask {
	parsed := CoderTask{
		Action:  "unknown",
		Context: make(map[string]string),
	}

	// Normalize whitespace
	task = strings.TrimSpace(task)
	if task == "" {
		return parsed
	}

	// Extract action (first word)
	parts := strings.Fields(task)
	if len(parts) > 0 {
		parsed.Action = strings.ToLower(parts[0])
	}

	// Parse key:value pairs
	kvRegex := regexp.MustCompile(`(\w+):([^\s]+|"[^"]+")`)
	matches := kvRegex.FindAllStringSubmatch(task, -1)
	for _, match := range matches {
		if len(match) >= 3 {
			key := strings.ToLower(match[1])
			value := strings.Trim(match[2], `"`)
			parsed.Context[key] = value

			// Map common keys
			switch key {
			case "file", "path", "target":
				parsed.Target = value
			case "spec", "instruction", "description", "error":
				parsed.Instruction = value
			case "in":
				if parsed.Target == "" {
					parsed.Target = value
				}
			}
		}
	}

	// If instruction wasn't captured, try to get everything after known keys
	if parsed.Instruction == "" {
		// Find the instruction part after spec:, instruction:, or description:
		for _, prefix := range []string{"spec:", "instruction:", "description:", "error:"} {
			idx := strings.Index(strings.ToLower(task), prefix)
			if idx != -1 {
				parsed.Instruction = strings.TrimSpace(task[idx+len(prefix):])
				// Remove any subsequent key:value pairs
				nextKeyIdx := strings.Index(parsed.Instruction, ":")
				if nextKeyIdx > 0 {
					// Find the word before the colon
					spaceIdx := strings.LastIndex(parsed.Instruction[:nextKeyIdx], " ")
					if spaceIdx > 0 {
						parsed.Instruction = strings.TrimSpace(parsed.Instruction[:spaceIdx])
					}
				}
				break
			}
		}
	}

	// Fallback: if no instruction, use everything after target
	if parsed.Instruction == "" && len(parts) > 2 {
		targetFound := false
		var instrParts []string
		for _, p := range parts[1:] {
			if strings.HasPrefix(p, "file:") || strings.HasPrefix(p, "path:") {
				targetFound = true
				continue
			}
			if targetFound && !strings.Contains(p, ":") {
				instrParts = append(instrParts, p)
			}
		}
		parsed.Instruction = strings.Join(instrParts, " ")
	}

	return parsed
}

// assertTaskFacts adds task-related facts to the kernel.
func (c *CoderShard) assertTaskFacts(task CoderTask) {
	if c.kernel == nil {
		return
	}

	// Assert coder_task fact
	_ = c.kernel.Assert(core.Fact{
		Predicate: "coder_task",
		Args:      []interface{}{c.id, "/" + task.Action, task.Target, task.Instruction},
	})

	// Assert language detection
	lang := detectLanguage(task.Target)
	if lang != "" {
		_ = c.kernel.Assert(core.Fact{
			Predicate: "detected_language",
			Args:      []interface{}{task.Target, "/" + lang},
		})
	}
}

// readFileContext reads the target file and extracts relevant context.
func (c *CoderShard) readFileContext(ctx context.Context, path string) (string, error) {
	if path == "" {
		return "", nil
	}

	// Resolve path
	fullPath := c.resolvePath(path)

	// Check if file exists
	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return "", nil // File doesn't exist, that's OK for create
	}
	if err != nil {
		return "", err
	}

	// Handle directories: list files instead of reading
	if info.IsDir() {
		return c.readDirectoryContext(fullPath)
	}

	// Skip large files (> 100KB)
	if info.Size() > 100*1024 {
		return fmt.Sprintf("// File too large (%d bytes), reading first 100KB\n", info.Size()), nil
	}

	// Read file content
	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", err
	}

	// If we have VirtualStore, use it and inject facts
	if c.virtualStore != nil {
		action := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"/read_file", fullPath},
		}
		_, _ = c.virtualStore.RouteAction(ctx, action)
	}

	// Inject file topology fact
	if c.kernel != nil {
		hash := hashContent(string(content))
		lang := detectLanguage(path)
		isTest := isTestFile(path)
		_ = c.kernel.Assert(core.Fact{
			Predicate: "file_topology",
			Args:      []interface{}{path, hash, "/" + lang, info.ModTime().Unix(), isTest},
		})
	}

	return string(content), nil
}

// readDirectoryContext reads a directory and returns an intelligent summary.
// Extracts package docs, exported types, and function signatures from ALL Go files.
// Uses a context budget to ensure we don't exceed reasonable limits.
func (c *CoderShard) readDirectoryContext(dirPath string) (string, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return "", err
	}

	const maxContextBytes = 32 * 1024 // 32KB total context budget

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// Directory: %s\n", dirPath))
	sb.WriteString(fmt.Sprintf("// Contains %d entries:\n\n", len(entries)))

	// Group by type and prioritize
	var dirs []string
	var mainFiles, regularFiles, testFiles []string
	var otherFiles []string

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			dirs = append(dirs, name+"/")
		} else if strings.HasSuffix(name, ".go") {
			if strings.HasSuffix(name, "_test.go") {
				testFiles = append(testFiles, name)
			} else if name == "main.go" || strings.Contains(name, "cmd") {
				mainFiles = append(mainFiles, name)
			} else {
				regularFiles = append(regularFiles, name)
			}
		} else {
			otherFiles = append(otherFiles, name)
		}
	}

	// List structure
	if len(dirs) > 0 {
		sb.WriteString("// Subdirectories:\n")
		for _, d := range dirs {
			sb.WriteString(fmt.Sprintf("//   %s\n", d))
		}
		sb.WriteString("\n")
	}

	allGoFiles := append(append(mainFiles, regularFiles...), testFiles...)
	if len(allGoFiles) > 0 {
		sb.WriteString(fmt.Sprintf("// Go files (%d total):\n", len(allGoFiles)))
		for _, f := range allGoFiles {
			sb.WriteString(fmt.Sprintf("//   %s\n", f))
		}
		sb.WriteString("\n")
	}

	if len(otherFiles) > 0 {
		sb.WriteString("// Other files:\n")
		for _, f := range otherFiles {
			sb.WriteString(fmt.Sprintf("//   %s\n", f))
		}
		sb.WriteString("\n")
	}

	// Read and extract from ALL Go files (prioritized order)
	// Main/cmd files first, then regular, then tests
	bytesUsed := sb.Len()
	for _, f := range allGoFiles {
		if bytesUsed >= maxContextBytes {
			sb.WriteString(fmt.Sprintf("\n// ... context budget exhausted, remaining files not shown\n"))
			break
		}

		filePath := filepath.Join(dirPath, f)
		content, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Extract intelligent summary based on file size
		contentStr := string(content)
		var summary string
		if len(contentStr) <= 4096 {
			// Small file: include full content
			summary = contentStr
		} else {
			// Large file: extract key parts
			summary = c.extractGoFileSummary(contentStr)
		}

		// Check if we have budget
		fileHeader := fmt.Sprintf("\n// === %s ===\n", f)
		if bytesUsed+len(fileHeader)+len(summary) > maxContextBytes {
			// Truncate to fit remaining budget
			remaining := maxContextBytes - bytesUsed - len(fileHeader) - 50
			if remaining > 500 {
				summary = summary[:remaining] + "\n// ... truncated to fit context budget\n"
			} else {
				continue
			}
		}

		sb.WriteString(fileHeader)
		sb.WriteString(summary)
		bytesUsed = sb.Len()
	}

	return sb.String(), nil
}

// extractGoFileSummary extracts the most important parts of a Go file:
// package doc, imports, type definitions, and function signatures.
func (c *CoderShard) extractGoFileSummary(content string) string {
	var sb strings.Builder
	lines := strings.Split(content, "\n")

	inDocComment := false
	inImportBlock := false
	inTypeBlock := false
	braceDepth := 0

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Package declaration and doc comments
		if strings.HasPrefix(trimmed, "// Package ") || strings.HasPrefix(trimmed, "/*") {
			inDocComment = true
		}
		if inDocComment {
			sb.WriteString(line + "\n")
			if strings.HasPrefix(trimmed, "package ") || strings.HasSuffix(trimmed, "*/") {
				inDocComment = false
			}
			continue
		}

		// Package line
		if strings.HasPrefix(trimmed, "package ") {
			sb.WriteString(line + "\n\n")
			continue
		}

		// Import block
		if strings.HasPrefix(trimmed, "import ") {
			inImportBlock = true
			sb.WriteString(line + "\n")
			if strings.Contains(line, ")") || !strings.Contains(line, "(") {
				inImportBlock = false
				sb.WriteString("\n")
			}
			continue
		}
		if inImportBlock {
			sb.WriteString(line + "\n")
			if strings.Contains(trimmed, ")") {
				inImportBlock = false
				sb.WriteString("\n")
			}
			continue
		}

		// Type definitions (struct, interface)
		if strings.HasPrefix(trimmed, "type ") {
			inTypeBlock = true
			braceDepth = 0
		}
		if inTypeBlock {
			sb.WriteString(line + "\n")
			braceDepth += strings.Count(line, "{") - strings.Count(line, "}")
			if braceDepth <= 0 && strings.Contains(line, "}") {
				inTypeBlock = false
				sb.WriteString("\n")
			}
			continue
		}

		// Function/method signatures (exported only)
		if strings.HasPrefix(trimmed, "func ") {
			// Check if exported (starts with uppercase after "func " or "func (receiver) ")
			funcPart := trimmed[5:]
			if strings.HasPrefix(funcPart, "(") {
				// Method - find the function name
				if idx := strings.Index(funcPart, ") "); idx != -1 {
					funcName := funcPart[idx+2:]
					if len(funcName) > 0 && funcName[0] >= 'A' && funcName[0] <= 'Z' {
						// Find end of signature
						sigEnd := strings.Index(line, "{")
						if sigEnd > 0 {
							sb.WriteString(line[:sigEnd] + "{ ... }\n")
						} else if i+1 < len(lines) && strings.Contains(lines[i+1], "{") {
							sb.WriteString(line + " { ... }\n")
						}
					}
				}
			} else if len(funcPart) > 0 && funcPart[0] >= 'A' && funcPart[0] <= 'Z' {
				// Exported function
				sigEnd := strings.Index(line, "{")
				if sigEnd > 0 {
					sb.WriteString(line[:sigEnd] + "{ ... }\n")
				} else if i+1 < len(lines) && strings.Contains(lines[i+1], "{") {
					sb.WriteString(line + " { ... }\n")
				}
			}
		}

		// Const/var blocks (exported only)
		if strings.HasPrefix(trimmed, "const ") || strings.HasPrefix(trimmed, "var ") {
			word := trimmed[strings.Index(trimmed, " ")+1:]
			if len(word) > 0 && word[0] >= 'A' && word[0] <= 'Z' {
				sb.WriteString(line + "\n")
			}
		}
	}

	return sb.String()
}

// checkImpact checks if editing the target would have unsafe impact.
func (c *CoderShard) checkImpact(target string) (blocked bool, reason string) {
	if c.kernel == nil {
		return false, ""
	}

	// Query for block conditions
	results, err := c.kernel.Query("coder_block_write")
	if err != nil {
		return false, ""
	}

	for _, fact := range results {
		if len(fact.Args) >= 2 {
			if file, ok := fact.Args[0].(string); ok && file == target {
				if r, ok := fact.Args[1].(string); ok {
					return true, r
				}
			}
		}
	}

	// Check Code DOM edit_unsafe predicate
	unsafeResults, _ := c.kernel.Query("edit_unsafe")
	for _, fact := range unsafeResults {
		if len(fact.Args) >= 2 {
			if ref, ok := fact.Args[0].(string); ok && strings.Contains(ref, target) {
				if reason, ok := fact.Args[1].(string); ok {
					return true, fmt.Sprintf("Code DOM safety: %s", reason)
				}
			}
		}
	}

	// Check for critical breaking change risk
	breakingResults, _ := c.kernel.Query("breaking_change_risk")
	for _, fact := range breakingResults {
		if len(fact.Args) >= 3 {
			if ref, ok := fact.Args[0].(string); ok && strings.Contains(ref, target) {
				if level, ok := fact.Args[1].(string); ok && level == "/critical" {
					if reason, ok := fact.Args[2].(string); ok {
						return true, fmt.Sprintf("Critical breaking change: %s", reason)
					}
				}
			}
		}
	}

	return false, ""
}

// generateCode uses LLM to generate code based on the task and context.
func (c *CoderShard) generateCode(ctx context.Context, task CoderTask, fileContext string) (*CoderResult, error) {
	if c.llmClient == nil {
		return nil, fmt.Errorf("no LLM client configured")
	}

	// Build system prompt
	systemPrompt := c.buildSystemPrompt(task)

	// Build user prompt
	userPrompt := c.buildUserPrompt(task, fileContext)

	// Call LLM
	response, err := c.llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM request failed: %w", err)
	}

	// Parse response into edits
	edits := c.parseCodeResponse(response, task)

	result := &CoderResult{
		Summary: fmt.Sprintf("%s: %s (%d edits)", task.Action, task.Target, len(edits)),
		Edits:   edits,
	}

	return result, nil
}

// buildSystemPrompt creates the system prompt for code generation.
func (c *CoderShard) buildSystemPrompt(task CoderTask) string {
	lang := detectLanguage(task.Target)
	langName := languageDisplayName(lang)

	// Build Code DOM context if we have safety information
	codeDOMContext := c.buildCodeDOMContext(task)

	return fmt.Sprintf(`You are an expert %s programmer. Generate clean, idiomatic, well-documented code.

RULES:
1. Follow language-specific best practices and idioms
2. Include appropriate error handling
3. Add concise comments for complex logic only
4. Do not include unnecessary imports or dependencies
5. Match the existing code style if modifying
%s
OUTPUT FORMAT:
Return your response as JSON with this structure:
{
  "file": "path/to/file",
  "content": "full file content here",
  "rationale": "brief explanation of changes"
}

For modifications, include the COMPLETE new file content, not a diff.
`, langName, codeDOMContext)
}

// buildCodeDOMContext builds Code DOM safety context for the prompt.
func (c *CoderShard) buildCodeDOMContext(task CoderTask) string {
	if c.kernel == nil {
		return ""
	}

	var warnings []string

	// Check if file is generated code
	generatedResults, _ := c.kernel.Query("generated_code")
	for _, fact := range generatedResults {
		if len(fact.Args) >= 2 {
			if file, ok := fact.Args[0].(string); ok && file == task.Target {
				if generator, ok := fact.Args[1].(string); ok {
					warnings = append(warnings, fmt.Sprintf("WARNING: This is generated code (%s). Changes will be overwritten on regeneration.", generator))
				}
			}
		}
	}

	// Check for breaking change risk
	breakingResults, _ := c.kernel.Query("breaking_change_risk")
	for _, fact := range breakingResults {
		if len(fact.Args) >= 3 {
			if ref, ok := fact.Args[0].(string); ok && strings.Contains(ref, task.Target) {
				if level, ok := fact.Args[1].(string); ok {
					if reason, ok := fact.Args[2].(string); ok {
						warnings = append(warnings, fmt.Sprintf("BREAKING CHANGE RISK (%s): %s", level, reason))
					}
				}
			}
		}
	}

	// Check for API client/handler functions
	apiClientResults, _ := c.kernel.Query("api_client_function")
	for _, fact := range apiClientResults {
		if len(fact.Args) >= 2 {
			if file, ok := fact.Args[1].(string); ok && file == task.Target {
				warnings = append(warnings, "NOTE: This file contains API client code. Ensure error handling for network failures.")
			}
			break // Only add once
		}
	}

	apiHandlerResults, _ := c.kernel.Query("api_handler_function")
	for _, fact := range apiHandlerResults {
		if len(fact.Args) >= 2 {
			if file, ok := fact.Args[1].(string); ok && file == task.Target {
				warnings = append(warnings, "NOTE: This file contains API handlers. Validate inputs and handle errors appropriately.")
			}
			break
		}
	}

	// Check for CGo code
	cgoResults, _ := c.kernel.Query("cgo_code")
	for _, fact := range cgoResults {
		if len(fact.Args) >= 1 {
			if file, ok := fact.Args[0].(string); ok && file == task.Target {
				warnings = append(warnings, "WARNING: This file contains CGo code. Be careful with memory management and type conversions.")
			}
		}
	}

	if len(warnings) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\nCODE CONTEXT:\n")
	for _, w := range warnings {
		sb.WriteString(fmt.Sprintf("- %s\n", w))
	}
	sb.WriteString("\n")
	return sb.String()
}

// buildUserPrompt creates the user prompt with task and context.
func (c *CoderShard) buildUserPrompt(task CoderTask, fileContext string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Task: %s\n", task.Action))
	sb.WriteString(fmt.Sprintf("Target: %s\n", task.Target))
	sb.WriteString(fmt.Sprintf("Instruction: %s\n", task.Instruction))

	if fileContext != "" {
		sb.WriteString("\nExisting file content:\n```\n")
		sb.WriteString(fileContext)
		sb.WriteString("\n```\n")
	}

	// Add any learned preferences
	if len(c.rejectionCount) > 0 {
		sb.WriteString("\nAvoid these patterns (previously rejected):\n")
		for pattern, count := range c.rejectionCount {
			if count >= 2 {
				sb.WriteString(fmt.Sprintf("- %s\n", pattern))
			}
		}
	}

	return sb.String()
}

// parseCodeResponse extracts code edits from LLM response.
func (c *CoderShard) parseCodeResponse(response string, task CoderTask) []CodeEdit {
	edits := make([]CodeEdit, 0)

	// Try to parse as JSON first
	var jsonResp struct {
		File      string `json:"file"`
		Content   string `json:"content"`
		Rationale string `json:"rationale"`
	}

	// Find JSON in response (may be wrapped in markdown code blocks)
	jsonStr := response
	if idx := strings.Index(response, "{"); idx != -1 {
		endIdx := strings.LastIndex(response, "}")
		if endIdx > idx {
			jsonStr = response[idx : endIdx+1]
		}
	}

	if err := json.Unmarshal([]byte(jsonStr), &jsonResp); err == nil && jsonResp.Content != "" {
		edit := CodeEdit{
			File:       jsonResp.File,
			NewContent: jsonResp.Content,
			Type:       task.Action,
			Language:   detectLanguage(jsonResp.File),
			Rationale:  jsonResp.Rationale,
		}
		if edit.File == "" {
			edit.File = task.Target
		}
		edits = append(edits, edit)
		return edits
	}

	// Fallback: extract code from markdown code blocks
	codeBlockRegex := regexp.MustCompile("```(?:\\w+)?\\n([\\s\\S]*?)```")
	matches := codeBlockRegex.FindAllStringSubmatch(response, -1)
	if len(matches) > 0 {
		// Use the last code block (often the final answer)
		content := matches[len(matches)-1][1]
		edit := CodeEdit{
			File:       task.Target,
			NewContent: strings.TrimSpace(content),
			Type:       task.Action,
			Language:   detectLanguage(task.Target),
			Rationale:  "Generated from LLM response",
		}
		edits = append(edits, edit)
		return edits
	}

	// Last resort: use raw response (for simple cases)
	if len(response) > 0 && strings.Contains(response, "\n") {
		edit := CodeEdit{
			File:       task.Target,
			NewContent: response,
			Type:       task.Action,
			Language:   detectLanguage(task.Target),
			Rationale:  "Raw LLM response",
		}
		edits = append(edits, edit)
	}

	return edits
}

// applyEdits writes the code changes via VirtualStore.
func (c *CoderShard) applyEdits(ctx context.Context, edits []CodeEdit) error {
	for _, edit := range edits {
		c.mu.Lock()
		c.editHistory = append(c.editHistory, edit)
		c.mu.Unlock()

		if c.virtualStore == nil {
			// No virtual store, write directly
			fullPath := c.resolvePath(edit.File)
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			if err := os.WriteFile(fullPath, []byte(edit.NewContent), 0644); err != nil {
				return fmt.Errorf("failed to write file: %w", err)
			}
			continue
		}

		// Use VirtualStore for proper action routing
		var action core.Fact
		switch edit.Type {
		case "create":
			action = core.Fact{
				Predicate: "next_action",
				Args: []interface{}{
					"/write_file",
					edit.File,
					map[string]interface{}{"content": edit.NewContent},
				},
			}
		case "modify", "refactor", "fix":
			action = core.Fact{
				Predicate: "next_action",
				Args: []interface{}{
					"/write_file",
					edit.File,
					map[string]interface{}{"content": edit.NewContent},
				},
			}
		case "delete":
			action = core.Fact{
				Predicate: "next_action",
				Args: []interface{}{
					"/delete_file",
					edit.File,
					map[string]interface{}{"confirmed": true},
				},
			}
		default:
			action = core.Fact{
				Predicate: "next_action",
				Args: []interface{}{
					"/write_file",
					edit.File,
					map[string]interface{}{"content": edit.NewContent},
				},
			}
		}

		_, err := c.virtualStore.RouteAction(ctx, action)
		if err != nil {
			return fmt.Errorf("failed to apply edit to %s: %w", edit.File, err)
		}

		// Inject modified fact
		if c.kernel != nil {
			_ = c.kernel.Assert(core.Fact{
				Predicate: "modified",
				Args:      []interface{}{edit.File},
			})
		}
	}

	return nil
}

// runBuildCheck executes a build command to verify the changes.
func (c *CoderShard) runBuildCheck(ctx context.Context) bool {
	if c.virtualStore == nil {
		return true // Assume success if no virtual store
	}

	// Detect build command from project type
	buildCmd := c.detectBuildCommand()
	if buildCmd == "" {
		return true // No build command, assume success
	}

	action := core.Fact{
		Predicate: "next_action",
		Args: []interface{}{
			"/build_project",
			buildCmd,
		},
	}

	output, err := c.virtualStore.RouteAction(ctx, action)
	if err != nil {
		// Parse diagnostics from output
		c.mu.Lock()
		c.diagnostics = c.parseBuildOutput(output)
		c.mu.Unlock()
		return false
	}

	return true
}

// detectBuildCommand returns the appropriate build command for the project.
func (c *CoderShard) detectBuildCommand() string {
	workDir := c.coderConfig.WorkingDir

	// Check for Go
	if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err == nil {
		return "go build ./..."
	}

	// Check for Node.js
	if _, err := os.Stat(filepath.Join(workDir, "package.json")); err == nil {
		return "npm run build"
	}

	// Check for Rust
	if _, err := os.Stat(filepath.Join(workDir, "Cargo.toml")); err == nil {
		return "cargo build"
	}

	// Check for Python
	if _, err := os.Stat(filepath.Join(workDir, "pyproject.toml")); err == nil {
		return "python -m py_compile"
	}

	return ""
}

// parseBuildOutput extracts diagnostics from build output.
func (c *CoderShard) parseBuildOutput(output string) []core.Diagnostic {
	diagnostics := make([]core.Diagnostic, 0)
	lines := strings.Split(output, "\n")

	// Go-style: file.go:line:col: message
	goErrorRegex := regexp.MustCompile(`^(.+\.go):(\d+):(\d+): (.+)$`)

	for _, line := range lines {
		if matches := goErrorRegex.FindStringSubmatch(line); len(matches) > 4 {
			lineNum := 0
			colNum := 0
			fmt.Sscanf(matches[2], "%d", &lineNum)
			fmt.Sscanf(matches[3], "%d", &colNum)
			diagnostics = append(diagnostics, core.Diagnostic{
				Severity: "error",
				FilePath: matches[1],
				Line:     lineNum,
				Column:   colNum,
				Message:  matches[4],
			})
		}
	}

	return diagnostics
}

// generateFacts creates Mangle facts from the result for propagation.
func (c *CoderShard) generateFacts(result *CoderResult) []core.Fact {
	facts := make([]core.Fact, 0)

	// File modifications
	for _, edit := range result.Edits {
		facts = append(facts, core.Fact{
			Predicate: "modified",
			Args:      []interface{}{edit.File},
		})

		// File topology
		hash := hashContent(edit.NewContent)
		facts = append(facts, core.Fact{
			Predicate: "file_topology",
			Args: []interface{}{
				edit.File,
				hash,
				"/" + edit.Language,
				time.Now().Unix(),
				isTestFile(edit.File),
			},
		})
	}

	// Build state
	if result.BuildPassed {
		facts = append(facts, core.Fact{
			Predicate: "build_state",
			Args:      []interface{}{"/passing"},
		})
	} else {
		facts = append(facts, core.Fact{
			Predicate: "build_state",
			Args:      []interface{}{"/failing"},
		})
	}

	// Diagnostics
	for _, diag := range result.Diagnostics {
		facts = append(facts, diag.ToFact())
	}

	// Autopoiesis: check for patterns to promote
	c.mu.RLock()
	for pattern, count := range c.rejectionCount {
		if count >= 2 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"style_preference", pattern},
			})
		}
	}
	for pattern, count := range c.acceptanceCount {
		if count >= 3 {
			facts = append(facts, core.Fact{
				Predicate: "promote_to_long_term",
				Args:      []interface{}{"preferred_pattern", pattern},
			})
		}
	}
	c.mu.RUnlock()

	return facts
}

// buildResponse creates a human-readable response from the result.
func (c *CoderShard) buildResponse(result *CoderResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Coding task completed in %v.\n", result.Duration.Round(time.Millisecond)))
	sb.WriteString(fmt.Sprintf("Summary: %s\n", result.Summary))

	if len(result.Edits) > 0 {
		sb.WriteString(fmt.Sprintf("Applied %d edit(s):\n", len(result.Edits)))
		for _, edit := range result.Edits {
			sb.WriteString(fmt.Sprintf("  - %s: %s (%s)\n", edit.Type, edit.File, edit.Language))
		}
	}

	if result.BuildPassed {
		sb.WriteString("Build: PASSED\n")
	} else if len(result.Diagnostics) > 0 {
		sb.WriteString(fmt.Sprintf("Build: FAILED (%d errors)\n", len(result.Diagnostics)))
		for i, diag := range result.Diagnostics {
			if i >= 3 {
				sb.WriteString(fmt.Sprintf("  ... and %d more\n", len(result.Diagnostics)-3))
				break
			}
			sb.WriteString(fmt.Sprintf("  - %s:%d: %s\n", diag.FilePath, diag.Line, diag.Message))
		}
	}

	return sb.String()
}

// trackRejection tracks a rejection pattern for autopoiesis.
func (c *CoderShard) trackRejection(action, reason string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	key := fmt.Sprintf("%s:%s", action, reason)
	c.rejectionCount[key]++
}

// trackAcceptance tracks an acceptance pattern for autopoiesis.
func (c *CoderShard) trackAcceptance(action string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.acceptanceCount[action]++
}

// resolvePath resolves a relative path to absolute.
func (c *CoderShard) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(c.coderConfig.WorkingDir, path)
}

// Helper functions

// hashContent returns SHA256 hash of content.
func hashContent(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:8]) // First 8 bytes for brevity
}

// detectLanguage detects programming language from file extension.
func detectLanguage(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return "go"
	case ".py":
		return "python"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescript"
	case ".js":
		return "javascript"
	case ".jsx":
		return "javascript"
	case ".rs":
		return "rust"
	case ".java":
		return "java"
	case ".kt":
		return "kotlin"
	case ".swift":
		return "swift"
	case ".rb":
		return "ruby"
	case ".php":
		return "php"
	case ".cs":
		return "csharp"
	case ".cpp", ".cc", ".cxx":
		return "cpp"
	case ".c":
		return "c"
	case ".h", ".hpp":
		return "c"
	case ".sql":
		return "sql"
	case ".sh", ".bash":
		return "bash"
	case ".yaml", ".yml":
		return "yaml"
	case ".json":
		return "json"
	case ".md":
		return "markdown"
	case ".html":
		return "html"
	case ".css":
		return "css"
	case ".scss", ".sass":
		return "scss"
	default:
		return "unknown"
	}
}

// languageDisplayName returns a display name for a language.
func languageDisplayName(lang string) string {
	names := map[string]string{
		"go":         "Go",
		"python":     "Python",
		"typescript": "TypeScript",
		"javascript": "JavaScript",
		"rust":       "Rust",
		"java":       "Java",
		"kotlin":     "Kotlin",
		"swift":      "Swift",
		"ruby":       "Ruby",
		"php":        "PHP",
		"csharp":     "C#",
		"cpp":        "C++",
		"c":          "C",
		"sql":        "SQL",
		"bash":       "Bash",
		"yaml":       "YAML",
		"json":       "JSON",
		"markdown":   "Markdown",
		"html":       "HTML",
		"css":        "CSS",
		"scss":       "SCSS",
	}
	if name, ok := names[lang]; ok {
		return name
	}
	return "code"
}

// isTestFile determines if a file is a test file.
func isTestFile(path string) bool {
	base := filepath.Base(path)
	lowerBase := strings.ToLower(base)

	// Go tests
	if strings.HasSuffix(lowerBase, "_test.go") {
		return true
	}

	// JavaScript/TypeScript tests
	if strings.Contains(lowerBase, ".test.") || strings.Contains(lowerBase, ".spec.") {
		return true
	}

	// Python tests
	if strings.HasPrefix(lowerBase, "test_") || strings.HasSuffix(lowerBase, "_test.py") {
		return true
	}

	// Test directories
	dir := filepath.Dir(path)
	if strings.Contains(dir, "/tests/") || strings.Contains(dir, "/test/") ||
		strings.Contains(dir, "/__tests__/") {
		return true
	}

	return false
}
