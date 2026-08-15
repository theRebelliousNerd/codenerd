package init

import (
	// coreshards removed - was only used by tool_generator
	"codenerd/internal/logging"
	// Domain shards removed - JIT clean loop handles research and tool generation:
	// "codenerd/internal/shards/researcher"
	// "codenerd/internal/shards/tool_generator"
	"codenerd/internal/store"
	"codenerd/internal/tools"
	"codenerd/internal/tools/research"
	"codenerd/internal/types"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// sendAgentProgress sends an agent-specific progress update.
func (i *Initializer) sendAgentProgress(name, agentType, status string, kbSize int) {
	if i.config.ProgressChan != nil {
		select {
		case i.config.ProgressChan <- InitProgress{
			Phase:   "agent_creation",
			Message: fmt.Sprintf("Agent %s: %s", name, status),
			AgentUpdate: &AgentCreationUpdate{
				AgentName: name,
				AgentType: agentType,
				Status:    status,
				KBSize:    kbSize,
			},
		}:
		default:
		}
	}
}

// registerAgentsWithShardManager registers created agents for dynamic calling.
func (i *Initializer) registerAgentsWithShardManager(agents []CreatedAgent) {
	if i.shardMgr == nil {
		return
	}

	for _, agent := range agents {
		// Create shard config for the agent
		config := types.ShardConfig{
			Name:          agent.Name,
			Type:          types.ShardTypePersistent,
			BaseType:      "researcher",
			KnowledgePath: agent.KnowledgePath,
			Timeout:       30 * time.Minute,
			MemoryLimit:   10000,
			Permissions: []types.ShardPermission{
				types.PermissionReadFile,
				types.PermissionCodeGraph,
			},
			Model: types.ModelConfig{
				Capability: types.CapabilityBalanced,
			},
			Tools:           agent.Tools,
			ToolPreferences: agent.ToolPreferences,
		}

		// Register the profile with shard manager
		i.shardMgr.DefineProfile(agent.Name, config)
	}
}

// saveAgentRegistry saves the agent registry to disk.
func (i *Initializer) saveAgentRegistry(path string, agents []CreatedAgent) error {
	registry := AgentRegistry{
		Version:   "1.5.0",
		CreatedAt: time.Now(),
		Agents:    agents,
	}

	data, err := json.MarshalIndent(registry, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// createCoreShardKnowledgeBases creates knowledge bases for Coder, Reviewer, Tester shards.
// In upgrade mode, appends new atoms without overwriting existing knowledge.
func (i *Initializer) createCoreShardKnowledgeBases(ctx context.Context, nerdDir string, profile ProjectProfile) (map[string]int, error) {
	shardsDir := filepath.Join(nerdDir, "shards")
	results := make(map[string]int)

	// Define core shards with their domain expertise
	coreShards := []struct {
		Name        string
		Description string
		Topics      []string
		Concepts    []struct{ Key, Value string }
	}{
		{
			Name:        "coder",
			Description: "Code generation and modification specialist",
			Topics:      []string{"code generation", "refactoring", "file editing", "impact analysis"},
			Concepts:    []struct{ Key, Value string }{{"role", "I am the Coder shard. I generate, modify, and refactor code following project conventions."}, {"capability_generate", "I can generate new code files, functions, and modules."}, {"capability_modify", "I can modify existing code with precise edits."}, {"capability_refactor", "I can refactor code for better structure and readability."}, {"safety_rule", "I always check impact radius before making changes."}, {"safety_rule", "I never modify files without understanding their purpose."}},
		},
		{
			Name:        "reviewer",
			Description: "Code review and security analysis specialist",
			Topics:      []string{"code review", "security audit", "style checking", "best practices"},
			Concepts:    []struct{ Key, Value string }{{"role", "I am the Reviewer shard. I review code for quality, security, and style issues."}, {"capability_review", "I can perform comprehensive code reviews."}, {"capability_security", "I can detect security vulnerabilities (OWASP top 10)."}, {"capability_style", "I can check code style and consistency."}, {"safety_rule", "Critical security issues block commit."}, {"safety_rule", "I provide constructive feedback with suggestions."}},
		},
		{
			Name:        "tester",
			Description: "Testing and TDD loop specialist",
			Topics:      []string{"unit testing", "TDD", "test coverage", "test generation"},
			Concepts:    []struct{ Key, Value string }{{"role", "I am the Tester shard. I manage tests, TDD loops, and coverage."}, {"capability_generate", "I can generate unit tests for functions and modules."}, {"capability_run", "I can execute tests and parse results."}, {"capability_tdd", "I can run TDD repair loops to fix failing tests."}, {"safety_rule", "Tests must pass before code is considered complete."}, {"safety_rule", "Coverage below goal triggers test generation."}},
		},
	}

	for _, shard := range coreShards {
		kbPath := filepath.Join(shardsDir, fmt.Sprintf("%s_knowledge.db", shard.Name))

		// Check if KB already exists (upgrade mode)
		upgradeMode := false
		if _, statErr := os.Stat(kbPath); statErr == nil {
			upgradeMode = true
			logging.Boot("Upgrading core shard %s KB", shard.Name)
		}

		shardDB, err := store.NewLocalStore(kbPath)
		if err != nil {
			continue
		}
		if err := i.ensureEmbeddingEngine(); err != nil {
			return nil, err
		}
		shardDB.SetEmbeddingEngine(i.embedEngine)

		// Get existing hashes for deduplication in upgrade mode
		var existingHashes map[string]bool
		if upgradeMode {
			existingAtoms, _ := shardDB.GetAllKnowledgeAtoms()
			existingHashes = buildAtomHashSet(existingAtoms)
		} else {
			existingHashes = make(map[string]bool)
		}

		atomCount := 0
		newAtoms := 0

		// Store shard identity
		added, err := appendKnowledgeAtom(shardDB, "shard_identity", shard.Description, 1.0, existingHashes)
		if err == nil {
			atomCount++
			if added {
				newAtoms++
			}
		}

		// Store concepts
		for _, concept := range shard.Concepts {
			added, err := appendKnowledgeAtom(shardDB, concept.Key, concept.Value, 0.95, existingHashes)
			if err == nil {
				atomCount++
				if added {
					newAtoms++
				}
			}
		}

		// Store project context
		added, err = appendKnowledgeAtom(shardDB, "project_language", profile.Language, 0.9, existingHashes)
		if err == nil {
			atomCount++
			if added {
				newAtoms++
			}
		}
		if profile.Framework != "" && profile.Framework != "unknown" {
			added, err = appendKnowledgeAtom(shardDB, "project_framework", profile.Framework, 0.9, existingHashes)
			if err == nil {
				atomCount++
				if added {
					newAtoms++
				}
			}
		}

		// Research shard-specific topics using modular tools
		// =========================================================================
		// Research uses the modular tool registry (internal/tools/research/)
		// =========================================================================
		if i.config.LLMClient != nil && !i.config.SkipResearch && len(shard.Topics) > 0 {
			registry := tools.NewRegistry()
			if err := research.RegisterAll(registry); err == nil {
				researchCtx, cancel := context.WithTimeout(ctx, 1*time.Minute)
				// Always release the timer goroutine; previously cancel was
				// never called, leaking one timer per shard per init pass.
				defer cancel()
				for _, topic := range shard.Topics {
					result, err := registry.Execute(researchCtx, "context7_fetch", map[string]any{"topic": topic})
					if err == nil && result.Result != "" && len(result.Result) > 100 {
						atoms := i.parseResearchResult(topic, result.Result)
						for _, atom := range atoms {
							added, err := appendKnowledgeAtom(shardDB, atom.Concept, atom.Content, atom.Confidence, existingHashes)
							if err == nil && added {
								newAtoms++
							}
						}
					}
				}
				cancel()
			}
		}

		// Get final count
		finalAtoms, _ := shardDB.GetAllKnowledgeAtoms()
		results[shard.Name] = len(finalAtoms)

		if upgradeMode {
			logging.Boot("Core shard %s KB upgraded (added %d new atoms, total %d)", shard.Name, newAtoms, len(finalAtoms))
		}

		shardDB.Close()
	}

	return results, nil
}

// ToolGenerationRequest represents a tool to be generated during init using Ouroboros.
type ToolGenerationRequest struct {
	Name       string
	Purpose    string
	Priority   float64
	Technology string // Language/framework that triggered this tool
	Reason     string
}

// generateProjectTools reports the project tool needs that phase 5 recorded as
// Mangle facts.
//
// Decision (was: "complete the Ouroboros call site or delete determineRequiredTools"):
// neither. determineRequiredTools is a real measurement — it is the only place
// that reads a project's language, framework, ORM, container and build system
// and names the tools that shape implies — but init is the wrong place to act
// on it. Acting means writing LLM-authored Go, compiling it and registering the
// binary in the user's workspace; doing that for up to eight tools during a
// cold start would add many minutes before the user has typed anything, and
// init holds the cheap *worker* LLM client, which is the wrong tier for
// codegen.
//
// So init measures and the kernel decides, which is the split the system is
// built on. generateFactsFile now emits one `missing_tool_for(/project_init,
// /capability)` fact per detected need into profile.mg. That is the same
// already-Declared predicate autopoiesis and campaign assert when they find a
// capability gap, so the needs enter the existing Ouroboros policy chain and
// any generation still runs through ExecuteOuroborosLoop with its full safety
// depth (go_safety.mg audit, Thunderdome, transition simulation, compile).
// Init adds no new path to ToolGenerator. This also answers OPEN-QUESTIONS #8.
//
// The returned slice names the recorded needs, not compiled tools.
func (i *Initializer) generateProjectTools(_ context.Context, _ string, profile ProjectProfile) ([]string, error) {
	toolDefs := i.determineRequiredTools(profile)
	if len(toolDefs) == 0 {
		return []string{}, nil
	}

	recorded := make([]string, 0, len(toolDefs))
	for _, toolDef := range toolDefs {
		recorded = append(recorded, toolDef.Name)
		logging.Boot("Recorded tool need: %s - %s (%s)", toolDef.Name, toolDef.Purpose, toolDef.Reason)
	}

	fmt.Printf("   Recorded %d project tool needs as missing_tool_for facts\n", len(recorded))
	fmt.Println("   The kernel decides when to generate them; Ouroboros builds them on demand")
	return recorded, nil
}

// projectToolNeedFacts renders the detected tool needs as `missing_tool_for`
// facts. The predicate is Declared in schemas_tools.mg as
// missing_tool_for(Intent, Capability) bound [/name, /name], so both arguments
// must be sanitized name constants.
func projectToolNeedFacts(toolDefs []ToolGenerationRequest) []string {
	if len(toolDefs) == 0 {
		return nil
	}
	facts := make([]string, 0, len(toolDefs))
	seen := make(map[string]bool, len(toolDefs))
	for _, toolDef := range toolDefs {
		capability := sanitizeForMangle(toolDef.Name)
		if capability == "" || capability == "unknown" || seen[capability] {
			continue
		}
		seen[capability] = true
		facts = append(facts, fmt.Sprintf(`missing_tool_for(/project_init, /%s).`, capability))
	}
	return facts
}

// determineRequiredTools determines which tools to generate based on project technologies.
func (i *Initializer) determineRequiredTools(profile ProjectProfile) []ToolGenerationRequest {
	tools := make([]ToolGenerationRequest, 0)

	// Language-specific tools
	switch strings.ToLower(profile.Language) {
	case "go", "golang":
		tools = append(tools, []ToolGenerationRequest{
			{
				Name:       "go_build_tool",
				Purpose:    "build Go projects with proper flags and caching",
				Priority:   1.0,
				Technology: "go",
				Reason:     "Essential for Go project compilation",
			},
			{
				Name:       "go_test_tool",
				Purpose:    "run Go tests with coverage and race detection",
				Priority:   1.0,
				Technology: "go",
				Reason:     "Essential for Go project testing",
			},
			{
				Name:       "go_lint_tool",
				Purpose:    "run golangci-lint with project-specific configuration",
				Priority:   0.8,
				Technology: "go",
				Reason:     "Code quality enforcement for Go",
			},
			{
				Name:       "go_mod_tidy_tool",
				Purpose:    "clean and organize Go module dependencies",
				Priority:   0.7,
				Technology: "go",
				Reason:     "Dependency management for Go modules",
			},
		}...)

	case "python":
		tools = append(tools, []ToolGenerationRequest{
			{
				Name:       "pytest_tool",
				Purpose:    "run pytest with coverage and parallel execution",
				Priority:   1.0,
				Technology: "python",
				Reason:     "Essential for Python testing",
			},
			{
				Name:       "pip_install_tool",
				Purpose:    "install Python dependencies from requirements.txt or pyproject.toml",
				Priority:   0.9,
				Technology: "python",
				Reason:     "Dependency management for Python",
			},
			{
				Name:       "black_format_tool",
				Purpose:    "format Python code with Black",
				Priority:   0.7,
				Technology: "python",
				Reason:     "Code formatting for Python",
			},
			{
				Name:       "mypy_check_tool",
				Purpose:    "run mypy type checking on Python code",
				Priority:   0.8,
				Technology: "python",
				Reason:     "Type safety for Python",
			},
		}...)

	case "typescript", "javascript":
		tools = append(tools, []ToolGenerationRequest{
			{
				Name:       "npm_build_tool",
				Purpose:    "build TypeScript/JavaScript projects with npm",
				Priority:   1.0,
				Technology: "typescript",
				Reason:     "Essential for TS/JS project compilation",
			},
			{
				Name:       "jest_test_tool",
				Purpose:    "run Jest tests with coverage",
				Priority:   1.0,
				Technology: "typescript",
				Reason:     "Essential for TS/JS testing",
			},
			{
				Name:       "eslint_tool",
				Purpose:    "run ESLint for code quality",
				Priority:   0.8,
				Technology: "typescript",
				Reason:     "Code quality for TS/JS",
			},
			{
				Name:       "npm_install_tool",
				Purpose:    "install npm dependencies",
				Priority:   0.9,
				Technology: "typescript",
				Reason:     "Dependency management for npm",
			},
		}...)

	case "rust":
		tools = append(tools, []ToolGenerationRequest{
			{
				Name:       "cargo_build_tool",
				Purpose:    "build Rust projects with cargo",
				Priority:   1.0,
				Technology: "rust",
				Reason:     "Essential for Rust compilation",
			},
			{
				Name:       "cargo_test_tool",
				Purpose:    "run Rust tests with cargo",
				Priority:   1.0,
				Technology: "rust",
				Reason:     "Essential for Rust testing",
			},
			{
				Name:       "cargo_clippy_tool",
				Purpose:    "run clippy lints on Rust code",
				Priority:   0.8,
				Technology: "rust",
				Reason:     "Code quality for Rust",
			},
		}...)
	}

	// Framework-specific tools
	switch strings.ToLower(profile.Framework) {
	case "react", "nextjs":
		tools = append(tools, []ToolGenerationRequest{
			{
				Name:       "react_dev_server_tool",
				Purpose:    "start React development server",
				Priority:   0.9,
				Technology: profile.Framework,
				Reason:     "Development workflow for React",
			},
			{
				Name:       "react_build_tool",
				Purpose:    "build React app for production",
				Priority:   0.8,
				Technology: profile.Framework,
				Reason:     "Production build for React",
			},
		}...)

	case "gin", "echo", "fiber":
		tools = append(tools, []ToolGenerationRequest{
			{
				Name:       "api_test_tool",
				Purpose:    "run API endpoint tests with proper setup/teardown",
				Priority:   0.9,
				Technology: profile.Framework,
				Reason:     fmt.Sprintf("API testing for %s framework", profile.Framework),
			},
		}...)
	}

	// Dependency-specific tools
	depNames := make(map[string]bool)
	for _, dep := range profile.Dependencies {
		depNames[dep.Name] = true
	}

	if depNames["docker"] {
		tools = append(tools, []ToolGenerationRequest{
			{
				Name:       "docker_build_tool",
				Purpose:    "build Docker images for the project",
				Priority:   0.8,
				Technology: "docker",
				Reason:     "Container workflow detected",
			},
			{
				Name:       "docker_compose_tool",
				Purpose:    "manage docker-compose services",
				Priority:   0.7,
				Technology: "docker",
				Reason:     "Multi-container workflow detected",
			},
		}...)
	}

	if depNames["rod"] || depNames["chromedp"] || depNames["playwright"] || depNames["puppeteer"] {
		tools = append(tools, []ToolGenerationRequest{
			{
				Name:       "browser_test_tool",
				Purpose:    "run browser automation tests",
				Priority:   0.8,
				Technology: "browser-automation",
				Reason:     "Browser automation detected",
			},
		}...)
	}

	if depNames["gorm"] || depNames["sqlx"] || depNames["prisma"] || depNames["typeorm"] {
		tools = append(tools, []ToolGenerationRequest{
			{
				Name:       "db_migrate_tool",
				Purpose:    "run database migrations",
				Priority:   0.8,
				Technology: "database",
				Reason:     "Database ORM detected",
			},
			{
				Name:       "db_seed_tool",
				Purpose:    "seed database with test data",
				Priority:   0.6,
				Technology: "database",
				Reason:     "Database workflow detected",
			},
		}...)
	}

	// Build system detection
	if profile.BuildSystem != "" {
		switch strings.ToLower(profile.BuildSystem) {
		case "makefile":
			tools = append(tools, ToolGenerationRequest{
				Name:       "make_tool",
				Purpose:    "run Makefile targets",
				Priority:   0.7,
				Technology: "make",
				Reason:     "Makefile build system detected",
			})
		case "gradle":
			tools = append(tools, ToolGenerationRequest{
				Name:       "gradle_build_tool",
				Purpose:    "build projects with Gradle",
				Priority:   0.9,
				Technology: "gradle",
				Reason:     "Gradle build system detected",
			})
		}
	}

	// Sort by priority (highest first)
	// Simple bubble sort since list is small
	for i := 0; i < len(tools)-1; i++ {
		for j := 0; j < len(tools)-i-1; j++ {
			if tools[j].Priority < tools[j+1].Priority {
				tools[j], tools[j+1] = tools[j+1], tools[j]
			}
		}
	}

	// Limit to top 8 tools to avoid overwhelming during init
	maxTools := 8
	if len(tools) > maxTools {
		tools = tools[:maxTools]
	}

	return tools
}
