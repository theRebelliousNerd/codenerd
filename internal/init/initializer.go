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
	AgentName string
	AgentType string
	Status    string // "creating", "researching", "ready", "failed"
	KBSize    int    // Knowledge base size (facts/atoms)
}

// RecommendedAgent represents an agent recommended by the Researcher.
type RecommendedAgent struct {
	Name        string   `json:"name"`
	Type        string   `json:"type"` // Type 3 category
	Description string   `json:"description"`
	Topics      []string `json:"topics"` // Research topics for KB
	Permissions []string `json:"permissions"`
	Priority    int      `json:"priority"` // Higher = more important
	Reason      string   `json:"reason"`   // Why this agent is needed
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
	Context7APIKey  string             // Context7 API key for LLM-optimized docs
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
	EntryPoints     []string `json:"entry_points,omitempty"`
	TestDirectories []string `json:"test_directories,omitempty"`
	ConfigFiles     []string `json:"config_files,omitempty"`

	// Stats
	FileCount      int `json:"file_count"`
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
	TestStyle        string `json:"test_style,omitempty"`        // "table_driven", "subtest", etc.
	ErrorHandling    string `json:"error_handling,omitempty"`    // "wrap", "sentinel", etc.
	NamingConvention string `json:"naming_convention,omitempty"` // "camelCase", "snake_case"

	// Behavior
	CommitStyle    string `json:"commit_style,omitempty"`    // "conventional", "descriptive"
	BranchStrategy string `json:"branch_strategy,omitempty"` // "gitflow", "trunk"

	// Safety
	RequireTests  bool `json:"require_tests"`  // Require tests before commits
	RequireReview bool `json:"require_review"` // Require review before merges

	// Communication
	Verbosity        string `json:"verbosity,omitempty"`         // "concise", "detailed"
	ExplanationLevel string `json:"explanation_level,omitempty"` // "beginner", "expert"
}

// InitResult represents the result of initialization.
type InitResult struct {
	Success        bool            `json:"success"`
	Profile        ProjectProfile  `json:"profile"`
	Preferences    UserPreferences `json:"preferences"`
	NerdDir        string          `json:"nerd_dir"`
	FilesCreated   []string        `json:"files_created"`
	FactsGenerated int             `json:"facts_generated"`
	Duration       time.Duration   `json:"duration"`
	Warnings       []string        `json:"warnings,omitempty"`

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
	config     InitConfig
	researcher *shards.ResearcherShard
	scanner    *world.Scanner
	localDB    *store.LocalStore
	shardMgr   *core.ShardManager
	kernel     *core.RealKernel

	// Concurrency
	mu            sync.RWMutex
	createdAgents []CreatedAgent
}

