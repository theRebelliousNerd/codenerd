package coder

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// =============================================================================
// BUILD CHECKING
// =============================================================================

// runBuildCheck executes a build command to verify the changes.
func (c *CoderShard) runBuildCheck(ctx context.Context) bool {
	if c.virtualStore == nil {
		logging.CoderDebug("runBuildCheck: no virtual store, skipping build check")
		return true // Assume success if no virtual store
	}

	// Detect build command from project type
	buildCmd := c.detectBuildCommand()
	if buildCmd == "" {
		logging.CoderDebug("runBuildCheck: no build command detected, skipping")
		return true // No build command, assume success
	}

	logging.Coder("runBuildCheck: executing build command=%s", buildCmd)

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
		diagCount := len(c.diagnostics)
		c.mu.Unlock()
		logging.CoderWarn("runBuildCheck: build failed with %d diagnostics: %v", diagCount, err)
		return false
	}

	logging.Coder("runBuildCheck: build succeeded")
	return true
}

// detectBuildCommand returns the appropriate build command for the project.
func (c *CoderShard) detectBuildCommand() string {
	workDir := c.coderConfig.WorkingDir

	// Check for Go
	if _, err := os.Stat(filepath.Join(workDir, "go.mod")); err == nil {
		logging.CoderDebug("detectBuildCommand: detected Go project")
		return "go build ./..."
	}

	// Check for Node.js
	if _, err := os.Stat(filepath.Join(workDir, "package.json")); err == nil {
		logging.CoderDebug("detectBuildCommand: detected Node.js project")
		return "npm run build"
	}

	// Check for Rust
	if _, err := os.Stat(filepath.Join(workDir, "Cargo.toml")); err == nil {
		logging.CoderDebug("detectBuildCommand: detected Rust project")
		return "cargo build"
	}

	// Check for Python
	if _, err := os.Stat(filepath.Join(workDir, "pyproject.toml")); err == nil {
		logging.CoderDebug("detectBuildCommand: detected Python project")
		return "python -m py_compile"
	}

	logging.CoderDebug("detectBuildCommand: no known project type detected in %s", workDir)
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

	logging.CoderDebug("parseBuildOutput: extracted %d diagnostics from output (%d lines)", len(diagnostics), len(lines))
	return diagnostics
}
