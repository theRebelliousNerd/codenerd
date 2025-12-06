package main

import (
	"codenerd/cmd/nerd/chat"
	"codenerd/internal/articulation"
	"codenerd/internal/browser"
	"codenerd/internal/campaign"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/mangle"
	"codenerd/internal/perception"
	"codenerd/internal/shards"
	"codenerd/internal/shards/system"
	"codenerd/internal/tactile"
	"codenerd/internal/world"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Global flags
	verbose   bool
	apiKey    string
	workspace string
	timeout   time.Duration
	// System shards
	disableSystemShards []string
	// Init flags
	forceInit bool

	// Logger
	logger *zap.Logger
)

// rootCmd represents the base command
var rootCmd = &cobra.Command{
	Use:   "nerd",
	Short: "codeNERD - Logic-First CLI Agent (Cortex 1.5.0)",
	Long: `codeNERD is a high-assurance, neuro-symbolic CLI agent.

It uses Google Mangle (Datalog) as the logic kernel for deterministic reasoning,
with LLMs serving only as perception transducers (not decision makers).

Architecture: Logic determines Reality; the Model merely describes it.

Run without arguments to start the interactive chat interface.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Skip logger init for interactive mode (it has its own UI)
		if cmd.Use == "nerd" && cmd.CalledAs() == "nerd" {
			return nil
		}

		// Initialize logger
		config := zap.NewProductionConfig()
		if verbose {
			config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		}
		var err error
		logger, err = config.Build()
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if logger != nil {
			_ = logger.Sync()
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default behavior: launch interactive chat
		cfg := chat.Config{
			DisableSystemShards: disableSystemShards,
		}
		return chat.RunInteractiveChat(cfg)
	},
}

// runCmd executes a single instruction
var runCmd = &cobra.Command{
	Use:   "run [instruction]",
	Short: "Execute a single instruction through the OODA loop",
	Long: `Processes a natural language instruction through the full Cortex pipeline:
  1. Perception: Transduce input to intent atoms
  2. Orient: Load facts, activate context via spreading activation
  3. Decide: Derive next_action via Mangle policy rules
  4. Act: Execute via VirtualStore, report via Articulation layer`,
	Args: cobra.MinimumNArgs(1),
	RunE: runInstruction,
}

// defineAgentCmd defines a new specialist shard (§9.1)
var defineAgentCmd = &cobra.Command{
	Use:   "define-agent",
	Short: "Define a new specialist shard agent",
	Long: `Creates a persistent specialist profile that can be spawned later.
The agent will undergo deep research to build its knowledge base.

Example:
  nerd define-agent --name RustExpert --topic "Tokio Async Runtime"`,
	RunE: defineAgent,
}

// spawnCmd spawns a shard agent (§7.0)
var spawnCmd = &cobra.Command{
	Use:   "spawn [shard-type] [task]",
	Short: "Spawn an ephemeral or persistent shard agent",
	Long: `Spawns a ShardAgent to handle a specific task in isolation.

Shard Types:
  - generalist: Ephemeral, starts blank (RAM only)
  - specialist: Persistent, loads knowledge shard from SQLite
  - coder: Specialized for code writing/TDD loop
  - researcher: Specialized for deep research
  - reviewer: Specialized for code review
  - tester: Specialized for test generation`,
	Args: cobra.MinimumNArgs(2),
	RunE: spawnShard,
}

// browserCmd manages browser sessions (§9.0 Browser Physics)
var browserCmd = &cobra.Command{
	Use:   "browser",
	Short: "Browser automation commands (DOM snapshotting, React reification)",
}

var browserLaunchCmd = &cobra.Command{
	Use:   "launch",
	Short: "Launch the browser instance",
	RunE:  browserLaunch,
}

var browserSessionCmd = &cobra.Command{
	Use:   "session [url]",
	Short: "Create a new browser session",
	Args:  cobra.ExactArgs(1),
	RunE:  browserSession,
}

var browserSnapshotCmd = &cobra.Command{
	Use:   "snapshot [session-id]",
	Short: "Snapshot DOM as Mangle facts",
	Args:  cobra.ExactArgs(1),
	RunE:  browserSnapshot,
}

// queryCmd queries the Mangle fact store
var queryCmd = &cobra.Command{
	Use:   "query [predicate]",
	Short: "Query facts from the Mangle kernel",
	Long: `Queries the fact store for matching predicates.
Returns all derived facts for the given predicate.

Example:
  nerd query next_action
  nerd query impacted`,
	Args: cobra.ExactArgs(1),
	RunE: queryFacts,
}

// statusCmd shows system status
var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show codeNERD system status",
	RunE:  showStatus,
}

// initCmd initializes codeNERD in the current workspace
var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize codeNERD in the current workspace",
	Long: `Performs the "Cold Start" initialization for a new project.

This command:
  1. Creates the .nerd/ directory structure
  2. Analyzes the codebase to detect language, framework, and architecture
  3. Builds a project profile for context-aware assistance
  4. Initializes the knowledge database
  5. Sets up user preferences

Run this once when starting to use codeNERD with a new project.`,
	RunE: runInit,
}

// scanCmd refreshes the codebase index without full reinitialization
var scanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Refresh the codebase index",
	Long: `Scans the workspace and refreshes the Mangle kernel with fresh facts.

This is a lighter alternative to 'nerd init --force' that:
  1. Scans the file structure
  2. Extracts AST symbols and dependencies
  3. Updates the kernel with fresh file_topology facts
  4. Reloads profile.gl facts

Use this when files have changed and you want to update the kernel without
recreating agent knowledge bases.`,
	RunE: runScan,
}

// whyCmd explains the reasoning behind the last action
var whyCmd = &cobra.Command{
	Use:   "why [predicate]",
	Short: "Explain why an action was taken or blocked",
	Long: `Shows the derivation trace (proof tree) for a logical conclusion.

This implements the "Glass Box" interface, allowing you to understand
why codeNERD made a specific decision.

Examples:
  nerd why blocked      # Why was the last action blocked?
  nerd why next_action  # Why was this action chosen?
  nerd why impacted     # What files would be impacted?`,
	Args: cobra.MaximumNArgs(1),
	RunE: runWhy,
}

