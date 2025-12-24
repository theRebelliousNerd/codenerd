package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"codenerd/internal/core"
	coresys "codenerd/internal/system"
	"codenerd/internal/testing/context_harness"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	testContextScenario        string
	testContextAll             bool
	testContextFormat          string
	testContextMaxTurns        int
	testContextBudget          int
	testContextWithPaging      bool
	testContextVerbose         bool
	testContextInspectPrompts  bool
	testContextTraceJIT        bool
	testContextTraceActivation bool
	testContextVisCompression  bool
	testContextTracePiggyback  bool
	testContextLogDir          string
	testContextConsoleOutput   bool
)

// testContextCmd runs context system stress tests
var testContextCmd = &cobra.Command{
	Use:   "test-context",
	Short: "Test the infinite context system with realistic simulations",
	Long: `Stress-tests codeNERD's infinite context system (compression, retrieval, paging)
with realistic coding session simulations.

Available scenarios:
  - debugging-marathon      50-turn debugging session testing context retention
  - feature-implementation  75-turn feature implementation testing multi-phase paging
  - refactoring-campaign   100-turn refactoring testing long-term stability
  - research-and-build      80-turn research + implementation testing cross-phase retrieval

Examples:
  nerd test-context --scenario debugging-marathon
  nerd test-context --all
  nerd test-context --scenario debugging-marathon --format json > results.json`,
	RunE: runTestContext,
}

func init() {
	testContextCmd.Flags().StringVar(&testContextScenario, "scenario", "", "Run a specific scenario (use --list to see available)")
	testContextCmd.Flags().BoolVar(&testContextAll, "all", false, "Run all scenarios")
	testContextCmd.Flags().StringVar(&testContextFormat, "format", "console", "Output format (console, json)")
	testContextCmd.Flags().IntVar(&testContextMaxTurns, "max-turns", 0, "Override scenario turn count")
	testContextCmd.Flags().IntVar(&testContextBudget, "token-budget", 8000, "Token budget for context retrieval")
	testContextCmd.Flags().BoolVar(&testContextWithPaging, "paging", true, "Enable context paging")

	// Observability flags
	testContextCmd.Flags().BoolVarP(&testContextVerbose, "verbose", "v", false, "Verbose output (show all details)")
	testContextCmd.Flags().BoolVar(&testContextInspectPrompts, "inspect-prompts", true, "Log full prompts sent to LLM")
	testContextCmd.Flags().BoolVar(&testContextTraceJIT, "trace-jit", true, "Trace JIT prompt compilation")
	testContextCmd.Flags().BoolVar(&testContextTraceActivation, "trace-activation", true, "Trace spreading activation")
	testContextCmd.Flags().BoolVar(&testContextVisCompression, "vis-compression", true, "Visualize compression (before/after)")
	testContextCmd.Flags().BoolVar(&testContextTracePiggyback, "trace-piggyback", true, "Trace Piggyback protocol")
	testContextCmd.Flags().StringVar(&testContextLogDir, "log-dir", ".nerd/context-tests", "Directory for log files")
	testContextCmd.Flags().BoolVar(&testContextConsoleOutput, "console", true, "Also print to console (in addition to files)")

	// Add to root command
	rootCmd.AddCommand(testContextCmd)
}

