// Package init implements the "nerd init" cold-start initialization system.
// This handles the first-time setup of codeNERD in a new project, creating
// the .nerd/ directory structure, project profile, and initial knowledge base.
//
// The initialization process follows Cortex 1.5.0 §9.0 Dynamic Shard Configuration:
// 1. Create .nerd/ directory structure
// 2. Deep scan the codebase for project profile
// 3. Kick off Researcher shard to analyze what Type 3 agents are needed
// 4. Create knowledge bases for each Type 3 agent
// 5. Auto-spawn Type 3 persistent agents
// 6. Enable dynamic agent calling from main kernel
package init

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/shards"
	"codenerd/internal/store"
	"codenerd/internal/world"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// InitProgress represents a progress update during initialization.
type InitProgress struct {
	Phase       string  // Current phase name
	Message     string  // Human-readable status message
	Percent     float64 // 0.0 - 1.0 completion percentage
	IsError     bool    // True if this is an error message
	AgentUpdate *AgentCreationUpdate
}

// AgentCreationUpdate provides details about agent creation progress.
type AgentCreationUpdate struct {
	AgentName   string
	AgentType   string
	Status      string // "creating", "researching", "ready", "failed"
	KBSize      int    // Knowledge base size (facts/atoms)
}

// RecommendedAgent represents an agent recommended by the Researcher.
type RecommendedAgent struct {
	Name          string   `json:"name"`
	Type          string   `json:"type"`           // Type 3 category
	Description   string   `json:"description"`
	Topics        []string `json:"topics"`         // Research topics for KB
	Permissions   []string `json:"permissions"`
	Priority      int      `json:"priority"`       // Higher = more important
	Reason        string   `json:"reason"`         // Why this agent is needed
}

// InitConfig holds configuration for initialization.
type InitConfig struct {
	Workspace       string
	LLMClient       perception.LLMClient
	ShardManager    *core.ShardManager // Shard manager for agent spawning
	Interactive     bool               // Whether to prompt user for preferences
	Timeout         time.Duration      // Maximum time for initialization
	SkipResearch    bool               // Skip deep research phase (faster init)
	SkipAgentCreate bool               // Skip Type 3 agent creation
	PreferenceHints []string           // User-provided hints about preferences
	ProgressChan    chan InitProgress  // Channel for progress updates
}

// DefaultInitConfig returns sensible defaults.
func DefaultInitConfig(workspace string) InitConfig {
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	return InitConfig{
		Workspace:    workspace,
		Interactive:  true,
		Timeout:      5 * time.Minute,
		SkipResearch: false,
	}
}

// ProjectProfile represents the persisted project identity.
type ProjectProfile struct {
	// Identity
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Technical
	Language     string   `json:"language"`
	Framework    string   `json:"framework,omitempty"`
	BuildSystem  string   `json:"build_system,omitempty"`
	Architecture string   `json:"architecture,omitempty"`
	Patterns     []string `json:"patterns,omitempty"`

	// Dependencies
	Dependencies []DependencyInfo `json:"dependencies,omitempty"`

	// Paths
	EntryPoints    []string `json:"entry_points,omitempty"`
	TestDirectories []string `json:"test_directories,omitempty"`
	ConfigFiles    []string `json:"config_files,omitempty"`

	// Stats
	FileCount   int `json:"file_count"`
	DirectoryCount int `json:"directory_count"`
}

// DependencyInfo represents a project dependency.
type DependencyInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type"` // direct, dev, indirect
}

// UserPreferences represents user coding preferences (learned via autopoiesis).
type UserPreferences struct {
	// Code style
	TestStyle        string `json:"test_style,omitempty"`         // "table_driven", "subtest", etc.
	ErrorHandling    string `json:"error_handling,omitempty"`     // "wrap", "sentinel", etc.
	NamingConvention string `json:"naming_convention,omitempty"`  // "camelCase", "snake_case"

	// Behavior
	CommitStyle      string `json:"commit_style,omitempty"`       // "conventional", "descriptive"
	BranchStrategy   string `json:"branch_strategy,omitempty"`    // "gitflow", "trunk"

	// Safety
	RequireTests     bool   `json:"require_tests"`                // Require tests before commits
	RequireReview    bool   `json:"require_review"`               // Require review before merges

	// Communication
	Verbosity        string `json:"verbosity,omitempty"`          // "concise", "detailed"
	ExplanationLevel string `json:"explanation_level,omitempty"`  // "beginner", "expert"
}

