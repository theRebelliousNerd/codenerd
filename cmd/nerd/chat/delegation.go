// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains shard spawning and task delegation helpers.
package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"codenerd/cmd/nerd/ui"
	"codenerd/internal/perception"

	tea "github.com/charmbracelet/bubbletea"
)

// =============================================================================
// SHARD DELEGATION
// =============================================================================
// Functions for formatting tasks, spawning shards, and handling delegation
// from natural language to specialized agents.

func formatShardTask(verb, target, constraint, workspace string) string {
	// Normalize target
	if target == "" || target == "none" {
		target = "codebase"
	}

	// Handle file paths - make them relative to workspace if needed
	if strings.HasPrefix(target, workspace) {
		if rel, err := filepath.Rel(workspace, target); err == nil {
			target = rel
		}
	}

	// Discover files if target is broad (codebase, all files, etc.)
	var fileList string
	if target == "codebase" || strings.Contains(strings.ToLower(target), "all") || strings.Contains(target, "*") {
		files := discoverFiles(workspace, constraint)
		if len(files) > 0 {
			fileList = strings.Join(files, ",")
		}
	}

	switch verb {
	case "/review":
		if fileList != "" {
			return fmt.Sprintf("review files:%s", fileList)
		}
		if target == "codebase" {
			return "review all"
		}
		return fmt.Sprintf("review file:%s", target)

	case "/security":
		if fileList != "" {
			return fmt.Sprintf("security_scan files:%s", fileList)
		}
		if target == "codebase" {
			return "security_scan all"
		}
		return fmt.Sprintf("security_scan file:%s", target)

	case "/analyze":
		if fileList != "" {
			return fmt.Sprintf("complexity files:%s", fileList)
		}
		if target == "codebase" {
			return "complexity all"
		}
		return fmt.Sprintf("complexity file:%s", target)

	case "/fix":
		return fmt.Sprintf("fix issue in %s", target)

	case "/refactor":
		return fmt.Sprintf("refactor %s", target)

	case "/create":
		return fmt.Sprintf("create %s", target)

	case "/test":
		if strings.Contains(target, "run") || target == "codebase" {
			return "run_tests"
		}
		return fmt.Sprintf("write_tests for %s", target)

	case "/debug":
		return fmt.Sprintf("debug %s", target)

	case "/research":
		return fmt.Sprintf("research %s", target)

	case "/explore":
		return fmt.Sprintf("explore %s", target)

	case "/document":
		return fmt.Sprintf("document %s", target)

	case "/diff":
		return fmt.Sprintf("review diff:%s", target)

	default:
		// Generic task format
		if constraint != "none" && constraint != "" {
			return fmt.Sprintf("%s %s with constraint: %s", verb, target, constraint)
		}
		return fmt.Sprintf("%s %s", verb, target)
	}
}

// formatDelegatedResponse creates a user-friendly response from shard execution.
func formatDelegatedResponse(intent perception.Intent, shardType, task, result string) string {
	// Build header based on verb
	var header string
	switch intent.Verb {
	case "/review":
		header = "## Code Review Results"
	case "/security":
		header = "## Security Analysis Results"
	case "/analyze":
		header = "## Code Analysis Results"
	case "/fix":
		header = "## Fix Applied"
	case "/refactor":
		header = "## Refactoring Complete"
	case "/test":
		header = "## Test Results"
	case "/debug":
		header = "## Debug Analysis"
	case "/research":
		header = "## Research Findings"
	default:
		header = fmt.Sprintf("## %s Results", strings.Title(strings.TrimPrefix(intent.Verb, "/")))
	}

	// Include the LLM's surface response if meaningful
	surfaceNote := ""
	if intent.Response != "" && len(intent.Response) < 500 {
		surfaceNote = fmt.Sprintf("\n\n> %s\n", intent.Response)
	}

	return fmt.Sprintf(`%s
%s
**Target**: %s
**Agent**: %s
**Task**: %s

### Output
%s`, header, surfaceNote, intent.Target, shardType, task, result)
}