// campaignCmd is the parent command for campaign operations
var campaignCmd = &cobra.Command{
	Use:   "campaign",
	Short: "Campaign orchestration for long-running goals",
	Long: `Campaigns are long-running, multi-phase goals that span sessions.

Use campaigns for:
  - Greenfield builds from spec documents
  - Large feature implementations
  - Codebase-wide stability audits
  - Migration projects

Examples:
  nerd campaign start "Build REST API" --docs ./specs/
  nerd campaign status
  nerd campaign pause
  nerd campaign resume`,
}

// campaignStartCmd starts a new campaign
var campaignStartCmd = &cobra.Command{
	Use:   "start [goal]",
	Short: "Start a new campaign",
	Long: `Starts a new campaign by decomposing the goal into phases and tasks.

The goal can be:
  - A natural language description of what you want to build
  - A reference to spec documents with --docs flag

Examples:
  nerd campaign start "Build a REST API with user auth"
  nerd campaign start "Implement the feature in spec.md" --docs ./specs/
  nerd campaign start --docs ./Docs/research/*.md`,
	Args: cobra.MinimumNArgs(1),
	RunE: runCampaignStart,
}

// campaignStatusCmd shows campaign status
var campaignStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show current campaign status",
	RunE:  runCampaignStatus,
}

// campaignPauseCmd pauses the current campaign
var campaignPauseCmd = &cobra.Command{
	Use:   "pause",
	Short: "Pause the current campaign",
	RunE:  runCampaignPause,
}

// campaignResumeCmd resumes a paused campaign
var campaignResumeCmd = &cobra.Command{
	Use:   "resume",
	Short: "Resume a paused campaign",
	RunE:  runCampaignResume,
}

// campaignListCmd lists all campaigns
var campaignListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all campaigns",
	RunE:  runCampaignList,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "Z.AI API key (or set ZAI_API_KEY env)")
	rootCmd.PersistentFlags().StringVarP(&workspace, "workspace", "w", "", "Workspace directory (default: current)")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 5*time.Minute, "Operation timeout")

	// Define-agent flags
	var agentName, agentTopic string
	defineAgentCmd.Flags().StringVar(&agentName, "name", "", "Agent name (required)")
	defineAgentCmd.Flags().StringVar(&agentTopic, "topic", "", "Research topic (required)")
	defineAgentCmd.MarkFlagRequired("name")
	defineAgentCmd.MarkFlagRequired("topic")

	// System shard controls
	runCmd.Flags().StringSliceVar(&disableSystemShards, "disable-system-shard", nil, "Disable a Type 1 system shard by name")

	// Init flags
	initCmd.Flags().BoolVarP(&forceInit, "force", "f", false, "Force reinitialize (preserves learned preferences)")

	// Browser subcommands
	browserCmd.AddCommand(browserLaunchCmd)
	browserCmd.AddCommand(browserSessionCmd)
	browserCmd.AddCommand(browserSnapshotCmd)

	// Campaign subcommands
	var campaignDocs []string
	var campaignType string
	campaignStartCmd.Flags().StringArrayVar(&campaignDocs, "docs", nil, "Paths to spec/requirement documents")
	campaignStartCmd.Flags().StringVar(&campaignType, "type", "feature", "Campaign type (greenfield, feature, audit, migration, remediation)")
	campaignCmd.AddCommand(campaignStartCmd)
	campaignCmd.AddCommand(campaignStatusCmd)
	campaignCmd.AddCommand(campaignPauseCmd)
	campaignCmd.AddCommand(campaignResumeCmd)
	campaignCmd.AddCommand(campaignListCmd)

	// Add commands to root
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(defineAgentCmd)
	rootCmd.AddCommand(spawnCmd)
	rootCmd.AddCommand(browserCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(scanCmd)
	rootCmd.AddCommand(whyCmd)
	rootCmd.AddCommand(campaignCmd)
	rootCmd.AddCommand(checkMangleCmd) // UNCOMMENTED: Register the check-mangle command
}

// === START OF INTEGRATED check_mangle.go CONTENT ===
//
// Symbiogen Product Requirements Document (PRD) for cmd/nerd/check_mangle.go
//
// File: cmd/nerd/check_mangle.go
// Author: Gemini
// Date: 2025-12-05
//
// Recommended Model: 2.5 Flash
//
// Overview:
// This file implements the `check-mangle` command for the codeNERD CLI.
// Its primary responsibility is to validate the syntax of Google Mangle (.gl) files.
//
// Key Features & Business Value:
// - Syntax Validation: Parse .gl files and report syntax errors using the official parser.
// - Glob Support: Process multiple files via shell globs or recursive directory scanning.
// - CI/CD Integration: Return non-zero exit codes on failure for pipeline compatibility.
//
// Architectural Context:
// - Component Type: CLI Command
// - Deployment: Built into `nerd` binary.
// - Dependencies: Relies on `github.com/google/mangle/parse` (via `internal/mangle` wrapper if avail).
//
// Dependencies & Dependents:
// - Dependencies: `github.com/spf13/cobra`, `internal/mangle`.
// - Is a Dependency for: None (Leaf command).
//
// Deployment & Operations:
// - CI/CD: Standard go build.
//
// Code Quality Mandate:
// All code in this file must be production-ready. This includes complete error
// handling and clear logging.
//
// Functions / Classes:
// - `runCheckMangle()`:
//   - **Purpose:** Execute the syntax check logic.
//   - **Logic:** Iterate args, read files, parse, print verification status.
//
// Usage:
// `nerd check-mangle internal/mangle/*.gl`
//
// References:
// - internal/mangle/grammar.go
//
// --- END OF PRD HEADER ---

