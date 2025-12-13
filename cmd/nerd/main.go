package main

import (
	"codenerd/cmd/nerd/chat"
	"codenerd/internal/articulation"
	"codenerd/internal/browser"
	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/shards"
	"codenerd/internal/store"
	coresys "codenerd/internal/system"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
	"codenerd/internal/usage"
	"codenerd/internal/world"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
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
	forceInit      bool
	cleanupBackups bool

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
		// Skip logger init for interactive mode (it has its own UI and logging setup)
		if cmd.Use == "nerd" && cmd.CalledAs() == "nerd" {
			return nil
		}

		// Initialize zap logger for CLI output
		config := zap.NewProductionConfig()
		if verbose {
			config.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		}
		var err error
		logger, err = config.Build()
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}

		// Initialize internal file-based logging system for telemetry/debugging
		// This enables .nerd/logs/ output for non-interactive commands
		ws := workspace
		if ws == "" {
			ws, _ = os.Getwd()
		}
		if err := logging.Initialize(ws); err != nil {
			// Don't fail hard on logging init, but warn
			fmt.Fprintf(os.Stderr, "Warning: Failed to initialize file logging: %v\n", err)
		}

		return nil
	},
	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if logger != nil {
			_ = logger.Sync()
		}
		// Close internal file-based logging system
		logging.CloseAll()
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		// Interactive mode should honor --workspace.
		// The Bubbletea chat UI uses os.Getwd() as its workspace root for `.nerd/` persistence.
		ws := workspace
		if ws == "" {
			ws, _ = os.Getwd()
		} else if abs, err := filepath.Abs(ws); err == nil {
			ws = abs
		}
		if ws != "" {
			if err := os.Chdir(ws); err != nil {
				return fmt.Errorf("chdir workspace: %w", err)
			}
		}

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
  4. Reloads profile.mg facts

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

// authCmd manages CLI engine authentication
var authCmd = &cobra.Command{
	Use:   "auth",
	Short: "Manage CLI engine authentication",
	Long: `Configure authentication for CLI-based LLM engines.

Available subcommands:
  claude - Authenticate and configure Claude Code CLI engine
  codex  - Authenticate and configure Codex CLI engine
  status - Show current authentication status`,
}

// authClaudeCmd authenticates with Claude Code CLI
var authClaudeCmd = &cobra.Command{
	Use:   "claude",
	Short: "Authenticate with Claude Code CLI",
	Long: `Authenticate with Claude Code CLI and configure codeNERD to use it.

This command:
1. Checks if Claude Code CLI is installed
2. Runs 'claude login' if not authenticated
3. Updates .nerd/config.json to use claude-cli engine`,
	RunE: runAuthClaude,
}

// authCodexCmd authenticates with Codex CLI
var authCodexCmd = &cobra.Command{
	Use:   "codex",
	Short: "Authenticate with Codex CLI",
	Long: `Authenticate with Codex CLI and configure codeNERD to use it.

This command:
1. Checks if Codex CLI is installed
2. Runs 'codex login' if not authenticated
3. Updates .nerd/config.json to use codex-cli engine`,
	RunE: runAuthCodex,
}

// authStatusCmd shows authentication status
var authStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show authentication status",
	RunE:  runAuthStatus,
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "Z.AI API key (or set ZAI_API_KEY env)")
	rootCmd.PersistentFlags().StringVarP(&workspace, "workspace", "w", "", "Workspace directory (default: current)")
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 25*time.Minute, "Operation timeout")

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
	initCmd.Flags().BoolVar(&cleanupBackups, "cleanup-backups", false, "Remove backup files from previous migrations")

	// Browser subcommands
	browserCmd.AddCommand(
		browserLaunchCmd,
		browserSessionCmd,
		browserSnapshotCmd,
	)

	// Campaign subcommands
	var campaignDocs []string
	var campaignType string
	campaignStartCmd.Flags().StringArrayVar(&campaignDocs, "docs", nil, "Paths to spec/requirement documents")
	campaignStartCmd.Flags().StringVar(&campaignType, "type", "feature", "Campaign type (greenfield, feature, audit, migration, remediation)")
	campaignCmd.AddCommand(
		campaignStartCmd,
		campaignStatusCmd,
		campaignPauseCmd,
		campaignResumeCmd,
		campaignListCmd,
	)

	// Auth subcommands
	authCmd.AddCommand(
		authClaudeCmd,
		authCodexCmd,
		authStatusCmd,
	)

	// Add commands to root
	rootCmd.AddCommand(
		runCmd,
		defineAgentCmd,
		spawnCmd,
		browserCmd,
		queryCmd,
		statusCmd,
		initCmd,
		scanCmd,
		whyCmd,
		campaignCmd,
		checkMangleCmd, // UNCOMMENTED: Register the check-mangle command
		authCmd,
	)

	// Direct action commands (mirror TUI verbs)
	rootCmd.AddCommand(
		reviewCmd,
		fixCmd,
		testCmd,
		pushCmd,
		commitCmd,
		explainCmd,
		createCmd,
		refactorCmd,
		perceptionCmd,
	)

	// Advanced commands (dream state, shadow mode, etc.)
	rootCmd.AddCommand(
		dreamCmd,
		shadowCmd,
		whatifCmd,
		logicCmd,
		agentsCmd,
		toolCmd,
		jitCmd,
		domCmd,
		embeddingCmd,
	)
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
// Its primary responsibility is to validate the syntax of Google Mangle (.mg) files.
//
// Key Features & Business Value:
// - Syntax Validation: Parse .mg files and report syntax errors using the official parser.
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
// `nerd check-mangle internal/mangle/*.mg`
//
// References:
// - internal/mangle/grammar.go
//
// --- END OF PRD HEADER ---

