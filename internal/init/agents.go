// Package init implements the "nerd init" cold-start initialization system.
package init

import (
	"codenerd/internal/core"
	"codenerd/internal/shards/researcher"
	"codenerd/internal/store"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// determineRequiredAgents analyzes the project and recommends Type 3 agents.
func (i *Initializer) determineRequiredAgents(profile ProjectProfile) []RecommendedAgent {
	agents := make([]RecommendedAgent, 0)

	// Language-specific agents
	switch strings.ToLower(profile.Language) {
	case "go", "golang":
		agents = append(agents, RecommendedAgent{
			Name:        "GoExpert",
			Type:        "persistent",
			Description: "Expert in Go idioms, concurrency patterns, and standard library",
			Topics:      []string{"go concurrency", "go error handling", "go interfaces", "go testing"},
			Permissions: []string{"read_file", "code_graph", "exec_cmd"},
			Priority:    100,
			Reason:      "Go project detected - expert knowledge improves code quality",
		})

	case "python":
		agents = append(agents, RecommendedAgent{
			Name:        "PythonExpert",
			Type:        "persistent",
			Description: "Expert in Python best practices, type hints, and async patterns",
			Topics:      []string{"python typing", "python async", "python testing", "python packaging"},
			Permissions: []string{"read_file", "code_graph", "exec_cmd"},
			Priority:    100,
			Reason:      "Python project detected - expert knowledge improves code quality",
		})

	case "typescript", "javascript":
		agents = append(agents, RecommendedAgent{
			Name:        "TSExpert",
			Type:        "persistent",
			Description: "Expert in TypeScript/JavaScript patterns and modern ES features",
			Topics:      []string{"typescript types", "javascript async", "react patterns", "node.js"},
			Permissions: []string{"read_file", "code_graph", "exec_cmd"},
			Priority:    100,
			Reason:      "TypeScript/JavaScript project detected",
		})

	case "rust":
		agents = append(agents, RecommendedAgent{
			Name:        "RustExpert",
			Type:        "persistent",
			Description: "Expert in Rust ownership, lifetimes, and async patterns",
			Topics:      []string{"rust ownership", "rust lifetimes", "rust async", "rust error handling"},
			Permissions: []string{"read_file", "code_graph", "exec_cmd"},
			Priority:    100,
			Reason:      "Rust project detected - ownership expertise critical",
		})
	}

	// Framework-specific agents
	switch strings.ToLower(profile.Framework) {
	case "gin", "echo", "fiber":
		agents = append(agents, RecommendedAgent{
			Name:        "WebAPIExpert",
			Type:        "persistent",
			Description: "Expert in REST API design and HTTP middleware patterns",
			Topics:      []string{"REST API design", "HTTP middleware", "API authentication", "OpenAPI"},
			Permissions: []string{"read_file", "network"},
			Priority:    80,
			Reason:      fmt.Sprintf("%s framework detected - API expertise beneficial", profile.Framework),
		})

	case "react", "nextjs", "vue":
		agents = append(agents, RecommendedAgent{
			Name:        "FrontendExpert",
			Type:        "persistent",
			Description: "Expert in modern frontend patterns and state management",
			Topics:      []string{"react hooks", "state management", "component patterns", "CSS-in-JS"},
			Permissions: []string{"read_file", "browser"},
			Priority:    80,
			Reason:      fmt.Sprintf("%s framework detected - frontend expertise beneficial", profile.Framework),
		})
	}

	// Dependency-specific agents
	depNames := make(map[string]bool)
	for _, dep := range profile.Dependencies {
		depNames[dep.Name] = true
	}

	// Browser automation experts
	if depNames["rod"] {
		agents = append(agents, RecommendedAgent{
			Name:        "RodExpert",
			Type:        "persistent",
			Description: "Expert in Rod browser automation, selectors, and CDP protocol",
			Topics:      []string{"rod browser automation", "CDP protocol", "web scraping", "headless chrome", "page selectors"},
			Permissions: []string{"read_file", "browser", "exec_cmd"},
			Priority:    95,
			Reason:      "Rod browser automation detected - specialized expertise beneficial",
		})
	}
	if depNames["chromedp"] || depNames["puppeteer"] || depNames["playwright"] {
		agents = append(agents, RecommendedAgent{
			Name:        "BrowserAutomationExpert",
			Type:        "persistent",
			Description: "Expert in browser automation patterns and CDP",
			Topics:      []string{"browser automation", "CDP protocol", "page navigation", "element interaction"},
			Permissions: []string{"read_file", "browser"},
			Priority:    90,
			Reason:      "Browser automation library detected",
		})
	}

	// Logic/Datalog experts
	if depNames["mangle"] {
		agents = append(agents, RecommendedAgent{
			Name:        "MangleExpert",
			Type:        "persistent",
			Description: "Expert in Google Mangle/Datalog, logic programming, and rule systems",
			Topics:      []string{"datalog", "mangle syntax", "logic programming", "horn clauses", "fact derivation", "negation as failure"},
			Permissions: []string{"read_file", "code_graph"},
			Priority:    95,
			Reason:      "Mangle/Datalog detected - logic programming expertise critical",
		})
	}

	// LLM integration experts
	if depNames["openai"] || depNames["anthropic"] {
		agents = append(agents, RecommendedAgent{
			Name:        "LLMIntegrationExpert",
			Type:        "persistent",
			Description: "Expert in LLM API integration, prompt engineering, and token optimization",
			Topics:      []string{"LLM APIs", "prompt engineering", "token optimization", "streaming responses", "function calling"},
			Permissions: []string{"read_file", "network"},
			Priority:    90,
			Reason:      "LLM API integration detected - expertise improves reliability",
		})
	}

	// CLI/TUI experts
	if depNames["bubbletea"] {
		agents = append(agents, RecommendedAgent{
			Name:        "BubbleTeaExpert",
			Type:        "persistent",
			Description: "Expert in Bubbletea TUI framework, Elm architecture, and terminal rendering",
			Topics:      []string{"bubbletea", "elm architecture", "terminal UI", "lipgloss styling", "bubbles components"},
			Permissions: []string{"read_file", "code_graph"},
			Priority:    85,
			Reason:      "Bubbletea TUI framework detected",
		})
	}
	if depNames["cobra"] {
		agents = append(agents, RecommendedAgent{
			Name:        "CobraExpert",
			Type:        "persistent",
			Description: "Expert in Cobra CLI framework, command structure, and flag handling",
			Topics:      []string{"cobra CLI", "command patterns", "flag handling", "CLI best practices"},
			Permissions: []string{"read_file"},
			Priority:    75,
			Reason:      "Cobra CLI framework detected",
		})
	}

	// Database experts
	if depNames["gorm"] || depNames["sqlx"] || depNames["sql"] || depNames["prisma"] || depNames["typeorm"] {
		agents = append(agents, RecommendedAgent{
			Name:        "DatabaseExpert",
			Type:        "persistent",
			Description: "Expert in database patterns, ORM usage, and query optimization",
			Topics:      []string{"database design", "ORM patterns", "SQL optimization", "migrations", "connection pooling"},
			Permissions: []string{"read_file", "code_graph"},
			Priority:    80,
			Reason:      "Database ORM/driver detected",
		})
	}

	// Always include core agents
	agents = append(agents,
		RecommendedAgent{
			Name:        "SecurityAuditor",
			Type:        "persistent",
			Description: "Security vulnerability detection and best practices",
			Topics:      []string{"OWASP top 10", "secure coding", "vulnerability patterns", "code injection"},
			Permissions: []string{"read_file", "code_graph"},
			Priority:    90,
			Reason:      "Security analysis is critical for all projects",
		},
		RecommendedAgent{
			Name:        "TestArchitect",
			Type:        "persistent",
			Description: "Test strategy, coverage analysis, and TDD patterns",
			Topics:      []string{"unit testing", "integration testing", "test coverage", "mocking patterns"},
			Permissions: []string{"read_file", "exec_cmd"},
			Priority:    85,
			Reason:      "Test quality directly impacts code reliability",
		},
	)

	return agents
}

// createType3Agents creates the knowledge bases and registers Type 3 agents.
func (i *Initializer) createType3Agents(ctx context.Context, nerdDir string, agents []RecommendedAgent, result *InitResult) ([]CreatedAgent, map[string]int) {
	created := make([]CreatedAgent, 0)
	kbSizes := make(map[string]int)

	shardsDir := filepath.Join(nerdDir, "shards")

	for idx, agent := range agents {
		// Progress update
		progress := 0.55 + (float64(idx)/float64(len(agents)))*0.25
		i.sendProgress("kb_creation", fmt.Sprintf("Creating %s...", agent.Name), progress)
		i.sendAgentProgress(agent.Name, agent.Type, "creating", 0)

		fmt.Printf("   Creating %s knowledge base...\n", agent.Name)

		// Create knowledge base path
		kbPath := filepath.Join(shardsDir, fmt.Sprintf("%s_knowledge.db", strings.ToLower(agent.Name)))

		// Create knowledge base for agent
		kbSize, err := i.createAgentKnowledgeBase(ctx, kbPath, agent)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create KB for %s: %v", agent.Name, err))
			i.sendAgentProgress(agent.Name, agent.Type, "failed", 0)
			continue
		}

		kbSizes[agent.Name] = kbSize
		i.sendAgentProgress(agent.Name, agent.Type, "ready", kbSize)

		createdAgent := CreatedAgent{
			Name:          agent.Name,
			Type:          agent.Type,
			KnowledgePath: kbPath,
			KBSize:        kbSize,
			CreatedAt:     time.Now(),
			Status:        "ready",
		}
		created = append(created, createdAgent)

		// Track created files
		result.FilesCreated = append(result.FilesCreated, kbPath)

		fmt.Printf("     ✓ %s ready (%d knowledge atoms)\n", agent.Name, kbSize)
	}

	return created, kbSizes
}