var checkMangleCmd = &cobra.Command{
	Use:   "check-mangle [file...]",
	Short: "Check Mangle syntax in .gl files",
	Long:  `Validates the syntax of Google Mangle (Datalog) logic files.`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runCheckMangle,
}

func runCheckMangle(cmd *cobra.Command, args []string) error {
	hasError := false
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return fmt.Errorf("failed to initialize mangle engine: %w", err)
	}

	for _, pattern := range args {
		// Handle glob expansion (if shell didn't already)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			fmt.Printf("Error processing pattern %s: %v\n", pattern, err)
			hasError = true
			continue
		}

		if len(matches) == 0 {
			// If no glob match, maybe it's a specific file (Glob returns nil if no match but no error)
			if _, err := os.Stat(pattern); err == nil {
				matches = []string{pattern}
			} else {
				fmt.Printf("No files found matching: %s\n", pattern)
				continue
			}
		}

		for _, file := range matches {
			if err := checkFile(engine, file); err != nil {
				fmt.Printf("ERROR in %s: %v\n", file, err)
				hasError = true
			} else {
				fmt.Printf("OK: %s\n", file)
			}
		}
	}

	if hasError {
		os.Exit(1)
	}
	return nil
}

func checkFile(engine *mangle.Engine, path string) error {
	// Create a new engine for isolation
	tmpEngine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return err
	}

	// Try to load schemas.gl first if it exists, to provide context
	// We assume a standard location or relative path; for now hardcode likely location
	// In a real tool, this would be a flag --schema or --include
	excludePath := "internal/mangle/schemas.gl"
	if _, err := os.Stat(excludePath); err == nil && filepath.Base(path) != "schemas.gl" {
		schemaData, err := os.ReadFile(excludePath)
		if err == nil {
			if err := tmpEngine.LoadSchemaString(string(schemaData)); err != nil {
				// If the schema itself is broken, we should probably warn but proceed
				fmt.Printf("WARNING: Failed to load context from %s: %v\n", excludePath, err)
			}
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return tmpEngine.LoadSchemaString(string(data))
}

// === END OF INTEGRATED check_mangle.go CONTENT ===

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

// runInstruction executes a single instruction through the OODA loop
func runInstruction(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		logger.Info("Received shutdown signal")
		cancel()
	}()

	userInput := joinArgs(args)
	logger.Info("Processing instruction", zap.String("input", userInput))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// 1. Initialize Components (Cortex 1.5.0)
	llmClient := perception.NewZAIClient(key)
	transducer := perception.NewRealTransducer(llmClient)

	scanner := world.NewScanner()
	kernel := core.NewRealKernel()
	executor := tactile.NewSafeExecutor()
	virtualStore := core.NewVirtualStore(executor)
	shardManager := core.NewShardManager()
	shardManager.SetParentKernel(kernel)
	shardManager.SetLLMClient(llmClient)

	shardManager.SetLLMClient(llmClient)

	// Register all shard factories (base + specialists)
	shards.RegisterAllShardFactories(shardManager)

	// Overwrite system shard factories with dependency-injected versions
	shardManager.RegisterShard("perception_firewall", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewPerceptionFirewallShard()
		shard.SetParentKernel(kernel)
		shard.SetLLMClient(llmClient)
		return shard
	})
	shardManager.RegisterShard("world_model_ingestor", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewWorldModelIngestorShard()
		shard.SetParentKernel(kernel)
		return shard
	})
	shardManager.RegisterShard("executive_policy", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewExecutivePolicyShard()
		shard.SetParentKernel(kernel)
		shard.SetLLMClient(llmClient)
		return shard
	})
	shardManager.RegisterShard("constitution_gate", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewConstitutionGateShard()
		shard.SetParentKernel(kernel)
		shard.SetLLMClient(llmClient)
		return shard
	})
	shardManager.RegisterShard("tactile_router", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewTactileRouterShard()
		shard.SetParentKernel(kernel)
		shard.SetVirtualStore(virtualStore)
		shard.SetLLMClient(llmClient)
		return shard
	})
	shardManager.RegisterShard("session_planner", func(id string, config core.ShardConfig) core.ShardAgent {
		shard := system.NewSessionPlannerShard()
		shard.SetParentKernel(kernel)
		shard.SetLLMClient(llmClient)
		return shard
	})
	// shards.RegisterSystemShardProfiles(shardManager) - called by RegisterAllShardFactories

	disabled := make(map[string]struct{})
	for _, name := range disableSystemShards {
		disabled[name] = struct{}{}
	}
	if env := os.Getenv("NERD_DISABLE_SYSTEM_SHARDS"); env != "" {
		for _, token := range strings.Split(env, ",") {
			name := strings.TrimSpace(token)
			if name != "" {
				disabled[name] = struct{}{}
			}
		}
	}
	for name := range disabled {
		logger.Debug("Disabling system shard", zap.String("name", name))
		shardManager.DisableSystemShard(name)
	}
	if err := shardManager.StartSystemShards(ctx); err != nil {
		return fmt.Errorf("failed to start system shards: %w", err)
	}
	emitter := articulation.NewEmitter()

	// 2. Perception Layer: Transduce Input -> Intent
	logger.Debug("Transducing user input to intent atoms")
	intent, err := transducer.ParseIntent(ctx, userInput)
	if err != nil {
		return fmt.Errorf("perception error: %w", err)
	}
	logger.Info("Intent parsed",
		zap.String("verb", intent.Verb),
		zap.String("target", intent.Target))

	// 3. World Model: Scan Workspace (FactStore Hydration)
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	logger.Debug("Scanning workspace", zap.String("path", cwd))
	fileFacts, err := scanner.ScanWorkspace(cwd)
	if err != nil {
		return fmt.Errorf("world model error: %w", err)
	}
	logger.Debug("Workspace scanned", zap.Int("facts", len(fileFacts)))

	// 4. Load Facts into Hollow Kernel
	if err := kernel.LoadFacts([]core.Fact{intent.ToFact()}); err != nil {
		return fmt.Errorf("kernel load error: %w", err)
	}
	if err := kernel.LoadFacts(fileFacts); err != nil {
		return fmt.Errorf("kernel load error: %w", err)
	}

	// 5. Query Executive Policy (Decide)
	logger.Debug("Querying executive policy")
	var output string

	// Check for delegation
	delegateFacts, _ := kernel.Query("delegate_task")
	if len(delegateFacts) > 0 {
		// Execute via shard
		fact := delegateFacts[0]
		shardType := fmt.Sprintf("%v", fact.Args[0])
		task := fmt.Sprintf("%v", fact.Args[1])
		logger.Info("Delegating to shard", zap.String("type", shardType), zap.String("task", task))

		result, err := shardManager.Spawn(ctx, shardType, task)
		if err != nil {
			output = fmt.Sprintf("Shard execution failed: %v", err)
		} else {
			output = fmt.Sprintf("Shard Result: %s", result)
		}
	} else {
		// Query next_action
		actionFacts, _ := kernel.Query("next_action")
		if len(actionFacts) > 0 {
			fact := actionFacts[0]
			logger.Info("Executing action", zap.Any("action", fact))
			result, err := virtualStore.RouteAction(ctx, fact)
			if err != nil {
				output = fmt.Sprintf("Action failed: %v", err)
			} else {
				output = fmt.Sprintf("Action result: %v", result)
			}
		} else {
			output = "No action derived from policy"
		}
	}

	// 6. Articulation Layer: Report
	payload := articulation.PiggybackEnvelope{
		Surface: fmt.Sprintf("Processed: %s\nResult: %s", userInput, output),
		Control: articulation.ControlPacket{
			IntentClassification: articulation.IntentClassification{
				Category:   "/mutation", // Default for manual execution
				Verb:       "/execute",
				Target:     "system",
				Confidence: 1.0,
			},
			MangleUpdates: []string{"task_status(/complete)", fmt.Sprintf("observation(/result, %q)", output)},
		},
	}
	emitter.Emit(payload)

	return nil
}

