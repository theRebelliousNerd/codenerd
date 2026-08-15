// Package main implements the codeNERD CLI - a high-assurance, neuro-symbolic CLI agent.
//
// This file provides CLI commands for the Northstar system (project vision and requirements).
//
// Every read command here goes through loadAuthoritativeVision, which opens the
// Northstar knowledge store and reconciles it with .nerd/northstar.json before
// answering (see internal/northstar/bridge.go). Before that, these commands read
// the JSON file directly while the Guardian read SQLite, so `nerd northstar show`
// could describe a vision that /alignment and the campaign risk gate had never
// heard of. The CLI is now a view onto the same authority the kernel projects.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/northstar"

	"github.com/spf13/cobra"
)

// =============================================================================
// NORTHSTAR CLI COMMANDS
// =============================================================================

// northstarCmd is the parent command for northstar operations
var northstarCmd = &cobra.Command{
	Args:  cobra.NoArgs,
	Use:   "northstar",
	Short: "Project vision and requirements management",
	Long: `The Northstar system defines your project's vision, target users,
capabilities, risks, and requirements.

This information informs the Mangle kernel's reasoning and provides
strategic context for campaigns and shards.

The knowledge store (.nerd/northstar_knowledge.db) is the durable authority;
.nerd/northstar.json and .nerd/northstar.mg are import/export surfaces that are
reconciled with it on every read.

Examples:
  nerd northstar show              # Display current northstar definition
  nerd northstar summary           # One-page summary
  nerd northstar query mission     # Query specific element
  nerd northstar facts             # Show Mangle facts
  nerd northstar history           # Alignment check history
  nerd northstar drift             # Drift events
  nerd northstar state             # Guardian state and metrics
  nerd northstar sync              # Reconcile JSON <-> store explicitly
  nerd northstar export            # Export to various formats`,
	RunE: parentGroupRunE,
}

// northstarWorkspace resolves the workspace root for northstar commands.
func northstarWorkspace() string {
	ws := workspace
	if ws == "" {
		ws, _ = os.Getwd()
	}
	return ws
}

func northstarNerdDir() string {
	return filepath.Join(northstarWorkspace(), ".nerd")
}

// loadAuthoritativeVision returns the reconciled vision, or an actionable error.
//
// It deliberately does not fall back to reading northstar.json on its own: a
// silent JSON fallback is exactly the dual-authority behaviour this command set
// used to have. If the store cannot be opened the operator needs to know.
func loadAuthoritativeVision() (*northstar.Vision, error) {
	nerdDir := northstarNerdDir()
	store, err := northstar.NewStore(nerdDir)
	if err != nil {
		return nil, fmt.Errorf("open northstar store at %s: %w", nerdDir, err)
	}
	defer func() { _ = store.Close() }()

	if _, err := northstar.SyncVisionAuthority(store, nerdDir); err != nil {
		return nil, fmt.Errorf("reconcile vision: %w", err)
	}
	vision, err := store.LoadVision()
	if err != nil {
		return nil, fmt.Errorf("load vision: %w", err)
	}
	if vision == nil {
		return nil, fmt.Errorf("northstar not defined - run '/northstar' in interactive mode or 'nerd northstar load <file.json>'")
	}
	return vision, nil
}

