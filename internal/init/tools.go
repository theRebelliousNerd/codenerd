// Package init implements the "nerd init" cold-start initialization system.
package init

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ToolDefinition represents a tool template available for specialist shards.
// It supports both static command-line tools and MCP (Model Context Protocol) tools.
type ToolDefinition struct {
	Name          string   `json:"name"`
	DisplayName   string   `json:"display_name,omitempty"`   // Human-readable name (optional)
	Category      string   `json:"category"`                 // build, test, lint, format, deps, git, docker, security, code_analysis, etc
	Description   string   `json:"description"`
	ShardAffinity string   `json:"shard_affinity,omitempty"` // Which shard primarily uses this (TesterShard, CoderShard, ReviewerShard, ResearcherShard)
	Conditions    []string `json:"conditions,omitempty"`     // Required conditions (e.g., "go.mod exists", "docker available")

	// Tool type: "mcp" for MCP tools, empty/omitted for static command tools
	Type string `json:"type,omitempty"`

	// Static tool fields (used when Type is empty)
	Command    string `json:"command,omitempty"`     // Actual command to execute
	WorkingDir string `json:"working_dir,omitempty"` // Where to run it (default ".")
	InputType  string `json:"input_type,omitempty"`  // stdin, args, file, none
	OutputType string `json:"output_type,omitempty"` // stdout, file, json

	// MCP tool fields (used when Type is "mcp")
	MCPServer   string `json:"mcp_server,omitempty"`   // MCP server ID (e.g., "code_graph", "browser")
	MCPTool     string `json:"mcp_tool,omitempty"`     // Tool name on the MCP server
	AutoAnalyze bool   `json:"auto_analyze,omitempty"` // Auto-analyze tool with LLM on discovery

	// Legacy fields for compatibility with existing code in agents.go
	Purpose    string  `json:"purpose,omitempty"`
	Priority   float64 `json:"priority,omitempty"`
	Technology string  `json:"technology,omitempty"`
	Reason     string  `json:"reason,omitempty"`
}

// IsMCPTool returns true if this is an MCP tool definition.
func (t *ToolDefinition) IsMCPTool() bool {
	return t.Type == "mcp"
}