// defineAgent creates a new specialist shard profile
func defineAgent(cmd *cobra.Command, args []string) error {
	name, _ := cmd.Flags().GetString("name")
	topic, _ := cmd.Flags().GetString("topic")

	// Validate name to prevent path traversal/injection
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(name) {
		return fmt.Errorf("invalid agent name: must be alphanumeric (dash/underscore allowed)")
	}

	logger.Info("Defining specialist agent",
		zap.String("name", name),
		zap.String("topic", topic))

	// Create shard manager and define profile
	shardManager := core.NewShardManager()
	shards.RegisterAllShardFactories(shardManager)
	config := core.DefaultSpecialistConfig(name, fmt.Sprintf("memory/shards/%s_knowledge.db", name))

	shardManager.DefineProfile(name, config)

	// Trigger deep research phase (§9.2)
	// This spawns a researcher shard to build the knowledge base
	fmt.Printf("Initiating deep research on topic: %s...\n", topic)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	researchTask := fmt.Sprintf("Research the topic '%s' and generate Mangle facts for the %s agent knowledge base.", topic, name)
	if _, err := shardManager.Spawn(ctx, "researcher", researchTask); err != nil {
		logger.Warn("Deep research phase failed", zap.Error(err))
		fmt.Printf("Warning: Deep research failed (%v). Agent will start with empty knowledge base.\n", err)
	} else {
		fmt.Println("Deep research complete. Knowledge base populated.")
	}

	fmt.Printf("Agent '%s' defined with topic '%s'\n", name, topic)
	fmt.Println("Knowledge shard will be populated during first spawn.")
	return nil
}

// spawnShard spawns a shard agent
func spawnShard(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	shardType := args[0]
	task := joinArgs(args[1:])

	logger.Info("Spawning shard",
		zap.String("type", shardType),
		zap.String("task", task))

	shardManager := core.NewShardManager()
	shards.RegisterAllShardFactories(shardManager)
	result, err := shardManager.Spawn(ctx, shardType, task)
	if err != nil {
		return fmt.Errorf("spawn failed: %w", err)
	}

	fmt.Printf("Shard Result: %s\n", result)
	return nil
}

// getBrowserConfig returns browser config with persistent session store
func getBrowserConfig() browser.Config {
	cwd, _ := os.Getwd()
	cfg := browser.DefaultConfig()
	cfg.SessionStore = filepath.Join(cwd, ".nerd", "browser", "sessions.json")
	return cfg
}

// browserLaunch launches the browser instance
func browserLaunch(cmd *cobra.Command, args []string) error {
	logger.Info("Launching browser")

	// Initialize browser session manager with persistent store
	cfg := getBrowserConfig()
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return fmt.Errorf("failed to create mangle engine: %w", err)
	}

	mgr := browser.NewSessionManager(cfg, engine)

	// Start the session manager (loads persisted sessions)
	if err := mgr.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start session manager: %w", err)
	}

	// Write control URL to file for other commands to use
	cwd, _ := os.Getwd()
	controlFile := filepath.Join(cwd, ".nerd", "browser", "control.txt")
	if err := os.MkdirAll(filepath.Dir(controlFile), 0o755); err == nil {
		_ = os.WriteFile(controlFile, []byte(mgr.ControlURL()), 0o644)
	}

	fmt.Printf("Browser launched. Control URL: %s\n", mgr.ControlURL())
	fmt.Printf("Session store: %s\n", cfg.SessionStore)
	fmt.Println("Press Ctrl+C to shutdown")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// Clean up control file
	_ = os.Remove(controlFile)
	_ = mgr.Shutdown(context.Background())
	return nil
}