// northstarShowCmd displays the current northstar definition
var northstarShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current northstar definition",
	Long:  `Shows the complete northstar definition including mission, vision, personas, capabilities, risks, requirements, and constraints.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := loadAuthoritativeVision()
		if err != nil {
			return err
		}

		fmt.Println("# Northstar Definition")
		fmt.Println()
		fmt.Println("## Mission")
		fmt.Printf("%s\n\n", v.Mission)

		fmt.Println("## Problem Statement")
		fmt.Printf("%s\n\n", v.Problem)

		fmt.Println("## Vision")
		fmt.Printf("%s\n\n", v.VisionStmt)

		if len(v.Personas) > 0 {
			fmt.Println("## Target Users")
			for i, p := range v.Personas {
				fmt.Printf("%d. **%s**\n", i+1, p.Name)
				if len(p.PainPoints) > 0 {
					fmt.Printf("   Pain Points: %s\n", strings.Join(p.PainPoints, ", "))
				}
				if len(p.Needs) > 0 {
					fmt.Printf("   Needs: %s\n", strings.Join(p.Needs, ", "))
				}
			}
			fmt.Println()
		}

		if len(v.Capabilities) > 0 {
			fmt.Println("## Capabilities")
			for i, c := range v.Capabilities {
				fmt.Printf("%d. [%s/%s] %s\n", i+1, c.Timeline, c.Priority, c.Description)
				if len(c.Serves) > 0 {
					fmt.Printf("   Serves: %s\n", strings.Join(c.Serves, ", "))
				}
			}
			fmt.Println()
		}

		if len(v.Risks) > 0 {
			fmt.Println("## Risks")
			for i, r := range v.Risks {
				fmt.Printf("%d. [%s/%s] %s\n", i+1, r.Likelihood, r.Impact, r.Description)
				if r.Mitigation != "" && r.Mitigation != "none" {
					fmt.Printf("   Mitigation: %s\n", r.Mitigation)
				}
			}
			fmt.Println()
		}

		if len(v.Requirements) > 0 {
			fmt.Println("## Requirements")
			for _, r := range v.Requirements {
				fmt.Printf("- [%s] %s: %s (%s)\n", r.ID, r.Type, r.Description, r.Priority)
				if len(r.Supports) > 0 {
					fmt.Printf("  Supports: %s\n", strings.Join(r.Supports, ", "))
				}
				if len(r.Addresses) > 0 {
					fmt.Printf("  Addresses: %s\n", strings.Join(r.Addresses, ", "))
				}
			}
			fmt.Println()
		}

		if len(v.Constraints) > 0 {
			fmt.Println("## Constraints")
			for i, c := range v.Constraints {
				fmt.Printf("%d. %s\n", i+1, c)
			}
			fmt.Println()
		}

		return nil
	},
}

// northstarSummaryCmd displays a one-page summary
var northstarSummaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "One-page northstar summary",
	Long:  `Displays a concise one-page summary of the northstar definition suitable for quick reference.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := loadAuthoritativeVision()
		if err != nil {
			return err
		}

		fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
		fmt.Println("║                     NORTHSTAR SUMMARY                            ║")
		fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
		fmt.Println()

		fmt.Printf("Mission: %s\n", v.Mission)
		fmt.Println(strings.Repeat("─", 70))

		fmt.Printf("Users: %d personas | Capabilities: %d | Risks: %d | Requirements: %d\n",
			len(v.Personas), len(v.Capabilities), len(v.Risks), len(v.Requirements))

		criticalCaps := 0
		for _, c := range v.Capabilities {
			if c.Priority == "critical" {
				criticalCaps++
			}
		}
		if criticalCaps > 0 {
			fmt.Printf("Critical Capabilities: %d\n", criticalCaps)
		}

		highRisks := 0
		for _, r := range v.Risks {
			if r.Impact == "high" {
				highRisks++
			}
		}
		if highRisks > 0 {
			fmt.Printf("High-Impact Risks: %d\n", highRisks)
		}

		mustHave := 0
		for _, r := range v.Requirements {
			if r.Priority == "must-have" || r.Priority == "must_have" {
				mustHave++
			}
		}
		if mustHave > 0 {
			fmt.Printf("Must-Have Requirements: %d\n", mustHave)
		}

		fmt.Println()
		fmt.Println("Run 'nerd northstar show' for full details.")

		return nil
	},
}

// northstarQueryCmd queries specific northstar elements
var northstarQueryCmd = &cobra.Command{
	Use:   "query [element]",
	Short: "Query specific northstar element",
	Long: `Query a specific element of the northstar definition.

Elements: mission, vision, problem, personas, capabilities, risks, requirements, constraints`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		element := strings.ToLower(args[0])

		v, err := loadAuthoritativeVision()
		if err != nil {
			return err
		}

		switch element {
		case "mission":
			fmt.Println(v.Mission)
		case "vision":
			fmt.Println(v.VisionStmt)
		case "problem":
			fmt.Println(v.Problem)
		case "personas", "users":
			for _, p := range v.Personas {
				fmt.Printf("%s: %s\n", p.Name, strings.Join(p.Needs, ", "))
			}
		case "capabilities", "caps":
			for _, c := range v.Capabilities {
				fmt.Printf("[%s/%s] %s\n", c.Timeline, c.Priority, c.Description)
			}
		case "risks":
			for _, r := range v.Risks {
				fmt.Printf("[%s/%s] %s\n", r.Likelihood, r.Impact, r.Description)
			}
		case "requirements", "reqs":
			for _, r := range v.Requirements {
				fmt.Printf("[%s] %s: %s\n", r.ID, r.Type, r.Description)
			}
		case "constraints":
			for _, c := range v.Constraints {
				fmt.Println(c)
			}
		default:
			return fmt.Errorf("unknown element: %s (try: mission, vision, problem, personas, capabilities, risks, requirements, constraints)", element)
		}

		return nil
	},
}

