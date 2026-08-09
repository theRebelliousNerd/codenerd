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
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/northstar"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"

	// researcher removed - JIT clean loop handles research
	"codenerd/internal/store"
	"codenerd/internal/tools/research"
	"codenerd/internal/world"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	initSecretKVPattern = regexp.MustCompile(`(?i)((?:"?(?:api[_-]?key|secret|token|password)"?\s*[:=]\s*"?|bearer\s+))([^"\s,&]+)`)
	initSecretPattern   = regexp.MustCompile(`(AIza[0-9A-Za-z_-]{10,}|sk-[A-Za-z0-9]{10,}|ctx7sk-[0-9a-f-]{8,}|gh[pousr]_[A-Za-z0-9]{10,}|xox[baprs]-[A-Za-z0-9-]{10,})`)
)

// InitProgress represents a progress update during initialization.
type InitProgress struct {
	Phase          string  // Current phase name
	Message        string  // Human-readable status message
	Percent        float64 // 0.0 - 1.0 completion percentage
	IsError        bool    // True if this is an error message
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
	QualityScore  float64 // E1: Atom-count population proxy (0-100), not semantic quality
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
	Tools           []string          `json:"tools,omitzero"`
	ToolPreferences map[string]string `json:"tool_preferences,omitzero"`
}

// InitConfig holds configuration for initialization.
type InitConfig struct {
	Workspace       string
	LLMClient       perception.LLMClient
	LLMProvider     string                   // Human-readable provider used in summaries
	LLMModel        string                   // Human-readable model used in summaries
	ShardManager    *coreshards.ShardManager // Shard manager for agent spawning
	Interactive     bool                     // Whether to prompt user for preferences
	Timeout         time.Duration            // Maximum time for initialization
	SkipResearch    bool                     // Skip deep research phase (faster init)
	SkipAgentCreate bool                     // Skip Type 3 agent creation
	PreferenceHints []string                 // User-provided hints about preferences
	ProgressChan    chan InitProgress        // Channel for progress updates
	Context7APIKey  string                   // Context7 API key for LLM-optimized docs
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
	Framework    string   `json:"framework,omitzero"`
	BuildSystem  string   `json:"build_system,omitzero"`
	Architecture string   `json:"architecture,omitzero"`
	Patterns     []string `json:"patterns,omitzero"`

	// Enhanced detection (B4, D2)
	BuildSystemInfo *BuildSystemInfo `json:"build_system_info,omitzero"`
	ProjectType     string           `json:"project_type,omitzero"` // "application", "library", "hybrid"

	// Dependencies
	Dependencies []DependencyInfo `json:"dependencies,omitzero"`

	// Paths
	EntryPoints     []string `json:"entry_points,omitzero"`
	TestDirectories []string `json:"test_directories,omitzero"`
	ConfigFiles     []string `json:"config_files,omitzero"`

	// Stats
	FileCount      int `json:"file_count"`
	DirectoryCount int `json:"directory_count"`
}

// DependencyInfo represents a project dependency.
type DependencyInfo struct {
	Name         string `json:"name"`
	Version      string `json:"version,omitzero"`
	MajorVersion string `json:"major_version,omitzero"` // D4: Major version for version-specific agents
	Type         string `json:"type"`                   // direct, dev, transitive
}

// UserPreferences represents user coding preferences (learned via autopoiesis).
type UserPreferences struct {
	// Code style
	TestStyle        string `json:"test_style,omitzero"`        // "table_driven", "subtest", etc.
	ErrorHandling    string `json:"error_handling,omitzero"`    // "wrap", "sentinel", etc.
	NamingConvention string `json:"naming_convention,omitzero"` // "camelCase", "snake_case"

	// Behavior
	CommitStyle    string `json:"commit_style,omitzero"`    // "conventional", "descriptive"
	BranchStrategy string `json:"branch_strategy,omitzero"` // "gitflow", "trunk"

	// Safety
	RequireTests  bool `json:"require_tests"`  // Require tests before commits
	RequireReview bool `json:"require_review"` // Require review before merges

	// Communication
	Verbosity        string `json:"verbosity,omitzero"`         // "concise", "detailed"
	ExplanationLevel string `json:"explanation_level,omitzero"` // "beginner", "expert"
}

// InitResult represents the result of initialization.
type InitResult struct {
	Success        bool               `json:"success"`
	Profile        ProjectProfile     `json:"profile"`
	Preferences    UserPreferences    `json:"preferences"`
	NerdDir        string             `json:"nerd_dir"`
	FilesCreated   []string           `json:"files_created"`
	FactsGenerated int                `json:"facts_generated"`
	Duration       time.Duration      `json:"duration"`
	Warnings       []string           `json:"warnings,omitzero"`
	Failures       []string           `json:"failures,omitzero"`
	LLMMetrics     InitLLMMetrics     `json:"llm_metrics"`
	Validation     *ValidationSummary `json:"validation,omitempty"`

	// Type 3 Agent Creation Results
	RecommendedAgents []RecommendedAgent `json:"recommended_agents,omitzero"`
	CreatedAgents     []CreatedAgent     `json:"created_agents,omitzero"`
	AgentKBs          map[string]int     `json:"agent_knowledge_bases,omitzero"` // agent name -> KB size

	// Gemini Grounding (when Gemini is the LLM provider)
	GroundingSources []string `json:"grounding_sources,omitzero"` // URLs used to ground LLM responses
	GroundingEnabled bool     `json:"grounding_enabled,omitzero"` // Whether grounding was active
}

