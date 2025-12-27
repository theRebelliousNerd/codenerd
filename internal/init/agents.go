// Package init implements the "nerd init" cold-start initialization system.
package init

import (
	// coreshards removed - was only used by tool_generator
	"codenerd/internal/logging"
	// Domain shards removed - JIT clean loop handles research and tool generation:
	// "codenerd/internal/shards/researcher"
	// "codenerd/internal/shards/tool_generator"
	"codenerd/internal/store"
	"codenerd/internal/types"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// LOCAL TYPE STUBS (previously in deleted shard packages)
// =============================================================================
// Research functionality has been removed from /init.
// The JIT clean loop now handles research via prompt atoms and ConfigFactory.

// initKnowledgeAtom is a stub type for knowledge atoms.
type initKnowledgeAtom struct {
	Concept    string
	Content    string
	Title      string
	Confidence float64
	SourceURL  string
}

// initQualityMetrics holds research quality metrics.
type initQualityMetrics struct {
	Score  float64
	Rating string
}

// initResearchResult holds the result of research.
type initResearchResult struct {
	Atoms           []initKnowledgeAtom
	FallbackUsed    int
	FallbackReason  string
	AttemptsMade    int
	EffectiveTopics []string
}

const initFallbackNone = 0

// generateAgentPromptsYAML generates a prompts.yaml template for a Type B (persistent) agent.
// Creates .nerd/agents/{name}/prompts.yaml with identity, methodology, and domain knowledge atoms.
func (i *Initializer) generateAgentPromptsYAML(agent RecommendedAgent) error {
	// Create agent directory
	agentDir := filepath.Join(i.config.Workspace, ".nerd", "agents", strings.ToLower(agent.Name))
	if err := os.MkdirAll(agentDir, 0755); err != nil {
		return fmt.Errorf("failed to create agent directory: %w", err)
	}

	// Generate prompts.yaml path
	promptsPath := filepath.Join(agentDir, "prompts.yaml")

	// Format topics as comma-separated string
	topicsStr := strings.Join(agent.Topics, ", ")

	// Build domain expertise from topics
	domainExpertise := formatDomainExpertise(agent.Topics)

	// Lowercase agent name for stable IDs and directory naming
	agentNameLower := strings.ToLower(agent.Name)

	// Build the YAML template
	template := fmt.Sprintf(`# Prompt atoms for %[2]s
# These are loaded into the JIT prompt compiler when the agent is spawned.
# Edit this file to customize the agent's identity, methodology, and domain knowledge.

- id: "%[1]s/identity"
  category: "identity"
  subcategory: "%[1]s"
  priority: 100
  is_mandatory: true
  description: "Identity and mission for %[2]s"
  content_concise: |
    You are %[2]s, a specialist agent in the codeNERD ecosystem.
    Role: %[3]s
    Topics: %[5]s
  content_min: |
    You are %[2]s (%[3]s). Operate under the codeNERD kernel.
  content: |
    You are %[2]s, a specialist agent in the codeNERD ecosystem.

    ## Role
    %[3]s

    ## Domain Expertise
%[4]s

    ## Research Topics
    %[5]s

    ## Core Responsibilities
    - Provide expert guidance in your domain
    - Follow best practices and established patterns
    - Maintain high code quality standards
    - Integrate seamlessly with the codeNERD architecture

    ## Execution Mode
    You operate under the control of the codeNERD kernel. You receive structured tasks
    with clear objectives, focus patterns, and success criteria. Execute precisely.

- id: "%[1]s/methodology"
  category: "methodology"
  subcategory: "%[1]s"
  priority: 80
  is_mandatory: false
  depends_on: ["%[1]s/identity"]
  description: "Methodology and quality bar for %[2]s"
  content_concise: |
    - Understand context before acting
    - Consider edge cases and failure modes
    - Write clear, maintainable code
    - Verify with tests when feasible
  content_min: |
    Be precise, verify assumptions, and preserve correctness.
  content: |
    ## Methodology

    ### Analysis Approach
    - Understand the full context before acting
    - Consider edge cases and failure modes
    - Think through implications of changes

    ### Implementation Standards
    - Follow language idioms and conventions
    - Write clear, maintainable code
    - Include comprehensive error handling
    - Document non-obvious decisions

    ### Quality Assurance
    - Verify assumptions before proceeding
    - Test critical paths
    - Consider performance implications
    - Ensure backward compatibility when applicable

- id: "%[1]s/domain"
  category: "domain"
  subcategory: "%[1]s"
  priority: 70
  is_mandatory: false
  depends_on: ["%[1]s/identity", "%[1]s/methodology"]
  description: "Domain knowledge, pitfalls, and references for %[2]s"
  content_concise: |
    Domain focus: %[3]s
    Topics: %[5]s
  content_min: |
    Apply domain best practices for: %[5]s
  content: |
    ## Domain-Specific Knowledge

    ### Key Concepts
    [Add specific concepts, patterns, or frameworks relevant to this domain]

    ### Common Pitfalls
    [Add known issues, gotchas, or anti-patterns to avoid]

    ### Best Practices
    [Add domain-specific best practices and guidelines]

    ### Resources
    Research Topics: %[5]s

    [Add additional references, documentation links, or learning resources]
`,
		agentNameLower,    // 1: stable id prefix
		agent.Name,        // 2: display name
		agent.Description, // 3: role/description
		domainExpertise,   // 4: domain expertise bullets
		topicsStr,         // 5: topics
	)

	// Write the template
	if err := os.WriteFile(promptsPath, []byte(template), 0644); err != nil {
		return fmt.Errorf("failed to write prompts.yaml: %w", err)
	}

	logging.Boot("Generated prompts.yaml for %s at %s", agent.Name, promptsPath)
	return nil
}

// formatDomainExpertise formats the topics as a bulleted list for the identity atom.
func formatDomainExpertise(topics []string) string {
	if len(topics) == 0 {
		return "    - General expertise"
	}

	var lines []string
	for _, topic := range topics {
		lines = append(lines, fmt.Sprintf("    - %s", topic))
	}
	return strings.Join(lines, "\n")
}

// AgentRegistry represents the persisted agent registry structure.
type AgentRegistry struct {
	Version   string         `json:"version"`
	CreatedAt time.Time      `json:"created_at"`
	Agents    []CreatedAgent `json:"agents"`
}

// KnowledgeBaseStats tracks statistics for KB upgrade operations.
type KnowledgeBaseStats struct {
	NewAtoms      int
	ExistingAtoms int
	SkippedAtoms  int
	TotalAtoms    int

	// Quality metrics from research
	QualityScore  float64
	QualityRating string
}

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

	// Assign tools to all agents based on their type and project language
	for idx := range agents {
		tools, prefs := GetToolsForAgentType(agents[idx].Name, profile.Language)
		agents[idx].Tools = tools
		agents[idx].ToolPreferences = prefs
	}

	return agents
}

