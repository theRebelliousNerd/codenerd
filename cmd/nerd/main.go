package main

import (
	"codenerd/internal/articulation"
	"codenerd/internal/browser"
	"codenerd/internal/core"
	nerdinit "codenerd/internal/init"
	"codenerd/internal/mangle"
	"codenerd/internal/perception"
	"codenerd/internal/tactile"
	"codenerd/internal/world"
	"context"
	"fmt"
	"os"
	"os/signal"
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
		return runInteractiveChat()
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

	// Browser subcommands
	browserCmd.AddCommand(browserLaunchCmd)
	browserCmd.AddCommand(browserSessionCmd)
	browserCmd.AddCommand(browserSnapshotCmd)

	// Add commands to root
	rootCmd.AddCommand(runCmd)
	rootCmd.AddCommand(defineAgentCmd)
	rootCmd.AddCommand(spawnCmd)
	rootCmd.AddCommand(browserCmd)
	rootCmd.AddCommand(queryCmd)
	rootCmd.AddCommand(statusCmd)
	rootCmd.AddCommand(initCmd)
	rootCmd.AddCommand(whyCmd)
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

	logger.Info("Defining specialist agent",
		zap.String("name", name),
		zap.String("topic", topic))

	// Create shard manager and define profile
	shardManager := core.NewShardManager()
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
	result, err := shardManager.Spawn(ctx, shardType, task)
	if err != nil {
		return fmt.Errorf("spawn failed: %w", err)
	}

	fmt.Printf("Shard Result: %s\n", result)
	return nil
}

// browserLaunch launches the browser instance
func browserLaunch(cmd *cobra.Command, args []string) error {
	logger.Info("Launching browser")

	// Initialize browser session manager
	cfg := browser.DefaultConfig()
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return fmt.Errorf("failed to create mangle engine: %w", err)
	}

	mgr := browser.NewSessionManager(cfg, engine)

	// Start the session manager
	if err := mgr.Start(context.Background()); err != nil {
		return fmt.Errorf("failed to start session manager: %w", err)
	}

	fmt.Printf("Browser launched. Control URL: %s\n", mgr.ControlURL())
	fmt.Println("Press Ctrl+C to shutdown")

	// Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	_ = mgr.Shutdown(context.Background())
	return nil
}

// browserSession creates a new browser session
func browserSession(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	url := args[0]
	logger.Info("Creating browser session", zap.String("url", url))

	cfg := browser.DefaultConfig()
	engine, err := mangle.NewEngine(mangle.DefaultConfig(), nil)
	if err != nil {
		return fmt.Errorf("failed to create mangle engine: %w", err)
	}

	mgr := browser.NewSessionManager(cfg, engine)
	if err := mgr.Start(ctx); err != nil {
		return fmt.Errorf("failed to start session manager: %w", err)
	}
	defer func() { _ = mgr.Shutdown(context.Background()) }()

	session, err := mgr.CreateSession(ctx, url)
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	fmt.Printf("Session created: %s\n", session.ID)
	return nil
}

// browserSnapshot snapshots DOM as Mangle facts
func browserSnapshot(cmd *cobra.Command, args []string) error {
	sessionID := args[0]
	logger.Info("Snapshotting DOM", zap.String("session", sessionID))

	// TODO: Implement persistent session registry + DOM fact export
	fmt.Printf("DOM snapshot for session %s\n", sessionID)
	fmt.Println("(Session persistence not yet implemented)")
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
	if nerdinit.IsInitialized(cwd) {
		fmt.Println("Project already initialized. Use 'nerd status' to view project info.")
		fmt.Println("To reinitialize, delete the .nerd/ directory first.")
		return nil
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