// InitLLMMetrics reports the enrichment calls made during initialization.
// LLM failures degrade enrichment but do not make structurally valid init
// artifacts unusable, so they are reported separately from required failures.
type InitLLMMetrics struct {
	Provider  string `json:"provider,omitzero"`
	Model     string `json:"model,omitzero"`
	Attempts  int    `json:"attempts"`
	Succeeded int    `json:"succeeded"`
	Failed    int    `json:"failed"`
	LastError string `json:"last_error,omitzero"`
}

// CreatedAgent represents a Type 3 agent that was created during init.
type CreatedAgent struct {
	Name            string            `json:"name"`
	Type            string            `json:"type"`
	KnowledgePath   string            `json:"knowledge_path"`
	KBSize          int               `json:"kb_size"`
	CreatedAt       time.Time         `json:"created_at"`
	Status          string            `json:"status"` // "ready", "partial", "failed"
	Tools           []string          `json:"tools,omitzero"`
	ToolPreferences map[string]string `json:"tool_preferences,omitzero"`

	// Legacy JSON field names retained for compatibility. Values measure KB
	// population by atom count; they do not measure semantic quality.
	QualityScore  float64 `json:"quality_score,omitzero"`
	QualityRating string  `json:"quality_rating,omitzero"`
}

// Initializer handles the cold-start initialization process.
type Initializer struct {
	config InitConfig
	// researcher removed - JIT clean loop handles research
	scanner     *world.Scanner
	localDB     *store.LocalStore
	shardMgr    *coreshards.ShardManager
	kernel      *core.RealKernel
	embedEngine embedding.EmbeddingEngine

	// Gemini grounding helper (nil if not Gemini or grounding unavailable)
	grounding        *research.GroundingHelper
	groundingSources []string // Accumulated grounding sources from all LLM calls

	// Concurrency
	mu         sync.RWMutex
	llmMetrics InitLLMMetrics

	// E2: ETA tracking
	etaTracker *ETATracker
}

// NewInitializer creates a new initializer.
func NewInitializer(initConfig InitConfig) (*Initializer, error) {
	workspace := strings.TrimSpace(initConfig.Workspace)
	if workspace == "" {
		var err error
		workspace, err = os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("resolve initializer workspace: %w", err)
		}
	}
	if abs, err := filepath.Abs(workspace); err == nil {
		workspace = abs
	}
	initConfig.Workspace = workspace

	// Researcher shard removed - JIT clean loop handles research
	// Auto-detect Context7 API key if not explicitly provided (C1 enhancement)
	context7Key := initConfig.Context7APIKey
	if context7Key == "" {
		context7Key = config.AutoDetectContext7APIKey()
		if context7Key != "" {
			logging.Boot("Auto-detected Context7 API key from environment/config")
			initConfig.Context7APIKey = context7Key // Store for later use
		}
	}

	kernel, err := core.NewRealKernelWithWorkspace(initConfig.Workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to create kernel: %w", err)
	}

	metrics := InitLLMMetrics{
		Provider: strings.TrimSpace(initConfig.LLMProvider),
		Model:    strings.TrimSpace(initConfig.LLMModel),
	}
	if metrics.Model == "" && initConfig.LLMClient != nil {
		if modelProvider, ok := initConfig.LLMClient.(interface{ GetModel() string }); ok {
			metrics.Model = strings.TrimSpace(modelProvider.GetModel())
		}
	}
	if metrics.Provider == "" && initConfig.LLMClient != nil {
		metrics.Provider = fmt.Sprintf("%T", initConfig.LLMClient)
	}

	init := &Initializer{
		config:      initConfig,
		scanner:     world.NewScanner(),
		kernel:      kernel,
		embedEngine: nil,
		llmMetrics:  metrics,
		etaTracker:  NewETATracker(22), // E2: 22 phases in total (see allPhases in Initialize)
	}

	// Use provided shard manager or create new one
	if initConfig.ShardManager != nil {
		init.shardMgr = initConfig.ShardManager
	} else {
		init.shardMgr = coreshards.NewShardManager()
	}
	if initConfig.LLMClient != nil {
		init.shardMgr.SetLLMClient(initConfig.LLMClient)

		// Initialize Gemini grounding helper if LLM client is Gemini
		init.grounding = research.NewGroundingHelper(initConfig.LLMClient)
		if init.grounding.IsGroundingAvailable() {
			// Enable Google Search grounding for init phases (strategic knowledge, doc analysis)
			init.grounding.EnableGoogleSearch()
			logging.Boot("Gemini grounding enabled for init (Google Search active)")
		}
	}

	return init, nil
}

// Close releases resources held by the initializer.
func (i *Initializer) Close() error {
	if i.localDB != nil {
		return i.localDB.Close()
	}
	return nil
}