// GetLanguageTools returns tool definitions for a language
func GetLanguageTools(language string) []ToolDefinition {
	tools := make([]ToolDefinition, 0)

	switch language {
	case "go", "golang":
		tools = []ToolDefinition{
			{
				Name:          "go_build",
				DisplayName:   "Go Build",
				Category:      "build",
				Description:   "Build Go project",
				Command:       "go build ./...",
				WorkingDir:    ".",
				InputType:     "args",
				OutputType:    "stdout",
				ShardAffinity: "CoderShard",
				Conditions:    []string{"go.mod exists"},
			},
			{
				Name:          "go_test",
				DisplayName:   "Go Test",
				Category:      "test",
				Description:   "Run Go tests",
				Command:       "go test -v ./...",
				WorkingDir:    ".",
				InputType:     "args",
				OutputType:    "stdout",
				ShardAffinity: "TesterShard",
				Conditions:    []string{"go.mod exists"},
			},
			{
				Name:          "go_lint",
				DisplayName:   "GolangCI-Lint",
				Category:      "lint",
				Description:   "Run golangci-lint",
				Command:       "golangci-lint run",
				WorkingDir:    ".",
				InputType:     "args",
				OutputType:    "stdout",
				ShardAffinity: "ReviewerShard",
				Conditions:    []string{"go.mod exists", "golangci-lint installed"},
			},
			{
				Name:          "go_fmt",
				DisplayName:   "Go Format",
				Category:      "format",
				Description:   "Format Go code",
				Command:       "go fmt ./...",
				WorkingDir:    ".",
				InputType:     "args",
				OutputType:    "stdout",
				ShardAffinity: "CoderShard",
				Conditions:    []string{"go.mod exists"},
			},
			{
				Name:          "go_mod_tidy",
				DisplayName:   "Go Mod Tidy",
				Category:      "deps",
				Description:   "Tidy Go modules",
				Command:       "go mod tidy",
				WorkingDir:    ".",
				InputType:     "none",
				OutputType:    "stdout",
				ShardAffinity: "CoderShard",
				Conditions:    []string{"go.mod exists"},
			},
			{
				Name:          "go_vet",
				DisplayName:   "Go Vet",
				Category:      "lint",
				Description:   "Run go vet",
				Command:       "go vet ./...",
				WorkingDir:    ".",
				InputType:     "args",
				OutputType:    "stdout",
				ShardAffinity: "ReviewerShard",
				Conditions:    []string{"go.mod exists"},
			},
		}

	case "python":
		tools = []ToolDefinition{
			{
				Name:        "pytest",
				Category:    "test",
				Description: "Run pytest",
				Command:     "pytest -v",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "black",
				Category:    "format",
				Description: "Format Python code with black",
				Command:     "black .",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "mypy",
				Category:    "typecheck",
				Description: "Type check with mypy",
				Command:     "mypy .",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "pylint",
				Category:    "lint",
				Description: "Lint Python code",
				Command:     "pylint **/*.py",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "pip_install",
				Category:    "dependencies",
				Description: "Install Python dependencies",
				Command:     "pip install -r requirements.txt",
				WorkingDir:  ".",
				InputType:   "none",
				OutputType:  "stdout",
			},
		}

	case "typescript", "javascript":
		tools = []ToolDefinition{
			{
				Name:        "npm_test",
				Category:    "test",
				Description: "Run npm tests",
				Command:     "npm test",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "npm_build",
				Category:    "build",
				Description: "Build npm project",
				Command:     "npm run build",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "eslint",
				Category:    "lint",
				Description: "Lint JavaScript/TypeScript",
				Command:     "eslint .",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "prettier",
				Category:    "format",
				Description: "Format code with prettier",
				Command:     "prettier --write .",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "tsc",
				Category:    "typecheck",
				Description: "TypeScript type checker",
				Command:     "tsc --noEmit",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
		}

	case "rust":
		tools = []ToolDefinition{
			{
				Name:        "cargo_build",
				Category:    "build",
				Description: "Build Rust project",
				Command:     "cargo build",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "cargo_test",
				Category:    "test",
				Description: "Run Rust tests",
				Command:     "cargo test",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "cargo_check",
				Category:    "check",
				Description: "Check Rust code",
				Command:     "cargo check",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "cargo_clippy",
				Category:    "lint",
				Description: "Lint Rust code with clippy",
				Command:     "cargo clippy",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "cargo_fmt",
				Category:    "format",
				Description: "Format Rust code",
				Command:     "cargo fmt",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
		}
	}

	return tools
}

// GetFrameworkTools returns tool definitions for a framework
func GetFrameworkTools(framework string) []ToolDefinition {
	tools := make([]ToolDefinition, 0)

	switch framework {
	case "bubbletea":
		tools = []ToolDefinition{
			{
				Name:        "run_tui",
				Category:    "run",
				Description: "Run Bubbletea TUI application",
				Command:     "go run ./cmd/...",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
		}
	case "gin", "echo", "fiber":
		tools = []ToolDefinition{
			{
				Name:        "api_server_run",
				Category:    "run",
				Description: "Start API server",
				Command:     "go run ./cmd/server",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
			{
				Name:        "api_test",
				Category:    "test",
				Description: "Test API endpoints",
				Command:     "go test -tags=integration ./...",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
		}
	case "react", "nextjs", "vue":
		tools = []ToolDefinition{
			{
				Name:        "dev_server",
				Category:    "run",
				Description: "Start development server",
				Command:     "npm run dev",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			},
		}
	}

	return tools
}

// GetDependencyTools returns tool definitions based on dependencies
func GetDependencyTools(dependencies []string) []ToolDefinition {
	tools := make([]ToolDefinition, 0)

	for _, dep := range dependencies {
		switch dep {
		case "rod":
			tools = append(tools, ToolDefinition{
				Name:        "rod_download_browser",
				Category:    "setup",
				Description: "Download browser for Rod",
				Command:     "go run github.com/go-rod/rod/lib/launcher/setup",
				WorkingDir:  ".",
				InputType:   "none",
				OutputType:  "stdout",
			})
		case "docker":
			tools = append(tools, ToolDefinition{
				Name:        "docker_compose_up",
				Category:    "docker",
				Description: "Start Docker Compose services",
				Command:     "docker-compose up -d",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			})
			tools = append(tools, ToolDefinition{
				Name:        "docker_compose_down",
				Category:    "docker",
				Description: "Stop Docker Compose services",
				Command:     "docker-compose down",
				WorkingDir:  ".",
				InputType:   "args",
				OutputType:  "stdout",
			})
		}
	}

	return tools
}

// GenerateToolsForProject generates all relevant tools for the project
func GenerateToolsForProject(detectedTech []string) []ToolDefinition {
	tools := make([]ToolDefinition, 0)
	seen := make(map[string]bool)

	for _, tech := range detectedTech {
		// Try as language
		langTools := GetLanguageTools(tech)
		for _, t := range langTools {
			if !seen[t.Name] {
				tools = append(tools, t)
				seen[t.Name] = true
			}
		}

		// Try as framework
		fwTools := GetFrameworkTools(tech)
		for _, t := range fwTools {
			if !seen[t.Name] {
				tools = append(tools, t)
				seen[t.Name] = true
			}
		}
	}

	return tools
}

// LoadToolsFromFile reads tools from available_tools.json
func LoadToolsFromFile(nerdDir string) ([]ToolDefinition, error) {
	toolsFile := filepath.Join(nerdDir, "tools", "available_tools.json")
	data, err := os.ReadFile(toolsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No tools file yet
		}
		return nil, fmt.Errorf("failed to read tools file: %w", err)
	}
	var tools []ToolDefinition
	if err := json.Unmarshal(data, &tools); err != nil {
		return nil, fmt.Errorf("failed to parse tools file: %w", err)
	}
	return tools, nil
}

// SaveToolsToFile saves tool definitions to .nerd/tools/available_tools.json
func SaveToolsToFile(nerdDir string, tools []ToolDefinition) error {
	toolsDir := filepath.Join(nerdDir, "tools")
	if err := os.MkdirAll(toolsDir, 0755); err != nil {
		return fmt.Errorf("failed to create tools directory: %w", err)
	}

	toolsFile := filepath.Join(toolsDir, "available_tools.json")
	data, err := json.MarshalIndent(tools, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal tools: %w", err)
	}

	if err := os.WriteFile(toolsFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write tools file: %w", err)
	}

	return nil
}

// GetToolsForAgentType returns the tool names for a specific agent type
func GetToolsForAgentType(agentName string, language string) ([]string, map[string]string) {
	tools := make([]string, 0)
	prefs := make(map[string]string)

	switch agentName {
	case "GoExpert":
		tools = []string{"go_build", "go_test", "go_lint", "go_mod_tidy", "go_fmt", "go_vet"}
		prefs = map[string]string{
			"build":  "go_build",
			"test":   "go_test",
			"lint":   "go_lint",
			"format": "go_fmt",
			"tidy":   "go_mod_tidy",
			"check":  "go_vet",
		}

	case "PythonExpert":
		tools = []string{"pytest", "black", "mypy", "pylint", "pip_install"}
		prefs = map[string]string{
			"test":      "pytest",
			"format":    "black",
			"lint":      "pylint",
			"typecheck": "mypy",
			"install":   "pip_install",
		}

	case "TSExpert":
		tools = []string{"npm_test", "npm_build", "eslint", "prettier", "tsc"}
		prefs = map[string]string{
			"test":      "npm_test",
			"build":     "npm_build",
			"lint":      "eslint",
			"format":    "prettier",
			"typecheck": "tsc",
		}

	case "RustExpert":
		tools = []string{"cargo_build", "cargo_test", "cargo_check", "cargo_clippy", "cargo_fmt"}
		prefs = map[string]string{
			"build":  "cargo_build",
			"test":   "cargo_test",
			"check":  "cargo_check",
			"lint":   "cargo_clippy",
			"format": "cargo_fmt",
		}

	case "RodExpert", "BrowserAutomationExpert":
		tools = []string{"rod_download_browser"}
		prefs = map[string]string{
			"setup": "rod_download_browser",
		}

	case "DatabaseExpert":
		// Database tools depend on the language
		if language == "go" || language == "golang" {
			tools = []string{"go_test"} // DB tests
			prefs = map[string]string{
				"test": "go_test",
			}
		}

	case "WebAPIExpert":
		tools = []string{"api_server_run", "api_test"}
		prefs = map[string]string{
			"run":  "api_server_run",
			"test": "api_test",
		}

	case "FrontendExpert":
		tools = []string{"dev_server", "npm_build", "npm_test"}
		prefs = map[string]string{
			"dev":   "dev_server",
			"build": "npm_build",
			"test":  "npm_test",
		}

	case "SecurityAuditor":
		// Security audit tools - language specific
		if language == "go" || language == "golang" {
			tools = []string{"go_vet", "go_lint"}
			prefs = map[string]string{
				"audit": "go_vet",
				"lint":  "go_lint",
			}
		} else if language == "python" {
			tools = []string{"pylint"}
			prefs = map[string]string{
				"audit": "pylint",
			}
		}

	case "TestArchitect":
		// Testing tools - language specific
		if language == "go" || language == "golang" {
			tools = []string{"go_test"}
			prefs = map[string]string{
				"test": "go_test",
			}
		} else if language == "python" {
			tools = []string{"pytest"}
			prefs = map[string]string{
				"test": "pytest",
			}
		} else if language == "typescript" || language == "javascript" {
			tools = []string{"npm_test"}
			prefs = map[string]string{
				"test": "npm_test",
			}
		}
	}

	return tools, prefs
}
