// Package chat provides the interactive TUI chat interface for codeNERD.
// This file contains shard spawning and task delegation helpers.
package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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

	switch verb {
	case "/review":
		if target == "codebase" {
			return "review all"
		}
		return fmt.Sprintf("review file:%s", target)

	case "/security":
		if target == "codebase" {
			return "security_scan all"
		}
		return fmt.Sprintf("security_scan file:%s", target)

	case "/analyze":
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