// ensureEmbeddingEngine initializes a shared embedding engine for sqlite-vec.
// Model/provider ALWAYS come from .nerd/config.json (via GetEmbeddingConfig).
func (i *Initializer) ensureEmbeddingEngine() error {
	if i.embedEngine != nil {
		return nil
	}
	// Prefer workspace config.json — that is the product rule of record.
	cfgPath := filepath.Join(i.config.Workspace, ".nerd", "config.json")
	var uc *config.UserConfig
	if _, err := os.Stat(cfgPath); err == nil {
		uc, err = config.LoadUserConfig(cfgPath)
		if err != nil {
			return fmt.Errorf("load workspace embedding config: %w", err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("inspect workspace embedding config: %w", err)
	} else {
		uc, err = config.GlobalConfig()
		if err != nil {
			logging.Boot("Global embedding config unavailable, using defaults: %v", err)
			uc = nil
		}
	}
	if uc == nil {
		uc = config.DefaultUserConfig()
	}
	ucEmb := uc.GetEmbeddingConfig()
	embCfg := embedding.Config{
		Provider:       ucEmb.Provider,
		OllamaEndpoint: ucEmb.OllamaEndpoint,
		OllamaModel:    ucEmb.OllamaModel,
		GenAIAPIKey:    ucEmb.GenAIAPIKey,
		GenAIModel:     ucEmb.GenAIModel,
		TaskType:       ucEmb.TaskType,
	}
	logging.Boot("Init embedding engine from config.json: provider=%s model=%s", embCfg.Provider, initEmbeddingModel(embCfg))
	engine, err := embedding.NewEngine(embCfg)
	if err != nil {
		return fmt.Errorf("failed to initialize embedding engine (required for sqlite-vec): %w", err)
	}
	i.embedEngine = engine
	return nil
}

func initEmbeddingModel(cfg embedding.Config) string {
	if strings.EqualFold(cfg.Provider, "genai") {
		return cfg.GenAIModel
	}
	return cfg.OllamaModel
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
	ctx, cancel := initializationContext(ctx, i.config.Timeout)
	defer cancel()
	result := &InitResult{
		FilesCreated:  make([]string, 0),
		Warnings:      make([]string, 0),
		AgentKBs:      make(map[string]int),
		CreatedAgents: make([]CreatedAgent, 0),
	}
	checkContext := func(stage string) error {
		if err := ctx.Err(); err != nil {
			wrapped := fmt.Errorf("initialization stopped during %s: %w", stage, err)
			result.Failures = append(result.Failures, wrapped.Error())
			result.LLMMetrics = i.snapshotLLMMetrics()
			result.Duration = time.Since(startTime)
			return wrapped
		}
		return nil
	}
	if err := checkContext("startup"); err != nil {
		return result, err
	}

	runner := newPhaseRunner(i)

	runner.start("setup", "Initializing codeNERD...", 0.0)
	fmt.Println("🚀 Initializing codeNERD...")
	fmt.Printf("   Workspace: %s\n\n", i.config.Workspace)

	// Ensure system shards are running before heavy lifting.
	if err := i.shardMgr.StartSystemShards(ctx); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to start system shards: %v", err))
	}
	runner.complete("setup")
	if err := checkContext("system shard startup"); err != nil {
		return result, err
	}

	// Phase 0: Migrations
	i.runPhase0Migrations(runner, result)
	if err := checkContext("database migration"); err != nil {
		return result, err
	}

	// Phase 1: Directory & DB Setup
	nerdDir, err := i.runPhase1DirectorySetup(runner, result)
	if err != nil {
		result.Failures = append(result.Failures, err.Error())
		result.Duration = time.Since(startTime)
		return result, err
	}
	result.NerdDir = nerdDir
	if err := checkContext("directory and database setup"); err != nil {
		return result, err
	}

	// Phase 2: Codebase Scan
	scanResult := i.runPhase2Scanning(ctx, runner, result)
	if err := checkContext("codebase scan"); err != nil {
		return result, err
	}

	// Phase 3: Analysis (STUBBED)
	i.runPhase3Analysis(runner)
	if err := checkContext("analysis framework setup"); err != nil {
		return result, err
	}

	// Phase 4: Build Profile
	profile := i.runPhase4Profile(runner, result, nerdDir, scanResult)
	if err := checkContext("project profile creation"); err != nil {
		return result, err
	}

	// Phase 5: Facts
	i.runPhase5Facts(runner, result, nerdDir, profile)
	if err := checkContext("fact generation"); err != nil {
		return result, err
	}

	// Phase 5b: Prompt Atoms
	i.runPhase5bPromptAtoms(runner, result, profile)
	if err := checkContext("prompt atom population"); err != nil {
		return result, err
	}

	// Phase 5c: Prompt DB
	i.runPhase5cPromptDB(ctx, runner, result, nerdDir)
	if err := checkContext("prompt database initialization"); err != nil {
		return result, err
	}

	// Phase 6: Analyze Agents
	recommendedAgents := i.runPhase6AnalyzeAgents(runner, result, profile)
	if err := checkContext("agent analysis"); err != nil {
		return result, err
	}

	// Phase 7: Create Agent Knowledge Bases
	i.runPhase7aCreateAgentKBs(ctx, runner, result, nerdDir, recommendedAgents)
	if err := checkContext("agent knowledge creation"); err != nil {
		return result, err
	}

	// Phase 7b: Create Codebase Knowledge Base
	i.runPhase7bCreateCodebaseKB(ctx, runner, result, nerdDir, profile, scanResult)
	if err := checkContext("codebase knowledge creation"); err != nil {
		return result, err
	}

	// Phase 7c: Create Core Shard Knowledge Bases
	i.runPhase7cCreateCoreShardKBs(ctx, runner, result, nerdDir, profile)
	if err := checkContext("core shard knowledge creation"); err != nil {
		return result, err
	}

	// Phase 7d: Create Campaign Knowledge Base
	i.runPhase7dCreateCampaignKB(ctx, runner, result, nerdDir, profile)
	if err := checkContext("campaign knowledge creation"); err != nil {
		return result, err
	}

	// Phase 7e: Generate Project-Specific Tools
	i.runPhase7eGenerateTools(ctx, runner, result, nerdDir, profile)
	if err := checkContext("project tool generation"); err != nil {
		return result, err
	}

	// Phase 8: Preferences
	i.runPhase8Preferences(runner, result, nerdDir)
	if err := checkContext("preference initialization"); err != nil {
		return result, err
	}

	// Phase 9: Session State
	i.runPhase9Session(runner, result, nerdDir)
	if err := checkContext("session initialization"); err != nil {
		return result, err
	}

	// Phase 10: Generate Tool Definitions
	i.runPhase10Tools(runner, result, nerdDir, profile)
	if err := checkContext("tool definition generation"); err != nil {
		return result, err
	}

	// Phase 11: Agent Registry
	i.runPhase11Registry(runner, result, nerdDir)
	if err := checkContext("agent registry creation"); err != nil {
		return result, err
	}

	// Phase 12: Prompt Sync
	i.runPhase12PromptSync(ctx, runner, result, nerdDir)
	if err := checkContext("prompt synchronization"); err != nil {
		return result, err
	}

	// COMPLETE
	return i.finalizeInitialization(runner, result, startTime, profile)
}

func initializationContext(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	if timeout > 0 {
		return context.WithTimeout(parent, timeout)
	}
	return context.WithCancel(parent)
}

func (i *Initializer) recordLLMCall(err error) {
	i.mu.Lock()
	defer i.mu.Unlock()

	i.llmMetrics.Attempts++
	if err == nil {
		i.llmMetrics.Succeeded++
		return
	}
	i.llmMetrics.Failed++
	i.llmMetrics.LastError = boundedInitError(err, 240)
}

func (i *Initializer) snapshotLLMMetrics() InitLLMMetrics {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.llmMetrics
}

func boundedInitError(err error, maxRunes int) string {
	if err == nil {
		return ""
	}
	clean := strings.Join(strings.Fields(err.Error()), " ")
	clean = initSecretKVPattern.ReplaceAllString(clean, "${1}[redacted]")
	clean = initSecretPattern.ReplaceAllString(clean, "[redacted]")
	runes := []rune(clean)
	if maxRunes > 0 && len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "..."
	}
	return clean
}