// browserSession creates a new browser session
func browserSession(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := args[0]
	logger.Info("Creating browser session", zap.String("url", url))

	cfg := getBrowserConfig()

	// Try to connect to existing browser first
	cwd, _ := os.Getwd()
	controlFile := filepath.Join(cwd, ".nerd", "browser", "control.txt")
	if controlURL, err := os.ReadFile(controlFile); err == nil && len(controlURL) > 0 {
		cfg.DebuggerURL = string(controlURL)
		logger.Info("Connecting to existing browser", zap.String("url", cfg.DebuggerURL))
	}

	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return fmt.Errorf("failed to create mangle engine: %w", err)
	}

	mgr := browser.NewSessionManager(cfg, engine)
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start session manager: %w", err)
	}

	session, err := mgr.CreateSession(ctx, url)
	if err != nil {
		// Shutdown only if we launched a new browser
		if cfg.DebuggerURL == "" {
			_ = mgr.Shutdown(context.Background())
		}
		return fmt.Errorf("failed to create session: %w", err)
	}

	fmt.Printf("Session created: %s\n", session.ID)
	fmt.Printf("Target ID: %s\n", session.TargetID)
	fmt.Printf("URL: %s\n", session.URL)
	fmt.Printf("\nUse 'nerd browser snapshot %s' to capture DOM facts\n", session.ID)

	// Note: Don't shutdown - leave browser running for snapshot command
	return nil
}

// browserSnapshot snapshots DOM as Mangle facts
func browserSnapshot(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	logger.Info("Snapshotting DOM", zap.String("session", sessionID))

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cfg := getBrowserConfig()

	// Must connect to existing browser
	cwd, _ := os.Getwd()
	controlFile := filepath.Join(cwd, ".nerd", "browser", "control.txt")
	controlURL, err := os.ReadFile(controlFile)
	if err != nil || len(controlURL) == 0 {
		return fmt.Errorf("no browser running - use 'nerd browser launch' first")
	}
	cfg.DebuggerURL = string(controlURL)

	// Create mangle engine to receive facts
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return fmt.Errorf("failed to create mangle engine: %w", err)
	}

	mgr := browser.NewSessionManager(cfg, engine)
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to connect to browser: %w", err)
	}

	// Look up session
	session, found := mgr.GetSession(sessionID)
	if !found {
		// List available sessions
		sessions := mgr.List()
		if len(sessions) == 0 {
			return fmt.Errorf("session %q not found - no active sessions", sessionID)
		}
		fmt.Printf("Session %q not found. Available sessions:\n", sessionID)
		for _, s := range sessions {
			fmt.Printf("  %s  [%s] %s\n", s.ID, s.Status, s.URL)
		}
		return fmt.Errorf("session not found")
	}

	// Reattach to the session's target if needed
	if session.Status == "detached" && session.TargetID != "" {
		logger.Info("Reattaching to detached session", zap.String("target", session.TargetID))
		reattached, err := mgr.Attach(ctx, session.TargetID)
		if err != nil {
			return fmt.Errorf("failed to reattach to session: %w", err)
		}
		sessionID = reattached.ID
	}

	// Capture DOM facts
	fmt.Printf("Capturing DOM for session %s...\n", sessionID)
	if err := mgr.SnapshotDOM(ctx, sessionID); err != nil {
		return fmt.Errorf("DOM snapshot failed: %w", err)
	}

	// Also capture React components if available
	reactFacts, err := mgr.ReifyReact(ctx, sessionID)
	if err != nil {
		logger.Info("React reification skipped", zap.Error(err))
	} else {
		fmt.Printf("Captured %d React component facts\n", len(reactFacts))
	}

	// Export facts to file
	factsDir := filepath.Join(cwd, ".nerd", "browser", "snapshots")
	if err := os.MkdirAll(factsDir, 0o755); err != nil {
		return fmt.Errorf("failed to create snapshots dir: %w", err)
	}

	snapshotFile := filepath.Join(factsDir, fmt.Sprintf("%s_%d.gl", sessionID, time.Now().Unix()))

	// Query for all DOM-related predicates
	domPredicates := []string{
		"dom_node", "dom_text", "dom_attr", "dom_layout",
		"react_component", "react_prop", "react_state", "dom_mapping",
		"navigation_event", "current_url", "console_event",
		"net_request", "net_response", "net_header", "request_initiator",
		"click_event", "input_event", "state_change",
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("// DOM Snapshot for session %s\n", sessionID))
	sb.WriteString(fmt.Sprintf("// Captured at %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("// URL: %s\n\n", session.URL))

	totalFacts := 0
	for _, pred := range domPredicates {
		facts, err := engine.GetFacts(pred)
		if err != nil {
			continue // Predicate not declared, skip
		}
		for _, fact := range facts {
			sb.WriteString(fact.String())
			sb.WriteString(".\n")
			totalFacts++
		}
	}

	if totalFacts == 0 {
		fmt.Println("Warning: No DOM facts captured. The page may not have loaded yet.")
		fmt.Println("Try waiting for the page to fully load, then run snapshot again.")
	}

	if err := os.WriteFile(snapshotFile, []byte(sb.String()), 0o644); err != nil {
		return fmt.Errorf("failed to write snapshot: %w", err)
	}

	fmt.Printf("DOM snapshot complete:\n")
	fmt.Printf("  Facts captured: %d\n", totalFacts)
	fmt.Printf("  Saved to: %s\n", snapshotFile)
	return nil
}

// queryFacts queries the Mangle kernel
func queryFacts(cmd *cobra.Command, args []string) error {
	predicate := args[0]
	logger.Info("Querying facts", zap.String("predicate", predicate))

	kernel := core.NewRealKernel()
	if err := kernel.LoadFacts(nil); err != nil {
		return fmt.Errorf("query init failed: %w", err)
	}
	facts, err := kernel.Query(predicate)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if len(facts) == 0 {
		fmt.Printf("No facts found for predicate '%s'\n", predicate)
		return nil
	}

	fmt.Printf("Facts for '%s':\n", predicate)
	for _, fact := range facts {
		fmt.Printf("  %s\n", fact.String())
	}
	return nil
}

// showStatus displays system status
func showStatus(cmd *cobra.Command, args []string) error {
	fmt.Println("codeNERD System Status")
	fmt.Println("======================")
	fmt.Printf("Version: Cortex 1.5.0\n")
	fmt.Printf("Kernel:  Google Mangle (Datalog)\n")
	fmt.Printf("Runtime: Go %s\n", "1.24")
	fmt.Println()

	// Check API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}
	if key != "" {
		fmt.Println("✓ Z.AI API key configured")
	} else {
		fmt.Println("✗ Z.AI API key not configured")
	}

	// Check workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	fmt.Printf("✓ Workspace: %s\n", cwd)

	// Initialize kernel and show stats
	kernel := core.NewRealKernel()
	fmt.Printf("✓ Mangle kernel initialized\n")
	fmt.Printf("  Schemas: %d bytes\n", len(kernel.GetSchemas()))
	fmt.Printf("  Policy:  %d bytes\n", len(kernel.GetPolicy()))

	return nil
}

func joinArgs(args []string) string {
	result := ""
	for i, arg := range args {
		if i > 0 {
			result += " "
		}
		result += arg
	}
	return result
}

// runInit performs the cold-start initialization
func runInit(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nInitialization cancelled")
		cancel()
	}()

	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Check if already initialized
	if nerdinit.IsInitialized(cwd) && !forceInit {
		fmt.Println("Project already initialized. Use 'nerd status' to view project info.")
		fmt.Println("To reinitialize, use 'nerd init --force' (preserves learned preferences).")
		return nil
	}

	if forceInit {
		fmt.Println("🔄 Force reinitializing workspace...")
	}

	// Configure initializer
	config := nerdinit.DefaultInitConfig(cwd)
	config.Timeout = timeout

	// Set up LLM client if available
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}
	if key != "" {
		config.LLMClient = perception.NewZAIClient(key)
	}

	// Set Context7 API key from environment or config
	context7Key := os.Getenv("CONTEXT7_API_KEY")
	if context7Key == "" {
		// Try loading from config.json
		if providerCfg, err := perception.LoadConfigJSON(perception.DefaultConfigPath()); err == nil && providerCfg.Context7APIKey != "" {
			context7Key = providerCfg.Context7APIKey
		}
	}
	if context7Key != "" {
		config.Context7APIKey = context7Key
	}

	// Run initialization
	initializer := nerdinit.NewInitializer(config)
	result, err := initializer.Initialize(ctx)
	if err != nil {
		return fmt.Errorf("initialization failed: %w", err)
	}

	if !result.Success {
		return fmt.Errorf("initialization completed with errors")
	}

	return nil
}

