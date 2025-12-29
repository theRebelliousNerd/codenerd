// Package main implements the codeNERD CLI commands.
// This file provides CLI access to core system status and visibility.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	coresys "codenerd/internal/system"

	"github.com/spf13/cobra"
)

// =============================================================================
// MCP CLI COMMANDS
// =============================================================================

// mcpCmd is the parent command for MCP operations
var mcpCmd = &cobra.Command{
	Use:   "mcp",
	Short: "Model Context Protocol server management",
	Long: `Manage MCP (Model Context Protocol) servers and tools.

MCP servers provide external tools and resources that codeNERD can use
during task execution.

Examples:
  nerd mcp list     # List connected MCP servers
  nerd mcp tools    # Show available MCP tools
  nerd mcp status   # Show MCP system status`,
}

// mcpListCmd lists connected MCP servers
var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List connected MCP servers",
	Long:  `Shows all MCP servers that are connected or configured.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		fmt.Println("🔌 MCP Servers")
		fmt.Println(strings.Repeat("─", 60))

		// Query MCP server facts
		servers, _ := cortex.Kernel.Query("mcp_server_registered")
		if len(servers) == 0 {
			fmt.Println("No MCP servers connected.")
			fmt.Println("\nConfigure servers in .nerd/config.json under 'mcp_servers'")
			return nil
		}

		for _, srv := range servers {
			if len(srv.Args) >= 2 {
				fmt.Printf("  - %v (%v)\n", srv.Args[0], srv.Args[1])
			}
		}

		fmt.Printf("\nTotal: %d servers\n", len(servers))
		return nil
	},
}

// mcpToolsCmd lists available MCP tools
var mcpToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Show available MCP tools",
	Long:  `Lists all tools provided by connected MCP servers.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		fmt.Println("🔧 MCP Tools")
		fmt.Println(strings.Repeat("─", 60))

		// Query tool capability facts
		tools, _ := cortex.Kernel.Query("mcp_tool_capability")
		if len(tools) == 0 {
			fmt.Println("No MCP tools available.")
			return nil
		}

		for _, tool := range tools {
			if len(tool.Args) >= 2 {
				fmt.Printf("  - %v: %v\n", tool.Args[0], tool.Args[1])
			}
		}

		fmt.Printf("\nTotal: %d tools\n", len(tools))
		return nil
	},
}

// mcpStatusCmd shows MCP system status
var mcpStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show MCP system status",
	Long:  `Displays the overall status of the MCP integration layer.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		fmt.Println("🔌 MCP Status")
		fmt.Println(strings.Repeat("─", 60))

		// Query facts
		servers, _ := cortex.Kernel.Query("mcp_server_registered")
		tools, _ := cortex.Kernel.Query("mcp_tool_capability")

		fmt.Printf("Connected Servers: %d\n", len(servers))
		fmt.Printf("Available Tools:   %d\n", len(tools))

		// MCP integration is active if we have any servers registered
		if len(servers) > 0 {
			fmt.Println("\nMCP Integration: Active")
		} else {
			fmt.Println("\nMCP Integration: No servers configured")
		}

		return nil
	},
}

// =============================================================================
// AUTOPOIESIS CLI COMMANDS
// =============================================================================

// autopoiesisCmd is the parent command for autopoiesis operations
var autopoiesisCmd = &cobra.Command{
	Use:     "autopoiesis",
	Aliases: []string{"auto"},
	Short:   "Self-modification and learning system",
	Long: `View and manage codeNERD's self-modification capabilities.

Autopoiesis encompasses:
- Ouroboros Loop (tool generation)
- Thunderdome (adversarial testing)
- Prompt Evolution (system prompt learning)
- Legislator (runtime rule creation)

Examples:
  nerd autopoiesis status   # Show autopoiesis status
  nerd autopoiesis learning # Show learning history
  nerd autopoiesis tools    # Show generated tools`,
}