// northstarFactsCmd displays Mangle facts
var northstarFactsCmd = &cobra.Command{
	Use:   "facts",
	Short: "Show Mangle facts for northstar",
	Long: `Displays the Mangle facts generated from the northstar definition that are
used by the kernel for reasoning.

The facts are rendered from the authoritative vision via Vision.ToFacts, so this
is exactly what the Guardian asserts into the kernel - not whatever a stale
.nerd/northstar.mg happens to contain.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := loadAuthoritativeVision()
		if err != nil {
			return err
		}
		for _, line := range northstar.FactStrings(v) {
			fmt.Println(line)
		}
		return nil
	},
}

// northstarExportCmd exports northstar to various formats
var northstarExportCmd = &cobra.Command{
	Use:   "export [format]",
	Short: "Export northstar to various formats",
	Long: `Export the northstar definition to different formats.

Formats:
  json     - JSON format (default)
  markdown - Markdown document
  mangle   - Mangle facts`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		format := "json"
		if len(args) > 0 {
			format = strings.ToLower(args[0])
		}

		v, err := loadAuthoritativeVision()
		if err != nil {
			return err
		}

		switch format {
		case "json":
			data, err := json.MarshalIndent(northstar.WizardDocumentFromVision(v), "", "  ")
			if err != nil {
				return fmt.Errorf("marshal northstar: %w", err)
			}
			fmt.Println(string(data))

		case "markdown", "md":
			fmt.Print(generateNorthstarMarkdown(v))

		case "mangle", "mg":
			fmt.Print(northstar.RenderVisionMangle(v))

		default:
			return fmt.Errorf("unknown format: %s (try: json, markdown, mangle)", format)
		}

		return nil
	},
}

// northstarStatsCmd shows northstar statistics
var northstarStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show northstar statistics",
	Long:  `Displays statistics about the northstar definition including counts and coverage.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		v, err := loadAuthoritativeVision()
		if err != nil {
			return err
		}

		fmt.Println("Northstar Statistics")
		fmt.Println(strings.Repeat("─", 40))

		fmt.Printf("Mission defined:      %v\n", v.Mission != "")
		fmt.Printf("Vision defined:       %v\n", v.VisionStmt != "")
		fmt.Printf("Problem defined:      %v\n", v.Problem != "")
		fmt.Printf("User Personas:        %d\n", len(v.Personas))
		fmt.Printf("Capabilities:         %d\n", len(v.Capabilities))
		fmt.Printf("Risks:                %d\n", len(v.Risks))
		fmt.Printf("Requirements:         %d\n", len(v.Requirements))
		fmt.Printf("Constraints:          %d\n", len(v.Constraints))
		fmt.Printf("Mangle facts:         %d\n", len(v.ToFacts()))

		if len(v.Capabilities) > 0 {
			fmt.Println()
			fmt.Println("Capabilities by Priority:")
			capsByPriority := map[string]int{}
			for _, c := range v.Capabilities {
				capsByPriority[c.Priority]++
			}
			for _, p := range sortedKeys(capsByPriority) {
				fmt.Printf("  %s: %d\n", p, capsByPriority[p])
			}
		}

		if len(v.Requirements) > 0 {
			fmt.Println()
			fmt.Println("Requirements by Type:")
			reqsByType := map[string]int{}
			for _, r := range v.Requirements {
				reqsByType[r.Type]++
			}
			for _, t := range sortedKeys(reqsByType) {
				fmt.Printf("  %s: %d\n", t, reqsByType[t])
			}
		}
		return nil
	},
}

// =============================================================================
// SQLITE-BACKED OPERATOR COMMANDS
// =============================================================================

var (
	northstarHistoryLimit int
	northstarDriftLimit   int
)

