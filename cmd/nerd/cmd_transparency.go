// Package main implements transparency and introspection CLI commands for codeNERD.
// This file handles glassbox, transparency reports, and reflection status.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	coresys "codenerd/internal/system"

	"github.com/spf13/cobra"
)

// =============================================================================
// KERNEL TRANSPARENCY COMMANDS
// =============================================================================

// glassboxCmd shows kernel transparency info
var glassboxCmd = &cobra.Command{
	Use:   "glassbox",
	Short: "Show Mangle kernel transparency info",
	Long: `Display transparency information about the Mangle kernel state.

Shows:
  - Kernel status
  - Sample predicates`,
	RunE: runGlassbox,
}

func runGlassbox(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	fmt.Println("🔍 Glassbox: Mangle Kernel Transparency")
	fmt.Println(strings.Repeat("═", 60))

	if cortex.Kernel == nil {
		fmt.Println("⚠️  No kernel attached")
		return nil
	}

	fmt.Println("📊 Kernel Status: Active")
	fmt.Println(strings.Repeat("─", 40))

	// Query some sample predicates
	predicates := []string{"route_to", "shard_routing", "tool_invocation", "file_state"}
	for _, pred := range predicates {
		facts, _ := cortex.Kernel.Query(pred)
		fmt.Printf("   %-20s %d facts\n", pred+":", len(facts))
	}

	fmt.Println(strings.Repeat("═", 60))
	return nil
}

func formatFactStr(f core.Fact) string {
	if len(f.Args) == 0 {
		return f.Predicate
	}
	args := make([]string, len(f.Args))
	for i, a := range f.Args {
		args[i] = fmt.Sprintf("%v", a)
	}
	return fmt.Sprintf("%s(%s)", f.Predicate, strings.Join(args, ", "))
}

// =============================================================================
// TRANSPARENCY REPORT COMMANDS
// =============================================================================

// transparencyCmd shows transparency/explainability info
var transparencyCmd = &cobra.Command{
	Use:   "transparency",
	Short: "Show transparency/explainability info",
	Long: `Display transparency information about recent decisions.

Shows:
  - Recent routing decisions and why
  - Shard selection reasoning
  - Tool invocations`,
	RunE: runTransparency,
}

func runTransparency(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	key := apiKey
	if key == "" {
		key = os.Getenv("ZAI_API_KEY")
	}

	cortex, err := coresys.GetOrBootCortex(ctx, workspace, key, nil)
	if err != nil {
		return fmt.Errorf("failed to boot cortex: %w", err)
	}
	defer cortex.Close()

	fmt.Println("🔬 Transparency Report")
	fmt.Println(strings.Repeat("═", 60))

	if cortex.Kernel == nil {
		fmt.Println("⚠️  No kernel attached")
		return nil
	}

	fmt.Println("\n📍 Recent Routing Decisions")
	fmt.Println(strings.Repeat("─", 40))
	routingFacts, _ := cortex.Kernel.Query("route_to")
	if len(routingFacts) == 0 {
		fmt.Println("  No routing decisions recorded")
	} else {
		limit := 5
		if len(routingFacts) < limit {
			limit = len(routingFacts)
		}
		for i := 0; i < limit; i++ {
			fmt.Printf("  %s\n", formatFactStr(routingFacts[i]))
		}
	}

	fmt.Println("\n🔧 Recent Tool Invocations")
	fmt.Println(strings.Repeat("─", 40))
	toolFacts, _ := cortex.Kernel.Query("tool_invocation")
	if len(toolFacts) == 0 {
		fmt.Println("  No tool invocations recorded")
	} else {
		limit := 5
		if len(toolFacts) < limit {
			limit = len(toolFacts)
		}
		for i := 0; i < limit; i++ {
			fmt.Printf("  %s\n", formatFactStr(toolFacts[i]))
		}
	}

	fmt.Println(strings.Repeat("═", 60))
	return nil
}

// =============================================================================
// REFLECTION STATUS COMMANDS
// =============================================================================

// reflectionCmd shows System 2 memory reflection status
var reflectionCmd = &cobra.Command{
	Use:   "reflection",
	Short: "Show System 2 memory reflection status",
	Long: `Display the status of the System 2 (reflection) memory layer.

Shows:
  - Reflection engine configuration
  - TopK and scoring settings`,
	RunE: runReflection,
}

func runReflection(cmd *cobra.Command, args []string) error {
	fmt.Println("💭 System 2 Reflection Status")
	fmt.Println(strings.Repeat("═", 60))

	cfg, _ := config.GlobalConfig()
	if cfg == nil {
		cfg = config.DefaultUserConfig()
	}

	reflectionCfg := cfg.GetReflectionConfig()
	fmt.Printf("Enabled:           %v\n", reflectionCfg.Enabled)
	fmt.Printf("TopK:              %d\n", reflectionCfg.TopK)
	fmt.Printf("MinScore:          %.2f\n", reflectionCfg.MinScore)
	fmt.Printf("RecencyHalfLife:   %d days\n", reflectionCfg.RecencyHalfLifeDays)
	fmt.Printf("BacklogWatermark:  %d\n", reflectionCfg.BacklogWatermark)

	fmt.Println(strings.Repeat("─", 40))
	fmt.Println("Use 'nerd run /reflect' to trigger manual reflection")
	fmt.Println(strings.Repeat("═", 60))

	return nil
}