func runTestContext(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Resolve API key
	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	// Set up file logging
	var consoleWriter io.Writer = os.Stdout
	if !testContextConsoleOutput {
		consoleWriter = nil
	}

	fileLogger, err := context_harness.NewFileLogger(testContextLogDir, consoleWriter)
	if err != nil {
		return fmt.Errorf("failed to create file logger: %w", err)
	}
	defer func() {
		if err := fileLogger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close file logger: %v\n", err)
		} else {
			fmt.Printf("\n📁 Logs saved to: %s\n", fileLogger.GetSessionDir())
		}
	}()

	// Create tracers/visualizers
	var promptInspector *context_harness.PromptInspector
	var jitTracer *context_harness.JITTracer
	var activationTracer *context_harness.ActivationTracer
	var compressionViz *context_harness.CompressionVisualizer

	if testContextInspectPrompts {
		promptInspector = context_harness.NewPromptInspector(
			fileLogger.GetPromptWriter(),
			testContextVerbose,
		)
	}

	if testContextTraceJIT {
		jitTracer = context_harness.NewJITTracer(
			fileLogger.GetJITWriter(),
			testContextVerbose,
		)
	}

	if testContextTraceActivation {
		activationTracer = context_harness.NewActivationTracer(
			fileLogger.GetActivationWriter(),
			testContextVerbose,
		)
	}

	if testContextVisCompression {
		compressionViz = context_harness.NewCompressionVisualizer(
			fileLogger.GetCompressionWriter(),
			testContextVerbose,
		)
	}

	// Boot Cortex (need kernel + compression system)
	fmt.Println("🚀 Booting codeNERD Cortex...")
	fmt.Printf("📊 Observability Enabled:\n")
	if testContextInspectPrompts {
		fmt.Println("  ✓ Prompt Inspection")
	}
	if testContextTraceJIT {
		fmt.Println("  ✓ JIT Compilation Tracing")
	}
	if testContextTraceActivation {
		fmt.Println("  ✓ Spreading Activation Tracing")
	}
	if testContextVisCompression {
		fmt.Println("  ✓ Compression Visualization")
	}
	if testContextTracePiggyback {
		fmt.Println("  ✓ Piggyback Protocol Tracing")
	}
	fmt.Println()

	cortex, err := coresys.BootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	// Configure simulator
	simConfig := context_harness.SimulatorConfig{
		MaxTurns:           testContextMaxTurns,
		TokenBudget:        testContextBudget,
		CompressionEnabled: true,
		PagingEnabled:      testContextWithPaging,
		VectorStoreEnabled: true,
	}

	// Create context engine integration
	// Type assert to get *core.RealKernel from the interface
	realKernel, _ := cortex.Kernel.(*core.RealKernel)

	var contextEngine *context_harness.RealContextEngine
	if cortex.LocalDB != nil && cortex.LLMClient != nil && realKernel != nil {
		contextEngine = context_harness.NewRealContextEngine(
			realKernel,
			cortex.LocalDB,
			cortex.LLMClient,
		)
	}

	// Create harness with observability
	harness := context_harness.NewHarnessWithObservability(
		realKernel,
		simConfig,
		os.Stdout,
		testContextFormat,
		promptInspector,
		jitTracer,
		activationTracer,
		compressionViz,
		contextEngine,
	)

	// List scenarios if requested
	if testContextScenario == "" && !testContextAll {
		fmt.Println("📋 Available Test Scenarios:")
		scenarios := harness.ListScenarios()
		for i, name := range scenarios {
			fmt.Printf("  %d. %s\n", i+1, name)
		}
		fmt.Println("\nUsage:")
		fmt.Println("  nerd test-context --scenario <name>")
		fmt.Println("  nerd test-context --all")
		fmt.Println("\nObservability Options:")
		fmt.Println("  --inspect-prompts      Log full prompts sent to LLM")
		fmt.Println("  --trace-jit            Trace JIT prompt compilation")
		fmt.Println("  --trace-activation     Trace spreading activation")
		fmt.Println("  --vis-compression      Visualize compression (before/after)")
		fmt.Println("  --trace-piggyback      Trace Piggyback protocol")
		fmt.Println("  --verbose, -v          Show all internal details")
		return nil
	}

	// Run scenarios
	if testContextAll {
		logger.Info("Running all context test scenarios")
		fmt.Println("🧪 Running All Context Test Scenarios")
		fmt.Println("This may take several minutes...")

		results, err := harness.RunAll(ctx)
		if err != nil {
			return fmt.Errorf("test suite failed: %w", err)
		}

		// Check for failures
		failures := 0
		for _, result := range results {
			if !result.Passed {
				failures++
			}
		}

		// Print summary
		if promptInspector != nil {
			promptInspector.Summary()
		}

		if failures > 0 {
			return fmt.Errorf("%d scenarios failed", failures)
		}

		fmt.Println("\n✅ All scenarios passed!")
		return nil

	} else {
		logger.Info("Running context test scenario", zap.String("scenario", testContextScenario))
		fmt.Printf("🧪 Running Scenario: %s\n\n", testContextScenario)

		result, err := harness.RunScenario(ctx, testContextScenario)
		if err != nil {
			return fmt.Errorf("scenario failed: %w", err)
		}

		// Print summary
		if promptInspector != nil {
			promptInspector.Summary()
		}

		if !result.Passed {
			return fmt.Errorf("scenario failed validation")
		}

		fmt.Println("\n✅ Scenario passed!")
		return nil
	}
}
