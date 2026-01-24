// Package main implements the codeNERD CLI - a high-assurance, neuro-symbolic CLI agent.
//
// This file serves as the entry point and command registration hub. The actual command
// implementations are split across multiple cmd_*.go files for maintainability.
//
// # File Index
//
// Entry Point & Global State:
//   - main.go            - Entry point, rootCmd, global flags, init()
//
// Core Commands:
//   - cmd_instruction.go - runCmd, runInstruction() OODA loop
//   - cmd_spawn.go       - defineAgentCmd, spawnCmd, defineAgent(), spawnShard()
//   - cmd_init_scan.go   - initCmd, scanCmd, runInit(), runScan(), writeScanFacts()
//
// Direct Action Commands (TUI verb mirrors):
//   - cmd_direct_actions.go - reviewCmd, fixCmd, testCmd, pushCmd, commitCmd,
//     explainCmd, createCmd, refactorCmd, perceptionCmd,
//     runDirectAction(), runPerceptionTest(), truncateResponse()
//
// Advanced Commands (Dream state, speculation):
//   - cmd_advanced.go    - dreamCmd, shadowCmd, whatifCmd, logicCmd, agentsCmd,
//     toolCmd, jitCmd, runDreamState(), runShadowSimulation(),
//     runWhatIf(), runLogicQuery(), runAgentsList(),
//     runToolCommand(), runJITStatus()
//
// Browser Automation:
//   - cmd_browser.go     - browserCmd, browserLaunchCmd, browserSessionCmd,
//     browserSnapshotCmd, getBrowserConfig(), browserLaunch(),
//     browserSession(), browserSnapshot()
//
// Mangle Validation & LSP:
//   - cmd_mangle_check.go - checkMangleCmd, runCheckMangle(), checkFile()
//   - cmd_mangle_lsp.go   - mangleLSPCmd, runMangleLSP() (Language Server Protocol for IDE integration)
//
// Query & Status:
//   - cmd_query.go       - queryCmd, statusCmd, whyCmd, queryFacts(), showStatus(),
//     runWhy(), joinArgs(), sanitizeFactForMangle()
//
// Campaign Management:
//   - cmd_campaign.go    - campaignCmd, campaignStartCmd, campaignStatusCmd,
//     campaignPauseCmd, campaignResumeCmd, campaignListCmd
//
// Authentication:
//   - cmd_auth.go        - authCmd, authClaudeCmd, authCodexCmd, authStatusCmd
//
// Context Testing:
//   - cmd_test_context.go - testContextCmd
//
// Helpers:
//   - system_results.go  - systemResultBaselines(), waitForSystemResults(), formatSystemResults()
//   - stats.go           - computeStats()
package main

import (
	"codenerd/cmd/nerd/chat"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"codenerd/internal/config"
	"codenerd/internal/logging"
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
		zapConfig := zap.NewProductionConfig()
		if verbose {
			zapConfig.Level = zap.NewAtomicLevelAt(zapcore.DebugLevel)
		}
		var err error
		logger, err = zapConfig.Build()
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
			// Fallback: If file logging fails, just warn and continue without it.
			// This prevents the CLI from crashing due to permission issues in valid workspaces.
			fmt.Fprintf(os.Stderr, "Warning: Failed to initialize file logging (telemetry disabled): %v\n", err)
		}

		// Load configuration to respect user defaults (e.g. timeout)
		// We do this after logging init so we can log config loading errors/success
		configPath := filepath.Join(ws, ".nerd", "config.yaml")
		if cfg, err := config.Load(configPath); err == nil {
			// If timeout flag wasn't set by user, use config default
			if !cmd.Flags().Changed("timeout") {
				timeout = cfg.GetExecutionTimeout()
				logging.BootDebug("Using configured timeout: %v", timeout)
			}
		} else {
			// Just log debug, as defaults are fine if config missing
			logging.BootDebug("No config loaded from %s (using defaults): %v", configPath, err)
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
				// Handle workspace permission errors gracefully
				if os.IsPermission(err) {
					return fmt.Errorf("cannot access workspace '%s': permission denied\n\nSuggestions:\n  - Check directory permissions with 'ls -la %s'\n  - Try running from a different directory\n  - Use --workspace with a writable path", ws, ws)
				}
				if os.IsNotExist(err) {
					return fmt.Errorf("workspace '%s' does not exist\n\nSuggestions:\n  - Create the directory with 'mkdir -p %s'\n  - Use --workspace with an existing path", ws, ws)
				}
				return fmt.Errorf("cannot change to workspace '%s': %w", ws, err)
			}
		}

		// Default behavior: launch interactive chat
		cfg := chat.Config{
			DisableSystemShards: disableSystemShards,
		}
		return chat.RunInteractiveChat(cfg)
	},
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose logging")
	rootCmd.PersistentFlags().StringVar(&apiKey, "api-key", "", "Z.AI API key (or set ZAI_API_KEY env)")
	rootCmd.PersistentFlags().StringVarP(&workspace, "workspace", "w", "", "Workspace directory (default: current)")
	// Default timeout is 25m, but can be overridden by config.yaml or --timeout flag
	rootCmd.PersistentFlags().DurationVar(&timeout, "timeout", 25*time.Minute, "Operation timeout")

	// Define-agent flags
	var agentName, agentTopic string
	defineAgentCmd.Flags().StringVar(&agentName, "name", "", "Agent name (required)")
	defineAgentCmd.Flags().StringVar(&agentTopic, "topic", "", "Research topic (required)")
	defineAgentCmd.MarkFlagRequired("name")
	defineAgentCmd.MarkFlagRequired("topic")

	// System shard controls
	runCmd.Flags().StringSliceVar(&disableSystemShards, "disable-system-shard", nil, "Disable a Type 1 system shard by name")

	// Interactive mode flag for direct action commands
	// Enables multi-turn feedback loops with refine/redo/approve meta-commands
	directActionCmds := []*cobra.Command{reviewCmd, fixCmd, testCmd, explainCmd, createCmd, refactorCmd}
	for _, cmd := range directActionCmds {
		cmd.Flags().BoolVarP(&interactiveMode, "interactive", "i", false, "Enable interactive mode with feedback loop")
	}

	// Debug flags for direct action commands
	// Enables verbose tracing, dry-run mode, kernel dump, and API tracing
	registerDebugFlags(directActionCmds...)

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
		authAntigravityCmd,
		authStatusCmd,
	)

	// Antigravity subcommands
	authAntigravityCmd.AddCommand(
		authAntigravityAddCmd,
		authAntigravityListCmd,
		authAntigravityRemoveCmd,
		authAntigravityStatusCmd,
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
		checkMangleCmd,
		mangleLSPCmd,
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
		securityCmd, // security analysis
		analyzeCmd,  // general analysis
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

	// Strategic planning commands
	rootCmd.AddCommand(
		northstarCmd,
	)

	// System visibility commands
	rootCmd.AddCommand(
		mcpCmd,
		autopoiesisCmd,
		memoryCmd,
	)

	// Session management commands
	rootCmd.AddCommand(
		sessionsCmd,
	)

	// Knowledge base commands
	rootCmd.AddCommand(
		knowledgeCmd,
	)

	// Transparency/introspection commands
	rootCmd.AddCommand(
		glassboxCmd,
		transparencyCmd,
		reflectionCmd,
	)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