func formatLLMIdentity(metrics InitLLMMetrics) string {
	provider := strings.TrimSpace(metrics.Provider)
	model := strings.TrimSpace(metrics.Model)
	switch {
	case provider != "" && model != "":
		return provider + "/" + model
	case provider != "":
		return provider
	case model != "":
		return model
	default:
		return "provider/model unknown"
	}
}

type phaseRunner struct {
	i               *Initializer
	allPhases       []string
	remainingPhases []string
	phaseNum        int
}

func newPhaseRunner(i *Initializer) *phaseRunner {
	allPhases := []string{
		"setup", "migration", "directory", "scanning", "analysis", "profile",
		"facts", "prompt_atoms", "prompt_db", "agents", "shared_kb", "kb_creation",
		"codebase_kb", "core_shards_kb", "campaign_kb", "tool_generation",
		"preferences", "session", "tools", "registry", "prompt_sync", "complete",
	}
	remainingPhases := make([]string, len(allPhases))
	copy(remainingPhases, allPhases)

	return &phaseRunner{
		i:               i,
		allPhases:       allPhases,
		remainingPhases: remainingPhases,
		phaseNum:        1,
	}
}

func (p *phaseRunner) advance() {
	if len(p.remainingPhases) > 0 {
		p.remainingPhases = p.remainingPhases[1:]
	}
	p.phaseNum++
}

func (p *phaseRunner) start(id, msg string, pct float64) {
	p.i.startPhaseWithETA(p.phaseNum, id, msg, pct, p.remainingPhases)
}

func (p *phaseRunner) complete(id string) {
	p.i.completePhaseWithETA(id)
	p.advance()
}

func (i *Initializer) runPhase0Migrations(runner *phaseRunner, result *InitResult) {
	existingNerdDir := filepath.Join(i.config.Workspace, ".nerd")
	if _, statErr := os.Stat(existingNerdDir); statErr == nil {
		runner.start("migration", "Checking database schemas...", 0.02)
		migrationResults, migErr := store.MigrateAllAgentDBs(existingNerdDir)
		if migErr != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("Migration check failed: %v", migErr))
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
		runner.complete("migration")
	} else {
		runner.advance()
	}
}

func (i *Initializer) runPhase1DirectorySetup(runner *phaseRunner, result *InitResult) (string, error) {
	runner.start("directory", "Creating directory structure...", 0.05)

	nerdDir, err := i.createDirectoryStructure()
	if err != nil {
		return "", fmt.Errorf("failed to create directory structure: %w", err)
	}

	if err := i.createMangleTemplates(nerdDir); err != nil {
		return "", fmt.Errorf("create mangle templates: %w", err)
	}

	fmt.Println("✓ Created .nerd/ directory structure")

	dbPath := filepath.Join(nerdDir, "knowledge.db")
	i.localDB, err = store.NewLocalStore(dbPath)
	if err != nil {
		return "", fmt.Errorf("initialize knowledge database: %w", err)
	} else {
		if err := i.ensureEmbeddingEngine(); err != nil {
			return "", err
		}
		i.localDB.SetEmbeddingEngine(i.embedEngine)
		result.FilesCreated = append(result.FilesCreated, dbPath)
		fmt.Println("✓ Initialized knowledge database")
	}

	northstarStore, err := northstar.NewStore(nerdDir)
	if err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to initialize Northstar store: %v", err))
	} else {
		northstarStore.Close()
		northstarDBPath := filepath.Join(nerdDir, "northstar_knowledge.db")
		result.FilesCreated = append(result.FilesCreated, northstarDBPath)
		fmt.Println("✓ Initialized Northstar vision guardian store")
	}

	runner.complete("directory")
	return nerdDir, nil
}

