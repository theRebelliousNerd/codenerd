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
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
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
	Phase          string        // Current phase name
	Message        string        // Human-readable status message
	Percent        float64       // 0.0 - 1.0 completion percentage
	IsError        bool          // True if this is an error message
	AgentUpdate    *AgentCreationUpdate
	ETARemaining   time.Duration // E2: Estimated time remaining
	ElapsedTime    time.Duration // E2: Time elapsed since init started
	CurrentPhaseNo int           // E2: Current phase number (1-based)
	TotalPhases    int           // E2: Total number of phases
}

// AgentCreationUpdate provides details about agent creation progress.
type AgentCreationUpdate struct {
	AgentName     string
	AgentType     string
	Status        string  // "creating", "researching", "ready", "failed"
	KBSize        int     // Knowledge base size (facts/atoms)
	AtomCount     int     // E1: Current atom count during research
	TopicProgress string  // E1: Current topic being researched
	QualityScore  float64 // E1: Research quality score (0-100)
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
		Timeout:      30 * time.Minute,
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

	// Enhanced detection (B4, D2)
	BuildSystemInfo *BuildSystemInfo `json:"build_system_info,omitempty"`
	ProjectType     string           `json:"project_type,omitempty"` // "application", "library", "hybrid"

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
	Name         string `json:"name"`
	Version      string `json:"version,omitempty"`
	MajorVersion string `json:"major_version,omitempty"` // D4: Major version for version-specific agents
	Type         string `json:"type"`                    // direct, dev, transitive
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

	// Quality metrics (populated during research)
	QualityScore  float64 `json:"quality_score,omitempty"`  // 0-100 quality score
	QualityRating string  `json:"quality_rating,omitempty"` // "Excellent", "Good", "Adequate", "Needs improvement"
}

// Initializer handles the cold-start initialization process.
type Initializer struct {
	config     InitConfig
	researcher *researcher.ResearcherShard
	scanner    *world.Scanner
	localDB    *store.LocalStore
	shardMgr   *core.ShardManager
	kernel     *core.RealKernel
	embedEngine embedding.EmbeddingEngine

	// Concurrency
	mu            sync.RWMutex
	createdAgents []CreatedAgent

	// E2: ETA tracking
	etaTracker *ETATracker
}

// ETATracker calculates estimated time remaining based on historical phase durations.
type ETATracker struct {
	mu              sync.RWMutex
	startTime       time.Time
	phaseDurations  map[string]time.Duration // Historical durations for each phase
	currentPhase    int
	totalPhases     int
	phaseStartTime  time.Time
}

// DefaultPhaseDurations returns expected durations for each init phase.
// These are baseline estimates that get refined based on actual performance.
func DefaultPhaseDurations() map[string]time.Duration {
	return map[string]time.Duration{
		"setup":         5 * time.Second,
		"scanning":      20 * time.Second,
		"analysis":      75 * time.Second,  // 60-90s average
		"profile":       5 * time.Second,
		"facts":         10 * time.Second,
		"agents":        5 * time.Second,
		"kb_creation":   105 * time.Second, // 90-120s average
		"core_shards":   30 * time.Second,
		"tools":         20 * time.Second,
		"preferences":   4 * time.Second,
		"registry":      5 * time.Second,
	}
}

// NewETATracker creates a new ETA tracker.
func NewETATracker(totalPhases int) *ETATracker {
	return &ETATracker{
		startTime:      time.Now(),
		phaseDurations: DefaultPhaseDurations(),
		totalPhases:    totalPhases,
		currentPhase:   0,
	}
}

// StartPhase marks the beginning of a new phase.
func (e *ETATracker) StartPhase(phaseNum int) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.currentPhase = phaseNum
	e.phaseStartTime = time.Now()
}

// CompletePhase records the actual duration of a completed phase.
func (e *ETATracker) CompletePhase(phaseName string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	actualDuration := time.Since(e.phaseStartTime)
	// Update with actual duration for better future estimates
	e.phaseDurations[phaseName] = actualDuration
}

