// Package init implements the "nerd init" cold-start initialization system.
// It creates the .nerd/ workspace, measures and orients the repository,
// derives the north star, recommends and builds the specialist agents, and
// writes the profile, facts, prompt corpus and tool definitions every later
// session boots from.
//
// Files:
//   - initializer.go: configuration, construction and the phase sequence
//   - phases.go: one function per phase, progress and completion
//   - llm_routing.go: which configured client serves which phase
//   - jit_integration.go: the withJITPrompt chokepoint every model call crosses
//   - phase_orient.go: orientation, document roles and the north-star draft
//   - agents*.go: specialist recommendation, prompts and knowledge bases
//   - project_tools.go: project command tools chosen from the repository's own commands
package init

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"codenerd/internal/atomjudge"
	"codenerd/internal/config"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/embedding"
	"codenerd/internal/logging"
	"codenerd/internal/northstar"
	"codenerd/internal/orient"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/store"
	"codenerd/internal/world"
)

var (
	initSecretKVPattern = regexp.MustCompile(`(?i)((?:"?(?:api[_-]?key|secret|token|password)"?\s*[:=]\s*"?|bearer\s+))([^"\s,&]+)`)
	initSecretPattern   = regexp.MustCompile(`(AIza[0-9A-Za-z_-]{10,}|sk-[A-Za-z0-9]{10,}|ctx7sk-[0-9a-f-]{8,}|gh[pousr]_[A-Za-z0-9]{10,}|xox[baprs]-[A-Za-z0-9-]{10,})`)
)

// InitProgress represents a progress update during initialization. Every
// field is measured: the running phase's number among TotalPhases, the time
// since init started, and Percent, the share of phases already finished.
type InitProgress struct {
	Phase          string  // Current phase name
	Message        string  // Human-readable status message
	Percent        float64 // Share of phases finished, 0.0 - 1.0
	IsError        bool    // True if this is an error message
	AgentUpdate    *AgentCreationUpdate
	ElapsedTime    time.Duration // Time elapsed since init started
	CurrentPhaseNo int           // Current phase number (1-based)
	TotalPhases    int           // Total number of phases
}

// AgentCreationUpdate provides details about agent creation progress.
type AgentCreationUpdate struct {
	AgentName     string
	AgentType     string
	Status        string // "creating", "researching", "ready", "failed"
	KBSize        int    // Knowledge base size (facts/atoms)
	AtomCount     int    // E1: Current atom count during research
	TopicProgress string // E1: Current topic being researched
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

	// Domains are the technology keys the agent is expert in: the catalog
	// keys that created it (orient_agent_domain). Written to agents.json.
	Domains []string `json:"domains,omitzero"`
}

// InitConfig holds configuration for initialization.
type InitConfig struct {
	Workspace string
	// LLMClient serves bulk enrichment (per-chunk document extraction,
	// grounded domain research), and every phase when PlannerClient is nil.
	LLMClient perception.LLMClient
	// PlannerClient serves the calls whose output every later phase and
	// session builds on: document roles and the north-star draft, the
	// domain-expert roster, strategic-knowledge synthesis, specialist prompt
	// generation and project tool selection. Nil runs them on LLMClient.
	// Provider and model identity are read from each client, never set here.
	PlannerClient perception.LLMClient
	// ShardManager, when set, is told about the agents init creates so a
	// running session can spawn them at once. Init never reconfigures it.
	ShardManager *coreshards.ShardManager
	// EmbeddingEngine, when set, is the engine init embeds with. Nil builds
	// one from the workspace .nerd/config.json.
	EmbeddingEngine embedding.EmbeddingEngine
	Interactive     bool          // Whether to prompt user for preferences
	Timeout         time.Duration // Bounds one model request; zero reads llm_timeouts.per_call_timeout at call time
	SkipResearch    bool          // Skip deep research phase (faster init)
	SkipAgentCreate bool          // Skip Type 3 agent creation
	PreferenceHints []string      // User-provided hints about preferences
	ProgressChan    chan InitProgress
	// Output receives the human-readable phase log. Nil discards it: a front
	// end that draws its own screen (the chat TUI) must not have init write
	// into it, and reads ProgressChan instead.
	Output         io.Writer
	Context7APIKey string // Context7 API key for LLM-optimized docs

	// TypeUAgents are user-defined agents from `nerd init --define-agent`.
	// They are merged into the recommended set in phase 6, so they receive the
	// same knowledge base, prompts.yaml and registry entry as detected agents.
	TypeUAgents []TypeUAgentDefinition

	// InteractiveIO overrides the reader/writer used for agent curation. When
	// nil and Interactive is set, init prompts on the process terminal (and
	// only when there is one). Tests and non-stdin front ends inject this to
	// drive selection deterministically.
	InteractiveIO *InteractiveConfig
}

