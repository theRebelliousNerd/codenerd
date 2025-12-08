// Package init implements the "nerd init" cold-start initialization system.
package init

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// detectLanguageFromFiles detects the primary language by looking for config files.
func (i *Initializer) detectLanguageFromFiles() string {
	workspace := i.config.Workspace

	// Check for language-specific config files
	checks := []struct {
		file     string
		language string
	}{
		{"go.mod", "go"},
		{"Cargo.toml", "rust"},
		{"package.json", "typescript"}, // Could be JS, but TS is more common now
		{"pyproject.toml", "python"},
		{"requirements.txt", "python"},
		{"setup.py", "python"},
		{"pom.xml", "java"},
		{"build.gradle", "java"},
		{"*.csproj", "csharp"},
		{"mix.exs", "elixir"},
		{"Gemfile", "ruby"},
	}

	for _, check := range checks {
		pattern := filepath.Join(workspace, check.file)
		matches, err := filepath.Glob(pattern)
		if err == nil && len(matches) > 0 {
			return check.language
		}
	}

	return "unknown"
}

// detectDependencies scans project files for key dependencies.
func (i *Initializer) detectDependencies() []DependencyInfo {
	deps := []DependencyInfo{}
	workspace := i.config.Workspace

	// Check go.mod for Go dependencies
	goModPath := filepath.Join(workspace, "go.mod")
	if data, err := os.ReadFile(goModPath); err == nil {
		content := string(data)

		// Key Go dependencies to detect
		goDeps := map[string]string{
			"github.com/go-rod/rod":              "rod",
			"github.com/chromedp/chromedp":       "chromedp",
			"github.com/playwright-community":    "playwright",
			"google/mangle":                      "mangle",
			"github.com/sashabaranov/go-openai":  "openai",
			"github.com/anthropics/anthropic":    "anthropic",
			"github.com/charmbracelet/bubbletea": "bubbletea",
			"github.com/spf13/cobra":             "cobra",
			"github.com/gin-gonic/gin":           "gin",
			"github.com/labstack/echo":           "echo",
			"github.com/gofiber/fiber":           "fiber",
			"gorm.io/gorm":                       "gorm",
			"github.com/jmoiron/sqlx":            "sqlx",
			"database/sql":                       "sql",
			"github.com/gorilla/mux":             "gorilla",
			"net/http":                           "http",
		}

		for pkg, name := range goDeps {
			if strings.Contains(content, pkg) {
				deps = append(deps, DependencyInfo{
					Name: name,
					Type: "direct",
				})
			}
		}
	}

	// Check package.json for Node dependencies
	pkgPath := filepath.Join(workspace, "package.json")
	if data, err := os.ReadFile(pkgPath); err == nil {
		content := string(data)

		nodeDeps := map[string]string{
			"\"puppeteer\"":  "puppeteer",
			"\"playwright\"": "playwright",
			"\"openai\"":     "openai",
			"\"@anthropic\"": "anthropic",
			"\"react\"":      "react",
			"\"vue\"":        "vue",
			"\"next\"":       "nextjs",
			"\"express\"":    "express",
			"\"fastify\"":    "fastify",
			"\"prisma\"":     "prisma",
			"\"typeorm\"":    "typeorm",
		}

		for pkg, name := range nodeDeps {
			if strings.Contains(content, pkg) {
				deps = append(deps, DependencyInfo{
					Name: name,
					Type: "direct",
				})
			}
		}
	}

	return deps
}

// createDirectoryStructure creates the .nerd/ directory and subdirectories.
func (i *Initializer) createDirectoryStructure() (string, error) {
	nerdDir := filepath.Join(i.config.Workspace, ".nerd")
	toolsDir := filepath.Join(nerdDir, "tools")

	dirs := []string{
		nerdDir,
		filepath.Join(nerdDir, "shards"),      // Knowledge shards for specialists
		filepath.Join(nerdDir, "sessions"),    // Session history
		filepath.Join(nerdDir, "cache"),       // Temporary cache
		filepath.Join(nerdDir, "mangle"),      // Mangle logic overlay (User Extensions)
		toolsDir,                              // Autopoiesis generated tools
		filepath.Join(toolsDir, ".compiled"),  // Compiled tool binaries
		filepath.Join(toolsDir, ".learnings"), // Tool execution learnings
		filepath.Join(toolsDir, ".profiles"),  // Tool quality profiles
		filepath.Join(toolsDir, ".traces"),    // Reasoning traces for tool generation
		filepath.Join(nerdDir, "agents"),      // Persistent agent definitions
		filepath.Join(nerdDir, "campaigns"),   // Campaign checkpoints
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return "", fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}

	// Create .gitignore for .nerd/
	gitignorePath := filepath.Join(nerdDir, ".gitignore")
	gitignoreContent := `# codeNERD local files
knowledge.db
knowledge.db-journal
sessions/
cache/
*.log

# Autopoiesis internal directories (always ignore)
tools/.compiled/
tools/.learnings/
tools/.profiles/
tools/.traces/

# Keep tools/ source and agents/ tracked (user may want to commit generated tools)
# Uncomment below to ignore tool source code:
# tools/*.go
# agents/
`
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return "", fmt.Errorf("failed to create .gitignore: %w", err)
	}

	return nerdDir, nil
}