// NewInitializer creates a new initializer.
func NewInitializer(config InitConfig) *Initializer {
	researcher := shards.NewResearcherShard()
	// Set Context7 API key if configured
	if config.Context7APIKey != "" {
		researcher.SetContext7APIKey(config.Context7APIKey)
	}

	init := &Initializer{
		config:        config,
		researcher:    researcher,
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
	if config.LLMClient != nil {
		init.shardMgr.SetLLMClient(config.LLMClient)
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
		FilesCreated:  make([]string, 0),
		Warnings:      make([]string, 0),
		AgentKBs:      make(map[string]int),
		CreatedAgents: make([]CreatedAgent, 0),
	}

	i.sendProgress("setup", "Initializing codeNERD...", 0.0)
	fmt.Println("🚀 Initializing codeNERD...")
	fmt.Printf("   Workspace: %s\n\n", i.config.Workspace)

	// Ensure system shards are running before heavy lifting.
	if err := i.shardMgr.StartSystemShards(ctx); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to start system shards: %v", err))
	}

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
	// PHASE 7b: Create Codebase Knowledge Base
	// =========================================================================
	i.sendProgress("codebase_kb", "Creating codebase knowledge base...", 0.80)
	fmt.Println("\n📖 Phase 7b: Creating Codebase Knowledge Base")

	codebaseKBPath, codebaseAtoms, err := i.createCodebaseKnowledgeBase(ctx, nerdDir, profile, scanResult)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create codebase KB: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, codebaseKBPath)
		fmt.Printf("   ✓ Codebase KB ready (%d atoms)\n", codebaseAtoms)
	}

	// =========================================================================
	// PHASE 7c: Create Core Shard Knowledge Bases (Coder, Reviewer, Tester)
	// =========================================================================
	i.sendProgress("core_shards_kb", "Creating core shard knowledge bases...", 0.82)
	fmt.Println("\n🔧 Phase 7c: Creating Core Shard Knowledge Bases")

	coreShardKBs, err := i.createCoreShardKnowledgeBases(ctx, nerdDir, profile)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create core shard KBs: %v", err))
	} else {
		for name, atoms := range coreShardKBs {
			fmt.Printf("   ✓ %s KB ready (%d atoms)\n", strings.Title(name), atoms)
		}
	}

	// =========================================================================
	// PHASE 7d: Create Campaign Knowledge Base
	// =========================================================================
	i.sendProgress("campaign_kb", "Creating campaign knowledge base...", 0.84)
	fmt.Println("\n🎯 Phase 7d: Creating Campaign Knowledge Base")

	campaignKBPath, campaignAtoms, err := i.createCampaignKnowledgeBase(ctx, nerdDir, profile)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create campaign KB: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, campaignKBPath)
		fmt.Printf("   ✓ Campaign KB ready (%d atoms)\n", campaignAtoms)
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
		agentResearcher := shards.NewResearcherShard()
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
		filepath.Join(nerdDir, "shards"),   // Knowledge shards for specialists
		filepath.Join(nerdDir, "sessions"), // Session history
		filepath.Join(nerdDir, "cache"),    // Temporary cache
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

	// Fallback: file-based language detection if kernel didn't provide it
	if profile.Language == "" || profile.Language == "unknown" {
		profile.Language = i.detectLanguageFromFiles()
	}

	// Detect dependencies for agent recommendations
	profile.Dependencies = i.detectDependencies()

	// Set defaults for any missing values
	if profile.Language == "" {
		profile.Language = "unknown"
	}
	if profile.Architecture == "" {
		profile.Architecture = "unknown"
	}

	return profile
}

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

	// Helper to escape strings for Mangle
	escapeString := func(s string) string {
		s = strings.ReplaceAll(s, `\`, `\\`)
		s = strings.ReplaceAll(s, `"`, `\"`)
		s = strings.ReplaceAll(s, "\n", " ")
		s = strings.ReplaceAll(s, "\r", "")
		return s
	}

	// Project identity facts - use escaped strings
	facts = append(facts, fmt.Sprintf(`project_profile("%s", "%s", "%s").`,
		escapeString(profile.ProjectID),
		escapeString(profile.Name),
		escapeString(profile.Description)))

	// Use sanitized name constants for language/framework/architecture
	if profile.Language != "" && profile.Language != "unknown" {
		facts = append(facts, fmt.Sprintf(`project_language(/%s).`, sanitizeForMangle(profile.Language)))
	}

	if profile.Framework != "" && profile.Framework != "unknown" {
		facts = append(facts, fmt.Sprintf(`project_framework(/%s).`, sanitizeForMangle(profile.Framework)))
	}

	if profile.Architecture != "" && profile.Architecture != "unknown" {
		facts = append(facts, fmt.Sprintf(`project_architecture(/%s).`, sanitizeForMangle(profile.Architecture)))
	}

	if profile.BuildSystem != "" {
		facts = append(facts, fmt.Sprintf(`build_system(/%s).`, sanitizeForMangle(profile.BuildSystem)))
	}

	// Pattern facts - sanitize each pattern
	for _, pattern := range profile.Patterns {
		facts = append(facts, fmt.Sprintf(`architectural_pattern(/%s).`, sanitizeForMangle(pattern)))
	}

	// Entry point facts - use escaped strings for paths
	for _, entry := range profile.EntryPoints {
		facts = append(facts, fmt.Sprintf(`entry_point("%s").`, escapeString(entry)))
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
	SessionID    string    `json:"session_id"`
	StartedAt    time.Time `json:"started_at"`
	LastActiveAt time.Time `json:"last_active_at"`
	TurnCount    int       `json:"turn_count"`

	// Suspension state (for pause/resume)
	Suspended       bool       `json:"suspended"`
	SuspendedAt     *time.Time `json:"suspended_at,omitempty"`
	PendingQuestion string     `json:"pending_question,omitempty"`
	PendingOptions  []string   `json:"pending_options,omitempty"`

	// Context state
	ActiveStrategy string   `json:"active_strategy,omitempty"`
	ActiveGoals    []string `json:"active_goals,omitempty"`
	WorkingFacts   []string `json:"working_facts,omitempty"`

	// Conversation history (stored separately in sessions/ folder)
	HistoryFile string `json:"history_file,omitempty"`
}

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role    string    `json:"role"`    // "user" or "assistant"
	Content string    `json:"content"`
	Time    time.Time `json:"time"`
}

