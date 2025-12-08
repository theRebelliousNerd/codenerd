// Package init implements the "nerd init" cold-start initialization system.
package init

import (
	"codenerd/internal/config"
	"encoding/json"
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

	// Create config.json with defaults if it doesn't exist
	configPath := filepath.Join(nerdDir, "config.json")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if err := i.createDefaultConfig(configPath); err != nil {
			return "", fmt.Errorf("failed to create config.json: %w", err)
		}
	}

	return nerdDir, nil
}

// createDefaultConfig creates a config.json with sensible defaults.
func (i *Initializer) createDefaultConfig(path string) error {
	cfg := &config.UserConfig{
		Provider: "zai",
		Model:    "glm-4.6",
		Theme:    "light",
		ContextWindow: &config.ContextWindowConfig{
			MaxTokens:              128000,
			CoreReservePercent:     5,
			AtomReservePercent:     30,
			HistoryReservePercent:  15,
			WorkingReservePercent:  50,
			RecentTurnWindow:       5,
			CompressionThreshold:   0.80,
			TargetCompressionRatio: 100.0,
			ActivationThreshold:    30.0,
		},
		Embedding: &config.EmbeddingConfig{
			Provider:       "ollama",
			OllamaEndpoint: "http://localhost:11434",
			OllamaModel:    "embeddinggemma",
			GenAIModel:     "gemini-embedding-001",
			TaskType:       "SEMANTIC_SIMILARITY",
		},
		ShardProfiles: map[string]config.ShardProfile{
			"coder": {
				Model:                 "glm-4.6",
				Temperature:           0.7,
				TopP:                  0.9,
				MaxContextTokens:      30000,
				MaxOutputTokens:       6000,
				MaxExecutionTimeSec:   600,
				MaxRetries:            3,
				MaxFactsInShardKernel: 30000,
				EnableLearning:        true,
			},
			"tester": {
				Model:                 "glm-4.6",
				Temperature:           0.5,
				TopP:                  0.9,
				MaxContextTokens:      20000,
				MaxOutputTokens:       4000,
				MaxExecutionTimeSec:   300,
				MaxRetries:            3,
				MaxFactsInShardKernel: 20000,
				EnableLearning:        true,
			},
			"reviewer": {
				Model:                 "glm-4.6",
				Temperature:           0.3,
				TopP:                  0.9,
				MaxContextTokens:      40000,
				MaxOutputTokens:       8000,
				MaxExecutionTimeSec:   900,
				MaxRetries:            2,
				MaxFactsInShardKernel: 30000,
				EnableLearning:        false,
			},
			"researcher": {
				Model:                 "glm-4.6",
				Temperature:           0.6,
				TopP:                  0.95,
				MaxContextTokens:      25000,
				MaxOutputTokens:       5000,
				MaxExecutionTimeSec:   600,
				MaxRetries:            3,
				MaxFactsInShardKernel: 25000,
				EnableLearning:        true,
			},
		},
		DefaultShard: &config.ShardProfile{
			Model:                 "glm-4.6",
			Temperature:           0.7,
			TopP:                  0.9,
			MaxContextTokens:      20000,
			MaxOutputTokens:       4000,
			MaxExecutionTimeSec:   300,
			MaxRetries:            3,
			MaxFactsInShardKernel: 20000,
			EnableLearning:        true,
		},
		CoreLimits: &config.CoreLimits{
			MaxTotalMemoryMB:      12288,
			MaxConcurrentShards:   4,
			MaxSessionDurationMin: 120,
			MaxFactsInKernel:      250000,
			MaxDerivedFactsLimit:  100000,
		},
		Integrations: &config.IntegrationsConfig{
			CodeGraph: config.CodeGraphIntegration{
				Enabled: true,
				BaseURL: "http://localhost:8080",
				Timeout: "30s",
			},
			Browser: config.BrowserIntegration{
				Enabled: true,
				BaseURL: "http://localhost:8081",
				Timeout: "60s",
			},
			Scraper: config.ScraperIntegration{
				Enabled: true,
				BaseURL: "http://localhost:8082",
				Timeout: "120s",
			},
		},
		Execution: &config.ExecutionConfig{
			AllowedBinaries: []string{
				"go", "git", "grep", "ls", "mkdir", "cp", "mv",
				"npm", "npx", "node", "python", "python3", "pip",
				"cargo", "rustc", "make", "cmake",
			},
			DefaultTimeout:   "30s",
			WorkingDirectory: ".",
			AllowedEnvVars:   []string{"PATH", "HOME", "GOPATH", "GOROOT"},
		},
		Logging: &config.LoggingConfig{
			Level:  "info",
			Format: "text",
			File:   "codenerd.log",
		},
		ToolGeneration: &config.ToolGenerationConfig{
			TargetOS:   "windows",
			TargetArch: "amd64",
		},
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	return os.WriteFile(path, data, 0644)
}