// InitResult represents the result of initialization.
type InitResult struct {
	Success        bool              `json:"success"`
	Profile        ProjectProfile    `json:"profile"`
	Preferences    UserPreferences   `json:"preferences"`
	NerdDir        string            `json:"nerd_dir"`
	FilesCreated   []string          `json:"files_created"`
	FactsGenerated int               `json:"facts_generated"`
	Duration       time.Duration     `json:"duration"`
	Warnings       []string          `json:"warnings,omitempty"`

	// Type 3 Agent Creation Results
	RecommendedAgents []RecommendedAgent `json:"recommended_agents,omitempty"`
	CreatedAgents     []CreatedAgent     `json:"created_agents,omitempty"`
	AgentKBs          map[string]int     `json:"agent_knowledge_bases,omitempty"` // agent name -> KB size
}

// CreatedAgent represents a Type 3 agent that was created during init.
type CreatedAgent struct {
	Name          string    `json:"name"`
	Type          string    `json:"type"`
	KnowledgePath string    `json:"knowledge_path"`
	KBSize        int       `json:"kb_size"`
	CreatedAt     time.Time `json:"created_at"`
	Status        string    `json:"status"` // "ready", "partial", "failed"
}

// Initializer handles the cold-start initialization process.
type Initializer struct {
	config       InitConfig
	researcher   *shards.ResearcherShard
	scanner      *world.Scanner
	localDB      *store.LocalStore
	shardMgr     *core.ShardManager
	kernel       *core.RealKernel

	// Concurrency
	mu           sync.RWMutex
	createdAgents []CreatedAgent
}

// NewInitializer creates a new initializer.
func NewInitializer(config InitConfig) *Initializer {
	init := &Initializer{
		config:        config,
		researcher:    shards.NewResearcherShard(),
		scanner:       world.NewScanner(),
		kernel:        core.NewRealKernel(),
		createdAgents: make([]CreatedAgent, 0),
	}

	// Use provided shard manager or create new one
	if config.ShardManager != nil {
		init.shardMgr = config.ShardManager
	} else {
		init.shardMgr = core.NewShardManager()
	}

	return init
}