// GetETARemaining calculates the estimated time remaining.
func (e *ETATracker) GetETARemaining(remainingPhases []string) time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var remaining time.Duration
	for _, phase := range remainingPhases {
		if dur, ok := e.phaseDurations[phase]; ok {
			remaining += dur
		} else {
			// Default estimate for unknown phases
			remaining += 10 * time.Second
		}
	}
	return remaining
}

// GetElapsed returns the time elapsed since init started.
func (e *ETATracker) GetElapsed() time.Duration {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return time.Since(e.startTime)
}

// GetCurrentPhase returns the current phase number.
func (e *ETATracker) GetCurrentPhase() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.currentPhase
}

// GetTotalPhases returns the total number of phases.
func (e *ETATracker) GetTotalPhases() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.totalPhases
}

// sendProgressWithETA sends a progress update with ETA information.
func (i *Initializer) sendProgressWithETA(phase, message string, percent float64, remainingPhases []string) {
	if i.config.ProgressChan == nil {
		return
	}

	var eta time.Duration
	var elapsed time.Duration
	var currentPhase, totalPhases int

	if i.etaTracker != nil {
		eta = i.etaTracker.GetETARemaining(remainingPhases)
		elapsed = i.etaTracker.GetElapsed()
		currentPhase = i.etaTracker.GetCurrentPhase()
		totalPhases = i.etaTracker.GetTotalPhases()
	}

	select {
	case i.config.ProgressChan <- InitProgress{
		Phase:          phase,
		Message:        message,
		Percent:        percent,
		ETARemaining:   eta,
		ElapsedTime:    elapsed,
		CurrentPhaseNo: currentPhase,
		TotalPhases:    totalPhases,
	}:
	default:
		// Don't block if channel is full
	}
}

// startPhaseWithETA starts a new phase and sends a progress update.
func (i *Initializer) startPhaseWithETA(phaseNum int, phaseName, message string, percent float64, remainingPhases []string) {
	if i.etaTracker != nil {
		i.etaTracker.StartPhase(phaseNum)
	}
	i.sendProgressWithETA(phaseName, message, percent, remainingPhases)
}

// completePhaseWithETA completes a phase and updates the ETA tracker.
func (i *Initializer) completePhaseWithETA(phaseName string) {
	if i.etaTracker != nil {
		i.etaTracker.CompletePhase(phaseName)
	}
}

// NewInitializer creates a new initializer.
func NewInitializer(initConfig InitConfig) (*Initializer, error) {
	researcher := researcher.NewResearcherShard()
	researcher.SetWorkspaceRoot(initConfig.Workspace) // Ensure .nerd paths resolve correctly

	// Auto-detect Context7 API key if not explicitly provided (C1 enhancement)
	context7Key := initConfig.Context7APIKey
	if context7Key == "" {
		context7Key = config.AutoDetectContext7APIKey()
		if context7Key != "" {
			logging.Boot("Auto-detected Context7 API key from environment/config")
			initConfig.Context7APIKey = context7Key // Store for later use
		}
	}

	// Set Context7 API key if available (explicit or auto-detected)
	if context7Key != "" {
		researcher.SetContext7APIKey(context7Key)
	}

	kernel, err := core.NewRealKernel()
	if err != nil {
		return nil, fmt.Errorf("failed to create kernel: %w", err)
	}
	kernel.SetWorkspace(initConfig.Workspace) // Ensure .nerd paths resolve correctly

	init := &Initializer{
		config:        initConfig,
		researcher:    researcher,
		scanner:       world.NewScanner(),
		kernel:        kernel,
		createdAgents: make([]CreatedAgent, 0),
		embedEngine:   nil,
		etaTracker:    NewETATracker(11), // E2: 11 phases in total
	}

	// Use provided shard manager or create new one
	if initConfig.ShardManager != nil {
		init.shardMgr = initConfig.ShardManager
	} else {
		init.shardMgr = core.NewShardManager()
	}
	if initConfig.LLMClient != nil {
		init.shardMgr.SetLLMClient(initConfig.LLMClient)
	}

	return init, nil
}