func (i *Initializer) runPhase2Scanning(ctx context.Context, runner *phaseRunner, result *InitResult) *world.ScanResult {
	runner.start("scanning", "Scanning codebase...", 0.10)
	fmt.Println("\n📊 Phase 2: Deep Codebase Scan")

	scanResult, err := i.scanner.ScanDirectory(ctx, i.config.Workspace)
	if err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Codebase scan failed: %v", err))
		runner.complete("scanning")
		return nil
	}

	fmt.Printf("   Scanned %d files in %d directories\n", scanResult.FileCount, scanResult.DirectoryCount)

	facts := scanResult.ToFacts()
	if len(facts) > 0 {
		if err := i.kernel.LoadFacts(facts); err != nil {
			result.Failures = append(result.Failures, fmt.Sprintf("Failed to load scan facts: %v", err))
		}
	}
	runner.complete("scanning")
	return scanResult
}

func (i *Initializer) runPhase3Analysis(runner *phaseRunner) {
	runner.start("analysis", "Preparing analysis framework...", 0.20)
	fmt.Println("\n🔬 Phase 3: Analysis Framework Setup")
	fmt.Println("   Analysis will be performed on-demand via JIT clean loop")
	runner.complete("analysis")
}

func (i *Initializer) runPhase4Profile(runner *phaseRunner, result *InitResult, nerdDir string, scanResult *world.ScanResult) ProjectProfile {
	runner.start("profile", "Building project profile...", 0.35)
	fmt.Println("\n📋 Phase 4: Building Project Profile")

	profile := i.buildProjectProfile()
	if scanResult != nil {
		profile.FileCount = scanResult.FileCount
		profile.DirectoryCount = scanResult.DirectoryCount
	}
	result.Profile = profile

	profilePath := filepath.Join(nerdDir, "profile.json")
	if err := i.saveProfile(profilePath, profile); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to save profile: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, profilePath)
		fmt.Println("✓ Saved project profile")
	}
	runner.complete("profile")
	return profile
}

func (i *Initializer) runPhase5Facts(runner *phaseRunner, result *InitResult, nerdDir string, profile ProjectProfile) {
	runner.start("facts", "Generating Mangle facts...", 0.45)
	fmt.Println("\n🧠 Phase 5: Generating Mangle Facts")

	factsPath := filepath.Join(nerdDir, "profile.mg")
	factsCount, err := i.generateFactsFile(factsPath, profile)
	if err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to generate facts: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, factsPath)
		result.FactsGenerated = factsCount
		fmt.Printf("✓ Generated %d Mangle facts\n", factsCount)
	}
	runner.complete("facts")
}

func (i *Initializer) runPhase5bPromptAtoms(runner *phaseRunner, result *InitResult, profile ProjectProfile) {
	runner.start("prompt_atoms", "Populating project-specific prompt atoms...", 0.47)
	fmt.Println("\n📝 Phase 5b: Populating Project-Specific Prompt Atoms")

	if err := i.populateProjectAtoms(profile); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to populate prompt atoms: %v", err))
	}
	runner.complete("prompt_atoms")
}

func (i *Initializer) runPhase5cPromptDB(ctx context.Context, runner *phaseRunner, result *InitResult, nerdDir string) {
	runner.start("prompt_db", "Initializing prompt corpus database...", 0.48)
	fmt.Println("\n📦 Phase 5c: Initializing Prompt Corpus Database")

	if err := i.initializePromptDatabase(ctx, nerdDir); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to initialize prompt database: %v", err))
	} else {
		corpusDBPath := filepath.Join(nerdDir, "prompts", "corpus.db")
		result.FilesCreated = append(result.FilesCreated, corpusDBPath)
	}
	runner.complete("prompt_db")
}

func (i *Initializer) runPhase6AnalyzeAgents(runner *phaseRunner, result *InitResult, profile ProjectProfile) []RecommendedAgent {
	runner.start("agents", "Analyzing required agents...", 0.50)
	fmt.Println("\n🤖 Phase 6: Determining Required Type 3 Agents")

	recommendedAgents := i.determineRequiredAgents(profile)
	result.RecommendedAgents = recommendedAgents
	fmt.Printf("   Recommended %d Type 3 agents for this project\n", len(recommendedAgents))

	for _, agent := range recommendedAgents {
		fmt.Printf("   • %s: %s\n", agent.Name, agent.Reason)
	}
	runner.complete("agents")
	return recommendedAgents
}