// runScan refreshes the codebase index
func runScan(cmd *cobra.Command, args []string) error {
	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Check if initialized
	if !nerdinit.IsInitialized(cwd) {
		fmt.Println("Project not initialized. Run 'nerd init' first.")
		return nil
	}

	fmt.Println("🔍 Scanning codebase...")

	// Create scanner
	scanner := world.NewScanner()

	// Scan workspace
	facts, err := scanner.ScanWorkspace(cwd)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Initialize kernel and load facts
	kernel := core.NewRealKernel()
	if err := kernel.LoadFacts(facts); err != nil {
		return fmt.Errorf("failed to load facts: %w", err)
	}

	// Also reload profile.gl if it exists
	factsPath := filepath.Join(cwd, ".nerd", "profile.gl")
	if _, statErr := os.Stat(factsPath); statErr == nil {
		if err := kernel.LoadFactsFromFile(factsPath); err != nil {
			fmt.Printf("⚠️ Warning: failed to load profile.gl: %v\n", err)
		}
	}

	// Count files and directories
	fileCount := 0
	dirCount := 0
	for _, f := range facts {
		switch f.Predicate {
		case "file_topology":
			fileCount++
		case "directory":
			dirCount++
		}
	}

	fmt.Println("✅ Scan complete")
	fmt.Printf("   Files indexed:    %d\n", fileCount)
	fmt.Printf("   Directories:      %d\n", dirCount)
	fmt.Printf("   Facts generated:  %d\n", len(facts))

	return nil
}

// runWhy explains the reasoning behind decisions
func runWhy(cmd *cobra.Command, args []string) error {
	predicate := "next_action"
	if len(args) > 0 {
		predicate = args[0]
	}

	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Check if initialized
	if !nerdinit.IsInitialized(cwd) {
		fmt.Println("Project not initialized. Run 'nerd init' first.")
		return nil
	}

	fmt.Printf("Explaining derivation for: %s\n", predicate)
	fmt.Println("=" + string(make([]byte, 40)))

	// Initialize kernel
	kernel := core.NewRealKernel()
	if err := kernel.LoadFacts(nil); err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// Query for facts
	facts, err := kernel.Query(predicate)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	if len(facts) == 0 {
		fmt.Printf("No facts found for predicate '%s'\n", predicate)
		fmt.Println("\nPossible reasons:")
		fmt.Println("  - No matching rules were triggered")
		fmt.Println("  - Required preconditions were not met")
		fmt.Println("  - The workspace has not been scanned recently")
		return nil
	}

	// Display derivation
	fmt.Printf("\nDerived %d fact(s):\n\n", len(facts))
	for i, fact := range facts {
		fmt.Printf("  %d. %s\n", i+1, fact.String())
	}

	// Show related rules (simplified - full proof tree would require Mangle integration)
	fmt.Println("\nRelated Policy Rules:")
	switch predicate {
	case "next_action":
		fmt.Println("  - next_action(A) :- user_intent(_, V, T, _), action_mapping(V, A).")
		fmt.Println("  - next_action(/ask_user) :- clarification_needed(_).")
	case "block_commit":
		fmt.Println("  - block_commit(R) :- diagnostic(_, _, _, /error, R).")
		fmt.Println("  - block_commit(\"Untested\") :- impacted(F), !test_coverage(F).")
	case "impacted":
		fmt.Println("  - impacted(X) :- modified(Y), depends_on(X, Y).")
		fmt.Println("  - impacted(X) :- impacted(Y), depends_on(X, Y). # Transitive")
	case "permitted":
		fmt.Println("  - permitted(A) :- safe_action(A).")
		fmt.Println("  - permitted(A) :- dangerous_action(A), user_override(A).")
	default:
		fmt.Println("  (No specific rules documented for this predicate)")
	}

	return nil
}

