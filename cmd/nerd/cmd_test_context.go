package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/testing/context_harness"
	coresys "codenerd/internal/system"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	testContextScenario  string
	testContextAll       bool
	testContextFormat    string
	testContextMaxTurns  int
	testContextBudget    int
	testContextWithPaging bool
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

	// Boot Cortex (need kernel + compression system)
	fmt.Println("🚀 Booting codeNERD Cortex...")
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

	// Create harness
	harness := context_harness.NewHarness(
		cortex.Kernel,
		simConfig,
		os.Stdout,
		testContextFormat,
	)

	// List scenarios if requested
	if testContextScenario == "" && !testContextAll {
		fmt.Println("📋 Available Test Scenarios:\n")
		scenarios := harness.ListScenarios()
		for i, name := range scenarios {
			fmt.Printf("  %d. %s\n", i+1, name)
		}
		fmt.Println("\nUsage:")
		fmt.Println("  nerd test-context --scenario <name>")
		fmt.Println("  nerd test-context --all")
		return nil
	}

	// Run scenarios
	if testContextAll {
		logger.Info("Running all context test scenarios")
		fmt.Println("🧪 Running All Context Test Scenarios")
		fmt.Println("This may take several minutes...\n")

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

		if !result.Passed {
			return fmt.Errorf("scenario failed validation")
		}

		fmt.Println("\n✅ Scenario passed!")
		return nil
	}
}