// loadExistingAgentRegistry loads the agent registry from .nerd/agents.json if it exists.
// Returns nil map if the file doesn't exist (new installation).
func (i *Initializer) loadExistingAgentRegistry(nerdDir string) (map[string]CreatedAgent, error) {
	registryPath := filepath.Join(nerdDir, "agents.json")

	data, err := os.ReadFile(registryPath)
	if err != nil {
		if os.IsNotExist(err) {
			logging.Boot("No existing agent registry found at %s (new installation)", registryPath)
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read agent registry: %w", err)
	}

	var registry AgentRegistry
	if err := json.Unmarshal(data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse agent registry: %w", err)
	}

	// Convert to map for easy lookup
	agentMap := make(map[string]CreatedAgent)
	for _, agent := range registry.Agents {
		agentMap[agent.Name] = agent
	}

	logging.Boot("Loaded existing agent registry with %d agents", len(agentMap))
	return agentMap, nil
}

// agentCreationResult holds the result of creating a single agent KB.
type agentCreationResult struct {
	Agent       CreatedAgent
	KBSize      int
	Stats       KnowledgeBaseStats
	KBPath      string
	UpgradeMode bool
	Error       error
}

// createType3Agents creates the knowledge bases and registers Type 3 agents.
// In upgrade mode (--force with existing KB), it appends new knowledge rather than overwriting.
// Uses parallel creation with a worker pool for improved performance.
func (i *Initializer) createType3Agents(ctx context.Context, nerdDir string, agents []RecommendedAgent, result *InitResult) ([]CreatedAgent, map[string]int) {
	created := make([]CreatedAgent, 0)
	kbSizes := make(map[string]int)

	shardsDir := filepath.Join(nerdDir, "shards")

	// Load existing agent registry for upgrade detection
	existingAgents, err := i.loadExistingAgentRegistry(nerdDir)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to load existing agent registry: %v", err))
		existingAgents = nil
	}

	// Use parallel creation for better performance (3 concurrent workers)
	const maxWorkers = 3
	if len(agents) > 1 {
		results := i.createAgentsParallel(ctx, shardsDir, agents, existingAgents, maxWorkers)

		for _, res := range results {
			if res.Error != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create KB for %s: %v", res.Agent.Name, res.Error))
				i.sendAgentProgress(res.Agent.Name, res.Agent.Type, "failed", 0)
				continue
			}

			kbSizes[res.Agent.Name] = res.KBSize
			created = append(created, res.Agent)

			if !res.UpgradeMode {
				result.FilesCreated = append(result.FilesCreated, res.KBPath)
			}

			// Log result
			if res.UpgradeMode {
				fmt.Printf("     + %s upgraded (added %d new, skipped %d existing, total %d atoms)\n",
					res.Agent.Name, res.Stats.NewAtoms, res.Stats.SkippedAtoms, res.Stats.TotalAtoms)
			} else {
				fmt.Printf("     + %s ready (%d knowledge atoms)\n", res.Agent.Name, res.KBSize)
			}
		}

		return created, kbSizes
	}

	// Sequential fallback for single agent
	for idx, agent := range agents {
		progress := 0.55 + (float64(idx)/float64(len(agents)))*0.25
		i.sendProgress("kb_creation", fmt.Sprintf("Creating %s...", agent.Name), progress)
		i.sendAgentProgress(agent.Name, agent.Type, "creating", 0)

		kbPath := filepath.Join(shardsDir, fmt.Sprintf("%s_knowledge.db", strings.ToLower(agent.Name)))

		upgradeMode := false
		var existingAtomCount int
		if _, statErr := os.Stat(kbPath); statErr == nil {
			upgradeMode = true
			existingAtomCount = i.getExistingAtomCount(kbPath)
			logging.Boot("Upgrading %s (existing KB: %d atoms)", agent.Name, existingAtomCount)
			fmt.Printf("   Upgrading %s knowledge base (existing: %d atoms)...\n", agent.Name, existingAtomCount)
		} else {
			logging.Boot("Creating fresh %s knowledge base", agent.Name)
			fmt.Printf("   Creating %s knowledge base...\n", agent.Name)
		}

		stats, err := i.createAgentKnowledgeBase(ctx, kbPath, agent, upgradeMode)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create KB for %s: %v", agent.Name, err))
			i.sendAgentProgress(agent.Name, agent.Type, "failed", 0)
			continue
		}

		// Generate prompts.yaml for the agent (only for new agents, not upgrades)
		if !upgradeMode {
			if promptErr := i.generateAgentPromptsYAML(agent); promptErr != nil {
				logging.Boot("Warning: failed to generate prompts.yaml for %s: %v", agent.Name, promptErr)
			}
		}

		totalKBSize := stats.TotalAtoms
		kbSizes[agent.Name] = totalKBSize
		i.sendAgentProgress(agent.Name, agent.Type, "ready", totalKBSize)

		creationTime := time.Now()
		if existingAgent, exists := existingAgents[agent.Name]; exists && upgradeMode {
			creationTime = existingAgent.CreatedAt
		}

		createdAgent := CreatedAgent{
			Name:            agent.Name,
			Type:            agent.Type,
			KnowledgePath:   kbPath,
			KBSize:          totalKBSize,
			CreatedAt:       creationTime,
			Status:          "ready",
			Tools:           agent.Tools,
			ToolPreferences: agent.ToolPreferences,
			QualityScore:    stats.QualityScore,
			QualityRating:   stats.QualityRating,
		}
		created = append(created, createdAgent)

		if !upgradeMode {
			result.FilesCreated = append(result.FilesCreated, kbPath)
		}

		if upgradeMode {
			fmt.Printf("     + %s upgraded (added %d new, skipped %d existing, total %d atoms)\n",
				agent.Name, stats.NewAtoms, stats.SkippedAtoms, stats.TotalAtoms)
		} else if stats.QualityScore > 0 {
			fmt.Printf("     + %s ready (%d atoms, Quality: %.0f%% - %s)\n",
				agent.Name, totalKBSize, stats.QualityScore, stats.QualityRating)
		} else {
			fmt.Printf("     + %s ready (%d knowledge atoms)\n", agent.Name, totalKBSize)
		}
	}

	return created, kbSizes
}