// spawnShard spawns a shard agent for a task
func (m Model) spawnShard(shardType, task string) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		result, err := m.shardMgr.Spawn(ctx, shardType, task)

		// Generate a shard ID for fact tracking
		shardID := fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano())

		// CRITICAL FIX: Convert shard result to facts and inject into kernel
		// This is the missing bridge that enables cross-turn context propagation
		facts := m.shardMgr.ResultToFacts(shardID, shardType, task, result, err)
		if m.kernel != nil && len(facts) > 0 {
			if loadErr := m.kernel.LoadFacts(facts); loadErr != nil {
				// Log but don't fail - the response should still be shown
				fmt.Printf("[ShardFacts] Warning: failed to inject facts: %v\n", loadErr)
			}
		}

		if err != nil {
			return errorMsg(fmt.Errorf("shard spawn failed: %w", err))
		}

		response := fmt.Sprintf(`## Shard Execution Complete

**Agent**: %s
**Task**: %s

### Result
%s`, shardType, task, result)

		return responseMsg(response)
	}
}

// createDirIfNotExists creates a directory if it doesn't exist
func createDirIfNotExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return os.MkdirAll(path, 0755)
	}
	return nil
}

// ProjectTypeInfo holds detected project characteristics
type ProjectTypeInfo struct {
	Language     string
	Framework    string
	Architecture string
}

// detectProjectType analyzes the workspace to determine project type
func detectProjectType(workspace string) ProjectTypeInfo {
	// Get UI styles for consistent formatting
	styles := getUIStyles()
	_ = styles // Ensure styles are available for future enhancements

	pt := ProjectTypeInfo{
		Language:     "unknown",
		Framework:    "unknown",
		Architecture: "unknown",
	}

	// Check for language markers
	markers := map[string]struct {
		lang  string
		build string
	}{
		"go.mod":           {"go", "go"},
		"Cargo.toml":       {"rust", "cargo"},
		"package.json":     {"javascript", "npm"},
		"requirements.txt": {"python", "pip"},
		"pom.xml":          {"java", "maven"},
	}

	for file, info := range markers {
		if _, err := os.Stat(workspace + "/" + file); err == nil {
			pt.Language = info.lang
			break
		}
	}

	// Detect architecture based on directory structure
	dirs := []string{"cmd", "internal", "pkg", "api", "services"}
	foundDirs := 0
	for _, dir := range dirs {
		if info, err := os.Stat(workspace + "/" + dir); err == nil && info.IsDir() {
			foundDirs++
		}
	}

	if foundDirs >= 3 {
		pt.Architecture = "clean_architecture"
	} else if _, err := os.Stat(workspace + "/docker-compose.yml"); err == nil {
		pt.Architecture = "microservices"
	} else {
		pt.Architecture = "monolith"
	}

	return pt
}

func getUIStyles() ui.Styles {
	return ui.DefaultStyles()
}

// =============================================================================
// MULTI-STEP TASK HANDLING
// =============================================================================

// TaskStep represents a single step in a multi-step task
type TaskStep struct {
	Verb       string
	Target     string
	ShardType  string
	Task       string
	DependsOn  []int // Indices of steps that must complete first
}

// detectMultiStepTask checks if input requires multiple steps
func detectMultiStepTask(input string, intent perception.Intent) bool {
	lower := strings.ToLower(input)

	// Multi-step indicators
	multiStepKeywords := []string{
		"and then", "after that", "next", "then",
		"first", "second", "third", "finally",
		"step 1", "step 2", "1.", "2.", "3.",
		"also", "additionally", "furthermore",
	}

	for _, keyword := range multiStepKeywords {
		if strings.Contains(lower, keyword) {
			return true
		}
	}

	// Check for multiple verbs in the input
	verbCount := 0
	for _, entry := range perception.VerbCorpus {
		for _, synonym := range entry.Synonyms {
			if strings.Contains(lower, synonym) {
				verbCount++
				if verbCount >= 2 {
					return true
				}
			}
		}
	}

	// Check for compound tasks (review + test, fix + test, etc.)
	compoundPatterns := []string{
		"review.*test", "fix.*test", "refactor.*test",
		"create.*test", "implement.*test",
	}

	for _, pattern := range compoundPatterns {
		if matched, _ := regexp.MatchString(pattern, lower); matched {
			return true
		}
	}

	return false
}