// DefaultInitConfig returns the defaults for a command-line init: an
// interactive run that logs to standard output. Timeout stays zero so each
// model request reads llm_timeouts.per_call_timeout when it is made, after the
// workspace configuration has loaded.
func DefaultInitConfig(workspace string) InitConfig {
	if workspace == "" {
		workspace, _ = os.Getwd()
	}
	return InitConfig{
		Workspace:   workspace,
		Interactive: true,
		Output:      os.Stdout,
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
	// Modules is the per-module breakdown for multi-module workspaces.
	// The flat Dependencies field remains the merged view every existing consumer already reads.
	Modules []ModuleProfile `json:"modules,omitempty"`

	// Paths
	EntryPoints     []string `json:"entry_points,omitzero"`
	TestDirectories []string `json:"test_directories,omitzero"`
	ConfigFiles     []string `json:"config_files,omitzero"`

	// Stats
	FileCount      int `json:"file_count"`
	DirectoryCount int `json:"directory_count"`
	// TestFileCount is how many test files the scan found; tests on disk
	// are the scan's evidence for TestArchitect.
	TestFileCount int `json:"test_file_count,omitzero"`
}

// ModuleProfile describes one module in a multi-module workspace. A single
// module repository produces exactly one, rooted at ".".
type ModuleProfile struct {
	// Path is the module root relative to the workspace, slash-separated,
	// with "." for the workspace root itself.
	Path string `json:"path"`
	// Name is the module's declared name: the go.mod module path or the
	// package.json "name". Empty when the manifest does not declare one.
	Name string `json:"name,omitempty"`
	// Manifest is the manifest file this module was discovered from,
	// relative to the workspace (for example "services/api/go.mod").
	Manifest string `json:"manifest"`
	// Language is inferred from the manifest kind: go.mod -> "go",
	// package.json -> "javascript".
	Language string `json:"language,omitempty"`
	// Dependencies are the direct dependencies declared by THIS module only.
	Dependencies []DependencyInfo `json:"dependencies,omitempty"`
	// EntryPoints are entry points found under this module's root.
	EntryPoints []string `json:"entry_points,omitempty"`
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
	VisionDerived  bool               `json:"vision_derived"`
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

	// LogProblems counts the warnings and errors logged while this run
	// executed (every category, mirrored to the session problems log) and
	// names that file. Until 2026-10-27 the run reported only its own
	// curated Warnings: a cold start logged 44 embedding/store errors to
	// the problems log and its summary showed none of them.
	LogProblems InitLogProblems `json:"log_problems"`

	// Type 3 Agent Creation Results
	RecommendedAgents []RecommendedAgent `json:"recommended_agents,omitzero"`
	CreatedAgents     []CreatedAgent     `json:"created_agents,omitzero"`
	AgentKBs          map[string]int     `json:"agent_knowledge_bases,omitzero"` // agent name -> KB size

	// ProjectTools names the project command tools phase 7e chose from the
	// repository's own commands; phase 10 writes them to available_tools.json.
	ProjectTools []string `json:"project_tools,omitzero"`
}

// InitLogProblems is the delta of the session problems log across one
// Initialize run: how many warnings and errors were logged during it, and
// the file they were mirrored to ("" when logging never opened it).
type InitLogProblems struct {
	Path     string `json:"path,omitzero"`
	Warnings int64  `json:"warnings"`
	Errors   int64  `json:"errors"`
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

	// Domains are the technology keys the agent is expert in. Boot asserts
	// them as shard_agent_domain (internal/system/shard_expert_facts.go), so
	// the kernel can derive which tasks need the agent.
	Domains []string `json:"domains,omitzero"`
}

// Initializer handles the cold-start initialization process.
type Initializer struct {
	// atomCapOnce resolves jit.atom_max_chars once (atomMaxChars).
	atomCapOnce sync.Once
	atomCap     int
	// admissionOnce resolves the generated-atom admission check once
	// (atomAdmission, agents_admission.go).
	admissionOnce sync.Once
	admission     atomjudge.Admission

	config              InitConfig
	out                 io.Writer
	scanner             *world.Scanner
	localDB             *store.LocalStore
	shardMgr            *coreshards.ShardManager
	kernel              *core.RealKernel
	embedEngine         embedding.EmbeddingEngine
	orientation         *orient.Engine
	orientationSnapshot *orient.Snapshot
	orientedVision      *northstar.Vision

	// Concurrency
	mu          sync.RWMutex
	llmMetrics  InitLLMMetrics
	jitMu       sync.Mutex
	jitCompiler *prompt.JITPromptCompiler
	jitClosed   bool

	// projectAtoms carries phase 5b's generated atoms to phase 5c, which
	// ingests them into .nerd/prompts/corpus.db where the JIT compiler can see
	// them. Guarded by mu because the summary path reads it off the main
	// goroutine while KB phases run.
	projectAtoms []*store.PromptAtom

	// projectTools carries phase 7e's selection to phase 10, which writes it
	// into available_tools.json beside the language tools.
	projectTools []ToolDefinition

	// registeredTools is what phase 10 wrote to available_tools.json; phase
	// 11 limits agent tool grants to it.
	registeredTools []ToolDefinition

	// clock measures the running phase and the elapsed time for progress.
	clock *phaseClock

	// fetchTopic fetches one research topic's documentation. Nil means
	// Context7 through the modular research registry (topicFetcher); tests
	// set it to observe which topics a KB build researches.
	fetchTopic func(ctx context.Context, topic string) (string, error)

	// research shares fetched topics across the agents of one run, so a topic
	// two specialists both need is fetched once.
	research topicCache

	// problemsAtStart / embeddingFailuresAtStart snapshot the process-wide
	// counters captureLogProblems diffs into InitResult.LogProblems.
	problemsStartWarns, problemsStartErrs int64
	embeddingFailuresAtStart              int64
	embeddingFailureWarningRecorded       bool
}

// captureLogProblems records, on result, what this run logged: the
// problems-log delta since Initialize began, and — once — a warning naming
// how many knowledge atoms were stored without an embedding (each of those
// is stored but semantically unsearchable, and the store only logs it).
// Called by finalizeInitialization before the summary prints and again from
// Initialize's defer so early cancellation returns carry it too.
func (i *Initializer) captureLogProblems(result *InitResult) {
	path, warns, errs := logging.ProblemsSnapshot()
	result.LogProblems = InitLogProblems{
		Path:     path,
		Warnings: warns - i.problemsStartWarns,
		Errors:   errs - i.problemsStartErrs,
	}
	if n := store.KnowledgeEmbeddingFailures() - i.embeddingFailuresAtStart; n > 0 && !i.embeddingFailureWarningRecorded {
		i.embeddingFailureWarningRecorded = true
		result.Warnings = append(result.Warnings, fmt.Sprintf(
			"%d knowledge atom(s) were stored without a semantic embedding (embedding failed — see the session problems log); those atoms are not reachable by semantic search until a re-embed succeeds", n))
	}
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

	// Auto-detect Context7 API key if not explicitly provided (C1 enhancement)
	if initConfig.Context7APIKey == "" {
		if key := config.AutoDetectContext7APIKey(); key != "" {
			logging.Boot("Auto-detected Context7 API key from environment/config")
			initConfig.Context7APIKey = key
		}
	}

	kernel, err := core.NewRealKernelWithWorkspace(initConfig.Workspace)
	if err != nil {
		return nil, fmt.Errorf("failed to create kernel: %w", err)
	}

	out := initConfig.Output
	if out == nil {
		out = io.Discard
	}
	return &Initializer{
		config:      initConfig,
		out:         out,
		scanner:     world.NewScanner(),
		kernel:      kernel,
		embedEngine: initConfig.EmbeddingEngine,
		// A caller's shard manager is only told about created agents. Init
		// used to set its LLM client to LLMClient and start its system
		// shards: in a chat session that moved every later shard off the
		// worker client and spawned a second copy of each system shard.
		shardMgr: initConfig.ShardManager,
		clock:    newPhaseClock(len(initPhases)),
	}, nil
}

// output is where the human-readable phase log goes; an Initializer built
// without NewInitializer logs nowhere.
func (i *Initializer) output() io.Writer {
	if i.out == nil {
		return io.Discard
	}
	return i.out
}

// Close releases resources held by the initializer. Every resource is
// released even when an earlier one fails to close.
func (i *Initializer) Close() error {
	var errs []error
	if i.orientation != nil {
		if err := i.orientation.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close orientation engine: %w", err))
		}
		i.orientation = nil
	}
	i.jitMu.Lock()
	i.jitClosed = true
	compiler := i.jitCompiler
	i.jitCompiler = nil
	i.jitMu.Unlock()
	if compiler != nil {
		if err := compiler.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close JIT compiler: %w", err))
		}
	}
	if i.localDB != nil {
		if err := i.localDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close knowledge database: %w", err))
		}
		i.localDB = nil
	}
	return errors.Join(errs...)
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
	embCfg := ucEmb.EngineConfig()
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