// northstarHistoryCmd shows recorded alignment checks.
var northstarHistoryCmd = &cobra.Command{
	Use:   "history",
	Short: "Show alignment check history",
	Long: `Lists the alignment checks recorded in the Northstar knowledge store,
newest first. These are the checks the Guardian, /alignment, and the campaign
observer actually performed.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		nerdDir := northstarNerdDir()
		store, err := northstar.NewStore(nerdDir)
		if err != nil {
			return fmt.Errorf("open northstar store at %s: %w", nerdDir, err)
		}
		defer func() { _ = store.Close() }()

		checks, err := store.GetAlignmentHistory(northstarHistoryLimit)
		if err != nil {
			return fmt.Errorf("read alignment history: %w", err)
		}
		if len(checks) == 0 {
			fmt.Println("No alignment checks recorded yet.")
			fmt.Println("Run '/alignment' in interactive mode or start a campaign to generate them.")
			return nil
		}

		fmt.Printf("Alignment history (%d most recent)\n", len(checks))
		fmt.Println(strings.Repeat("─", 78))
		for _, c := range checks {
			fmt.Printf("%s  %-8s score=%.2f  %-14s %s\n",
				c.Timestamp.Format("2006-01-02 15:04:05"),
				c.Result, c.Score, c.Trigger, truncateForCLI(c.Subject, 30))
			if c.Explanation != "" {
				fmt.Printf("    %s\n", truncateForCLI(c.Explanation, 200))
			}
			for _, s := range c.Suggestions {
				fmt.Printf("    → %s\n", truncateForCLI(s, 150))
			}
		}
		return nil
	},
}

var northstarDriftAll bool

// northstarDriftCmd shows drift events.
var northstarDriftCmd = &cobra.Command{
	Use:   "drift",
	Short: "Show vision drift events",
	Long: `Lists drift events recorded when an alignment check came back failed or
blocked. By default only unresolved drift is shown; --all includes resolved
events with their resolutions.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		nerdDir := northstarNerdDir()
		store, err := northstar.NewStore(nerdDir)
		if err != nil {
			return fmt.Errorf("open northstar store at %s: %w", nerdDir, err)
		}
		defer func() { _ = store.Close() }()

		var events []northstar.DriftEvent
		if northstarDriftAll {
			events, err = store.GetDriftHistory(northstarDriftLimit)
		} else {
			events, err = store.GetActiveDriftEvents()
		}
		if err != nil {
			return fmt.Errorf("read drift events: %w", err)
		}

		if len(events) == 0 {
			if northstarDriftAll {
				fmt.Println("No drift events recorded.")
			} else {
				fmt.Println("No unresolved drift. (Use --all to include resolved events.)")
			}
			return nil
		}

		fmt.Printf("Drift events (%d)\n", len(events))
		fmt.Println(strings.Repeat("─", 78))
		for _, e := range events {
			status := "OPEN"
			if e.Resolved {
				status = "RESOLVED"
			}
			fmt.Printf("%s  [%s] %-8s %s\n",
				e.Timestamp.Format("2006-01-02 15:04:05"), status, e.Severity, e.Category)
			if e.Description != "" {
				fmt.Printf("    %s\n", truncateForCLI(e.Description, 200))
			}
			for _, ev := range e.Evidence {
				fmt.Printf("    evidence: %s\n", truncateForCLI(ev, 150))
			}
			if e.Resolved && e.Resolution != "" {
				fmt.Printf("    resolution: %s\n", truncateForCLI(e.Resolution, 150))
			}
		}
		return nil
	},
}

// northstarStateCmd shows guardian state and alignment metrics.
var northstarStateCmd = &cobra.Command{
	Use:   "state",
	Short: "Show Guardian state and alignment metrics",
	Long: `Reports the Guardian rollup held in the knowledge store: whether a vision
is defined, the running alignment average, open drift, and the aggregate
metrics (total checks, blocked rate, mean score).`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		nerdDir := northstarNerdDir()
		store, err := northstar.NewStore(nerdDir)
		if err != nil {
			return fmt.Errorf("open northstar store at %s: %w", nerdDir, err)
		}
		defer func() { _ = store.Close() }()

		state, err := store.GetState()
		if err != nil {
			return fmt.Errorf("read guardian state: %w", err)
		}
		metrics, err := store.GetMetrics()
		if err != nil {
			return fmt.Errorf("read alignment metrics: %w", err)
		}

		fmt.Println("Northstar Guardian State")
		fmt.Println(strings.Repeat("─", 46))
		fmt.Printf("Store:                %s\n", store.Path())
		fmt.Printf("Vision defined:       %v\n", state.VisionDefined)
		if !state.LastCheck.IsZero() {
			fmt.Printf("Last check:           %s\n", state.LastCheck.Format("2006-01-02 15:04:05"))
		} else {
			fmt.Printf("Last check:           never\n")
		}
		fmt.Printf("Tasks since check:    %d\n", state.TasksSinceCheck)
		fmt.Printf("Session observations: %d\n", state.SessionObservations)
		fmt.Println()

		fmt.Println("Alignment metrics")
		fmt.Println(strings.Repeat("─", 46))
		fmt.Printf("Total checks:         %d\n", metrics.TotalChecks)
		fmt.Printf("Mean score:           %.3f\n", metrics.MeanScore)
		fmt.Printf("Overall alignment:    %.3f\n", metrics.OverallAlignment)
		fmt.Printf("Blocked rate:         %.1f%%\n", metrics.BlockedRate*100)
		fmt.Printf("Failed rate:          %.1f%%\n", metrics.FailedRate*100)
		fmt.Printf("Active drift:         %d\n", metrics.ActiveDrift)
		fmt.Printf("Resolved drift:       %d\n", metrics.ResolvedDrift)
		fmt.Printf("Ingested docs:        %d\n", metrics.IngestedDocs)

		if metrics.TotalChecks > 0 {
			fmt.Println()
			fmt.Println("Checks by result")
			for _, r := range metrics.SortedResults() {
				fmt.Printf("  %-8s %d\n", r, metrics.ChecksByResult[r])
			}
		}
		return nil
	},
}