// decomposeTask breaks a complex task into discrete steps
func decomposeTask(input string, intent perception.Intent, workspace string) []TaskStep {
	var steps []TaskStep

	lower := strings.ToLower(input)

	// Pattern 1: "fix X and test it" or "create X and test"
	if strings.Contains(lower, "test") && (intent.Verb == "/fix" || intent.Verb == "/create" || intent.Verb == "/refactor") {
		// Step 1: Primary action
		step1 := TaskStep{
			Verb:      intent.Verb,
			Target:    intent.Target,
			ShardType: perception.GetShardTypeForVerb(intent.Verb),
		}
		step1.Task = formatShardTask(step1.Verb, step1.Target, intent.Constraint, workspace)
		steps = append(steps, step1)

		// Step 2: Testing
		step2 := TaskStep{
			Verb:      "/test",
			Target:    intent.Target,
			ShardType: "tester",
			DependsOn: []int{0}, // Depends on step 1
		}
		step2.Task = formatShardTask(step2.Verb, step2.Target, "none", workspace)
		steps = append(steps, step2)

		return steps
	}

	// Pattern 2: "review codebase" or "review all files" - already handled by multi-file discovery
	// Single step with multiple files
	if intent.Verb == "/review" || intent.Verb == "/security" || intent.Verb == "/analyze" {
		step := TaskStep{
			Verb:      intent.Verb,
			Target:    intent.Target,
			ShardType: perception.GetShardTypeForVerb(intent.Verb),
		}
		step.Task = formatShardTask(step.Verb, step.Target, intent.Constraint, workspace)
		steps = append(steps, step)
		return steps
	}

	// Pattern 3: Explicit step markers ("first X, then Y")
	// This is complex - for now, return single step
	// Future: parse explicit step sequences

	// Default: single step
	if len(steps) == 0 {
		step := TaskStep{
			Verb:      intent.Verb,
			Target:    intent.Target,
			ShardType: perception.GetShardTypeForVerb(intent.Verb),
		}
		step.Task = formatShardTask(step.Verb, step.Target, intent.Constraint, workspace)
		steps = append(steps, step)
	}

	return steps
}

// discoverFiles finds files in the workspace based on constraint filters
func discoverFiles(workspace, constraint string) []string {
	var files []string

	// Determine file patterns based on constraint
	var extensions []string
	constraintLower := strings.ToLower(constraint)

	switch {
	case strings.Contains(constraintLower, "go"):
		extensions = []string{".go"}
	case strings.Contains(constraintLower, "python") || strings.Contains(constraintLower, "py"):
		extensions = []string{".py"}
	case strings.Contains(constraintLower, "javascript") || strings.Contains(constraintLower, "js"):
		extensions = []string{".js", ".jsx", ".ts", ".tsx"}
	case strings.Contains(constraintLower, "rust"):
		extensions = []string{".rs"}
	case strings.Contains(constraintLower, "java"):
		extensions = []string{".java"}
	default:
		// Default: all common code file extensions
		extensions = []string{".go", ".py", ".js", ".jsx", ".ts", ".tsx", ".rs", ".java", ".c", ".cpp", ".h"}
	}

	// Walk workspace and collect matching files
	filepath.Walk(workspace, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}

		// Skip hidden directories and files
		if strings.Contains(path, "/.") || strings.Contains(path, "\\.") {
			return nil
		}

		// Skip vendor, node_modules, etc.
		skipDirs := []string{"vendor", "node_modules", ".git", ".nerd", "dist", "build"}
		for _, skip := range skipDirs {
			if strings.Contains(path, string(filepath.Separator)+skip+string(filepath.Separator)) {
				return nil
			}
		}

		// Check if file matches extension filter
		ext := filepath.Ext(path)
		for _, allowedExt := range extensions {
			if ext == allowedExt {
				// Convert to relative path
				if relPath, err := filepath.Rel(workspace, path); err == nil {
					files = append(files, relPath)
				}
				break
			}
		}

		return nil
	})

	// Limit to 50 files for safety (avoid overwhelming the shard)
	if len(files) > 50 {
		files = files[:50]
	}

	return files
}