// Initialize runs every phase in order. Required failures land in
// InitResult.Failures and make Success false; degraded enrichment lands in
// Warnings. Cancellation is checked between phases.
func (i *Initializer) Initialize(ctx context.Context) (*InitResult, error) {
	startTime := time.Now()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := &InitResult{
		FilesCreated:  make([]string, 0),
		Warnings:      make([]string, 0),
		AgentKBs:      make(map[string]int),
		CreatedAgents: make([]CreatedAgent, 0),
	}
	_, i.problemsStartWarns, i.problemsStartErrs = logging.ProblemsSnapshot()
	i.embeddingFailuresAtStart = store.KnowledgeEmbeddingFailures()
	defer func() { i.captureLogProblems(result) }()
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
	fmt.Fprintln(i.output(), "🚀 Initializing codeNERD...")
	fmt.Fprintf(i.output(), "   Workspace: %s\n\n", i.config.Workspace)

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

	// Phase 3: Orientation, which also drafts the north star. Everything after
	// it (profile, agents, knowledge bases) consumes what orientation derived.
	if err := i.runPhase3Orientation(ctx, runner, result, nerdDir); err != nil {
		return result, err
	}
	if err := checkContext("repository orientation"); err != nil {
		return result, err
	}

	// Phase 4: Build Profile
	profile := i.runPhase4Profile(runner, result, nerdDir, scanResult)
	i.persistOrientationKnowledge(ctx, profile, result)
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
	recommendedAgents := i.runPhase6AnalyzeAgents(ctx, runner, result, profile)
	if err := checkContext("agent analysis"); err != nil {
		return result, err
	}

	// Phase 7a: Create Agent Knowledge Bases
	i.runPhase7aCreateAgentKBs(ctx, runner, result, nerdDir, recommendedAgents)
	if err := checkContext("agent knowledge creation"); err != nil {
		return result, err
	}

	// Phase 7e: Choose project command tools
	i.runPhase7eProjectTools(ctx, runner, result, nerdDir, profile)
	if err := checkContext("project tool selection"); err != nil {
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

	return i.finalizeInitialization(runner, result, startTime, profile)
}