// northstarSyncCmd runs the JSON <-> store reconciliation explicitly.
var northstarSyncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Reconcile .nerd/northstar.json with the knowledge store",
	Long: `Runs the vision authority reconciliation that every Guardian boot performs,
and reports which direction it moved.

The store is the durable authority; northstar.json and northstar.mg are import
and export surfaces. When both hold a vision and they differ, the newer of the
two wins (file mtime versus the store's updated_at); ties go to the store.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		nerdDir := northstarNerdDir()
		store, err := northstar.NewStore(nerdDir)
		if err != nil {
			return fmt.Errorf("open northstar store at %s: %w", nerdDir, err)
		}
		defer func() { _ = store.Close() }()

		result, err := northstar.SyncVisionAuthority(store, nerdDir)
		if err != nil {
			return fmt.Errorf("reconcile vision: %w", err)
		}

		switch result.Direction {
		case northstar.SyncImported:
			fmt.Printf("Imported %s into %s\n", result.JSONPath, store.Path())
		case northstar.SyncExported:
			fmt.Printf("Exported %s to %s and %s\n", store.Path(), result.JSONPath, result.ManglePat)
		default:
			if result.Vision == nil {
				fmt.Println("No vision defined in either surface - nothing to reconcile.")
				return nil
			}
			fmt.Println("Already in sync.")
		}
		if result.Vision != nil {
			fmt.Printf("Mission: %s\n", result.Vision.Mission)
			fmt.Printf("Facts:   %d\n", len(result.Vision.ToFacts()))
		}
		return nil
	},
}

// northstarLoadCmd loads a northstar definition from a JSON file
var northstarLoadCmd = &cobra.Command{
	Use:   "load <path>",
	Short: "Load northstar definition from a JSON file",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", args[0], err)
		}

		var doc northstar.WizardDocument
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("failed to parse %s: %w", args[0], err)
		}

		vision := doc.ToVision()
		if vision == nil || vision.Mission == "" {
			return fmt.Errorf("invalid northstar: Mission must not be empty")
		}

		nerdDir := northstarNerdDir()
		if err := os.MkdirAll(nerdDir, 0755); err != nil {
			return fmt.Errorf("failed to create .nerd directory: %w", err)
		}

		// Store first: it is the authority, and the JSON/.mg surfaces are
		// derived from what it accepted. Writing the files first would leave a
		// visible vision the kernel never received if the store write failed.
		store, err := northstar.NewStore(nerdDir)
		if err != nil {
			return fmt.Errorf("open northstar store: %w", err)
		}
		defer func() { _ = store.Close() }()

		if err := store.SaveVision(vision); err != nil {
			return fmt.Errorf("save vision to store: %w", err)
		}
		fmt.Printf("Wrote %s\n", store.Path())

		if _, err := northstar.WriteVisionJSON(nerdDir, vision); err != nil {
			return fmt.Errorf("export vision JSON (store write succeeded): %w", err)
		}
		fmt.Printf("Wrote %s\n", filepath.Join(nerdDir, northstar.VisionJSONFileName))

		if err := northstar.WriteVisionMangle(nerdDir, vision); err != nil {
			return fmt.Errorf("export vision facts (store write succeeded): %w", err)
		}
		fmt.Printf("Wrote %s\n", filepath.Join(nerdDir, northstar.VisionMangleFileName))

		fmt.Printf("Mission: %s\n", vision.Mission)
		fmt.Printf("User Personas:        %d\n", len(vision.Personas))
		fmt.Printf("Capabilities:         %d\n", len(vision.Capabilities))
		fmt.Printf("Risks:                %d\n", len(vision.Risks))
		fmt.Printf("Requirements:         %d\n", len(vision.Requirements))
		fmt.Printf("Constraints:          %d\n", len(vision.Constraints))
		fmt.Printf("Representations succeeded: store, json, mangle\n")

		return nil
	},
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func sortedKeys(m map[string]int) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func truncateForCLI(s string, maxLen int) string {
	s = strings.ReplaceAll(strings.TrimSpace(s), "\n", " ")
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

// generateNorthstarMarkdown creates a markdown document from the vision.
func generateNorthstarMarkdown(v *northstar.Vision) string {
	var sb strings.Builder

	sb.WriteString("# Project Northstar\n\n")

	sb.WriteString("## Mission\n\n")
	sb.WriteString(v.Mission + "\n\n")

	sb.WriteString("## Problem Statement\n\n")
	sb.WriteString(v.Problem + "\n\n")

	sb.WriteString("## Vision\n\n")
	sb.WriteString(v.VisionStmt + "\n\n")

	if len(v.Personas) > 0 {
		sb.WriteString("## Target Users\n\n")
		for _, p := range v.Personas {
			sb.WriteString(fmt.Sprintf("### %s\n\n", p.Name))
			if len(p.PainPoints) > 0 {
				sb.WriteString("**Pain Points:**\n")
				for _, pp := range p.PainPoints {
					sb.WriteString(fmt.Sprintf("- %s\n", pp))
				}
				sb.WriteString("\n")
			}
			if len(p.Needs) > 0 {
				sb.WriteString("**Needs:**\n")
				for _, n := range p.Needs {
					sb.WriteString(fmt.Sprintf("- %s\n", n))
				}
				sb.WriteString("\n")
			}
		}
	}

	if len(v.Capabilities) > 0 {
		sb.WriteString("## Capabilities\n\n")
		sb.WriteString("| ID | Description | Timeline | Priority | Serves |\n")
		sb.WriteString("|----|-------------|----------|----------|--------|\n")
		for _, c := range v.Capabilities {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				c.ID, c.Description, c.Timeline, c.Priority, strings.Join(c.Serves, ", ")))
		}
		sb.WriteString("\n")
	}

	if len(v.Risks) > 0 {
		sb.WriteString("## Risks\n\n")
		sb.WriteString("| ID | Description | Likelihood | Impact | Mitigation |\n")
		sb.WriteString("|----|-------------|------------|--------|------------|\n")
		for _, r := range v.Risks {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				r.ID, r.Description, r.Likelihood, r.Impact, r.Mitigation))
		}
		sb.WriteString("\n")
	}

	if len(v.Requirements) > 0 {
		sb.WriteString("## Requirements\n\n")
		sb.WriteString("| ID | Type | Description | Priority | Supports | Addresses |\n")
		sb.WriteString("|----|------|-------------|----------|----------|-----------|\n")
		for _, r := range v.Requirements {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s | %s |\n",
				r.ID, r.Type, r.Description, r.Priority,
				strings.Join(r.Supports, ", "), strings.Join(r.Addresses, ", ")))
		}
		sb.WriteString("\n")
	}

	if len(v.Constraints) > 0 {
		sb.WriteString("## Constraints\n\n")
		for _, c := range v.Constraints {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

func init() {
	northstarHistoryCmd.Flags().IntVar(&northstarHistoryLimit, "limit", 20, "maximum number of records to show")
	northstarDriftCmd.Flags().IntVar(&northstarDriftLimit, "limit", 20, "maximum number of records to show (with --all)")
	northstarDriftCmd.Flags().BoolVar(&northstarDriftAll, "all", false, "include resolved drift events")

	// Add subcommands
	northstarCmd.AddCommand(
		northstarShowCmd,
		northstarSummaryCmd,
		northstarQueryCmd,
		northstarFactsCmd,
		northstarExportCmd,
		northstarStatsCmd,
		northstarLoadCmd,
		northstarHistoryCmd,
		northstarDriftCmd,
		northstarStateCmd,
		northstarSyncCmd,
	)
}