var checkMangleCmd = &cobra.Command{
	Use:   "check-mangle [file...]",
	Short: "Check Mangle syntax in .mg files",
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

	// Try to load schemas.mg first if it exists, to provide context
	// We assume a standard location or relative path; for now hardcode likely location
	// In a real tool, this would be a flag --schema or --include
	// Try to find and load schemas.mg to provide context
	// Iterate through search paths to find where schemas.mg actually lives
	// The canonical location is internal/core/defaults/schemas.mg
	searchPaths := []string{
		"internal/core/defaults",
		".",
		"../internal/core/defaults",
		"../../internal/core/defaults",
	}

	var schemaData []byte
	foundSchema := false

	for _, basePath := range searchPaths {
		excludePath := filepath.Join(basePath, "schemas.mg")
		if _, err := os.Stat(excludePath); err == nil {
			if filepath.Base(path) != "schemas.mg" {
				data, err := os.ReadFile(excludePath)
				if err == nil {
					schemaData = data
					foundSchema = true
					break
				}
			}
		}
	}

	if foundSchema {
		if err := tmpEngine.LoadSchemaString(string(schemaData)); err != nil {
			// If the schema itself is broken, we should probably warn but proceed
			fmt.Printf("WARNING: Failed to load context from schemas.mg: %v\n", err)
		}
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return tmpEngine.LoadSchemaString(string(data))
}

// === END OF INTEGRATED check_mangle.go CONTENT ===

// =============================================================================
// DIRECT ACTION COMMANDS - Mirror TUI verbs for CLI testing
// =============================================================================

// reviewCmd runs code review directly
var reviewCmd = &cobra.Command{
	Use:   "review <target>",
	Short: "Run code review on a file or directory",
	Long: `Spawns ReviewerShard to analyze code for issues.
Equivalent to typing "review <target>" in the TUI.

Example:
  nerd review internal/core/kernel.go
  nerd review ./internal/shards/`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("reviewer", "/review"),
}

// fixCmd runs code fix directly
var fixCmd = &cobra.Command{
	Use:   "fix <target>",
	Short: "Fix bugs or issues in code",
	Long: `Spawns CoderShard to fix bugs in the specified target.
Equivalent to typing "fix <target>" in the TUI.

Example:
  nerd fix "the null pointer in auth.go"
  nerd fix internal/core/kernel.go`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("coder", "/fix"),
}

// testCmd runs tests directly
var testCmd = &cobra.Command{
	Use:   "test <target>",
	Short: "Run or generate tests",
	Long: `Spawns TesterShard to run or generate tests.
Equivalent to typing "test <target>" in the TUI.

Example:
  nerd test ./internal/core/...
  nerd test "add tests for kernel.go"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("tester", "/test"),
}

// pushCmd runs git push directly
var pushCmd = &cobra.Command{
	Use:   "push [remote] [branch]",
	Short: "Push commits to remote repository",
	Long: `Executes git push to push commits to the remote repository.

Example:
  nerd push              # pushes to origin
  nerd push origin main  # pushes main to origin`,
	RunE: func(cmd *cobra.Command, args []string) error {
		gitArgs := []string{"push"}
		if len(args) > 0 {
			gitArgs = append(gitArgs, args...)
		}

		fmt.Printf("🚀 Executing: git %s\n", strings.Join(gitArgs, " "))
		fmt.Println(strings.Repeat("─", 50))

		gitCmd := exec.Command("git", gitArgs...)
		gitCmd.Dir = workspace
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		return gitCmd.Run()
	},
}

// commitCmd runs git commit directly
var commitCmd = &cobra.Command{
	Use:   "commit <message>",
	Short: "Commit changes with a message",
	Long: `Executes git commit with the provided message.

Example:
  nerd commit "fix: resolve auth bug"`,
	Args: cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		message := strings.Join(args, " ")

		fmt.Printf("📝 Executing: git commit -m %q\n", message)
		fmt.Println(strings.Repeat("─", 50))

		// First check status
		statusCmd := exec.Command("git", "status", "--porcelain")
		statusCmd.Dir = workspace
		status, _ := statusCmd.Output()

		if len(status) == 0 {
			fmt.Println("ℹ️  Nothing to commit, working tree clean")
			return nil
		}

		// Add all changes
		addCmd := exec.Command("git", "add", "-A")
		addCmd.Dir = workspace
		if err := addCmd.Run(); err != nil {
			return fmt.Errorf("git add failed: %w", err)
		}

		// Commit
		gitCmd := exec.Command("git", "commit", "-m", message)
		gitCmd.Dir = workspace
		gitCmd.Stdout = os.Stdout
		gitCmd.Stderr = os.Stderr
		return gitCmd.Run()
	},
}

// explainCmd explains code directly
var explainCmd = &cobra.Command{
	Use:   "explain <target>",
	Short: "Explain what code does",
	Long: `Analyzes and explains the specified code.
Equivalent to typing "explain <target>" in the TUI.

Example:
  nerd explain internal/core/kernel.go
  nerd explain "the OODA loop"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("researcher", "/explain"),
}

// createCmd creates new code directly
var createCmd = &cobra.Command{
	Use:   "create <description>",
	Short: "Create new code or files",
	Long: `Spawns CoderShard to create new code.
Equivalent to typing "create <description>" in the TUI.

Example:
  nerd create "a retry wrapper for HTTP calls"
  nerd create internal/utils/retry.go`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("coder", "/create"),
}

// refactorCmd refactors code directly
var refactorCmd = &cobra.Command{
	Use:   "refactor <target>",
	Short: "Refactor existing code",
	Long: `Spawns CoderShard to refactor code.
Equivalent to typing "refactor <target>" in the TUI.

Example:
  nerd refactor internal/core/kernel.go
  nerd refactor "extract helper functions from process.go"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDirectAction("coder", "/refactor"),
}

// perceptionCmd tests perception/intent recognition
var perceptionCmd = &cobra.Command{
	Use:   "perception <input>",
	Short: "Test perception transducer (diagnostic)",
	Long: `Tests how the perception layer interprets user input.
Shows parsed intent, verb, target, and shard routing.

Example:
  nerd perception "review my code"
  nerd perception "push to github"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runPerceptionTest,
}

// runDirectAction creates a handler for direct action commands
func runDirectAction(shardType, verb string) func(cmd *cobra.Command, args []string) error {
	return func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		// Handle graceful shutdown
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigCh
			fmt.Println("\n⏹️  Interrupted")
			cancel()
		}()

		target := strings.Join(args, " ")
		task := fmt.Sprintf("%s %s", strings.TrimPrefix(verb, "/"), target)

		fmt.Printf("🔧 Action: %s\n", verb)
		fmt.Printf("🎯 Target: %s\n", target)
		fmt.Printf("🤖 Shard:  %s\n", shardType)
		fmt.Println(strings.Repeat("─", 50))

		// Resolve API key
		key := apiKey
		if key == "" {
			key = os.Getenv("ZAI_API_KEY")
		}

		// Boot Cortex
		cortex, err := coresys.BootCortex(ctx, workspace, key, nil)
		if err != nil {
			return fmt.Errorf("failed to boot cortex: %w", err)
		}
		defer cortex.Close()

		// Add usage tracker
		if cortex.UsageTracker != nil {
			ctx = usage.NewContext(ctx, cortex.UsageTracker)
		}

		// Spawn shard directly
		fmt.Printf("⏳ Spawning %s shard...\n", shardType)
		result, err := cortex.ShardManager.Spawn(ctx, shardType, task)
		if err != nil {
			return fmt.Errorf("shard execution failed: %w", err)
		}

		fmt.Println(strings.Repeat("─", 50))
		fmt.Println("📋 Result:")
		fmt.Println(result)

		return nil
	}
}

// runPerceptionTest tests the perception transducer
func runPerceptionTest(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	input := strings.Join(args, " ")

	fmt.Printf("🎤 Input: %q\n", input)
	fmt.Println(strings.Repeat("─", 50))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex (lightweight - just need transducer)
	cortex, err := coresys.BootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Parse intent
	intent, err := cortex.Transducer.ParseIntent(ctx, input)
	if err != nil {
		return fmt.Errorf("perception error: %w", err)
	}

	// Get shard routing
	shardType := perception.GetShardTypeForVerb(intent.Verb)

	fmt.Printf("📊 Perception Results:\n")
	fmt.Printf("   Category:   %s\n", intent.Category)
	fmt.Printf("   Verb:       %s\n", intent.Verb)
	fmt.Printf("   Target:     %s\n", intent.Target)
	fmt.Printf("   Constraint: %s\n", intent.Constraint)
	fmt.Printf("   Confidence: %.2f\n", intent.Confidence)
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("🔀 Routing:\n")
	if shardType == "" || shardType == "/none" {
		fmt.Printf("   Shard: (none - direct response)\n")
	} else {
		fmt.Printf("   Shard: %s\n", shardType)
	}
	fmt.Println(strings.Repeat("─", 50))
	fmt.Printf("💬 Response Preview:\n%s\n", truncateResponse(intent.Response, 500))

	return nil
}

// truncateResponse truncates long responses for display
func truncateResponse(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "\n... (truncated)"
	}
	return s
}

// =============================================================================
// DREAM STATE & ADVANCED COMMANDS
// =============================================================================

// dreamCmd runs dream state multi-agent consultation
var dreamCmd = &cobra.Command{
	Use:   "dream <scenario>",
	Short: "Run dream state multi-agent consultation",
	Long: `Consults multiple shard agents about a hypothetical scenario.
Each agent provides their perspective without executing any changes.
Equivalent to typing "what if <scenario>" or using /dream in the TUI.

Example:
  nerd dream "we migrated from REST to GraphQL"
  nerd dream "implementing caching with Redis"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runDreamState,
}