// Initialize performs the full initialization process.
// This implements Cortex 1.5.0 §9.0 Dynamic Shard Configuration:
// 1. Create .nerd/ directory structure
// 2. Deep scan the codebase for project profile
// 3. Generate initial Mangle facts from codebase analysis
// 4. Kick off Researcher shard to analyze what Type 3 agents are needed
// 5. Create knowledge bases for each Type 3 agent
// 6. Auto-spawn Type 3 persistent agents
// 7. Register agents with shard manager for dynamic calling
func (i *Initializer) Initialize(ctx context.Context) (*InitResult, error) {
	startTime := time.Now()
	result := &InitResult{
		FilesCreated:   make([]string, 0),
		Warnings:       make([]string, 0),
		AgentKBs:       make(map[string]int),
		CreatedAgents:  make([]CreatedAgent, 0),
	}

	i.sendProgress("setup", "Initializing codeNERD...", 0.0)
	fmt.Println("🚀 Initializing codeNERD...")
	fmt.Printf("   Workspace: %s\n\n", i.config.Workspace)

	// =========================================================================
	// PHASE 1: Directory Structure & Database Setup
	// =========================================================================
	i.sendProgress("setup", "Creating directory structure...", 0.05)

	nerdDir, err := i.createDirectoryStructure()
	if err != nil {
		return nil, fmt.Errorf("failed to create directory structure: %w", err)
	}
	result.NerdDir = nerdDir
	fmt.Println("✓ Created .nerd/ directory structure")

	// Initialize local database
	dbPath := filepath.Join(nerdDir, "knowledge.db")
	i.localDB, err = store.NewLocalStore(dbPath)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to initialize database: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, dbPath)
		i.researcher.SetLocalDB(i.localDB)
		fmt.Println("✓ Initialized knowledge database")
	}

	// Set LLM client if provided
	if i.config.LLMClient != nil {
		i.researcher.SetLLMClient(i.config.LLMClient)
	}

	// =========================================================================
	// PHASE 2: Deep Codebase Scan
	// =========================================================================
	i.sendProgress("scanning", "Scanning codebase...", 0.10)
	fmt.Println("\n📊 Phase 2: Deep Codebase Scan")

	// Use the scanner for comprehensive file analysis
	scanResult, err := i.scanner.ScanDirectory(ctx, i.config.Workspace)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Codebase scan failed: %v", err))
	} else {
		fmt.Printf("   Scanned %d files in %d directories\n", scanResult.FileCount, scanResult.DirectoryCount)

		// Assert scan results as Mangle facts to kernel
		for _, fact := range scanResult.ToFacts() {
			_ = i.kernel.Assert(fact)
		}
	}

	// =========================================================================
	// PHASE 3: Run Researcher Shard for Analysis
	// =========================================================================
	i.sendProgress("analysis", "Running deep analysis...", 0.20)
	fmt.Println("\n🔬 Phase 3: Deep Analysis via Researcher Shard")

	researchTask := fmt.Sprintf("analyze codebase: %s", i.config.Workspace)
	summary, err := i.researcher.Execute(ctx, researchTask)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Codebase analysis failed: %v", err))
	} else {
		fmt.Println(summary)
	}

	// =========================================================================
	// PHASE 4: Build Project Profile
	// =========================================================================
	i.sendProgress("profile", "Building project profile...", 0.35)
	fmt.Println("\n📋 Phase 4: Building Project Profile")

	profile := i.buildProjectProfile()
	if scanResult != nil {
		profile.FileCount = scanResult.FileCount
		profile.DirectoryCount = scanResult.DirectoryCount
	}
	result.Profile = profile

	// Save profile to disk
	profilePath := filepath.Join(nerdDir, "profile.json")
	if err := i.saveProfile(profilePath, profile); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to save profile: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, profilePath)
		fmt.Println("✓ Saved project profile")
	}

	// =========================================================================
	// PHASE 5: Generate Initial Mangle Facts
	// =========================================================================
	i.sendProgress("facts", "Generating Mangle facts...", 0.45)
	fmt.Println("\n🧠 Phase 5: Generating Mangle Facts")

	factsPath := filepath.Join(nerdDir, "profile.gl")
	factsCount, err := i.generateFactsFile(factsPath, profile)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to generate facts: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, factsPath)
		result.FactsGenerated = factsCount
		fmt.Printf("✓ Generated %d Mangle facts\n", factsCount)
	}

	// =========================================================================
	// PHASE 6: Determine Required Type 3 Agents
	// =========================================================================
	i.sendProgress("agents", "Analyzing required agents...", 0.50)
	fmt.Println("\n🤖 Phase 6: Determining Required Type 3 Agents")

	recommendedAgents := i.determineRequiredAgents(profile)
	result.RecommendedAgents = recommendedAgents
	fmt.Printf("   Recommended %d Type 3 agents for this project\n", len(recommendedAgents))

	for _, agent := range recommendedAgents {
		fmt.Printf("   • %s: %s\n", agent.Name, agent.Reason)
	}

	// =========================================================================
	// PHASE 7: Create Knowledge Bases & Type 3 Agents
	// =========================================================================
	if !i.config.SkipAgentCreate && len(recommendedAgents) > 0 {
		i.sendProgress("kb_creation", "Creating agent knowledge bases...", 0.55)
		fmt.Println("\n📚 Phase 7: Creating Agent Knowledge Bases")

		createdAgents, agentKBs := i.createType3Agents(ctx, nerdDir, recommendedAgents, result)
		result.CreatedAgents = createdAgents
		result.AgentKBs = agentKBs

		// Register agents with shard manager for dynamic calling
		i.registerAgentsWithShardManager(createdAgents)

		fmt.Printf("   Created %d Type 3 agents with knowledge bases\n", len(createdAgents))
	}

	// =========================================================================
	// PHASE 8: Initialize Preferences
	// =========================================================================
	i.sendProgress("preferences", "Initializing preferences...", 0.85)
	fmt.Println("\n⚙️ Phase 8: Initializing Preferences")

	preferences := i.initPreferences()
	result.Preferences = preferences

	prefsPath := filepath.Join(nerdDir, "preferences.json")
	if err := i.savePreferences(prefsPath, preferences); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to save preferences: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, prefsPath)
		fmt.Println("✓ Initialized preferences")
	}

	// =========================================================================
	// PHASE 9: Create Session State
	// =========================================================================
	i.sendProgress("session", "Creating session state...", 0.90)

	sessionPath := filepath.Join(nerdDir, "session.json")
	if err := i.initSessionState(sessionPath); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to init session: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, sessionPath)
	}

	// =========================================================================
	// PHASE 10: Generate Agent Registry
	// =========================================================================
	i.sendProgress("registry", "Generating agent registry...", 0.95)

	registryPath := filepath.Join(nerdDir, "agents.json")
	if err := i.saveAgentRegistry(registryPath, result.CreatedAgents); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to save agent registry: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, registryPath)
	}

	// =========================================================================
	// COMPLETE
	// =========================================================================
	result.Success = true
	result.Duration = time.Since(startTime)
	i.sendProgress("complete", "Initialization complete!", 1.0)

	// Print summary
	i.printSummary(result, profile)

	return result, nil
}