// createAgentsParallel creates agent knowledge bases concurrently using a worker pool.
func (i *Initializer) createAgentsParallel(ctx context.Context, shardsDir string, agents []RecommendedAgent, existingAgents map[string]CreatedAgent, maxWorkers int) []agentCreationResult {
	results := make([]agentCreationResult, len(agents))
	var wg sync.WaitGroup
	sem := make(chan struct{}, maxWorkers)

	fmt.Printf("   Creating %d agent KBs in parallel (max %d workers)...\n", len(agents), maxWorkers)

	for idx, agent := range agents {
		wg.Add(1)
		go func(idx int, agent RecommendedAgent) {
			defer wg.Done()

			// Acquire semaphore
			sem <- struct{}{}
			defer func() { <-sem }()

			// Check context cancellation
			select {
			case <-ctx.Done():
				results[idx] = agentCreationResult{
					Agent: CreatedAgent{Name: agent.Name, Type: agent.Type},
					Error: ctx.Err(),
				}
				return
			default:
			}

			kbPath := filepath.Join(shardsDir, fmt.Sprintf("%s_knowledge.db", strings.ToLower(agent.Name)))

			// Check upgrade mode
			upgradeMode := false
			if _, statErr := os.Stat(kbPath); statErr == nil {
				upgradeMode = true
				existingCount := i.getExistingAtomCount(kbPath)
				logging.Boot("Parallel: Upgrading %s (existing KB: %d atoms)", agent.Name, existingCount)
			} else {
				logging.Boot("Parallel: Creating fresh %s knowledge base", agent.Name)
			}

			// Create/upgrade knowledge base
			stats, err := i.createAgentKnowledgeBase(ctx, kbPath, agent, upgradeMode)
			if err != nil {
				results[idx] = agentCreationResult{
					Agent: CreatedAgent{Name: agent.Name, Type: agent.Type},
					Error: err,
				}
				return
			}

			// Generate prompts.yaml for the agent (only for new agents, not upgrades)
			if !upgradeMode {
				if promptErr := i.generateAgentPromptsYAML(agent); promptErr != nil {
					logging.Boot("Warning: failed to generate prompts.yaml for %s: %v", agent.Name, promptErr)
				}
			}

			// Determine creation time
			creationTime := time.Now()
			if existingAgent, exists := existingAgents[agent.Name]; exists && upgradeMode {
				creationTime = existingAgent.CreatedAt
			}

			results[idx] = agentCreationResult{
				Agent: CreatedAgent{
					Name:            agent.Name,
					Type:            agent.Type,
					KnowledgePath:   kbPath,
					KBSize:          stats.TotalAtoms,
					CreatedAt:       creationTime,
					Status:          "ready",
					Tools:           agent.Tools,
					ToolPreferences: agent.ToolPreferences,
					QualityScore:    stats.QualityScore,
					QualityRating:   stats.QualityRating,
				},
				KBSize:      stats.TotalAtoms,
				Stats:       stats,
				KBPath:      kbPath,
				UpgradeMode: upgradeMode,
				Error:       nil,
			}

			i.sendAgentProgress(agent.Name, agent.Type, "ready", stats.TotalAtoms)
		}(idx, agent)
	}

	wg.Wait()
	return results
}