// shadowCmd runs shadow mode simulation
var shadowCmd = &cobra.Command{
	Use:   "shadow <action>",
	Short: "Simulate an action without executing",
	Long: `Runs a shadow simulation showing what would happen.
No actual changes are made - purely descriptive output.
Equivalent to /shadow in the TUI.

Example:
  nerd shadow "delete all test files"
  nerd shadow "refactor the auth module"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runShadowSimulation,
}

// whatifCmd runs counterfactual query
var whatifCmd = &cobra.Command{
	Use:   "whatif <change>",
	Short: "Run counterfactual analysis",
	Long: `Analyzes what would happen if a change were made.
Uses the Mangle kernel to derive implications.
Equivalent to /whatif in the TUI.

Example:
  nerd whatif "we removed the database connection pooling"
  nerd whatif "error handling was centralized"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runWhatIf,
}

// logicCmd shows kernel facts
var logicCmd = &cobra.Command{
	Use:   "logic [predicate]",
	Short: "Show Mangle kernel facts",
	Long: `Displays facts currently in the Mangle kernel.
Optionally filter by predicate name.
Equivalent to /logic in the TUI.

Example:
  nerd logic              # Show all facts
  nerd logic user_intent  # Show only user_intent facts
  nerd logic shard_result # Show shard results`,
	RunE: runLogicQuery,
}

// agentsCmd lists available agents
var agentsCmd = &cobra.Command{
	Use:   "agents",
	Short: "List available shard agents",
	Long: `Shows all registered shard agents with their capabilities.
Includes both ephemeral and persistent specialists.
Equivalent to /agents in the TUI.`,
	RunE: runAgentsList,
}

// toolCmd manages generated tools
var toolCmd = &cobra.Command{
	Use:   "tool <list|run|info|generate> [args]",
	Short: "Manage generated tools (Ouroboros)",
	Long: `Manage tools generated by the Ouroboros Loop.

Subcommands:
  list              - List all generated tools
  run <name> [in]   - Execute a tool with optional input
  info <name>       - Show tool details and source
  generate <desc>   - Generate a new tool from description

Examples:
  nerd tool list
  nerd tool run json-validator '{"test": 123}'
  nerd tool generate "a tool that validates JSON syntax"`,
	Args: cobra.MinimumNArgs(1),
	RunE: runToolCommand,
}

// jitCmd shows JIT compiler status
var jitCmd = &cobra.Command{
	Use:   "jit",
	Short: "Show JIT prompt compiler status",
	Long: `Displays the JIT Prompt Compiler's current state.
Shows loaded atoms, token budget, and compilation stats.
Equivalent to /jit in the TUI.`,
	RunE: runJITStatus,
}

// runDreamState executes dream state consultation
func runDreamState(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	scenario := strings.Join(args, " ")

	fmt.Printf("🌙 Dream State Consultation\n")
	fmt.Printf("📝 Scenario: %s\n", scenario)
	fmt.Println(strings.Repeat("─", 60))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex
	cortex, err := coresys.BootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Add usage tracker
	if cortex.UsageTracker != nil {
		ctx = usage.NewContext(ctx, cortex.UsageTracker)
	}

	// Get available shards
	shards := cortex.ShardManager.ListAvailableShards()
	fmt.Printf("🤖 Consulting %d agents...\n\n", len(shards))

	// Consult each shard in dream mode
	dreamCtx := &core.SessionContext{DreamMode: true}

	for i, shard := range shards {
		// Skip internal/system shards
		if shard.Type == core.ShardTypeSystem {
			continue
		}

		fmt.Printf("[%d] %s (%s)...\n", i+1, shard.Name, shard.Type)

		prompt := fmt.Sprintf("Dream Mode Consultation:\n\nScenario: %s\n\nProvide your perspective on this hypothetical. Do NOT execute any actions - only describe what you would do and the implications.", scenario)

		// Dream mode = low priority (background speculation)
		result, err := cortex.ShardManager.SpawnWithPriority(ctx, shard.Name, prompt, dreamCtx, core.PriorityLow)
		if err != nil {
			fmt.Printf("   ❌ Error: %v\n\n", err)
			continue
		}

		fmt.Printf("   ✓ Response:\n")
		// Indent the response
		for _, line := range strings.Split(truncateResponse(result, 500), "\n") {
			fmt.Printf("     %s\n", line)
		}
		fmt.Println()
	}

	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("✅ Dream state consultation complete")

	return nil
}

// runShadowSimulation runs shadow mode
func runShadowSimulation(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	action := strings.Join(args, " ")

	fmt.Printf("👻 Shadow Mode Simulation\n")
	fmt.Printf("🎯 Action: %s\n", action)
	fmt.Println(strings.Repeat("─", 60))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex
	cortex, err := coresys.BootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Use coder shard in shadow mode
	shadowCtx := &core.SessionContext{DreamMode: true}
	prompt := fmt.Sprintf("SHADOW MODE - Describe what would happen without executing:\n\n%s\n\nList the files that would be affected, changes that would be made, and potential risks. Do NOT actually make any changes.", action)

	// Shadow mode = normal priority (user CLI command but speculative)
	result, err := cortex.ShardManager.SpawnWithPriority(ctx, "coder", prompt, shadowCtx, core.PriorityNormal)
	if err != nil {
		return fmt.Errorf("shadow simulation failed: %w", err)
	}

	fmt.Println("📋 Simulation Result:")
	fmt.Println(result)

	return nil
}