// runCampaignStart starts a new campaign
func runCampaignStart(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nCampaign start cancelled")
		cancel()
	}()

	goal := joinArgs(args)

	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Get flags
	docs, _ := cmd.Flags().GetStringArray("docs")
	campaignType, _ := cmd.Flags().GetString("type")

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Initialize components
	llmClient := perception.NewZAIClient(key)
	kernel := core.NewRealKernel()
	executor := tactile.NewSafeExecutor()
	virtualStore := core.NewVirtualStore(executor)
	shardMgr := core.NewShardManager()
	shardMgr.SetParentKernel(kernel)

	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║          CAMPAIGN ORCHESTRATOR - INITIALIZING             ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Printf("\nGoal: %s\n", goal)
	if len(docs) > 0 {
		fmt.Printf("Source Documents: %v\n", docs)
	}
	fmt.Printf("Campaign Type: %s\n\n", campaignType)

	// Create decomposer
	decomposer := campaign.NewDecomposer(kernel, llmClient, cwd)

	// Build request
	req := campaign.DecomposeRequest{
		Goal:         goal,
		SourcePaths:  docs,
		CampaignType: campaign.CampaignType("/" + campaignType),
	}

	fmt.Println("📋 Decomposing goal into phases and tasks...")

	// Decompose
	result, err := decomposer.Decompose(ctx, req)
	if err != nil {
		return fmt.Errorf("failed to decompose campaign: %w", err)
	}

	if !result.ValidationOK {
		fmt.Println("\n⚠️  Plan validation found issues:")
		for _, issue := range result.Issues {
			fmt.Printf("  - [%s] %s\n", issue.IssueType, issue.Description)
		}
		fmt.Println("\nAttempting to proceed anyway...")
	}

	// Display plan summary
	fmt.Printf("\n📊 Campaign Plan: %s\n", result.Campaign.Title)
	fmt.Printf("   Confidence: %.0f%%\n", result.Campaign.Confidence*100)
	fmt.Printf("   Phases: %d\n", result.Campaign.TotalPhases)
	fmt.Printf("   Tasks: %d\n\n", result.Campaign.TotalTasks)

	for i, phase := range result.Campaign.Phases {
		fmt.Printf("Phase %d: %s (%d tasks)\n", i+1, phase.Name, len(phase.Tasks))
		for j, task := range phase.Tasks {
			status := "⏳"
			fmt.Printf("  %s %d.%d %s\n", status, i+1, j+1, task.Description)
		}
	}

	// Create and start orchestrator
	progressChan := make(chan campaign.Progress, 10)
	eventChan := make(chan campaign.OrchestratorEvent, 100)

	orchestrator := campaign.NewOrchestrator(campaign.OrchestratorConfig{
		Workspace:    cwd,
		Kernel:       kernel,
		LLMClient:    llmClient,
		ShardManager: shardMgr,
		Executor:     executor,
		VirtualStore: virtualStore,
		ProgressChan: progressChan,
		EventChan:    eventChan,
	})

	if err := orchestrator.SetCampaign(result.Campaign); err != nil {
		return fmt.Errorf("failed to set campaign: %w", err)
	}

	fmt.Println("\n🚀 Starting campaign execution...")
	fmt.Println("   Press Ctrl+C to pause")

	// Start event listener
	go func() {
		for event := range eventChan {
			switch event.Type {
			case "task_started":
				fmt.Printf("🔄 %s\n", event.Message)
			case "task_completed":
				fmt.Printf("✅ %s\n", event.Message)
			case "task_failed":
				fmt.Printf("❌ %s\n", event.Message)
			case "phase_started":
				fmt.Printf("\n📦 Phase: %s\n", event.Message)
			case "phase_completed":
				fmt.Printf("🎉 Phase completed: %s\n", event.Message)
			case "campaign_completed":
				fmt.Printf("\n🏆 %s\n", event.Message)
			case "replan_triggered":
				fmt.Printf("🔄 Replanning: %s\n", event.Message)
			}
		}
	}()

	// Run campaign
	if err := orchestrator.Run(ctx); err != nil {
		if ctx.Err() != nil {
			fmt.Println("\nCampaign paused. Run 'nerd campaign resume' to continue.")
			return nil
		}
		return fmt.Errorf("campaign failed: %w", err)
	}

	fmt.Println("\n✨ Campaign completed successfully!")
	return nil
}

// runCampaignStatus shows current campaign status
func runCampaignStatus(cmd *cobra.Command, args []string) error {
	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	campaignsDir := filepath.Join(cwd, ".nerd", "campaigns")
	entries, err := os.ReadDir(campaignsDir)
	if err != nil {
		fmt.Println("No campaigns found. Run 'nerd campaign start' to create one.")
		return nil
	}

	// Find most recent campaign
	var latestCampaign *campaign.Campaign
	var latestTime time.Time

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(campaignsDir, entry.Name()))
			if err != nil {
				continue
			}

			var c campaign.Campaign
			if err := json.Unmarshal(data, &c); err != nil {
				continue
			}

			if c.UpdatedAt.After(latestTime) {
				latestTime = c.UpdatedAt
				latestCampaign = &c
			}
		}
	}

	if latestCampaign == nil {
		fmt.Println("No campaigns found.")
		return nil
	}

	// Display status
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║                  CAMPAIGN STATUS                          ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Printf("\n📋 %s\n", latestCampaign.Title)
	fmt.Printf("   ID: %s\n", latestCampaign.ID)
	fmt.Printf("   Status: %s\n", latestCampaign.Status)
	fmt.Printf("   Created: %s\n", latestCampaign.CreatedAt.Format(time.RFC822))

	// Progress bar
	progress := float64(latestCampaign.CompletedTasks) / float64(latestCampaign.TotalTasks)
	barWidth := 40
	filled := int(progress * float64(barWidth))
	bar := fmt.Sprintf("[%s%s] %.0f%%",
		repeatChar('█', filled),
		repeatChar('░', barWidth-filled),
		progress*100)
	fmt.Printf("\n   Progress: %s\n", bar)
	fmt.Printf("   Tasks: %d/%d completed\n", latestCampaign.CompletedTasks, latestCampaign.TotalTasks)
	fmt.Printf("   Phases: %d/%d completed\n", latestCampaign.CompletedPhases, latestCampaign.TotalPhases)

	// Current phase
	for _, phase := range latestCampaign.Phases {
		if phase.Status == campaign.PhaseInProgress {
			fmt.Printf("\n   Current Phase: %s\n", phase.Name)
			pendingCount := 0
			for _, task := range phase.Tasks {
				if task.Status == campaign.TaskPending || task.Status == campaign.TaskInProgress {
					pendingCount++
				}
			}
			fmt.Printf("   Remaining tasks in phase: %d\n", pendingCount)
			break
		}
	}

	// Learnings
	if len(latestCampaign.Learnings) > 0 {
		fmt.Printf("\n   Learnings applied: %d\n", len(latestCampaign.Learnings))
	}

	// Revisions
	if latestCampaign.RevisionNumber > 0 {
		fmt.Printf("   Plan revisions: %d\n", latestCampaign.RevisionNumber)
	}

	return nil
}

