package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/mcp"

	"github.com/spf13/cobra"
)

// `nerd mcp select` and `nerd mcp metrics` make the MCP executive path
// inspectable from the CLI.
//
// `select` is the important one: it boots the kernel, replays the persisted
// catalog into it as EDB facts, and asks policy_mcp.mg which tools a shard
// should get. Before the policy was loaded and the facts emitted, that question
// had no answer — selection silently fell back to a Go heuristic and nothing
// reported it. The command prints which path decided.

var (
	mcpSelectShard  string
	mcpSelectVerb   string
	mcpSelectTarget string
	mcpSelectTask   string
	mcpSelectBudget int

	mcpMetricsPrometheus bool
)

var mcpSelectCmd = &cobra.Command{
	Use:   "select",
	Short: "Show which MCP tools the kernel selects for a shard",
	Long: `Replays the persisted MCP catalog into a fresh kernel and asks the
Mangle policy (section 50) which tools a shard should receive, at which render
mode. Reports whether the kernel or the Go fallback made the decision.`,
	Example: `  nerd mcp select --shard coder
  nerd mcp select --shard tester --verb test --target main.go
  nerd mcp select --shard coder --task "search the repository for a symbol"`,
	RunE: runMCPSelect,
}

var mcpMetricsCmd = &cobra.Command{
	Use:   "metrics",
	Short: "Show MCP call counts, failure counts and latency",
	Long:  `Reports per-tool invocation counters recorded in the MCP store.`,
	Example: `  nerd mcp metrics
  nerd mcp metrics --prometheus`,
	RunE: runMCPMetrics,
}

func init() {
	mcpSelectCmd.Flags().StringVar(&mcpSelectShard, "shard", "coder", "Shard type to select tools for")
	mcpSelectCmd.Flags().StringVar(&mcpSelectVerb, "verb", "", "Intent verb providing the capability boost (e.g. write, search)")
	mcpSelectCmd.Flags().StringVar(&mcpSelectTarget, "target", "", "Target file providing the domain boost")
	mcpSelectCmd.Flags().StringVar(&mcpSelectTask, "task", "", "Task description (unused without an embedding engine)")
	mcpSelectCmd.Flags().IntVar(&mcpSelectBudget, "budget", 4000, "Token budget for the compiled tool set")

	mcpMetricsCmd.Flags().BoolVar(&mcpMetricsPrometheus, "prometheus", false, "Emit Prometheus text exposition format")

	mcpCmd.AddCommand(mcpSelectCmd, mcpMetricsCmd)
}

func runMCPSelect(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	store, err := openMCPStore()
	if err != nil {
		return err
	}
	defer store.Close()

	tools, err := store.GetAllTools(ctx)
	if err != nil {
		return fmt.Errorf("failed to read MCP tools: %w", err)
	}
	if len(tools) == 0 {
		fmt.Println("No MCP tools in the catalog. Connect a server first (nerd mcp status).")
		return nil
	}
	servers, err := store.GetAllServers(ctx)
	if err != nil {
		return fmt.Errorf("failed to read MCP servers: %w", err)
	}

	root := workspace
	if root == "" {
		root = "."
	}
	kernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		return fmt.Errorf("failed to boot kernel: %w", err)
	}
	adapter := &cliMCPKernel{kernel: kernel}

	// Replay the persisted catalog. A CLI process holds no session facts, so
	// without this the policy has nothing to join against.
	emitter := mcp.NewFactEmitter(adapter)
	for _, server := range servers {
		emitter.EmitServer(server)
	}
	for _, tool := range tools {
		emitter.EmitTool(tool)
	}
	if err := assertMCPSelectContext(adapter); err != nil {
		return err
	}

	compiler := mcp.NewJITToolCompiler(store, nil, adapter)
	compiled, err := compiler.Compile(ctx, mcp.ToolCompilationContext{
		ShardType:       mcpSelectShard,
		TaskDescription: mcpSelectTask,
		IntentVerb:      mcpSelectVerb,
		TokenBudget:     mcpSelectBudget,
	})
	if err != nil {
		return fmt.Errorf("tool compilation failed: %w", err)
	}

	fmt.Printf("🧠 MCP tool selection for shard %q\n", mcpSelectShard)
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("Decided by:  %s\n", selectionPathLabel(compiled.Stats.SelectionPath))
	fmt.Printf("Catalog:     %d tool(s), %d server(s)\n", compiled.Stats.TotalTools, len(servers))
	fmt.Printf("Selected:    %d (skeleton=%d, flesh=%d)\n",
		compiled.Stats.SelectedTools, compiled.Stats.SkeletonTools, compiled.Stats.FleshTools)
	fmt.Printf("Budget:      %d/%d tokens\n\n", compiled.Stats.TokensUsed, compiled.Stats.TokenBudget)

	if len(compiled.FullTools) > 0 {
		fmt.Println("Full:")
		for _, tool := range compiled.FullTools {
			fmt.Printf("  • %-30s %s\n", tool.Name, tool.Condensed)
		}
	}
	if len(compiled.CondensedTools) > 0 {
		fmt.Println("Condensed:")
		for _, tool := range compiled.CondensedTools {
			fmt.Printf("  • %-30s %s\n", tool.Name, tool.Condensed)
		}
	}
	if len(compiled.MinimalTools) > 0 {
		fmt.Printf("Minimal: %s\n", strings.Join(compiled.MinimalTools, ", "))
	}
	if compiled.Stats.SelectedTools == 0 {
		fmt.Println("No tool cleared the relevance floor for this shard.")
	}
	return nil
}