// autopoiesisStatusCmd shows autopoiesis status
var autopoiesisStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show autopoiesis system status",
	Long:  `Displays the status of all self-modification subsystems.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		fmt.Println("🧬 Autopoiesis Status")
		fmt.Println(strings.Repeat("─", 60))

		// Check subsystems
		fmt.Print("Ouroboros Loop:    ")
		tools, _ := cortex.Kernel.Query("tool_registered")
		fmt.Printf("%d tools generated\n", len(tools))

		fmt.Print("Prompt Evolution:  ")
		evolutions, _ := cortex.Kernel.Query("prompt_evolved")
		fmt.Printf("%d evolutions\n", len(evolutions))

		fmt.Print("Learning Store:    ")
		patterns, _ := cortex.Kernel.Query("learned_pattern")
		fmt.Printf("%d patterns\n", len(patterns))

		fmt.Print("Thunderdome:       ")
		battles, _ := cortex.Kernel.Query("thunderdome_result")
		fmt.Printf("%d battles\n", len(battles))

		// Check if orchestrator is active
		if cortex.Orchestrator != nil {
			fmt.Println("\nOrchestrator: Active")
		} else {
			fmt.Println("\nOrchestrator: Standby")
		}

		return nil
	},
}

// autopoiesisLearningCmd shows learning history
var autopoiesisLearningCmd = &cobra.Command{
	Use:   "learning",
	Short: "Show learned patterns and preferences",
	Long:  `Displays patterns that have been learned from execution feedback.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		fmt.Println("📚 Learning History")
		fmt.Println(strings.Repeat("─", 60))

		// Query learned patterns
		patterns, _ := cortex.Kernel.Query("learned_pattern")
		if len(patterns) == 0 {
			fmt.Println("No patterns learned yet.")
			fmt.Println("\nPatterns are learned from repeated rejections/acceptances.")
			return nil
		}

		fmt.Printf("Found %d learned patterns:\n\n", len(patterns))
		for i, p := range patterns {
			if i >= 20 {
				fmt.Printf("... and %d more\n", len(patterns)-20)
				break
			}
			fmt.Printf("  %s\n", p.String())
		}

		return nil
	},
}

// autopoiesisToolsCmd shows generated tools
var autopoiesisToolsCmd = &cobra.Command{
	Use:   "tools",
	Short: "Show tools generated by Ouroboros",
	Long:  `Lists all tools that have been automatically generated.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		fmt.Println("🔧 Generated Tools (Ouroboros)")
		fmt.Println(strings.Repeat("─", 60))

		// Query tool facts
		tools, _ := cortex.Kernel.Query("tool_registered")
		if len(tools) == 0 {
			fmt.Println("No tools generated yet.")
			fmt.Println("\nTools are generated when the Ouroboros Loop detects")
			fmt.Println("missing capabilities during task execution.")
			return nil
		}

		for _, tool := range tools {
			if len(tool.Args) >= 1 {
				fmt.Printf("  - %v\n", tool.Args[0])
			}
		}

		fmt.Printf("\nTotal: %d tools\n", len(tools))
		return nil
	},
}

// =============================================================================
// MEMORY/CONTEXT CLI COMMANDS
// =============================================================================

// memoryCmd is the parent command for memory operations
var memoryCmd = &cobra.Command{
	Use:   "memory",
	Short: "Memory tier and context management",
	Long: `View and manage codeNERD's 4-tier memory system.

Memory Tiers:
  RAM    - In-memory working facts (session-scoped)
  Vector - SQLite + embeddings (semantic search)
  Graph  - Knowledge graph relationships
  Cold   - Long-term learned preferences

Examples:
  nerd memory status   # Show memory statistics
  nerd memory query    # Query specific memories`,
}

// memoryStatusCmd shows memory status
var memoryStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show memory tier statistics",
	Long:  `Displays statistics for each memory tier.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		fmt.Println("🧠 Memory Status")
		fmt.Println(strings.Repeat("─", 60))

		// RAM tier - kernel facts
		allFacts, _ := cortex.Kernel.Query("*")
		fmt.Printf("RAM (Working Memory):  %d facts\n", len(allFacts))

		// Vector tier
		if cortex.LocalDB != nil {
			stats, err := cortex.LocalDB.GetStats()
			if err == nil {
				// Sum up all entries
				var total int64
				for _, count := range stats {
					total += count
				}
				fmt.Printf("Vector (Embeddings):   %d entries\n", total)
			} else {
				fmt.Println("Vector (Embeddings):   unavailable")
			}
		}

		// Graph tier
		graphEntries, _ := cortex.Kernel.Query("knowledge_edge")
		fmt.Printf("Graph (Relationships): %d edges\n", len(graphEntries))

		// Cold tier
		coldEntries, _ := cortex.Kernel.Query("cold_storage_entry")
		fmt.Printf("Cold (Long-term):      %d entries\n", len(coldEntries))

		// Context compression stats
		compressed, _ := cortex.Kernel.Query("compressed_context")
		fmt.Printf("\nCompressed Contexts:   %d\n", len(compressed))

		return nil
	},
}

func init() {
	// MCP subcommands
	mcpCmd.AddCommand(
		mcpListCmd,
		mcpToolsCmd,
		mcpStatusCmd,
	)

	// Autopoiesis subcommands
	autopoiesisCmd.AddCommand(
		autopoiesisStatusCmd,
		autopoiesisLearningCmd,
		autopoiesisToolsCmd,
	)

	// Memory subcommands
	memoryCmd.AddCommand(
		memoryStatusCmd,
	)
}