func (i *Initializer) runPhase7aCreateAgentKBs(ctx context.Context, runner *phaseRunner, result *InitResult, nerdDir string, recommendedAgents []RecommendedAgent) {
	if i.config.SkipAgentCreate || len(recommendedAgents) == 0 {
		runner.advance() // Skip shared_kb
		runner.advance() // Skip kb_creation
		return
	}

	runner.start("shared_kb", "Creating shared knowledge pool...", 0.52)
	fmt.Println("\n📚 Phase 7a: Creating Shared Knowledge Pool")

	sharedPoolErr := CreateSharedKnowledgePool(ctx, i.config.Workspace, func(status string, progress float64) {
		i.sendProgressWithETA("shared_kb", status, 0.52+progress*0.03, runner.remainingPhases)
	})
	if sharedPoolErr != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Shared knowledge pool creation had issues: %v", sharedPoolErr))
	} else {
		fmt.Println("   ✓ Shared knowledge pool ready")
	}
	runner.complete("shared_kb")

	runner.start("kb_creation", "Creating agent knowledge bases...", 0.55)
	fmt.Println("\n📚 Phase 7a.2: Creating Agent Knowledge Bases")

	createdAgents, agentKBs := i.createType3Agents(ctx, nerdDir, recommendedAgents, result)
	result.CreatedAgents = createdAgents
	result.AgentKBs = agentKBs

	i.registerAgentsWithShardManager(createdAgents)

	fmt.Printf("   Created %d Type 3 agents with knowledge bases\n", len(createdAgents))
	runner.complete("kb_creation")
}

func (i *Initializer) runPhase7bCreateCodebaseKB(ctx context.Context, runner *phaseRunner, result *InitResult, nerdDir string, profile ProjectProfile, scanResult *world.ScanResult) {
	runner.start("codebase_kb", "Creating codebase knowledge base...", 0.80)
	fmt.Println("\n📖 Phase 7b: Creating Codebase Knowledge Base")

	codebaseKBPath, codebaseAtoms, err := i.createCodebaseKnowledgeBase(ctx, nerdDir, profile, scanResult)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create codebase KB: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, codebaseKBPath)
		fmt.Printf("   ✓ Codebase KB ready (%d atoms)\n", codebaseAtoms)
	}

	fmt.Println("   🧠 Generating strategic knowledge...")
	if i.config.LLMClient != nil && i.localDB != nil {
		strategicKnowledge, err := i.generateStrategicKnowledge(ctx, profile, scanResult)
		if err != nil {
			result.Warnings = append(result.Warnings, fmt.Sprintf("Strategic knowledge generation failed: %v", err))
		} else if strategicKnowledge != nil {
			strategicAtoms, err := i.PersistStrategicKnowledge(ctx, strategicKnowledge, i.localDB)
			if err != nil {
				result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to persist strategic knowledge: %v", err))
			} else {
				fmt.Printf("   ✓ Strategic knowledge generated (%d atoms)\n", strategicAtoms)
				fmt.Printf("      Vision: %s\n", truncateString(strategicKnowledge.ProjectVision, 80))
			}
		}
	} else {
		result.Warnings = append(result.Warnings, "Strategic knowledge skipped (no LLM client or DB)")
	}
	runner.complete("codebase_kb")
}

func (i *Initializer) runPhase7cCreateCoreShardKBs(ctx context.Context, runner *phaseRunner, result *InitResult, nerdDir string, profile ProjectProfile) {
	runner.start("core_shards_kb", "Creating core shard knowledge bases...", 0.82)
	fmt.Println("\n🔧 Phase 7c: Creating Core Shard Knowledge Bases")

	coreShardKBs, err := i.createCoreShardKnowledgeBases(ctx, nerdDir, profile)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create core shard KBs: %v", err))
	} else {
		for name, atoms := range coreShardKBs {
			fmt.Printf("   ✓ %s KB ready (%d atoms)\n", strings.Title(name), atoms)
		}
	}
	runner.complete("core_shards_kb")
}

func (i *Initializer) runPhase7dCreateCampaignKB(ctx context.Context, runner *phaseRunner, result *InitResult, nerdDir string, profile ProjectProfile) {
	runner.start("campaign_kb", "Creating campaign knowledge base...", 0.84)
	fmt.Println("\n🎯 Phase 7d: Creating Campaign Knowledge Base")

	campaignKBPath, campaignAtoms, err := i.createCampaignKnowledgeBase(ctx, nerdDir, profile)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to create campaign KB: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, campaignKBPath)
		fmt.Printf("   ✓ Campaign KB ready (%d atoms)\n", campaignAtoms)
	}
	runner.complete("campaign_kb")
}

func (i *Initializer) runPhase7eGenerateTools(ctx context.Context, runner *phaseRunner, result *InitResult, nerdDir string, profile ProjectProfile) {
	runner.start("tool_generation", "Generating project-specific tools...", 0.86)
	fmt.Println("\n🛠️  Phase 7e: Generating Project-Specific Tools")

	generatedTools, err := i.generateProjectTools(ctx, nerdDir, profile)
	if err != nil {
		result.Warnings = append(result.Warnings, fmt.Sprintf("Failed to generate tools: %v", err))
	} else if len(generatedTools) > 0 {
		fmt.Printf("   ✓ Generated %d tools\n", len(generatedTools))
		if result.AgentKBs == nil {
			result.AgentKBs = make(map[string]int)
		}
		result.AgentKBs["_generated_tools"] = len(generatedTools)
	} else {
		fmt.Println("   ⓘ No tools generated (may be skipped or not needed)")
	}
	runner.complete("tool_generation")
}

