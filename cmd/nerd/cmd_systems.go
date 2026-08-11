// Package main implements the codeNERD CLI commands.
// This file provides CLI access to core system status and visibility.
package main

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/mcp"
	coresys "codenerd/internal/system"

	"github.com/spf13/cobra"
)

// =============================================================================
// MCP CLI COMMANDS
// =============================================================================

// mcpCmd is the parent command for MCP operations
var mcpCmd = &cobra.Command{
	Args:  cobra.NoArgs,
	Use:   "mcp",
	Short: "Model Context Protocol server management",
	Long: `Manage MCP (Model Context Protocol) servers and tools.

MCP servers provide external tools and resources that codeNERD can use
during task execution.

Examples:
  nerd mcp list     # List connected MCP servers
  nerd mcp tools    # Show available MCP tools
  nerd mcp status   # Show MCP system status`,
	RunE: parentGroupRunE,
}

// mcpListCmd lists connected MCP servers
var mcpListCmd = &cobra.Command{
	Use:   "list",
	Short: "List connected MCP servers",
	Long:  `Shows all MCP servers that are connected or configured.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		fmt.Println("🔌 MCP Servers")
		fmt.Println(strings.Repeat("─", 60))

		servers, err := readMCPServers(ctx)
		if err != nil {
			return err
		}

		if len(servers) == 0 {
			fmt.Println("No MCP servers recorded.")
			// The key name matters more than usual here: LoadUserConfig decodes
			// strictly, so an unknown top-level field is a hard load error, not
			// a warning. This line used to say 'mcp_servers', which is not a
			// field on UserConfig — following the instruction would have made
			// codeNERD refuse to start. The real path is integrations.servers.
			fmt.Println("\nConfigure servers in .nerd/config.json under 'integrations.servers':")
			fmt.Println(`  "integrations": {`)
			fmt.Println(`    "servers": {`)
			fmt.Println(`      "my_server": {`)
			fmt.Println(`        "enabled": true,`)
			fmt.Println(`        "protocol": "stdio",`)
			fmt.Println(`        "auto_connect": true`)
			fmt.Println(`      }`)
			fmt.Println(`    }`)
			fmt.Println(`  }`)
			return nil
		}

		for _, srv := range servers {
			fmt.Printf("  - %s (%s) %s [%s]\n", srv.ID, srv.Protocol, srv.Endpoint, srv.Status)
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

		fmt.Println("🔧 MCP Tools")
		fmt.Println(strings.Repeat("─", 60))

		tools, err := readMCPTools(ctx)
		if err != nil {
			return err
		}
		if len(tools) == 0 {
			fmt.Println("No MCP tools recorded.")
			fmt.Println("Tools are discovered on connect; run 'nerd mcp list' to check for servers.")
			return nil
		}

		for _, tool := range tools {
			desc := tool.Description
			if len(desc) > 70 {
				desc = desc[:70] + "..."
			}
			fmt.Printf("  - %s: %s\n", tool.Name, desc)
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

		fmt.Println("🔌 MCP Status")
		fmt.Println(strings.Repeat("─", 60))

		servers, err := readMCPServers(ctx)
		if err != nil {
			return err
		}
		tools, err := readMCPTools(ctx)
		if err != nil {
			return err
		}

		connected := 0
		for _, s := range servers {
			if s.Status == mcp.ServerStatusConnected {
				connected++
			}
		}

		fmt.Printf("Recorded Servers:  %d\n", len(servers))
		fmt.Printf("Connected:         %d\n", connected)
		fmt.Printf("Discovered Tools:  %d\n", len(tools))
		fmt.Printf("Store:             %s\n", mcpStorePath())

		switch {
		case connected > 0:
			fmt.Println("\nMCP Integration: Active")
		case len(servers) > 0:
			fmt.Println("\nMCP Integration: servers recorded but none currently connected")
		default:
			fmt.Println("\nMCP Integration: No servers configured")
		}

		return nil
	},
}

// The three commands above read the persisted MCP store rather than the kernel.
//
// They used to query mcp_server_registered and mcp_tool_capability. Both are
// declared in schemas_mcp.mg and neither has a producer anywhere in the repo —
// the only MCP fact ever asserted is mcp_tool_vector_score
// (internal/mcp/compiler.go:83). So all three commands reported zero
// unconditionally, and would have kept reporting zero with servers connected
// and tools discovered. Same family as F-GLASS-1 and F-LOGS-1.
//
// internal/mcp/store.go already persists both to .nerd/mcp_tools.db, which
// survives the process, so it is also the right source for a CLI invocation
// that boots a fresh kernel holding no session facts. Reading it by path rather
// than through Cortex.mcpBridge (unexported, and nil when no servers are
// configured) means the record is visible whether or not a bridge is live.

// mcpStorePath returns the on-disk location of the MCP store, matching
// NewMCPIntegrationBridge.
func mcpStorePath() string {
	root := workspace
	if root == "" {
		root = "."
	}
	return filepath.Join(root, ".nerd", "mcp_tools.db")
}

// openMCPStore opens the persisted MCP store read-only-ish. The constructor
// creates its tables if absent, so a workspace that never ran MCP yields an
// empty store rather than an error.
func openMCPStore() (*mcp.MCPToolStore, error) {
	store, err := mcp.NewMCPToolStore(mcpStorePath(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to open the MCP store at %s: %w", mcpStorePath(), err)
	}
	return store, nil
}

func readMCPServers(ctx context.Context) ([]*mcp.MCPServer, error) {
	store, err := openMCPStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	servers, err := store.GetAllServers(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read MCP servers: %w", err)
	}
	return servers, nil
}

func readMCPTools(ctx context.Context) ([]*mcp.MCPTool, error) {
	store, err := openMCPStore()
	if err != nil {
		return nil, err
	}
	defer store.Close()

	tools, err := store.GetAllTools(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to read MCP tools: %w", err)
	}
	return tools, nil
}

// =============================================================================
// AUTOPOIESIS CLI COMMANDS
// =============================================================================

// autopoiesisCmd is the parent command for autopoiesis operations
var autopoiesisCmd = &cobra.Command{
	Args:    cobra.NoArgs,
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
	RunE: parentGroupRunE,
}

// autopoiesisStatusCmd shows autopoiesis status
var autopoiesisStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show autopoiesis system status",
	Long:  `Displays the status of all self-modification subsystems.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		key := resolveAPIKey(apiKey, workspace)

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
		successPatterns, _ := cortex.Kernel.Query("success_pattern")
		failurePatterns, _ := cortex.Kernel.Query("failure_pattern")
		fmt.Printf("%d success / %d failure patterns\n", len(successPatterns), len(failurePatterns))

		// correction_pattern is asserted by Go but read by no policy rule, so a non-zero count means facts are being produced that nothing consumes.
		fmt.Print("Corrections:       ")
		corrections, _ := cortex.Kernel.Query("correction_pattern")
		fmt.Printf("%d patterns\n", len(corrections))

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

		key := resolveAPIKey(apiKey, workspace)

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

		key := resolveAPIKey(apiKey, workspace)

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
	// A parent command with no Run silently prints help and exits 0 for an
	// unrecognised subcommand, so `nerd memory stats` (the real one is
	// `status`) looked exactly like success. NoArgs makes cobra reject the
	// unknown argument instead; a bare invocation still prints help.
	Args:  cobra.NoArgs,
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
	RunE: parentGroupRunE,
}