// getExistingAtomCount returns the number of atoms in an existing KB.
func (i *Initializer) getExistingAtomCount(kbPath string) int {
	db, err := store.NewLocalStore(kbPath)
	if err != nil {
		return 0
	}
	defer db.Close()

	atoms, err := db.GetAllKnowledgeAtoms()
	if err != nil {
		return 0
	}
	return len(atoms)
}

// createAgentKnowledgeBase creates or upgrades the SQLite knowledge base for an agent.
// If upgradeMode is true, it appends new knowledge atoms without reinitializing the schema.
func (i *Initializer) createAgentKnowledgeBase(ctx context.Context, kbPath string, agent RecommendedAgent, upgradeMode bool) (KnowledgeBaseStats, error) {
	stats := KnowledgeBaseStats{}

	// Open the database (NewLocalStore handles schema creation idempotently)
	agentDB, err := store.NewLocalStore(kbPath)
	if err != nil {
		return stats, fmt.Errorf("failed to open agent DB: %w", err)
	}
	if err := i.ensureEmbeddingEngine(); err != nil {
		return stats, err
	}
	agentDB.SetEmbeddingEngine(i.embedEngine)
	defer agentDB.Close()

	// In upgrade mode, get existing atoms for deduplication
	var existingHashes map[string]bool
	if upgradeMode {
		existingAtoms, err := agentDB.GetAllKnowledgeAtoms()
		if err != nil {
			return stats, fmt.Errorf("failed to get existing atoms: %w", err)
		}
		existingHashes = buildAtomHashSet(existingAtoms)
		stats.ExistingAtoms = len(existingAtoms)
		logging.Boot("Upgrade mode: found %d existing atoms in %s", stats.ExistingAtoms, agent.Name)
	} else {
		existingHashes = make(map[string]bool)

		// Inherit shared knowledge pool for new agents (not in upgrade mode)
		sharedKBPath := GetSharedKnowledgePath(i.config.Workspace)
		if SharedKnowledgePoolExists(i.config.Workspace) {
			if inheritErr := InheritSharedKnowledge(agentDB, sharedKBPath); inheritErr != nil {
				logging.Boot("Warning: failed to inherit shared knowledge for %s: %v", agent.Name, inheritErr)
			} else {
				// Re-fetch existing hashes after inheritance
				inheritedAtoms, _ := agentDB.GetAllKnowledgeAtoms()
				existingHashes = buildAtomHashSet(inheritedAtoms)
				stats.NewAtoms += len(inheritedAtoms)
				logging.Boot("Inherited %d shared atoms for %s", len(inheritedAtoms), agent.Name)
			}
		}
	}

	// Add base knowledge atoms for the agent
	baseAtoms := i.generateBaseKnowledgeAtoms(agent)
	for _, atom := range baseAtoms {
		added, err := appendKnowledgeAtom(agentDB, atom.Concept, atom.Content, atom.Confidence, existingHashes)
		if err != nil {
			logging.Boot("Warning: failed to store base atom for %s: %v", agent.Name, err)
			continue
		}
		if added {
			stats.NewAtoms++
		} else {
			stats.SkippedAtoms++
		}
	}

	// Research topics - STUBBED OUT
	// =========================================================================
	// Research functionality has been removed from /init as part of JIT refactor.
	// The JIT clean loop now handles research via:
	// - Prompt atoms in internal/prompt/atoms/
	// - ConfigFactory providing tool sets per intent
	// - session.Executor with /researcher persona
	// =========================================================================
	if !i.config.SkipResearch && len(agent.Topics) > 0 {
		fmt.Printf("     Research disabled (JIT refactor) - using base atoms only for %s\n", agent.Name)
		// Set default quality metrics
		stats.QualityScore = 50.0
		stats.QualityRating = "Basic"
	} else if i.config.SkipResearch {
		fmt.Printf("     Skipping research for %s (--skip-research)\n", agent.Name)
	}

	// Calculate total atoms
	finalAtoms, _ := agentDB.GetAllKnowledgeAtoms()
	stats.TotalAtoms = len(finalAtoms)

	return stats, nil
}