// runWhatIf runs counterfactual analysis
func runWhatIf(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	change := strings.Join(args, " ")

	fmt.Printf("🔮 What-If Analysis\n")
	fmt.Printf("❓ Change: %s\n", change)
	fmt.Println(strings.Repeat("─", 60))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex
	cortex, err := coresys.BootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Assert the hypothetical to kernel
	hypFact := core.Fact{
		Predicate: "hypothetical",
		Args:      []interface{}{change},
	}
	_ = cortex.Kernel.Assert(hypFact)

	// Query implications
	implications, _ := cortex.Kernel.Query("derives_from_hypothetical")

	fmt.Println("📊 Kernel Implications:")
	if len(implications) > 0 {
		for _, imp := range implications {
			fmt.Printf("   - %s\n", imp.String())
		}
	} else {
		fmt.Println("   (no derived implications)")
	}
	fmt.Println()

	// Use researcher for deeper analysis
	prompt := fmt.Sprintf("Analyze the implications of this hypothetical change:\n\n%s\n\nConsider:\n1. What systems would be affected?\n2. What would break?\n3. What would improve?\n4. What risks exist?", change)

	result, err := cortex.ShardManager.Spawn(ctx, "researcher", prompt)
	if err != nil {
		fmt.Printf("Analysis failed: %v\n", err)
	} else {
		fmt.Println("📋 Analysis:")
		fmt.Println(result)
	}

	return nil
}

// runLogicQuery shows kernel facts
func runLogicQuery(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	predicate := "*"
	if len(args) > 0 {
		predicate = args[0]
	}

	fmt.Printf("🧠 Mangle Kernel Facts\n")
	fmt.Printf("🔍 Query: %s\n", predicate)
	fmt.Println(strings.Repeat("─", 60))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex
	cortex, err := coresys.BootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Query facts
	facts, err := cortex.Kernel.Query(predicate)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	fmt.Printf("📊 Found %d facts:\n\n", len(facts))
	for i, fact := range facts {
		if i >= 50 {
			fmt.Printf("... and %d more\n", len(facts)-50)
			break
		}
		fmt.Printf("  %s\n", fact.String())
	}

	return nil
}

// runAgentsList lists available agents
func runAgentsList(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("🤖 Available Shard Agents\n")
	fmt.Println(strings.Repeat("─", 60))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex
	cortex, err := coresys.BootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// List shards
	shards := cortex.ShardManager.ListAvailableShards()

	// Group by type
	typeGroups := make(map[core.ShardType][]core.ShardInfo)
	for _, shard := range shards {
		typeGroups[shard.Type] = append(typeGroups[shard.Type], shard)
	}

	typeNames := map[core.ShardType]string{
		core.ShardTypeEphemeral:  "Ephemeral (Type A)",
		core.ShardTypePersistent: "Persistent (Type B)",
		core.ShardTypeUser:       "User-Defined (Type U)",
		core.ShardTypeSystem:     "System (Type S)",
	}

	for shardType, shards := range typeGroups {
		fmt.Printf("\n### %s\n", typeNames[shardType])
		for _, shard := range shards {
			knowledgeStr := ""
			if shard.HasKnowledge {
				knowledgeStr = " [+knowledge]"
			}
			fmt.Printf("  - %s%s\n", shard.Name, knowledgeStr)
		}
	}

	fmt.Printf("\nTotal: %d agents\n", len(shards))
	return nil
}