// createAgentKnowledgeBase creates the SQLite knowledge base for an agent.
func (i *Initializer) createAgentKnowledgeBase(ctx context.Context, kbPath string, agent RecommendedAgent) (int, error) {
	// Create a dedicated local store for this agent
	agentDB, err := store.NewLocalStore(kbPath)
	if err != nil {
		return 0, fmt.Errorf("failed to create agent DB: %w", err)
	}
	defer agentDB.Close()

	kbSize := 0

	// Add base knowledge atoms for the agent first (always succeeds)
	baseAtoms := i.generateBaseKnowledgeAtoms(agent)
	for _, atom := range baseAtoms {
		if err := agentDB.StoreKnowledgeAtom(atom.Concept, atom.Content, atom.Confidence); err == nil {
			kbSize++
		}
	}

	// Research topics - use parallel research for efficiency
	if !i.config.SkipResearch && len(agent.Topics) > 0 {
		// Create a researcher for this specific agent
		agentResearcher := researcher.NewResearcherShard()
		if i.config.LLMClient != nil {
			agentResearcher.SetLLMClient(i.config.LLMClient)
		}
		if i.config.Context7APIKey != "" {
			agentResearcher.SetContext7APIKey(i.config.Context7APIKey)
		}
		agentResearcher.SetLocalDB(agentDB)

		fmt.Printf("     Researching %d topics for %s...\n", len(agent.Topics), agent.Name)

		// Use parallel topic research for efficiency
		result, err := agentResearcher.ResearchTopicsParallel(ctx, agent.Topics)
		if err != nil {
			fmt.Printf("     Warning: Research for %s had issues: %v\n", agent.Name, err)
		} else if result != nil {
			kbSize += len(result.Atoms)
			fmt.Printf("     Gathered %d knowledge atoms for %s\n", len(result.Atoms), agent.Name)
		}
	} else if i.config.SkipResearch {
		fmt.Printf("     Skipping research for %s (--skip-research)\n", agent.Name)
	}

	return kbSize, nil
}