// ensureEmbeddingEngine initializes a shared embedding engine for sqlite-vec.
func (i *Initializer) ensureEmbeddingEngine() error {
	if i.embedEngine != nil {
		return nil
	}
	engine, err := embedding.NewEngine(embedding.DefaultConfig())
	if err != nil {
		return fmt.Errorf("failed to initialize embedding engine (required for sqlite-vec): %w", err)
	}
	i.embedEngine = engine
	return nil
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
	// PHASE 0: Database Schema Migrations (for existing installations)
	// =========================================================================
	existingNerdDir := filepath.Join(i.config.Workspace, ".nerd")
	if _, statErr := os.Stat(existingNerdDir); statErr == nil {
		i.sendProgress("migration", "Checking database schemas...", 0.02)
		migrationResults, migErr := store.MigrateAllAgentDBs(existingNerdDir)
		if migErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Migration check failed: %v", migErr))
		} else if len(migrationResults) > 0 {
			for agentName, migResult := range migrationResults {
				if migResult.MigrationsRun > 0 {
					logging.Boot("Migrated %s: v%d → v%d (%d migrations, %d hashes backfilled)",
						agentName, migResult.FromVersion, migResult.ToVersion,
						migResult.MigrationsRun, migResult.HashesComputed)
					fmt.Printf("   ✓ Migrated %s database schema (v%d → v%d)\n",
						agentName, migResult.FromVersion, migResult.ToVersion)
				}
			}
		}
	}

	// =========================================================================
	// PHASE 1: Directory Structure & Database Setup
	// =========================================================================
	i.sendProgress("setup", "Creating directory structure...", 0.05)

	nerdDir, err := i.createDirectoryStructure()
	if err != nil {
		return nil, fmt.Errorf("failed to create directory structure: %w", err)
	}
	
	// Create Mangle overlay templates
	if err := i.createMangleTemplates(nerdDir); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create mangle templates: %v", err))
	}

	result.NerdDir = nerdDir
	fmt.Println("✓ Created .nerd/ directory structure")

	// Initialize local database
	dbPath := filepath.Join(nerdDir, "knowledge.db")
	i.localDB, err = store.NewLocalStore(dbPath)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to initialize database: %v", err))
	} else {
		if err := i.ensureEmbeddingEngine(); err != nil {
			return nil, err
		}
		i.localDB.SetEmbeddingEngine(i.embedEngine)
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
	// PHASE 5b: Populate Project-Specific Prompt Atoms
	// =========================================================================
	i.sendProgress("prompt_atoms", "Populating project-specific prompt atoms...", 0.47)
	fmt.Println("\n📝 Phase 5b: Populating Project-Specific Prompt Atoms")

	if err := i.populateProjectAtoms(profile); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to populate prompt atoms: %v", err))
	}

	// =========================================================================
	// PHASE 5c: Initialize Prompt Database (atoms.db)
	// =========================================================================
	i.sendProgress("prompt_db", "Initializing prompt atoms database...", 0.48)
	fmt.Println("\n📦 Phase 5c: Initializing Prompt Atoms Database")

	if err := i.initializePromptDatabase(ctx, nerdDir); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to initialize prompt database: %v", err))
	} else {
		atomsDBPath := filepath.Join(nerdDir, "prompts", "atoms.db")
		result.FilesCreated = append(result.FilesCreated, atomsDBPath)
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
		// Create shared knowledge pool first (common concepts all agents share)
		i.sendProgress("shared_kb", "Creating shared knowledge pool...", 0.52)
		fmt.Println("\n📚 Phase 7a: Creating Shared Knowledge Pool")

		sharedPoolErr := CreateSharedKnowledgePool(ctx, i.config.Workspace, i.researcher, func(status string, progress float64) {
			i.sendProgress("shared_kb", status, 0.52+progress*0.03)
		})
		if sharedPoolErr != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Shared knowledge pool creation had issues: %v", sharedPoolErr))
		} else {
			fmt.Println("   ✓ Shared knowledge pool ready")
		}

		i.sendProgress("kb_creation", "Creating agent knowledge bases...", 0.55)
		fmt.Println("\n📚 Phase 7b: Creating Agent Knowledge Bases")

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
	i.sendProgress("registry", "Generating agent registry...", 0.93)

	registryPath := filepath.Join(nerdDir, "agents.json")
	if err := i.saveAgentRegistry(registryPath, result.CreatedAgents); err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to save agent registry: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, registryPath)
	}

	// =========================================================================
	// PHASE 12: Sync Agent Prompts to Knowledge DBs
	// =========================================================================
	i.sendProgress("prompt_sync", "Syncing agent prompts to knowledge DBs...", 0.97)
	fmt.Println("\n📝 Phase 12: Syncing Agent Prompts")

	// Sync all .nerd/agents/{name}/prompts.yaml → .nerd/shards/{name}_knowledge.db
	// Uses upsert semantics: new atoms inserted, existing atoms updated
	promptCount, syncErr := prompt.ReloadAllPrompts(ctx, nerdDir, i.embedEngine)
	if syncErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to sync agent prompts: %v", syncErr))
		fmt.Printf("   ⚠ Warning: %v\n", syncErr)
	} else if promptCount > 0 {
		fmt.Printf("   ✓ Synced %d prompt atoms to knowledge DBs\n", promptCount)
		logging.Boot("Synced %d prompt atoms from YAML to knowledge DBs", promptCount)
	} else {
		fmt.Println("   ✓ No prompt atoms to sync")
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
// E2: Now includes ETA tracking data when available.
func (i *Initializer) sendProgress(phase, message string, percent float64) {
	if i.config.ProgressChan != nil {
		progress := InitProgress{
			Phase:   phase,
			Message: message,
			Percent: percent,
		}

		// E2: Wire in ETA tracker data if available
		if i.etaTracker != nil {
			progress.ElapsedTime = i.etaTracker.GetElapsed()
			progress.CurrentPhaseNo = i.etaTracker.GetCurrentPhase()
			progress.TotalPhases = i.etaTracker.GetTotalPhases()
			// Note: ETARemaining requires remaining phases list for accuracy
			// Use percent-based estimate as fallback
			if percent > 0 && percent < 1.0 {
				elapsed := i.etaTracker.GetElapsed()
				estimatedTotal := time.Duration(float64(elapsed) / percent)
				progress.ETARemaining = estimatedTotal - elapsed
			}
		}

		select {
		case i.config.ProgressChan <- progress:
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

// createMangleTemplates creates placeholder files for user extensions.
func (i *Initializer) createMangleTemplates(nerdDir string) error {
	mangleDir := filepath.Join(nerdDir, "mangle")

	// extensions.mg - For new schema definitions
	extPath := filepath.Join(mangleDir, "extensions.mg")
	extContent := `# User Schema Extensions
# Define project-specific predicates here.
# These will be loaded AFTER the core schemas.

# Example:
# Decl project_metadata(Key, Value).
# Decl deploy_target(Env, URL).
`
	if err := os.WriteFile(extPath, []byte(extContent), 0644); err != nil {
		return err
	}

	// policy_overrides.mg - For custom rules
	policyPath := filepath.Join(mangleDir, "policy_overrides.mg")
	policyContent := `# User Policy Overrides
# Define project-specific rules here.
# These can extend or override core behavior.

# Example: Allow deleting .tmp files even if modified
# permitted(Action) :- 
#     action_type(Action, /delete_file),
#     target_path(Action, Path),
#     fn:string_suffix(Path, ".tmp").
`
	if err := os.WriteFile(policyPath, []byte(policyContent), 0644); err != nil {
		return err
	}

	return nil
}

// printSummary prints the initialization summary with quality metrics.
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
			// Display with quality metrics if available
			if agent.QualityScore > 0 {
				fmt.Printf("   • %s: %d atoms (Quality: %.0f%% - %s)\n",
					agent.Name, agent.KBSize, agent.QualityScore, agent.QualityRating)
			} else {
				fmt.Printf("   • %s (%d KB atoms) - %s\n", agent.Name, agent.KBSize, agent.Status)
			}
		}

		// Show average quality score
		var totalQuality float64
		var qualityCount int
		for _, agent := range result.CreatedAgents {
			if agent.QualityScore > 0 {
				totalQuality += agent.QualityScore
				qualityCount++
			}
		}
		if qualityCount > 0 {
			avgQuality := totalQuality / float64(qualityCount)
			fmt.Printf("\n   📊 Average KB Quality: %.0f%%\n", avgQuality)
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

	// Post-init recommendations based on project analysis
	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("💡 Recommendations:")
	i.printRecommendations(result, profile)

	// Run post-init validation
	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("🔍 Validating knowledge bases...")
	validationSummary, err := ValidateAllAgentDBs(result.NerdDir)
	if err != nil {
		fmt.Printf("   ⚠ Validation failed: %v\n", err)
	} else {
		if validationSummary.OverallValid {
			fmt.Printf("   ✓ All %d knowledge bases validated successfully\n", validationSummary.TotalDBs)
		} else {
			fmt.Printf("   ⚠ %d/%d knowledge bases have issues\n", validationSummary.InvalidDBs, validationSummary.TotalDBs)
			for name, res := range validationSummary.Results {
				if !res.Valid {
					fmt.Printf("     - %s: %v\n", name, res.Errors)
				}
			}
		}

		// Report backup files
		if len(validationSummary.BackupFiles) > 0 {
			fmt.Printf("\n   📦 Found %d backup files from migration\n", len(validationSummary.BackupFiles))
			fmt.Println("      After verifying your data, clean them up with:")
			fmt.Println("      nerd init --cleanup-backups")
		}
	}

	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("🚀 Next steps:")
	fmt.Println("   • Run `nerd chat` to start interactive session")
	fmt.Println("   • Use `/agents` to see available agents")
	fmt.Println("   • Use `/spawn <agent> <task>` to delegate tasks")
	fmt.Println(strings.Repeat("─", 60))
}

// printRecommendations prints context-aware recommendations based on init results.
func (i *Initializer) printRecommendations(result *InitResult, profile ProjectProfile) {
	recommendations := []string{}

	// Check for low quality KBs
	for _, agent := range result.CreatedAgents {
		if agent.QualityScore > 0 && agent.QualityScore < 50 {
			recommendations = append(recommendations,
				fmt.Sprintf("Run `/init --force` to improve %s KB quality (currently %.0f%%)", agent.Name, agent.QualityScore))
		}
	}

	// Language-specific recommendations
	switch strings.ToLower(profile.Language) {
	case "go", "golang":
		if !hasAgent(result.CreatedAgents, "GoExpert") {
			recommendations = append(recommendations, "Consider adding a GoExpert agent for Go-specific guidance")
		}
	case "python":
		recommendations = append(recommendations, "Run `/review` to check type hints and async patterns")
	case "typescript", "javascript":
		recommendations = append(recommendations, "Run `/test` to verify test coverage")
	}

	// Security recommendation for all projects
	if !hasAgent(result.CreatedAgents, "SecurityAuditor") {
		recommendations = append(recommendations, "Consider adding SecurityAuditor for vulnerability scanning")
	} else {
		recommendations = append(recommendations, "Run `/review --security` for a security audit")
	}

	// Test recommendation
	if !hasAgent(result.CreatedAgents, "TestArchitect") {
		recommendations = append(recommendations, "Consider adding TestArchitect for test coverage analysis")
	} else {
		recommendations = append(recommendations, "Run `/test --coverage` to check test coverage")
	}

	// Warnings about missing research
	if i.config.SkipResearch {
		recommendations = append(recommendations, "Research was skipped - run `/init --force` to populate agent KBs")
	}

	// Print recommendations (max 4)
	maxRecs := 4
	if len(recommendations) > maxRecs {
		recommendations = recommendations[:maxRecs]
	}

	for _, rec := range recommendations {
		fmt.Printf("   • %s\n", rec)
	}

	if len(recommendations) == 0 {
		fmt.Println("   • Your project is ready! Start with `/review` or `/test`")
	}
}

// hasAgent checks if a specific agent was created.
func hasAgent(agents []CreatedAgent, name string) bool {
	for _, agent := range agents {
		if strings.EqualFold(agent.Name, name) {
			return true
		}
	}
	return false
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