// buildAtomHashSet creates a set of content hashes for existing atoms.
func buildAtomHashSet(atoms []store.KnowledgeAtom) map[string]bool {
	hashes := make(map[string]bool)
	for _, atom := range atoms {
		hash := computeAtomHash(atom.Concept, atom.Content)
		hashes[hash] = true
	}
	return hashes
}

// computeAtomHash generates a unique hash for a knowledge atom based on concept and content.
func computeAtomHash(concept, content string) string {
	combined := concept + "::" + content
	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes for brevity
}

// appendKnowledgeAtom adds a knowledge atom if it doesn't already exist.
// Returns true if the atom was added, false if it was skipped (duplicate).
func appendKnowledgeAtom(db *store.LocalStore, concept, content string, confidence float64, existingHashes map[string]bool) (bool, error) {
	hash := computeAtomHash(concept, content)

	// Check if this atom already exists
	if existingHashes[hash] {
		return false, nil
	}

	// Store the new atom
	if err := db.StoreKnowledgeAtom(concept, content, confidence); err != nil {
		return false, err
	}

	// Add to hash set to prevent duplicates within this session
	existingHashes[hash] = true
	return true, nil
}

// filterTopicsNeedingResearch checks existing atoms and returns only topics that lack coverage.
// A topic is considered "covered" if there are >= minAtomsPerTopic atoms with matching concepts.
// This prevents redundant Context7 API calls during /init --force upgrades.
func filterTopicsNeedingResearch(existingAtoms []store.KnowledgeAtom, topics []string, minAtomsPerTopic int) []string {
	if len(existingAtoms) == 0 {
		return topics // No existing atoms, research all topics
	}

	// Build a map of topic -> atom count by checking if atom concepts contain topic keywords
	topicCoverage := make(map[string]int)
	for _, topic := range topics {
		topicCoverage[topic] = 0
	}

	// Normalize topic keywords for matching
	topicKeywords := make(map[string][]string)
	for _, topic := range topics {
		// Split topic into keywords (e.g., "go concurrency" -> ["go", "concurrency"])
		keywords := strings.Fields(strings.ToLower(topic))
		topicKeywords[topic] = keywords
	}

	// Count atoms that match each topic
	// Skip inherited atoms from shared pool as they inflate coverage falsely
	for _, atom := range existingAtoms {
		// Skip inherited atoms - they don't represent genuine topic research
		if strings.HasPrefix(atom.Concept, "inherited:") {
			continue
		}
		// Skip base identity/mission atoms - they're boilerplate
		if atom.Concept == "agent_identity" || atom.Concept == "agent_mission" {
			continue
		}

		conceptLower := strings.ToLower(atom.Concept)
		contentLower := strings.ToLower(atom.Content)

		for topic, keywords := range topicKeywords {
			matchCount := 0
			for _, kw := range keywords {
				if strings.Contains(conceptLower, kw) || strings.Contains(contentLower, kw) {
					matchCount++
				}
			}
			// Require at least 2/3 of keywords to match for topic relevance (stricter than 50%)
			// This prevents broad matches like "go" matching everything
			requiredMatches := (len(keywords)*2 + 2) / 3 // ~67% threshold
			if matchCount >= requiredMatches {
				topicCoverage[topic]++
			}
		}
	}

	// Filter to topics needing more research
	needsResearch := make([]string, 0)
	for _, topic := range topics {
		coverage := topicCoverage[topic]
		if coverage < minAtomsPerTopic {
			needsResearch = append(needsResearch, topic)
			logging.Boot("Topic '%s' needs research (current coverage: %d atoms, min: %d)", topic, coverage, minAtomsPerTopic)
		} else {
			logging.Boot("Topic '%s' has sufficient coverage (%d atoms), skipping", topic, coverage)
		}
	}

	return needsResearch
}