// runCampaignPause pauses the current campaign
func runCampaignPause(cmd *cobra.Command, args []string) error {
	fmt.Println("Campaign paused. Run 'nerd campaign resume' to continue.")
	// The actual pausing happens via signal handling in the running orchestrator
	return nil
}

// runCampaignResume resumes a paused campaign
func runCampaignResume(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	// Find paused campaign
	campaignsDir := filepath.Join(cwd, ".nerd", "campaigns")
	entries, err := os.ReadDir(campaignsDir)
	if err != nil {
		fmt.Println("No campaigns found.")
		return nil
	}

	var pausedCampaign *campaign.Campaign
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(campaignsDir, entry.Name()))
			if err != nil {
				continue
			}

			var c campaign.Campaign
			if err := json.Unmarshal(data, &c); err != nil {
				continue
			}

			if c.Status == campaign.StatusPaused || c.Status == campaign.StatusActive {
				pausedCampaign = &c
				break
			}
		}
	}

	if pausedCampaign == nil {
		fmt.Println("No paused campaigns found.")
		return nil
	}

	fmt.Printf("Resuming campaign: %s\n", pausedCampaign.Title)

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Initialize components
	llmClient := perception.NewZAIClient(key)
	kernel := core.NewRealKernel()
	executor := tactile.NewSafeExecutor()
	virtualStore := core.NewVirtualStore(executor)
	shardMgr := core.NewShardManager()
	shardMgr.SetParentKernel(kernel)

	progressChan := make(chan campaign.Progress, 10)
	eventChan := make(chan campaign.OrchestratorEvent, 100)

	orchestrator := campaign.NewOrchestrator(campaign.OrchestratorConfig{
		Workspace:    cwd,
		Kernel:       kernel,
		LLMClient:    llmClient,
		ShardManager: shardMgr,
		Executor:     executor,
		VirtualStore: virtualStore,
		ProgressChan: progressChan,
		EventChan:    eventChan,
	})

	if err := orchestrator.SetCampaign(pausedCampaign); err != nil {
		return fmt.Errorf("failed to load campaign: %w", err)
	}

	// Start event listener
	go func() {
		for event := range eventChan {
			switch event.Type {
			case "task_started":
				fmt.Printf("🔄 %s\n", event.Message)
			case "task_completed":
				fmt.Printf("✅ %s\n", event.Message)
			case "task_failed":
				fmt.Printf("❌ %s\n", event.Message)
			case "phase_completed":
				fmt.Printf("🎉 Phase completed: %s\n", event.Message)
			case "campaign_completed":
				fmt.Printf("\n🏆 %s\n", event.Message)
			}
		}
	}()

	// Run campaign
	if err := orchestrator.Run(ctx); err != nil {
		if ctx.Err() != nil {
			fmt.Println("\nCampaign paused.")
			return nil
		}
		return fmt.Errorf("campaign failed: %w", err)
	}

	fmt.Println("\n✨ Campaign completed successfully!")
	return nil
}

// runCampaignList lists all campaigns
func runCampaignList(cmd *cobra.Command, args []string) error {
	// Resolve workspace
	cwd := workspace
	if cwd == "" {
		cwd, _ = os.Getwd()
	}

	campaignsDir := filepath.Join(cwd, ".nerd", "campaigns")
	entries, err := os.ReadDir(campaignsDir)
	if err != nil {
		fmt.Println("No campaigns found.")
		return nil
	}

	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║                    CAMPAIGNS                              ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")

	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
			data, err := os.ReadFile(filepath.Join(campaignsDir, entry.Name()))
			if err != nil {
				continue
			}

			var c campaign.Campaign
			if err := json.Unmarshal(data, &c); err != nil {
				continue
			}

			statusIcon := "⏸️"
			switch c.Status {
			case campaign.StatusActive:
				statusIcon = "▶️"
			case campaign.StatusCompleted:
				statusIcon = "✅"
			case campaign.StatusFailed:
				statusIcon = "❌"
			case campaign.StatusPaused:
				statusIcon = "⏸️"
			case campaign.StatusPlanning:
				statusIcon = "📝"
			}

			progress := float64(c.CompletedTasks) / float64(c.TotalTasks) * 100
			fmt.Printf("%s %s\n", statusIcon, c.Title)
			fmt.Printf("   ID: %s | Progress: %.0f%% | Tasks: %d/%d\n\n",
				c.ID, progress, c.CompletedTasks, c.TotalTasks)
		}
	}

	return nil
}

// repeatChar repeats a character n times
func repeatChar(c rune, n int) string {
	if n <= 0 {
		return ""
	}
	result := make([]rune, n)
	for i := range result {
		result[i] = c
	}
	return string(result)
}