// memoryStatusCmd shows memory status
var memoryStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show memory tier statistics",
	Long:  `Displays statistics for each memory tier.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		key := resolveAPIKey(apiKey, workspace)

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

// parentGroupRunE is the RunE for a command that exists only to hold
// subcommands.
//
// Without it, cobra reaches `if !c.Runnable() { return flag.ErrHelp }` before it
// validates arguments, and Execute treats ErrHelp as success. The result is that
// `nerd memory stats` — the real subcommand is `status` — printed the group's
// help text and exited 0, indistinguishable from a command that worked. Setting
// Args: cobra.NoArgs does not fix it, because the runnable check comes first.
//
// A typo'd subcommand reporting success is the same false-success family as the
// rest of this session's work: the caller cannot tell "did what you asked" from
// "did nothing at all". Scripts, campaigns and shards all consume these exit
// codes.
//
// Bare invocation still prints help and exits 0, which is the correct and
// expected behaviour for a group.
// Note on what actually does the work here: it is the mere PRESENCE of a RunE,
// not this function's body. With one attached, cobra treats the command as
// runnable, gets past the ErrHelp short-circuit, and emits its own
// `unknown command "stats" for "nerd memory"` with a non-zero exit. Verified by
// running it — the message on screen is cobra's, not one written here.
//
// So this body only ever runs for a bare invocation, and printing help is the
// right response to that. An args branch was written first and then removed
// once testing showed it unreachable; an unreachable branch with a confident
// comment explaining what it handles is the failure mode the rest of this
// session has been removing.
func parentGroupRunE(cmd *cobra.Command, _ []string) error {
	return cmd.Help()
}
