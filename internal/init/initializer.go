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
//
// This file contains the core initialization orchestration.
// Related files:
// - scanner.go: File system traversal and dependency detection
// - profile.go: Profile generation, facts generation, session management
// - agents.go: Agent recommendation, creation, and knowledge base hydration
package init

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/shards/researcher"
	"codenerd/internal/store"
	"codenerd/internal/world"
	"context"
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
	Name            string            `json:"name"`
	Type            string            `json:"type"` // Type 3 category
	Description     string            `json:"description"`
	Topics          []string          `json:"topics"` // Research topics for KB
	Permissions     []string          `json:"permissions"`
	Priority        int               `json:"priority"` // Higher = more important
	Reason          string            `json:"reason"`   // Why this agent is needed
	Tools           []string          `json:"tools,omitempty"`
	ToolPreferences map[string]string `json:"tool_preferences,omitempty"`
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
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	KnowledgePath   string            `json:"knowledge_path"`
	KBSize          int               `json:"kb_size"`
	CreatedAt       time.Time         `json:"created_at"`
	Status          string            `json:"status"` // "ready", "partial", "failed"
	Tools           []string          `json:"tools,omitempty"`
	ToolPreferences map[string]string `json:"tool_preferences,omitempty"`
}

// Initializer handles the cold-start initialization process.
type Initializer struct {
	config     InitConfig
	researcher *researcher.ResearcherShard
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
	researcher := researcher.NewResearcherShard()
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

	factsPath := filepath.Join(nerdDir, "profile.mg")
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
	// PHASE 7e: Generate Project-Specific Tools
	// =========================================================================
	i.sendProgress("tool_generation", "Generating project-specific tools...", 0.86)
	fmt.Println("\n🛠️  Phase 7e: Generating Project-Specific Tools")

	generatedTools, err := i.generateProjectTools(ctx, nerdDir, profile)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to generate tools: %v", err))
	} else if len(generatedTools) > 0 {
		fmt.Printf("   ✓ Generated %d tools\n", len(generatedTools))
		// Store tool names in result for summary
		if result.AgentKBs == nil {
			result.AgentKBs = make(map[string]int)
		}
		result.AgentKBs["_generated_tools"] = len(generatedTools)
	} else {
		fmt.Println("   ⓘ No tools generated (may be skipped or not needed)")
	}

	// =========================================================================
	// PHASE 8: Initialize Preferences
	// =========================================================================
	i.sendProgress("preferences", "Initializing preferences...", 0.88)
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
	// PHASE 10: Generate Tool Definitions
	// =========================================================================
	i.sendProgress("tools", "Generating tool definitions...", 0.92)
	fmt.Println("\n🔧 Phase 10: Generating Tool Definitions")

	detectedTech := []string{profile.Language}
	if profile.Framework != "" && profile.Framework != "unknown" {
		detectedTech = append(detectedTech, profile.Framework)
	}

	tools := GenerateToolsForProject(detectedTech)
	if err := SaveToolsToFile(nerdDir, tools); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to save tools: %v", err))
	} else {
		toolsFile := filepath.Join(nerdDir, "tools", "available_tools.json")
		result.FilesCreated = append(result.FilesCreated, toolsFile)
		fmt.Printf("   ✓ Generated %d tool definitions\n", len(tools))

		// Print tool breakdown by category
		categories := make(map[string]int)
		for _, tool := range tools {
			categories[tool.Category]++
		}
		for cat, count := range categories {
			fmt.Printf("      - %s: %d\n", cat, count)
		}
	}

	// =========================================================================
	// PHASE 11: Generate Agent Registry
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

// Type definitions (implementations moved to separate files)
// determineRequiredAgents - see agents.go
// createType3Agents - see agents.go
// createAgentKnowledgeBase - see agents.go
// generateBaseKnowledgeAtoms - see agents.go
// sendAgentProgress - see agents.go
// registerAgentsWithShardManager - see agents.go
// saveAgentRegistry - see agents.go
// createCoreShardKnowledgeBases - see agents.go
// generateProjectTools - see agents.go
// determineRequiredTools - see agents.go
// createDirectoryStructure - see scanner.go
// detectLanguageFromFiles - see scanner.go
// detectDependencies - see scanner.go
// buildProjectProfile - see profile.go
// saveProfile - see profile.go
// generateFactsFile - see profile.go
// initPreferences - see profile.go
// savePreferences - see profile.go
// initSessionState - see profile.go
// createCodebaseKnowledgeBase - see profile.go
// createCampaignKnowledgeBase - see profile.go
// LoadProjectProfile - see profile.go
// LoadPreferences - see profile.go
// LoadSessionState - see profile.go
// SaveSessionState - see profile.go
// SaveSessionHistory - see profile.go
// LoadSessionHistory - see profile.go
// ListSessionHistories - see profile.go
// GetLatestSession - see profile.go
// IsInitialized - see profile.go
// generateProjectID - see profile.go
// generateSessionID - see profile.go
// cleanNameConstant - see profile.go
// sanitizeForMangle - see profile.go

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

	// Show generated tools
	if toolCount, ok := result.AgentKBs["_generated_tools"]; ok && toolCount > 0 {
		fmt.Printf("\n🛠️  Generated Tools: %d\n", toolCount)
		fmt.Printf("   Tools are ready to use in .nerd/tools/\n")
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