// runToolCommand handles tool management
func runToolCommand(cmd *cobra.Command, args []string) error {
	if len(args) == 0 {
		fmt.Println("Usage: nerd tool <list|run|info|generate> [args]")
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex
	cortex, err := coresys.BootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	subCmd := args[0]
	switch subCmd {
	case "list":
		fmt.Println("🔧 Generated Tools (Ouroboros)")
		fmt.Println(strings.Repeat("─", 60))

		// Query tool facts
		tools, _ := cortex.Kernel.Query("tool_registered")
		if len(tools) == 0 {
			fmt.Println("No tools generated yet.")
			fmt.Println("\nUse 'nerd tool generate <description>' to create one.")
		} else {
			for _, tool := range tools {
				fmt.Printf("  - %s\n", tool.Args[0])
			}
		}

	case "run":
		if len(args) < 2 {
			return fmt.Errorf("usage: nerd tool run <name> [input]")
		}
		toolName := args[1]
		toolInput := ""
		if len(args) > 2 {
			toolInput = strings.Join(args[2:], " ")
		}

		fmt.Printf("🔧 Running tool: %s\n", toolName)
		fmt.Println(strings.Repeat("─", 60))

		// Find tool binary path from kernel facts
		var binaryPath string
		allBinaries, _ := cortex.Kernel.Query("tool_binary_path")
		for _, b := range allBinaries {
			if len(b.Args) >= 2 && fmt.Sprintf("%v", b.Args[0]) == toolName {
				binaryPath = fmt.Sprintf("%v", b.Args[1])
				break
			}
		}

		if binaryPath == "" {
			return fmt.Errorf("tool '%s' not found. Run 'nerd tool list' to see available tools", toolName)
		}

		// Execute tool binary directly
		toolCmd := exec.CommandContext(ctx, binaryPath)
		if toolInput != "" {
			toolCmd.Stdin = strings.NewReader(toolInput)
		}
		toolCmd.Stdout = os.Stdout
		toolCmd.Stderr = os.Stderr

		if err := toolCmd.Run(); err != nil {
			return fmt.Errorf("tool execution failed: %w", err)
		}

	case "info":
		if len(args) < 2 {
			return fmt.Errorf("usage: nerd tool info <name>")
		}
		toolName := args[1]

		fmt.Printf("🔧 Tool Info: %s\n", toolName)
		fmt.Println(strings.Repeat("─", 60))

		// Query tool details (filter results in Go since kernel.Query returns all facts)
		allDetails, _ := cortex.Kernel.Query("tool_description")
		for _, d := range allDetails {
			if len(d.Args) >= 2 && fmt.Sprintf("%v", d.Args[0]) == toolName {
				fmt.Printf("Description: %v\n", d.Args[1])
				break
			}
		}

		allBinaries, _ := cortex.Kernel.Query("tool_binary_path")
		for _, b := range allBinaries {
			if len(b.Args) >= 2 && fmt.Sprintf("%v", b.Args[0]) == toolName {
				fmt.Printf("Binary: %v\n", b.Args[1])
				break
			}
		}

	case "generate":
		if len(args) < 2 {
			return fmt.Errorf("usage: nerd tool generate <description>")
		}
		description := strings.Join(args[1:], " ")

		fmt.Printf("🔧 Generating Tool\n")
		fmt.Printf("📝 Description: %s\n", description)
		fmt.Println(strings.Repeat("─", 60))

		// Use tool_generator shard
		result, err := cortex.ShardManager.Spawn(ctx, "tool_generator", description)
		if err != nil {
			return fmt.Errorf("tool generation failed: %w", err)
		}
		fmt.Println(result)

	default:
		return fmt.Errorf("unknown subcommand: %s (use list, run, info, or generate)", subCmd)
	}

	return nil
}

// runJITStatus shows JIT compiler status
func runJITStatus(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	fmt.Printf("⚡ JIT Prompt Compiler Status\n")
	fmt.Println(strings.Repeat("─", 60))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex
	cortex, err := coresys.BootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Get compiler stats
	if cortex.JITCompiler != nil {
		stats := cortex.JITCompiler.GetStats()
		fmt.Printf("Embedded Atoms:   %d\n", stats.EmbeddedAtomCount)
		fmt.Printf("Project Atoms:    %d\n", stats.ProjectAtomCount)
		fmt.Printf("Shard DBs:        %d\n", stats.ShardDBCount)
		fmt.Printf("Compilations:     %d\n", stats.TotalCompilations)
		fmt.Printf("Avg Time (ms):    %.2f\n", stats.AverageTimeMs)
	} else {
		fmt.Println("JIT Compiler not initialized")
	}

	// Show loaded prompt atoms
	atoms, _ := cortex.Kernel.Query("prompt_atom")
	fmt.Printf("\nLoaded Prompt Atoms: %d\n", len(atoms))
	if len(atoms) > 0 && len(atoms) <= 10 {
		for _, atom := range atoms {
			fmt.Printf("  - %v\n", atom.Args[0])
		}
	}

	return nil
}

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

	// Boot Cortex (System Stabilization)
	cortex, err := coresys.BootCortex(ctx, workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Add usage tracker to context if available
	if cortex.UsageTracker != nil {
		ctx = usage.NewContext(ctx, cortex.UsageTracker)
	}

	baseRouting, baseExec := systemResultBaselines(cortex.Kernel)

	emitter := articulation.NewEmitter()

	// 2. Perception Layer: Transduce Input -> Intent
	logger.Debug("Transducing user input to intent atoms")
	intent, err := cortex.Transducer.ParseIntent(ctx, userInput)
	if err != nil {
		return fmt.Errorf("perception error: %w", err)
	}
	logger.Info("Intent parsed",
		zap.String("verb", intent.Verb),
		zap.String("target", intent.Target))

	// /stats is deterministic and should not require running shards or policy.
	if intent.Verb == "/stats" {
		stats, err := computeStats(ctx, cortex.Workspace, intent.Target)
		if err != nil {
			stats = fmt.Sprintf("Stats error: %v", err)
		}
		emitter.Emit(articulation.PiggybackEnvelope{
			Surface: stats,
			Control: articulation.ControlPacket{
				IntentClassification: articulation.IntentClassification{
					Category:   intent.Category,
					Verb:       intent.Verb,
					Target:     intent.Target,
					Constraint: intent.Constraint,
					Confidence: intent.Confidence,
				},
				MangleUpdates: []string{fmt.Sprintf("observation(/stats, %q)", stats)},
			},
		})
		return nil
	}

	// 3. World Model: Incremental Scan Workspace (fast)
	logger.Debug("Scanning workspace incrementally", zap.String("path", cortex.Workspace))
	scanRes, err := cortex.Scanner.ScanWorkspaceIncremental(ctx, cortex.Workspace, cortex.LocalDB, world.IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		return fmt.Errorf("world model error: %w", err)
	}
	if scanRes != nil && !scanRes.Unchanged {
		if err := world.ApplyIncrementalResult(cortex.Kernel, scanRes); err != nil {
			return fmt.Errorf("world model apply error: %w", err)
		}
		logger.Debug("Workspace scan applied", zap.Int("facts", len(scanRes.NewFacts)))
	} else {
		logger.Debug("Workspace unchanged, using cached facts")
	}

	// 4. Load Facts into Hollow Kernel
	if err := cortex.Kernel.LoadFacts([]core.Fact{intent.ToFact()}); err != nil {
		return fmt.Errorf("kernel load error: %w", err)
	}

	// Update system facts (Time, etc.)
	if err := cortex.Kernel.UpdateSystemFacts(); err != nil {
		return fmt.Errorf("system facts update error: %w", err)
	}

	// 5. Query Executive Policy (Decide)
	logger.Debug("Querying executive policy")
	var output string

	// Check for delegation
	delegateFacts, _ := cortex.Kernel.Query("delegate_task")
	if len(delegateFacts) > 0 {
		// Execute via shard
		fact := delegateFacts[0]
		shardType := fmt.Sprintf("%v", fact.Args[0])
		task := fmt.Sprintf("%v", fact.Args[1])
		logger.Info("Delegating to shard", zap.String("type", shardType), zap.String("task", task))

		// Special handling for System Components
		if shardType == "/tool_generator" || shardType == "tool_generator" {
			// Autopoiesis: Tool Generation
			count, err := cortex.Orchestrator.ProcessKernelDelegations(ctx)
			if err != nil {
				output = fmt.Sprintf("Tool generation failed: %v", err)
			} else {
				output = fmt.Sprintf("Autopoiesis: Generated %d tools", count)
			}
		} else {
			// Standard Shard
			result, err := cortex.ShardManager.Spawn(ctx, shardType, task)
			if err != nil {
				output = fmt.Sprintf("Shard execution failed: %v", err)
			} else {
				output = fmt.Sprintf("Shard Result: %s", result)
			}
		}

	} else {
		// Query next_action
		actionFacts, _ := cortex.Kernel.Query("next_action")
		if len(actionFacts) > 0 {
			fact := actionFacts[0]
			logger.Info("Derived next_action (unary; executed by system shards if enabled)", zap.Any("action", fact))
			output = fmt.Sprintf("Next action: %v", fact.Args[0])
		} else {
			output = "No action derived from policy"
		}
	}

	routingNew, execNew := waitForSystemResults(ctx, cortex.Kernel, baseRouting, baseExec, 3*time.Second)
	if summary := formatSystemResults(routingNew, execNew); summary != "" {
		output = output + "\n\n" + summary
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

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex to get wired environment
	cortex, err := coresys.BootCortex(cmd.Context(), workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	config := core.DefaultSpecialistConfig(name, fmt.Sprintf("memory/shards/%s_knowledge.db", name))

	cortex.ShardManager.DefineProfile(name, config)

	// Trigger deep research phase (§9.2)
	// This spawns a researcher shard to build the knowledge base
	fmt.Printf("Initiating deep research on topic: %s...\n", topic)

	// Use 10 minute timeout for research
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Minute)
	defer cancel()

	researchTask := fmt.Sprintf("Research the topic '%s' and generate Mangle facts for the %s agent knowledge base.", topic, name)
	if _, err := cortex.ShardManager.Spawn(ctx, "researcher", researchTask); err != nil {
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
	ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
	defer cancel()

	shardType := args[0]
	task := joinArgs(args[1:])

	logger.Info("Spawning shard",
		zap.String("type", shardType),
		zap.String("task", task))

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex
	cortex, err := coresys.BootCortex(ctx, workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Generate shard ID for fact recording
	shardID := fmt.Sprintf("%s-%d", shardType, time.Now().UnixNano())

	result, spawnErr := cortex.ShardManager.Spawn(ctx, shardType, task)

	// Record execution facts regardless of success/failure
	facts := cortex.ShardManager.ResultToFacts(shardID, shardType, task, result, spawnErr)
	if len(facts) > 0 {
		if loadErr := cortex.Kernel.LoadFacts(facts); loadErr != nil {
			logger.Warn("Failed to load shard facts into kernel", zap.Error(loadErr))
		} else {
			logger.Debug("Recorded shard execution facts", zap.Int("count", len(facts)))
		}
	}

	if spawnErr != nil {
		return fmt.Errorf("spawn failed: %w", spawnErr)
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

	snapshotFile := filepath.Join(factsDir, fmt.Sprintf("%s_%d.mg", sessionID, time.Now().Unix()))

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

	baseCtx := cmd.Context()
	if baseCtx == nil {
		baseCtx = context.Background()
	}
	ctx, cancel := context.WithTimeout(baseCtx, timeout)
	defer cancel()

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Boot Cortex to load all persisted facts (including scan.mg)
	cortex, err := coresys.BootCortex(ctx, workspace, key, disableSystemShards)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	facts, err := cortex.Kernel.Query(predicate)
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
	kernel, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
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

	// Handle backup cleanup (can run standalone without full init)
	if cleanupBackups {
		nerdDir := filepath.Join(cwd, ".nerd")
		deleted, err := nerdinit.CleanupBackups(nerdDir, false)
		if err != nil {
			return fmt.Errorf("failed to cleanup backups: %w", err)
		}
		if deleted == 0 {
			fmt.Println("No backup files found to clean up.")
		}
		return nil
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
	initializer, err := nerdinit.NewInitializer(config)
	if err != nil {
		return fmt.Errorf("failed to create initializer: %w", err)
	}
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

	// Persist fast world snapshot to knowledge.db for incremental boots.
	dbPath := filepath.Join(cwd, ".nerd", "knowledge.db")
	if db, dbErr := store.NewLocalStore(dbPath); dbErr == nil {
		_ = world.PersistFastSnapshotToDB(db, facts)
		_ = db.Close()
	}

	// Initialize kernel and load facts
	kernel, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
	if err := kernel.LoadFacts(facts); err != nil {
		return fmt.Errorf("failed to load facts: %w", err)
	}

	// Also reload profile.mg if it exists
	factsPath := filepath.Join(cwd, ".nerd", "profile.mg")
	if _, statErr := os.Stat(factsPath); statErr == nil {
		if err := kernel.LoadFactsFromFile(factsPath); err != nil {
			fmt.Printf("⚠️ Warning: failed to load profile.mg: %v\n", err)
		}
	}

	// Persist scan facts to .nerd/mangle/scan.mg for reloading on boot
	scanPath := filepath.Join(cwd, ".nerd", "mangle", "scan.mg")
	if writeErr := writeScanFacts(scanPath, facts); writeErr != nil {
		fmt.Printf("⚠️ Warning: failed to persist scan facts: %v\n", writeErr)
	} else {
		fmt.Printf("   Facts persisted: %s\n", scanPath)
	}

	// Count files and directories
	fileCount := 0
	dirCount := 0
	langStats := make(map[string]int)
	symbolCount := 0

	for _, f := range facts {
		switch f.Predicate {
		case "file_topology":
			fileCount++
			if len(f.Args) > 2 {
				// file_topology(Path, Hash, /Lang, ...)
				if langAtom, ok := f.Args[2].(core.MangleAtom); ok {
					lang := strings.TrimPrefix(string(langAtom), "/")
					langStats[lang]++
				}
			}
		case "directory":
			dirCount++
		case "symbol_graph":
			symbolCount++
		}
	}

	fmt.Println("✅ Scan complete")
	fmt.Printf("   Files indexed:    %d\n", fileCount)
	fmt.Printf("   Directories:      %d\n", dirCount)
	fmt.Printf("   Symbols extracted: %d\n", symbolCount)
	fmt.Printf("   Facts generated:  %d\n", len(facts))

	if len(langStats) > 0 {
		fmt.Println("\n   Language Breakdown:")
		for lang, count := range langStats {
			fmt.Printf("     %-12s: %d\n", lang, count)
		}
	}

	return nil
}

// writeScanFacts persists scan facts to a .mg file for reloading on boot.
func writeScanFacts(path string, facts []core.Fact) error {
	// Ensure parent directory exists
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Build content
	var sb strings.Builder
	sb.WriteString("# Auto-generated scan facts - DO NOT EDIT\n")
	sb.WriteString("# Re-run 'nerd scan' to update\n\n")

	for _, fact := range facts {
		// Sanitize fact args to remove characters that Mangle parser can't handle
		sanitizedFact := sanitizeFactForMangle(fact)
		sb.WriteString(sanitizedFact.String())
		sb.WriteString("\n")
	}

	// Write atomically via temp file
	tmpPath := path + ".tmp"
	if err := os.WriteFile(tmpPath, []byte(sb.String()), 0644); err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	if err := os.Rename(tmpPath, path); err != nil {
		os.Remove(tmpPath) // Clean up
		return fmt.Errorf("failed to rename temp file: %w", err)
	}

	return nil
}

// sanitizeFactForMangle sanitizes fact arguments to be Mangle-parser friendly.
// Mangle's parser doesn't handle certain escape sequences like \r \n \t in strings.
func sanitizeFactForMangle(fact core.Fact) core.Fact {
	sanitized := core.Fact{
		Predicate: fact.Predicate,
		Args:      make([]interface{}, len(fact.Args)),
	}

	for i, arg := range fact.Args {
		switch v := arg.(type) {
		case string:
			// Replace newlines, carriage returns, tabs with spaces
			s := strings.ReplaceAll(v, "\r\n", " ")
			s = strings.ReplaceAll(s, "\n", " ")
			s = strings.ReplaceAll(s, "\r", " ")
			s = strings.ReplaceAll(s, "\t", " ")
			// Collapse multiple spaces into one
			for strings.Contains(s, "  ") {
				s = strings.ReplaceAll(s, "  ", " ")
			}
			sanitized.Args[i] = strings.TrimSpace(s)
		default:
			sanitized.Args[i] = arg
		}
	}

	return sanitized
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
	kernel, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
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

	// FIX: Respect authenticated engine configuration instead of hardcoding ZAI
	// This uses the same detection logic as the chat session (claude-cli, codex-cli, etc.)
	llmClient, clientErr := perception.NewClientFromEnv()
	if clientErr != nil {
		// Fallback to ZAI if config detection fails
		key := apiKey
		if key == "" {
			key = os.Getenv("ZAI_API_KEY")
		}
		llmClient = perception.NewZAIClient(key)
		fmt.Printf("⚠ Using fallback ZAI client: %v\n", clientErr)
	}

	// Resolve .nerd directory for JIT prompt system
	nerdDir := filepath.Join(cwd, ".nerd")

	// Initialize components
	kernel, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
	executor := tactile.NewSafeExecutor()
	virtualStore := core.NewVirtualStore(executor)

	// FIX: Wire persistence layers - enables autopoiesis and knowledge persistence
	// Without this, shards are "amnesiac" and cannot save learnings or access knowledge graph
	knowledgeDBPath := filepath.Join(nerdDir, "knowledge.db")
	if localDB, err := store.NewLocalStore(knowledgeDBPath); err == nil {
		virtualStore.SetLocalDB(localDB)
		virtualStore.SetKernel(kernel)
	} else {
		fmt.Fprintf(os.Stderr, "Warning: Failed to open knowledge DB: %v\n", err)
	}

	learningStorePath := filepath.Join(nerdDir, "shards")
	if learningStore, err := store.NewLearningStore(learningStorePath); err == nil {
		virtualStore.SetLearningStore(learningStore)
	} else {
		fmt.Fprintf(os.Stderr, "Warning: Failed to open learning store: %v\n", err)
	}

	shardMgr := core.NewShardManager()
	shardMgr.SetParentKernel(kernel)

	// Initialize limits enforcer and spawn queue for backpressure management
	userCfgPath := filepath.Join(nerdDir, "config.json")
	appCfg, _ := config.LoadUserConfig(userCfgPath)
	if appCfg == nil {
		appCfg = config.DefaultUserConfig()
	}
	coreLimits := appCfg.GetCoreLimits()

	// Configure global LLM API concurrency before shards start
	schedulerCfg := core.DefaultAPISchedulerConfig()
	schedulerCfg.MaxConcurrentAPICalls = coreLimits.MaxConcurrentAPICalls
	core.ConfigureGlobalAPIScheduler(schedulerCfg)

	limitsEnforcer := core.NewLimitsEnforcer(core.LimitsConfig{
		MaxTotalMemoryMB:      coreLimits.MaxTotalMemoryMB,
		MaxConcurrentShards:   coreLimits.MaxConcurrentShards,
		MaxSessionDurationMin: coreLimits.MaxSessionDurationMin,
		MaxFactsInKernel:      coreLimits.MaxFactsInKernel,
		MaxDerivedFactsLimit:  coreLimits.MaxDerivedFactsLimit,
	})
	shardMgr.SetLimitsEnforcer(limitsEnforcer)
	spawnQueue := core.NewSpawnQueue(shardMgr, limitsEnforcer, core.DefaultSpawnQueueConfig())
	shardMgr.SetSpawnQueue(spawnQueue)
	_ = spawnQueue.Start()

	// Initialize JIT Prompt Compiler - CRITICAL for shard prompt formatting
	jitCompiler, err := prompt.NewJITPromptCompiler(
		prompt.WithKernel(coresys.NewKernelAdapter(kernel)),
	)
	if err != nil {
		return fmt.Errorf("failed to init JIT compiler: %w", err)
	}

	// FIX: Wire JIT lifecycle callbacks - CRITICAL for Type B/U shard prompt hydration
	// Without this, specialist agents launch without their specialized prompts (Identity,
	// Methodology, Domain Knowledge) and revert to generic base behavior.
	shardMgr.SetNerdDir(nerdDir)
	shardMgr.SetJITRegistrar(prompt.CreateJITDBRegistrar(jitCompiler))
	shardMgr.SetJITUnregistrar(prompt.CreateJITDBUnregistrar(jitCompiler))

	// Register shard factories - CRITICAL: without this, shards fall back to BaseShardAgent
	shardMgr.SetLLMClient(llmClient)
	shards.RegisterAllShardFactories(shardMgr, shards.RegistryContext{
		Kernel:       kernel,
		LLMClient:    llmClient,
		VirtualStore: virtualStore,
		Workspace:    cwd,
		JITCompiler:  jitCompiler,
	})

	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║          CAMPAIGN ORCHESTRATOR - INITIALIZING             ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Printf("\nGoal: %s\n", goal)
	if len(docs) > 0 {
		fmt.Printf("Source Documents: %v\n", docs)
	}
	fmt.Printf("Campaign Type: %s\n\n", campaignType)

	// Create a PromptAssembler-backed provider so campaign roles use JIT atoms by default.
	var campaignPromptProvider campaign.PromptProvider
	if pa, err := articulation.NewPromptAssemblerWithJIT(kernel, jitCompiler); err == nil {
		campaignPromptProvider = &CampaignJITProvider{assembler: pa}
	}

	// Create decomposer
	decomposer := campaign.NewDecomposer(kernel, llmClient, cwd)
	decomposer.SetShardLister(shardMgr) // Enable shard-aware planning
	if campaignPromptProvider != nil {
		decomposer.SetPromptProvider(campaignPromptProvider)
	}

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
	if campaignPromptProvider != nil {
		orchestrator.SetPromptProvider(campaignPromptProvider)
	}

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
	llmClient, clientErr := perception.NewClientFromEnv()
	if clientErr != nil {
		llmClient = perception.NewZAIClient(key)
		fmt.Printf("? Using fallback ZAI client: %v\n", clientErr)
	}
	kernel, err := core.NewRealKernel()
	if err != nil {
		return fmt.Errorf("failed to create kernel: %w", err)
	}
	executor := tactile.NewSafeExecutor()
	virtualStore := core.NewVirtualStore(executor)
	shardMgr := core.NewShardManager()
	shardMgr.SetParentKernel(kernel)

	// Initialize limits enforcer and spawn queue for backpressure management
	cfgPath := config.DefaultUserConfigPath()
	appCfg, _ := config.LoadUserConfig(cfgPath)
	if appCfg == nil {
		appCfg = config.DefaultUserConfig()
	}
	coreLimits := appCfg.GetCoreLimits()

	// Configure global LLM API concurrency before shards start
	schedulerCfg := core.DefaultAPISchedulerConfig()
	schedulerCfg.MaxConcurrentAPICalls = coreLimits.MaxConcurrentAPICalls
	core.ConfigureGlobalAPIScheduler(schedulerCfg)

	limitsEnforcer := core.NewLimitsEnforcer(core.LimitsConfig{
		MaxTotalMemoryMB:      coreLimits.MaxTotalMemoryMB,
		MaxConcurrentShards:   coreLimits.MaxConcurrentShards,
		MaxSessionDurationMin: coreLimits.MaxSessionDurationMin,
		MaxFactsInKernel:      coreLimits.MaxFactsInKernel,
		MaxDerivedFactsLimit:  coreLimits.MaxDerivedFactsLimit,
	})
	shardMgr.SetLimitsEnforcer(limitsEnforcer)
	spawnQueue := core.NewSpawnQueue(shardMgr, limitsEnforcer, core.DefaultSpawnQueueConfig())
	shardMgr.SetSpawnQueue(spawnQueue)
	_ = spawnQueue.Start()

	// Initialize JIT Prompt Compiler - CRITICAL for shard prompt formatting
	jitCompiler, err := prompt.NewJITPromptCompiler(
		prompt.WithKernel(coresys.NewKernelAdapter(kernel)),
	)
	if err != nil {
		return fmt.Errorf("failed to init JIT compiler: %w", err)
	}

	// Register shard factories - CRITICAL: without this, shards fall back to BaseShardAgent
	shardMgr.SetLLMClient(llmClient)
	shards.RegisterAllShardFactories(shardMgr, shards.RegistryContext{
		Kernel:       kernel,
		LLMClient:    llmClient,
		VirtualStore: virtualStore,
		Workspace:    cwd,
		JITCompiler:  jitCompiler,
	})

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

// KernelAdapter adapts core.RealKernel to types.KernelInterface
type KernelAdapter struct {
	RealKernel *core.RealKernel
}

func (k *KernelAdapter) AssertFact(f types.KernelFact) error {
	return k.RealKernel.Assert(core.Fact{
		Predicate: f.Predicate,
		Args:      f.Args,
	})
}

func (k *KernelAdapter) QueryPredicate(predicate string) ([]types.KernelFact, error) {
	facts, err := k.RealKernel.Query(predicate)
	if err != nil {
		return nil, err
	}
	results := make([]types.KernelFact, len(facts))
	for i, f := range facts {
		results[i] = types.KernelFact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}
	}
	return results, nil
}

func (k *KernelAdapter) QueryBool(predicate string) bool {
	facts, err := k.RealKernel.Query(predicate)
	if err != nil {
		return false
	}
	return len(facts) > 0
}

func (k *KernelAdapter) RetractFact(f types.KernelFact) error {
	return k.RealKernel.RetractFact(core.Fact{
		Predicate: f.Predicate,
		Args:      f.Args,
	})
}

// =============================================================================
// AUTH COMMANDS - CLI Engine Authentication
// =============================================================================

// runAuthClaude authenticates with Claude Code CLI and configures codeNERD
func runAuthClaude(cmd *cobra.Command, args []string) error {
	fmt.Println("Configuring Claude Code CLI engine...")

	// Check if claude CLI is installed
	claudePath, err := findExecutable("claude")
	if err != nil {
		fmt.Println("\n❌ Claude Code CLI not found.")
		fmt.Println("\nTo install:")
		fmt.Println("  npm install -g @anthropic-ai/claude-code")
		fmt.Println("\nThen run 'claude login' to authenticate with your subscription.")
		return fmt.Errorf("claude CLI not installed")
	}
	fmt.Printf("✓ Found Claude CLI at: %s\n", claudePath)

	// Check authentication status by trying a simple command
	fmt.Println("Checking authentication status...")
	checkCmd := newExecCommand(cmd.Context(), "claude", "--version")
	if output, err := checkCmd.CombinedOutput(); err != nil {
		fmt.Printf("Claude CLI check failed: %s\n", string(output))
		fmt.Println("\nPlease run 'claude login' to authenticate with your Claude subscription.")
		return fmt.Errorf("claude CLI not authenticated")
	}
	fmt.Println("✓ Claude CLI is authenticated")

	// Update config
	cfg, err := loadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.SetEngine("claude-cli"); err != nil {
		return fmt.Errorf("failed to set engine: %w", err)
	}

	// Ensure claude_cli config exists
	if cfg.ClaudeCLI == nil {
		cfg.ClaudeCLI = &config.ClaudeCLIConfig{
			Model:   "sonnet",
			Timeout: 300,
		}
	}

	if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\n✓ Configuration updated!")
	fmt.Println("  Engine: claude-cli")
	fmt.Printf("  Model: %s\n", cfg.ClaudeCLI.Model)
	fmt.Println("\ncodeNERD will now use your Claude subscription for LLM calls.")
	return nil
}

// runAuthCodex authenticates with Codex CLI and configures codeNERD
func runAuthCodex(cmd *cobra.Command, args []string) error {
	fmt.Println("Configuring Codex CLI engine...")

	// Check if codex CLI is installed
	codexPath, err := findExecutable("codex")
	if err != nil {
		fmt.Println("\n❌ Codex CLI not found.")
		fmt.Println("\nTo install:")
		fmt.Println("  npm install -g @openai/codex")
		fmt.Println("\nThen run 'codex login' to authenticate with your ChatGPT subscription.")
		return fmt.Errorf("codex CLI not installed")
	}
	fmt.Printf("✓ Found Codex CLI at: %s\n", codexPath)

	// Check authentication status
	fmt.Println("Checking authentication status...")
	checkCmd := newExecCommand(cmd.Context(), "codex", "--version")
	if output, err := checkCmd.CombinedOutput(); err != nil {
		fmt.Printf("Codex CLI check failed: %s\n", string(output))
		fmt.Println("\nPlease run 'codex login' to authenticate with your ChatGPT subscription.")
		return fmt.Errorf("codex CLI not authenticated")
	}
	fmt.Println("✓ Codex CLI is authenticated")

	// Update config
	cfg, err := loadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.SetEngine("codex-cli"); err != nil {
		return fmt.Errorf("failed to set engine: %w", err)
	}

	// Ensure codex_cli config exists
	if cfg.CodexCLI == nil {
		cfg.CodexCLI = &config.CodexCLIConfig{
			Model:   "gpt-5",
			Sandbox: "read-only",
			Timeout: 300,
		}
	}

	if err := cfg.Save(config.DefaultUserConfigPath()); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	fmt.Println("\n✓ Configuration updated!")
	fmt.Println("  Engine: codex-cli")
	fmt.Printf("  Model: %s\n", cfg.CodexCLI.Model)
	fmt.Printf("  Sandbox: %s\n", cfg.CodexCLI.Sandbox)
	fmt.Println("\ncodeNERD will now use your ChatGPT subscription for LLM calls.")
	return nil
}

// runAuthStatus shows current authentication status
func runAuthStatus(cmd *cobra.Command, args []string) error {
	cfg, err := loadOrCreateConfig()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	engine := cfg.GetEngine()
	fmt.Printf("Current engine: %s\n\n", engine)

	switch engine {
	case "claude-cli":
		fmt.Println("Backend: Claude Code CLI (subscription)")
		cliCfg := cfg.GetClaudeCLIConfig()
		fmt.Printf("  Model: %s\n", cliCfg.Model)
		fmt.Printf("  Timeout: %ds\n", cliCfg.Timeout)

		// Check CLI status
		if _, err := findExecutable("claude"); err != nil {
			fmt.Println("  Status: ❌ CLI not installed")
		} else {
			fmt.Println("  Status: ✓ CLI installed")
		}

	case "codex-cli":
		fmt.Println("Backend: Codex CLI (ChatGPT subscription)")
		cliCfg := cfg.GetCodexCLIConfig()
		fmt.Printf("  Model: %s\n", cliCfg.Model)
		fmt.Printf("  Sandbox: %s\n", cliCfg.Sandbox)
		fmt.Printf("  Timeout: %ds\n", cliCfg.Timeout)

		// Check CLI status
		if _, err := findExecutable("codex"); err != nil {
			fmt.Println("  Status: ❌ CLI not installed")
		} else {
			fmt.Println("  Status: ✓ CLI installed")
		}

	default:
		fmt.Println("Backend: HTTP API")
		provider, _ := cfg.GetActiveProvider()
		fmt.Printf("  Provider: %s\n", provider)
		if cfg.Model != "" {
			fmt.Printf("  Model: %s\n", cfg.Model)
		}
	}

	return nil
}

// Helper functions for auth commands

// findExecutable searches for an executable in PATH
func findExecutable(name string) (string, error) {
	// Try exec.LookPath first
	path, err := execLookPath(name)
	if err == nil {
		return path, nil
	}

	// On Windows, try with .exe extension
	if strings.HasSuffix(os.Getenv("OS"), "Windows_NT") || os.Getenv("GOOS") == "windows" {
		path, err = execLookPath(name + ".exe")
		if err == nil {
			return path, nil
		}
		path, err = execLookPath(name + ".cmd")
		if err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("%s not found in PATH", name)
}

// execLookPath wraps exec.LookPath for testability
var execLookPath = func(file string) (string, error) {
	return exec.LookPath(file)
}

// newExecCommand creates an exec.Cmd for testability
var newExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
	return exec.CommandContext(ctx, name, args...)
}

// loadOrCreateConfig loads user config or creates default
func loadOrCreateConfig() (*config.UserConfig, error) {
	path := config.DefaultUserConfigPath()
	cfg, err := config.LoadUserConfig(path)
	if err != nil {
		// Create new config if doesn't exist
		cfg = config.DefaultUserConfig()
	}
	return cfg, nil
}