// convertStoreAtomsToInitAtoms converts store.KnowledgeAtom to initKnowledgeAtom.
// STUB: Research functionality removed as part of JIT refactor.
func convertStoreAtomsToInitAtoms(storeAtoms []store.KnowledgeAtom) []initKnowledgeAtom {
	atoms := make([]initKnowledgeAtom, 0, len(storeAtoms))
	for _, sa := range storeAtoms {
		atoms = append(atoms, initKnowledgeAtom{
			Concept:    sa.Concept,
			Content:    sa.Content,
			Title:      sa.Concept,
			Confidence: sa.Confidence,
			SourceURL:  "",
		})
	}
	return atoms
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

		// Research shard-specific topics - STUBBED OUT
		// =========================================================================
		// Research functionality removed as part of JIT refactor.
		// The JIT clean loop now handles research via prompt atoms.
		// =========================================================================
		if i.config.LLMClient != nil && !i.config.SkipResearch {
			logging.Boot("Research disabled (JIT refactor) for core shard %s", shard.Name)
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

// generateProjectTools generates tools based on detected technologies during init.
// =========================================================================
// Tool generation via ToolGenerator shard has been removed as part of JIT refactor.
// The Ouroboros system now handles tool generation through VirtualStore.
// This function is stubbed to preserve the interface.
// =========================================================================
func (i *Initializer) generateProjectTools(ctx context.Context, nerdDir string, profile ProjectProfile) ([]string, error) {
	generatedTools := make([]string, 0)

	// Determine which tools to generate based on project profile
	toolDefs := i.determineRequiredTools(profile)

	if len(toolDefs) == 0 {
		return generatedTools, nil
	}

	fmt.Printf("\n[tools] Tool generation disabled (JIT refactor) - %d tools would be generated\n", len(toolDefs))
	fmt.Println("   Tools are now generated on-demand via Ouroboros/VirtualStore")

	// Log which tools would have been generated
	for _, toolDef := range toolDefs {
		logging.Boot("Tool definition available: %s - %s", toolDef.Name, toolDef.Purpose)
	}

	return generatedTools, nil
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