func (i *Initializer) runPhase8Preferences(runner *phaseRunner, result *InitResult, nerdDir string) {
	runner.start("preferences", "Initializing preferences...", 0.88)
	fmt.Println("\n⚙️ Phase 8: Initializing Preferences")

	preferences := i.initPreferences()
	result.Preferences = preferences

	prefsPath := filepath.Join(nerdDir, "preferences.json")
	if err := i.savePreferences(prefsPath, preferences); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to save preferences: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, prefsPath)
		fmt.Println("✓ Initialized preferences")
	}
	runner.complete("preferences")
}

func (i *Initializer) runPhase9Session(runner *phaseRunner, result *InitResult, nerdDir string) {
	runner.start("session", "Creating session state...", 0.90)

	sessionPath := filepath.Join(nerdDir, "session.json")
	if err := i.initSessionState(sessionPath); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to init session: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, sessionPath)
	}
	runner.complete("session")
}

func (i *Initializer) runPhase10Tools(runner *phaseRunner, result *InitResult, nerdDir string, profile ProjectProfile) {
	runner.start("tools", "Generating tool definitions...", 0.92)
	fmt.Println("\n🔧 Phase 10: Generating Tool Definitions")

	detectedTech := []string{profile.Language}
	if profile.Framework != "" && profile.Framework != "unknown" {
		detectedTech = append(detectedTech, profile.Framework)
	}

	tools := GenerateToolsForProject(detectedTech)
	if err := SaveToolsToFile(nerdDir, tools); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to save tools: %v", err))
	} else {
		toolsFile := filepath.Join(nerdDir, "tools", "available_tools.json")
		result.FilesCreated = append(result.FilesCreated, toolsFile)
		fmt.Printf("   ✓ Generated %d tool definitions\n", len(tools))

		categories := make(map[string]int)
		for _, tool := range tools {
			categories[tool.Category]++
		}
		for cat, count := range categories {
			fmt.Printf("      - %s: %d\n", cat, count)
		}
	}
	runner.complete("tools")
}

func (i *Initializer) runPhase11Registry(runner *phaseRunner, result *InitResult, nerdDir string) {
	runner.start("registry", "Generating agent registry...", 0.93)

	registryPath := filepath.Join(nerdDir, "agents.json")
	if err := i.saveAgentRegistry(registryPath, result.CreatedAgents); err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to save agent registry: %v", err))
	} else {
		result.FilesCreated = append(result.FilesCreated, registryPath)
	}
	runner.complete("registry")
}

func (i *Initializer) runPhase12PromptSync(ctx context.Context, runner *phaseRunner, result *InitResult, nerdDir string) {
	runner.start("prompt_sync", "Syncing agent prompts to knowledge DBs...", 0.97)
	fmt.Println("\n📝 Phase 12: Syncing Agent Prompts")

	promptCount, syncErr := prompt.ReloadAllPrompts(ctx, nerdDir, i.embedEngine)
	if syncErr != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Failed to sync agent prompts: %v", syncErr))
		fmt.Printf("   ✗ Required failure: %v\n", syncErr)
	} else if promptCount > 0 {
		fmt.Printf("   ✓ Synced %d prompt atoms to knowledge DBs\n", promptCount)
		logging.Boot("Synced %d prompt atoms from YAML to knowledge DBs", promptCount)
	} else {
		fmt.Println("   ✓ No prompt atoms to sync")
	}
	runner.complete("prompt_sync")
}

func (i *Initializer) finalizeInitialization(runner *phaseRunner, result *InitResult, startTime time.Time, profile ProjectProfile) (*InitResult, error) {
	result.LLMMetrics = i.snapshotLLMMetrics()
	if result.LLMMetrics.Failed > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"LLM enrichment degraded: %d/%d calls failed (%s; last error: %s)",
			result.LLMMetrics.Failed, result.LLMMetrics.Attempts,
			formatLLMIdentity(result.LLMMetrics), result.LLMMetrics.LastError,
		))
	}
	validation, validationErr := ValidateAllAgentDBs(result.NerdDir)
	result.Validation = validation
	recordValidationOutcome(result, validation, validationErr)
	result.Success = initializationSucceeded(result)
	result.Duration = time.Since(startTime)

	if i.grounding != nil && i.grounding.IsGroundingAvailable() {
		result.GroundingEnabled = true
		i.mu.RLock()
		if len(i.groundingSources) > 0 {
			seen := make(map[string]bool)
			for _, src := range i.groundingSources {
				if !seen[src] {
					seen[src] = true
					result.GroundingSources = append(result.GroundingSources, src)
				}
			}
		}
		i.mu.RUnlock()
		if len(result.GroundingSources) > 0 {
			logging.Boot("Init grounded with %d unique sources from Gemini", len(result.GroundingSources))
		}
	}

	completionMessage := "Initialization complete!"
	if !result.Success {
		completionMessage = "Initialization completed with required failures"
	} else if result.LLMMetrics.Failed > 0 {
		completionMessage = "Initialization complete with degraded LLM enrichment"
	}
	runner.start("complete", completionMessage, 1.0)
	runner.complete("complete")

	i.printSummary(result, profile)

	return result, nil
}