// generateBaseKnowledgeAtoms generates foundational knowledge for an agent.
func (i *Initializer) generateBaseKnowledgeAtoms(agent RecommendedAgent) []struct {
	Concept    string
	Content    string
	Confidence float64
} {
	atoms := make([]struct {
		Concept    string
		Content    string
		Confidence float64
	}, 0)

	// Add agent identity
	atoms = append(atoms, struct {
		Concept    string
		Content    string
		Confidence float64
	}{
		Concept:    "agent_identity",
		Content:    fmt.Sprintf("I am %s, a specialist agent. %s", agent.Name, agent.Description),
		Confidence: 1.0,
	})

	// Add mission statement
	atoms = append(atoms, struct {
		Concept    string
		Content    string
		Confidence float64
	}{
		Concept:    "agent_mission",
		Content:    fmt.Sprintf("My primary mission is: %s", agent.Reason),
		Confidence: 1.0,
	})

	// Add expertise areas
	for _, topic := range agent.Topics {
		atoms = append(atoms, struct {
			Concept    string
			Content    string
			Confidence float64
		}{
			Concept:    "expertise_area",
			Content:    topic,
			Confidence: 0.9,
		})
	}

	return atoms
}

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
		config := core.ShardConfig{
			Name:          agent.Name,
			Type:          core.ShardTypePersistent,
			KnowledgePath: agent.KnowledgePath,
			Timeout:       30 * time.Minute,
			MemoryLimit:   10000,
			Permissions: []core.ShardPermission{
				core.PermissionReadFile,
				core.PermissionCodeGraph,
			},
			Model: core.ModelConfig{
				Capability: core.CapabilityBalanced,
			},
		}

		// Register the profile with shard manager
		i.shardMgr.DefineProfile(agent.Name, config)
	}
}