// assertMCPSelectContext supplies the intent facts the boost rules join on, so
// --verb and --target actually change the answer.
func assertMCPSelectContext(adapter *cliMCPKernel) error {
	if mcpSelectVerb == "" && mcpSelectTarget == "" {
		return nil
	}
	verb := mcpSelectVerb
	if verb == "" {
		verb = "analyze"
	}
	if !strings.HasPrefix(verb, "/") {
		verb = "/" + verb
	}
	const intentID = "cli_mcp_select"
	facts := []string{
		fmt.Sprintf("current_intent(%q)", intentID),
		fmt.Sprintf("user_intent(%q, /query, %s, %q, %q)", intentID, verb, mcpSelectTarget, "none"),
	}
	for _, fact := range facts {
		if err := adapter.Assert(fact); err != nil {
			return fmt.Errorf("failed to assert selection context: %w", err)
		}
	}
	return nil
}

func selectionPathLabel(path string) string {
	switch path {
	case mcp.SelectionPathMangle:
		return "Mangle kernel (policy_mcp.mg section 50)"
	case mcp.SelectionPathFallback:
		return "Go fallback heuristic (kernel derived nothing)"
	default:
		return "n/a (empty catalog)"
	}
}

func runMCPMetrics(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	store, err := openMCPStore()
	if err != nil {
		return err
	}
	defer store.Close()

	metrics, err := mcp.CollectMetrics(ctx, store, nil)
	if err != nil {
		return err
	}

	if mcpMetricsPrometheus {
		fmt.Print(metrics.RenderPrometheus())
		return nil
	}

	fmt.Println("📊 MCP Tool Metrics")
	fmt.Println(strings.Repeat("─", 72))
	if len(metrics.Tools) == 0 {
		fmt.Println("No tools in the catalog.")
		return nil
	}
	fmt.Printf("%-36s %8s %8s %10s\n", "TOOL", "CALLS", "FAILS", "AVG_MS")
	for _, tool := range metrics.Tools {
		fmt.Printf("%-36s %8d %8d %10d\n", tool.ToolID, tool.Calls, tool.Failures, tool.AvgLatencyMs)
	}
	fmt.Println(strings.Repeat("─", 72))
	fmt.Printf("%-36s %8d %8d\n", "TOTAL", metrics.TotalCalls, metrics.TotalFailures)
	if metrics.ToolsWithHistory == 0 {
		fmt.Println("\nNo tool has been invoked yet; the kernel's usage boost has nothing to score.")
	}
	return nil
}

// cliMCPKernel adapts core.RealKernel to mcp.KernelInterface for one-shot CLI
// use. It mirrors internal/system.mcpKernelAdapter; the CLI cannot reach that
// one because it is unexported and only built during a full Cortex boot.
type cliMCPKernel struct {
	kernel *core.RealKernel
}

func (k *cliMCPKernel) Assert(fact string) error {
	return k.kernel.AssertString(strings.TrimSuffix(strings.TrimSpace(fact), "."))
}

func (k *cliMCPKernel) Retract(fact string) error {
	parsed, err := core.ParseFactString(strings.TrimSuffix(strings.TrimSpace(fact), "."))
	if err != nil {
		return err
	}
	return k.kernel.RetractExactFactsBatch([]core.Fact{parsed})
}

func (k *cliMCPKernel) Query(query string) ([]map[string]any, error) {
	pattern, err := core.ParseFactString(query)
	if err != nil {
		return nil, fmt.Errorf("invalid query %q: %w", query, err)
	}
	variables := make(map[int]string)
	for i, arg := range pattern.Args {
		if s, ok := arg.(string); ok && strings.HasPrefix(s, "?") {
			variables[i] = s[1:]
		}
	}

	facts, err := k.kernel.Query(query)
	if err != nil {
		return nil, err
	}
	results := make([]map[string]any, 0, len(facts))
	for _, fact := range facts {
		binding := make(map[string]any, len(variables))
		for idx, name := range variables {
			if idx < len(fact.Args) {
				binding[name] = fact.Args[idx]
			}
		}
		results = append(results, binding)
	}
	return results, nil
}