func recordValidationOutcome(result *InitResult, summary *ValidationSummary, err error) {
	if result == nil {
		return
	}
	if err != nil {
		result.Failures = append(result.Failures, fmt.Sprintf("Knowledge-base validation failed: %v", err))
		return
	}
	if summary == nil {
		result.Failures = append(result.Failures, "Knowledge-base validation returned no result")
		return
	}
	if summary.TotalDBs == 0 {
		result.Failures = append(result.Failures, "Knowledge-base validation found no shard databases")
		return
	}
	if !summary.OverallValid {
		result.Failures = append(result.Failures, fmt.Sprintf(
			"Knowledge-base validation found %d/%d invalid shard databases",
			summary.InvalidDBs, summary.TotalDBs,
		))
	}
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
	if err := writeFileIfAbsent(extPath, []byte(extContent), 0644); err != nil {
		return fmt.Errorf("create extensions template: %w", err)
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
	if err := writeFileIfAbsent(policyPath, []byte(policyContent), 0644); err != nil {
		return fmt.Errorf("create policy overrides template: %w", err)
	}

	return nil
}

func initializationSucceeded(result *InitResult) bool {
	return result != nil && len(result.Failures) == 0
}

// writeFileIfAbsent atomically creates a seed file without replacing user
// content. O_EXCL closes the check-then-write race during concurrent init runs.
func writeFileIfAbsent(path string, content []byte, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		if os.IsExist(err) {
			return nil
		}
		return err
	}
	if _, err := f.Write(content); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return err
	}
	return nil
}

// printSummary prints structural results and separately labels enrichment.
func (i *Initializer) printSummary(result *InitResult, profile ProjectProfile) {
	fmt.Println("\n" + strings.Repeat("═", 60))
	if result.Success && result.LLMMetrics.Failed == 0 {
		fmt.Println("✅ INITIALIZATION COMPLETE")
	} else if result.Success {
		fmt.Println("⚠️ INITIALIZATION COMPLETE (ENRICHMENT DEGRADED)")
	} else {
		fmt.Println("❌ INITIALIZATION INCOMPLETE")
	}
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
			// This score is an atom-count population proxy, not semantic quality.
			if agent.QualityScore > 0 {
				fmt.Printf("   • %s: %d atoms (KB population score: %.0f%% - %s)\n",
					agent.Name, agent.KBSize, agent.QualityScore, agent.QualityRating)
			} else {
				fmt.Printf("   • %s (%d KB atoms) - %s\n", agent.Name, agent.KBSize, agent.Status)
			}
		}

		// Show average atom-count population score.
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
			fmt.Printf("\n   📊 Average KB Population Score: %.0f%% (atom-count proxy)\n", avgQuality)
		}
	}

	if result.LLMMetrics.Attempts > 0 {
		fmt.Printf("\n🤖 LLM Enrichment: %d/%d succeeded; %d failed (%s)\n",
			result.LLMMetrics.Succeeded, result.LLMMetrics.Attempts,
			result.LLMMetrics.Failed, formatLLMIdentity(result.LLMMetrics))
	} else {
		fmt.Println("\n🤖 LLM Enrichment: no calls attempted")
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
	if len(result.Failures) > 0 {
		fmt.Println("\n❌ Required initialization failures:")
		for _, failure := range result.Failures {
			fmt.Printf("   - %s\n", failure)
		}
	}

	// Post-init recommendations based on project analysis
	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("💡 Recommendations:")
	i.printRecommendations(result, profile)

	// Run post-init validation
	fmt.Println("\n" + strings.Repeat("─", 60))
	fmt.Println("🔍 Validating knowledge bases...")
	validationSummary := result.Validation
	if validationSummary == nil {
		fmt.Println("   ✗ Structural validation did not return a result")
	} else {
		if validationSummary.OverallValid {
			fmt.Printf("   ✓ All %d knowledge bases structurally validated\n", validationSummary.TotalDBs)
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
	if result.Success {
		fmt.Println("🚀 Next steps:")
		fmt.Println("   • Run `nerd chat` to start interactive session")
		fmt.Println("   • Use `/northstar` to define your project vision")
		fmt.Println("   • Use `/agents` to see available agents")
		fmt.Println("   • Use `/spawn <agent> <task>` to delegate tasks")
	} else {
		fmt.Println("🛠️ Recovery:")
		fmt.Println("   • Resolve the required failures above")
		fmt.Println("   • Re-run `nerd init --force` before starting chat")
	}
	fmt.Println(strings.Repeat("─", 60))
}

// printRecommendations prints context-aware recommendations based on init results.
func (i *Initializer) printRecommendations(result *InitResult, profile ProjectProfile) {
	recommendations := []string{}

	// Check for sparsely populated KBs. The score is based on atom count.
	for _, agent := range result.CreatedAgents {
		if agent.QualityScore > 0 && agent.QualityScore < 50 {
			recommendations = append(recommendations,
				fmt.Sprintf("Run `/init --force` to improve %s KB population (currently %.0f%%)", agent.Name, agent.QualityScore))
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
	SuspendedAt     *time.Time `json:"suspended_at,omitzero"`
	PendingQuestion string     `json:"pending_question,omitzero"`
	PendingOptions  []string   `json:"pending_options,omitzero"`

	// Context state
	ActiveStrategy string   `json:"active_strategy,omitzero"`
	ActiveGoals    []string `json:"active_goals,omitzero"`
	WorkingFacts   []string `json:"working_facts,omitzero"`

	// Conversation history (stored separately in sessions/ folder)
	HistoryFile string `json:"history_file,omitzero"`
}

// ChatMessage represents a single message in the conversation.
type ChatMessage struct {
	Role    string    `json:"role"` // "user" or "assistant"
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