// saveAgentRegistry saves the agent registry to disk.
func (i *Initializer) saveAgentRegistry(path string, agents []CreatedAgent) error {
	registry := struct {
		Version   string         `json:"version"`
		CreatedAt time.Time      `json:"created_at"`
		Agents    []CreatedAgent `json:"agents"`
	}{
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
			Concepts: []struct{ Key, Value string }{
				{"role", "I am the Coder shard. I generate, modify, and refactor code following project conventions."},
				{"capability_generate", "I can generate new code files, functions, and modules."},
				{"capability_modify", "I can modify existing code with precise edits."},
				{"capability_refactor", "I can refactor code for better structure and readability."},
				{"safety_rule", "I always check impact radius before making changes."},
				{"safety_rule", "I never modify files without understanding their purpose."},
			},
		},
		{
			Name:        "reviewer",
			Description: "Code review and security analysis specialist",
			Topics:      []string{"code review", "security audit", "style checking", "best practices"},
			Concepts: []struct{ Key, Value string }{
				{"role", "I am the Reviewer shard. I review code for quality, security, and style issues."},
				{"capability_review", "I can perform comprehensive code reviews."},
				{"capability_security", "I can detect security vulnerabilities (OWASP top 10)."},
				{"capability_style", "I can check code style and consistency."},
				{"safety_rule", "Critical security issues block commit."},
				{"safety_rule", "I provide constructive feedback with suggestions."},
			},
		},
		{
			Name:        "tester",
			Description: "Testing and TDD loop specialist",
			Topics:      []string{"unit testing", "TDD", "test coverage", "test generation"},
			Concepts: []struct{ Key, Value string }{
				{"role", "I am the Tester shard. I manage tests, TDD loops, and coverage."},
				{"capability_generate", "I can generate unit tests for functions and modules."},
				{"capability_run", "I can execute tests and parse results."},
				{"capability_tdd", "I can run TDD repair loops to fix failing tests."},
				{"safety_rule", "Tests must pass before code is considered complete."},
				{"safety_rule", "Coverage below goal triggers test generation."},
			},
		},
	}

	for _, shard := range coreShards {
		kbPath := filepath.Join(shardsDir, fmt.Sprintf("%s_knowledge.db", shard.Name))

		shardDB, err := store.NewLocalStore(kbPath)
		if err != nil {
			continue
		}

		atomCount := 0

		// Store shard identity
		if err := shardDB.StoreKnowledgeAtom("shard_identity", shard.Description, 1.0); err == nil {
			atomCount++
		}

		// Store concepts
		for _, concept := range shard.Concepts {
			if err := shardDB.StoreKnowledgeAtom(concept.Key, concept.Value, 0.95); err == nil {
				atomCount++
			}
		}

		// Store project context
		if err := shardDB.StoreKnowledgeAtom("project_language", profile.Language, 0.9); err == nil {
			atomCount++
		}
		if profile.Framework != "" && profile.Framework != "unknown" {
			if err := shardDB.StoreKnowledgeAtom("project_framework", profile.Framework, 0.9); err == nil {
				atomCount++
			}
		}

		// Research shard-specific topics if LLM available
		if i.config.LLMClient != nil && !i.config.SkipResearch {
			researcher := researcher.NewResearcherShard()
			researcher.SetLLMClient(i.config.LLMClient)
			if i.config.Context7APIKey != "" {
				researcher.SetContext7APIKey(i.config.Context7APIKey)
			}
			researcher.SetLocalDB(shardDB)

			// Research 1-2 topics per shard (quick)
			for j, topic := range shard.Topics {
				if j >= 2 {
					break
				}
				task := fmt.Sprintf("research docs: %s for %s (brief)", topic, profile.Language)
				researcher.Execute(ctx, task)
				atomCount += 5 // Approximate
			}
		}

		shardDB.Close()
		results[shard.Name] = atomCount
	}

	return results, nil
}
