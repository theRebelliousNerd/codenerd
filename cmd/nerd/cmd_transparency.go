// Package main implements transparency and introspection CLI commands for codeNERD.
// This file handles glassbox, transparency reports, and reflection status.
package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
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

	key := resolveAPIKey(apiKey, workspace)

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

	// Predicates that actually carry facts.
	//
	// This used to sample route_to, shard_routing, tool_invocation and
	// file_state — four names with no producer anywhere in the repo, so every
	// line read "0 facts" on every run and the command looked like a report on
	// an idle system rather than a report on nothing.
	//
	// The two groups below are kept apart deliberately: world facts are loaded
	// from .nerd/mangle on boot and are present in any process, while session
	// facts are asserted during a turn and die with it. A CLI invocation shows
	// zero for the session group by construction, and saying so is the whole
	// point of a glass box.
	fmt.Println("  Persistent (loaded from .nerd/mangle at boot):")
	for _, pred := range []string{"file_topology", "symbol_graph", "dependency_link", "tool_registered"} {
		facts, _ := cortex.Kernel.Query(pred)
		fmt.Printf("     %-20s %d facts\n", pred+":", len(facts))
	}

	fmt.Println("\n  Session-scoped (asserted during a turn, empty in a fresh process):")
	for _, pred := range []string{"user_intent", "next_action", "pending_action", "permitted"} {
		facts, _ := cortex.Kernel.Query(pred)
		fmt.Printf("     %-20s %d facts\n", pred+":", len(facts))
	}

	if path, err := logging.LatestAuditLogPath(); err == nil {
		fmt.Printf("\n  Durable decision record: %s\n", path)
		fmt.Println("  Run 'nerd transparency' to read it.")
	} else {
		fmt.Println("\n  No audit log (audit logging requires logging.debug_mode).")
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

	key := resolveAPIKey(apiKey, workspace)

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

	// Read the durable record, not the kernel.
	//
	// This used to query route_to and tool_invocation, neither of which is
	// produced anywhere in the repo, so both sections printed "none recorded"
	// on every run since the commands were written. Pointing them at the
	// predicates that DO record decisions would not have helped either:
	// user_intent, next_action and permitted are session-scoped and die with
	// the process, so a fresh CLI invocation sees an empty kernel by
	// construction. The audit log is the record that survives.
	path, err := logging.LatestAuditLogPath()
	if err != nil {
		fmt.Println("\n⚠️  No audit log available.")
		fmt.Println("   Audit logging is gated on debug mode. Set logging.debug_mode")
		fmt.Println("   in .nerd/config.json to record decisions.")
		fmt.Println(strings.Repeat("═", 60))
		return nil
	}

	fmt.Printf("\nSource: %s\n", path)

	counts, err := logging.CountAuditEventTypes(path)
	if err != nil {
		return fmt.Errorf("failed to read audit log: %w", err)
	}

	printAuditSection("📍 Recent Shard Routing", path, counts, []logging.AuditEventType{
		logging.AuditShardSpawn, logging.AuditShardExecute, logging.AuditShardComplete,
	})
	printAuditSection("🧭 Recent Action Routing", path, counts, []logging.AuditEventType{
		logging.AuditActionRoute, logging.AuditActionExecute, logging.AuditActionComplete,
	})
	printAuditSection("🔧 Recent Tool Invocations", path, counts, []logging.AuditEventType{
		logging.AuditToolInvoke, logging.AuditToolComplete, logging.AuditToolError,
	})

	fmt.Println(strings.Repeat("═", 60))
	return nil
}

// printAuditSection renders one event family, distinguishing "nothing happened"
// from "nothing records this". Several declared families — action_route,
// tool_invoke, file_write — have no production call site, and reporting those
// as "none recorded" invites the reader to conclude the system sat idle.
func printAuditSection(title, path string, counts map[logging.AuditEventType]int, types []logging.AuditEventType) {
	fmt.Printf("\n%s\n", title)
	fmt.Println(strings.Repeat("─", 40))

	total := 0
	for _, t := range types {
		total += counts[t]
	}
	if total == 0 {
		names := make([]string, len(types))
		for i, t := range types {
			names[i] = string(t)
		}
		fmt.Printf("  Not instrumented — no %s events have ever been written.\n", strings.Join(names, "/"))
		return
	}

	events, err := logging.ReadRecentAuditEvents(path, types, 5)
	if err != nil {
		fmt.Printf("  Failed to read: %v\n", err)
		return
	}

	for _, e := range events {
		fmt.Printf("  %s\n", formatAuditEvent(e))
	}
	fmt.Printf("  (%d total today)\n", total)
}

// formatAuditEvent renders one event on a single line, preferring the fields
// that identify what was decided over the free-text message.
func formatAuditEvent(e logging.AuditEvent) string {
	var sb strings.Builder

	sb.WriteString(time.UnixMilli(e.Timestamp).Format("15:04:05"))
	sb.WriteString(" ")
	sb.WriteString(string(e.EventType))

	if e.ShardID != "" {
		sb.WriteString(" shard=" + e.ShardID)
	}
	if e.Target != "" {
		sb.WriteString(" target=" + e.Target)
	}
	if e.Action != "" {
		sb.WriteString(" action=" + e.Action)
	}
	if e.DurationMs > 0 {
		sb.WriteString(fmt.Sprintf(" %dms", e.DurationMs))
	}
	if !e.Success {
		sb.WriteString(" FAILED")
		if e.Error != "" {
			sb.WriteString(": " + e.Error)
		}
	}

	return sb.String()
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