// SessionHistory represents the full conversation history for a session.
type SessionHistory struct {
	SessionID string        `json:"session_id"`
	Messages  []ChatMessage `json:"messages"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
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

// SaveSessionHistory saves the conversation history to the sessions folder.
func SaveSessionHistory(workspace string, sessionID string, messages []ChatMessage) error {
	sessionsDir := filepath.Join(workspace, ".nerd", "sessions")
	if err := os.MkdirAll(sessionsDir, 0755); err != nil {
		return fmt.Errorf("failed to create sessions directory: %w", err)
	}

	historyFile := filepath.Join(sessionsDir, sessionID+".json")
	history := SessionHistory{
		SessionID: sessionID,
		Messages:  messages,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// If file exists, preserve CreatedAt
	if existing, err := LoadSessionHistory(workspace, sessionID); err == nil {
		history.CreatedAt = existing.CreatedAt
	}

	data, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(historyFile, data, 0644)
}

// LoadSessionHistory loads the conversation history for a session.
func LoadSessionHistory(workspace string, sessionID string) (*SessionHistory, error) {
	historyFile := filepath.Join(workspace, ".nerd", "sessions", sessionID+".json")
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return nil, err
	}

	var history SessionHistory
	if err := json.Unmarshal(data, &history); err != nil {
		return nil, err
	}
	return &history, nil
}

// ListSessionHistories returns all available session histories.
func ListSessionHistories(workspace string) ([]string, error) {
	sessionsDir := filepath.Join(workspace, ".nerd", "sessions")
	entries, err := os.ReadDir(sessionsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var sessions []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			sessions = append(sessions, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	return sessions, nil
}

// GetLatestSession returns the most recent session ID.
func GetLatestSession(workspace string) (string, error) {
	state, err := LoadSessionState(workspace)
	if err != nil {
		return "", err
	}
	return state.SessionID, nil
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

// sanitizeForMangle converts a string to a valid Mangle name constant.
// Mangle name constants must be lowercase alphanumeric with underscores.
func sanitizeForMangle(s string) string {
	if s == "" {
		return "unknown"
	}

	// Convert to lowercase
	s = strings.ToLower(s)

	// Replace spaces and special characters with underscores
	var result strings.Builder
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			result.WriteRune(c)
		} else if c == ' ' || c == '-' || c == '.' || c == '/' {
			result.WriteRune('_')
		}
		// Skip other characters
	}

	// Ensure it doesn't start with a number
	sanitized := result.String()
	if len(sanitized) > 0 && sanitized[0] >= '0' && sanitized[0] <= '9' {
		sanitized = "n" + sanitized
	}

	// Remove consecutive underscores and trim
	for strings.Contains(sanitized, "__") {
		sanitized = strings.ReplaceAll(sanitized, "__", "_")
	}
	sanitized = strings.Trim(sanitized, "_")

	if sanitized == "" {
		return "unknown"
	}

	return sanitized
}

// =============================================================================
// NEW KNOWLEDGE BASES: Codebase, Core Shards, Campaign
// =============================================================================

// createCodebaseKnowledgeBase creates a knowledge base with project-specific facts.
func (i *Initializer) createCodebaseKnowledgeBase(ctx context.Context, nerdDir string, profile ProjectProfile, scanResult *world.ScanResult) (string, int, error) {
	kbPath := filepath.Join(nerdDir, "shards", "codebase_knowledge.db")

	// Create the knowledge store
	codebaseDB, err := store.NewLocalStore(kbPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create codebase KB: %w", err)
	}
	defer codebaseDB.Close()

	atomCount := 0

	// Store project identity
	if err := codebaseDB.StoreKnowledgeAtom("project_identity", fmt.Sprintf(
		"This is %s, a %s project. %s",
		profile.Name, profile.Language, profile.Description), 1.0); err == nil {
		atomCount++
	}

	// Store language and framework
	if profile.Language != "" && profile.Language != "unknown" {
		if err := codebaseDB.StoreKnowledgeAtom("primary_language", profile.Language, 1.0); err == nil {
			atomCount++
		}
	}
	if profile.Framework != "" && profile.Framework != "unknown" {
		if err := codebaseDB.StoreKnowledgeAtom("framework", profile.Framework, 0.95); err == nil {
			atomCount++
		}
	}

	// Store file topology summary
	if scanResult != nil {
		summary := fmt.Sprintf("Project contains %d files in %d directories (%d test files).",
			scanResult.FileCount, scanResult.DirectoryCount, scanResult.TestFileCount)
		if err := codebaseDB.StoreKnowledgeAtom("file_topology_summary", summary, 0.9); err == nil {
			atomCount++
		}

		// Store language breakdown
		for lang, count := range scanResult.Languages {
			content := fmt.Sprintf("Language %s: %d files", lang, count)
			if err := codebaseDB.StoreKnowledgeAtom("language_stats", content, 0.85); err == nil {
				atomCount++
			}
		}
	}

	// Store entry points
	for _, entry := range profile.EntryPoints {
		if err := codebaseDB.StoreKnowledgeAtom("entry_point", entry, 0.95); err == nil {
			atomCount++
		}
	}

	// Store dependencies
	for _, dep := range profile.Dependencies {
		content := fmt.Sprintf("Dependency: %s (%s)", dep.Name, dep.Type)
		if err := codebaseDB.StoreKnowledgeAtom("dependency", content, 0.9); err == nil {
			atomCount++
		}
	}

	// Store architectural patterns
	for _, pattern := range profile.Patterns {
		if err := codebaseDB.StoreKnowledgeAtom("architectural_pattern", pattern, 0.85); err == nil {
			atomCount++
		}
	}

	return kbPath, atomCount, nil
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
			researcher := shards.NewResearcherShard()
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

// createCampaignKnowledgeBase creates a knowledge base for campaign orchestration.
func (i *Initializer) createCampaignKnowledgeBase(ctx context.Context, nerdDir string, profile ProjectProfile) (string, int, error) {
	kbPath := filepath.Join(nerdDir, "shards", "campaign_knowledge.db")

	campaignDB, err := store.NewLocalStore(kbPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to create campaign KB: %w", err)
	}
	defer campaignDB.Close()

	atomCount := 0

	// Campaign orchestration concepts
	concepts := []struct{ Key, Value string }{
		{"campaign_identity", "I am the Campaign orchestrator. I manage multi-phase development tasks."},
		{"capability_planning", "I can break complex tasks into phases with dependencies."},
		{"capability_delegation", "I can delegate tasks to specialized shards (Coder, Reviewer, Tester)."},
		{"capability_checkpoints", "I validate phase completion before proceeding."},
		{"capability_replanning", "I can replan when tasks fail or requirements change."},
		{"phase_types", "Common phases: research, design, implement, test, review, integrate."},
		{"safety_rule", "Failed checkpoints block phase advancement."},
		{"safety_rule", "Critical security findings block campaign completion."},
		{"learning_rule", "Successful patterns are promoted to long-term memory."},
		{"learning_rule", "Repeated failures trigger strategy adjustment."},
	}

	for _, concept := range concepts {
		if err := campaignDB.StoreKnowledgeAtom(concept.Key, concept.Value, 0.95); err == nil {
			atomCount++
		}
	}

	// Store project-specific campaign context
	if err := campaignDB.StoreKnowledgeAtom("project_context",
		fmt.Sprintf("Campaigns for %s (%s project)", profile.Name, profile.Language), 0.9); err == nil {
		atomCount++
	}

	// Store build system info for execution
	if profile.BuildSystem != "" {
		if err := campaignDB.StoreKnowledgeAtom("build_system", profile.BuildSystem, 0.9); err == nil {
			atomCount++
		}
	}

	// Research campaign patterns if LLM available
	if i.config.LLMClient != nil && !i.config.SkipResearch {
		researcher := shards.NewResearcherShard()
		researcher.SetLLMClient(i.config.LLMClient)
		if i.config.Context7APIKey != "" {
			researcher.SetContext7APIKey(i.config.Context7APIKey)
		}
		researcher.SetLocalDB(campaignDB)

		// Research software development workflows
		researcher.Execute(ctx, "research docs: software development workflow patterns (brief)")
		atomCount += 5
	}

	return kbPath, atomCount, nil
}