// sendProgress sends a progress update if channel is configured.
func (i *Initializer) sendProgress(phase, message string, percent float64) {
	if i.config.ProgressChan != nil {
		select {
		case i.config.ProgressChan <- InitProgress{
			Phase:   phase,
			Message: message,
			Percent: percent,
		}:
		default:
			// Don't block if channel is full
		}
	}
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

	// If we have an LLM client and topics, research the topics
	if i.config.LLMClient != nil && !i.config.SkipResearch && len(agent.Topics) > 0 {
		// Create a researcher for this specific agent
		agentResearcher := shards.NewResearcherShard()
		agentResearcher.SetLLMClient(i.config.LLMClient)
		agentResearcher.SetLocalDB(agentDB)

		for _, topic := range agent.Topics {
			select {
			case <-ctx.Done():
				return kbSize, ctx.Err()
			default:
			}

			// Research the topic (limited scope for init)
			researchTask := fmt.Sprintf("research docs: %s (brief overview)", topic)
			_, err := agentResearcher.Execute(ctx, researchTask)
			if err != nil {
				// Log but don't fail - partial KB is acceptable
				continue
			}
			kbSize += 10 // Approximate atoms per topic
		}
	}

	// Add base knowledge atoms for the agent
	baseAtoms := i.generateBaseKnowledgeAtoms(agent)
	for _, atom := range baseAtoms {
		if err := agentDB.StoreKnowledgeAtom(atom.Concept, atom.Content, atom.Confidence); err == nil {
			kbSize++
		}
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

// printSummary prints the initialization summary.
func (i *Initializer) printSummary(result *InitResult, profile ProjectProfile) {
	fmt.Println("\n" + strings.Repeat("═", 60))
	fmt.Println("✅ INITIALIZATION COMPLETE")
	fmt.Println(strings.Repeat("═", 60))

	fmt.Printf("\n📁 Project: %s\n", profile.Name)
	fmt.Printf("   Language: %s\n", profile.Language)
	if profile.Framework != "" {
		fmt.Printf("   Framework: %s\n", profile.Framework)
	}
	fmt.Printf("   Architecture: %s\n", profile.Architecture)
	fmt.Printf("   Files: %d | Directories: %d\n", profile.FileCount, profile.DirectoryCount)

	fmt.Printf("\n🧠 Logic Kernel:\n")
	fmt.Printf("   Facts generated: %d\n", result.FactsGenerated)

	if len(result.CreatedAgents) > 0 {
		fmt.Printf("\n🤖 Type 3 Agents Created:\n")
		for _, agent := range result.CreatedAgents {
			fmt.Printf("   • %s (%d KB atoms) - %s\n", agent.Name, agent.KBSize, agent.Status)
		}
	}

	fmt.Printf("\n📂 Files Created: %d\n", len(result.FilesCreated))
	fmt.Printf("⏱️ Duration: %.2fs\n", result.Duration.Seconds())

	if len(result.Warnings) > 0 {
		fmt.Println("\n⚠️ Warnings:")
		for _, w := range result.Warnings {
			fmt.Printf("   - %s\n", w)
		}
	}

	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("💡 Next steps:")
	fmt.Println("   • Run `nerd chat` to start interactive session")
	fmt.Println("   • Use `/agents` to see available agents")
	fmt.Println("   • Use `/spawn <agent> <task>` to delegate tasks")
	fmt.Println(strings.Repeat("─", 60))
}

// createDirectoryStructure creates the .nerd/ directory and subdirectories.
func (i *Initializer) createDirectoryStructure() (string, error) {
	nerdDir := filepath.Join(i.config.Workspace, ".nerd")

	dirs := []string{
		nerdDir,
		filepath.Join(nerdDir, "shards"),    // Knowledge shards for specialists
		filepath.Join(nerdDir, "sessions"),  // Session history
		filepath.Join(nerdDir, "cache"),     // Temporary cache
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
`
	if err := os.WriteFile(gitignorePath, []byte(gitignoreContent), 0644); err != nil {
		return "", fmt.Errorf("failed to create .gitignore: %w", err)
	}

	return nerdDir, nil
}

// buildProjectProfile constructs the project profile from analysis.
func (i *Initializer) buildProjectProfile() ProjectProfile {
	profile := ProjectProfile{
		ProjectID: generateProjectID(i.config.Workspace),
		Name:      filepath.Base(i.config.Workspace),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Extract from researcher's analysis if available
	// The researcher shard stores results in its kernel
	if i.researcher != nil {
		// Try to get project type facts
		if kernel := i.researcher.GetKernel(); kernel != nil {
			// Query for project profile facts
			langFacts, _ := kernel.Query("project_language")
			if len(langFacts) > 0 && len(langFacts[0].Args) > 0 {
				profile.Language = cleanNameConstant(fmt.Sprintf("%v", langFacts[0].Args[0]))
			}

			fwFacts, _ := kernel.Query("project_framework")
			if len(fwFacts) > 0 && len(fwFacts[0].Args) > 0 {
				profile.Framework = cleanNameConstant(fmt.Sprintf("%v", fwFacts[0].Args[0]))
			}

			archFacts, _ := kernel.Query("project_architecture")
			if len(archFacts) > 0 && len(archFacts[0].Args) > 0 {
				profile.Architecture = cleanNameConstant(fmt.Sprintf("%v", archFacts[0].Args[0]))
			}

			// Count files
			fileFacts, _ := kernel.Query("file_topology")
			profile.FileCount = len(fileFacts)
		}
	}

	// Set defaults for any missing values
	if profile.Language == "" {
		profile.Language = "unknown"
	}
	if profile.Architecture == "" {
		profile.Architecture = "unknown"
	}

	return profile
}

// saveProfile writes the project profile to disk.
func (i *Initializer) saveProfile(path string, profile ProjectProfile) error {
	data, err := json.MarshalIndent(profile, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// generateFactsFile creates a Mangle facts file for the project.
func (i *Initializer) generateFactsFile(path string, profile ProjectProfile) (int, error) {
	var facts []string

	// Project identity facts
	facts = append(facts, fmt.Sprintf(`project_profile("%s", "%s", "%s").`,
		profile.ProjectID, profile.Name, profile.Description))

	if profile.Language != "" {
		facts = append(facts, fmt.Sprintf(`project_language(/%s).`, profile.Language))
	}

	if profile.Framework != "" {
		facts = append(facts, fmt.Sprintf(`project_framework(/%s).`, profile.Framework))
	}

	if profile.Architecture != "" {
		facts = append(facts, fmt.Sprintf(`project_architecture(/%s).`, profile.Architecture))
	}

	if profile.BuildSystem != "" {
		facts = append(facts, fmt.Sprintf(`build_system(/%s).`, profile.BuildSystem))
	}

	// Pattern facts
	for _, pattern := range profile.Patterns {
		facts = append(facts, fmt.Sprintf(`architectural_pattern(/%s).`, pattern))
	}

	// Entry point facts
	for _, entry := range profile.EntryPoints {
		facts = append(facts, fmt.Sprintf(`entry_point("%s").`, entry))
	}

	// Write facts file
	content := "# codeNERD Project Profile Facts\n"
	content += "# Generated by: nerd init\n"
	content += fmt.Sprintf("# Timestamp: %s\n\n", time.Now().Format(time.RFC3339))

	for _, fact := range facts {
		content += fact + "\n"
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return 0, err
	}

	return len(facts), nil
}

// initPreferences initializes user preferences.
func (i *Initializer) initPreferences() UserPreferences {
	prefs := UserPreferences{
		Verbosity:        "concise",
		ExplanationLevel: "intermediate",
		RequireTests:     false,
		RequireReview:    false,
	}

	// Apply hints if provided
	for _, hint := range i.config.PreferenceHints {
		switch hint {
		case "table_driven_tests":
			prefs.TestStyle = "table_driven"
		case "conventional_commits":
			prefs.CommitStyle = "conventional"
		case "strict":
			prefs.RequireTests = true
			prefs.RequireReview = true
		case "beginner":
			prefs.ExplanationLevel = "beginner"
		case "expert":
			prefs.ExplanationLevel = "expert"
		}
	}

	return prefs
}

// savePreferences writes user preferences to disk.
func (i *Initializer) savePreferences(path string, prefs UserPreferences) error {
	data, err := json.MarshalIndent(prefs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// SessionState represents the current session state.
type SessionState struct {
	SessionID       string    `json:"session_id"`
	StartedAt       time.Time `json:"started_at"`
	LastActiveAt    time.Time `json:"last_active_at"`
	TurnCount       int       `json:"turn_count"`

	// Suspension state (for pause/resume)
	Suspended       bool      `json:"suspended"`
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	PendingQuestion string    `json:"pending_question,omitempty"`
	PendingOptions  []string  `json:"pending_options,omitempty"`

	// Context state
	ActiveStrategy  string    `json:"active_strategy,omitempty"`
	ActiveGoals     []string  `json:"active_goals,omitempty"`
	WorkingFacts    []string  `json:"working_facts,omitempty"`
}

// initSessionState creates the initial session state file.
func (i *Initializer) initSessionState(path string) error {
	state := SessionState{
		SessionID:    generateSessionID(),
		StartedAt:    time.Now(),
		LastActiveAt: time.Now(),
		TurnCount:    0,
		Suspended:    false,
	}

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// LoadProjectProfile loads the project profile from .nerd/profile.json
func LoadProjectProfile(workspace string) (*ProjectProfile, error) {
	path := filepath.Join(workspace, ".nerd", "profile.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var profile ProjectProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

// LoadPreferences loads user preferences from .nerd/preferences.json
func LoadPreferences(workspace string) (*UserPreferences, error) {
	path := filepath.Join(workspace, ".nerd", "preferences.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var prefs UserPreferences
	if err := json.Unmarshal(data, &prefs); err != nil {
		return nil, err
	}

	return &prefs, nil
}

// LoadSessionState loads the session state from .nerd/session.json
func LoadSessionState(workspace string) (*SessionState, error) {
	path := filepath.Join(workspace, ".nerd", "session.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var state SessionState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}

	return &state, nil
}

// SaveSessionState saves the session state to disk.
func SaveSessionState(workspace string, state *SessionState) error {
	path := filepath.Join(workspace, ".nerd", "session.json")
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// IsInitialized checks if the workspace has been initialized.
func IsInitialized(workspace string) bool {
	nerdDir := filepath.Join(workspace, ".nerd")
	profilePath := filepath.Join(nerdDir, "profile.json")

	if _, err := os.Stat(profilePath); err == nil {
		return true
	}
	return false
}

// Helper functions

func generateProjectID(workspace string) string {
	// Simple hash-based ID
	h := uint64(0)
	for _, c := range workspace {
		h = h*31 + uint64(c)
	}
	return fmt.Sprintf("proj_%x", h)[:16]
}

func generateSessionID() string {
	return fmt.Sprintf("sess_%d", time.Now().UnixNano())
}

func cleanNameConstant(s string) string {
	// Remove leading "/" from Mangle name constants
	if len(s) > 0 && s[0] == '/' {
		return s[1:]
	}
	return s
}
