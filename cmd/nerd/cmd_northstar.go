// Package main implements the codeNERD CLI - a high-assurance, neuro-symbolic CLI agent.
//
// This file provides CLI commands for the Northstar system (project vision and requirements).
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

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

Examples:
  nerd northstar show              # Display current northstar definition
  nerd northstar summary           # One-page summary
  nerd northstar query mission     # Query specific element
  nerd northstar facts             # Show Mangle facts
  nerd northstar export            # Export to various formats`,
	RunE: parentGroupRunE,
}

// northstarShowCmd displays the current northstar definition
var northstarShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current northstar definition",
	Long:  `Shows the complete northstar definition including mission, vision, personas, capabilities, risks, requirements, and constraints.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ws := workspace
		if ws == "" {
			ws, _ = os.Getwd()
		}

		// Try to load from JSON (more detailed)
		jsonPath := filepath.Join(ws, ".nerd", "northstar.json")
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			return fmt.Errorf("northstar not defined - run '/northstar' in interactive mode or 'nerd northstar wizard'")
		}

		var ns NorthstarState
		if err := json.Unmarshal(data, &ns); err != nil {
			return fmt.Errorf("failed to parse northstar.json: %w", err)
		}

		// Render to stdout
		fmt.Println("# Northstar Definition")
		fmt.Println()
		fmt.Println("## Mission")
		fmt.Printf("%s\n\n", ns.Mission)

		fmt.Println("## Problem Statement")
		fmt.Printf("%s\n\n", ns.Problem)

		fmt.Println("## Vision")
		fmt.Printf("%s\n\n", ns.Vision)

		if len(ns.Personas) > 0 {
			fmt.Println("## Target Users")
			for i, p := range ns.Personas {
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

		if len(ns.Capabilities) > 0 {
			fmt.Println("## Capabilities")
			for i, c := range ns.Capabilities {
				fmt.Printf("%d. [%s/%s] %s\n", i+1, c.Timeline, c.Priority, c.Description)
			}
			fmt.Println()
		}

		if len(ns.Risks) > 0 {
			fmt.Println("## Risks")
			for i, r := range ns.Risks {
				fmt.Printf("%d. [%s/%s] %s\n", i+1, r.Likelihood, r.Impact, r.Description)
				if r.Mitigation != "" && r.Mitigation != "none" {
					fmt.Printf("   Mitigation: %s\n", r.Mitigation)
				}
			}
			fmt.Println()
		}

		if len(ns.Requirements) > 0 {
			fmt.Println("## Requirements")
			for _, r := range ns.Requirements {
				fmt.Printf("- [%s] %s: %s (%s)\n", r.ID, r.Type, r.Description, r.Priority)
			}
			fmt.Println()
		}

		if len(ns.Constraints) > 0 {
			fmt.Println("## Constraints")
			for i, c := range ns.Constraints {
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
		ws := workspace
		if ws == "" {
			ws, _ = os.Getwd()
		}

		jsonPath := filepath.Join(ws, ".nerd", "northstar.json")
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			return fmt.Errorf("northstar not defined - run '/northstar' in interactive mode")
		}

		var ns NorthstarState
		if err := json.Unmarshal(data, &ns); err != nil {
			return fmt.Errorf("failed to parse northstar.json: %w", err)
		}

		fmt.Println("╔══════════════════════════════════════════════════════════════════╗")
		fmt.Println("║                     NORTHSTAR SUMMARY                            ║")
		fmt.Println("╚══════════════════════════════════════════════════════════════════╝")
		fmt.Println()

		fmt.Printf("Mission: %s\n", ns.Mission)
		fmt.Println(strings.Repeat("─", 70))

		fmt.Printf("Users: %d personas | Capabilities: %d | Risks: %d | Requirements: %d\n",
			len(ns.Personas), len(ns.Capabilities), len(ns.Risks), len(ns.Requirements))

		// Critical capabilities
		criticalCaps := 0
		for _, c := range ns.Capabilities {
			if c.Priority == "critical" {
				criticalCaps++
			}
		}
		if criticalCaps > 0 {
			fmt.Printf("Critical Capabilities: %d\n", criticalCaps)
		}

		// High-impact risks
		highRisks := 0
		for _, r := range ns.Risks {
			if r.Impact == "high" {
				highRisks++
			}
		}
		if highRisks > 0 {
			fmt.Printf("High-Impact Risks: %d\n", highRisks)
		}

		// Must-have requirements
		mustHave := 0
		for _, r := range ns.Requirements {
			if r.Priority == "must-have" {
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

		ws := workspace
		if ws == "" {
			ws, _ = os.Getwd()
		}

		jsonPath := filepath.Join(ws, ".nerd", "northstar.json")
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			return fmt.Errorf("northstar not defined")
		}

		var ns NorthstarState
		if err := json.Unmarshal(data, &ns); err != nil {
			return fmt.Errorf("failed to parse northstar.json: %w", err)
		}

		switch element {
		case "mission":
			fmt.Println(ns.Mission)
		case "vision":
			fmt.Println(ns.Vision)
		case "problem":
			fmt.Println(ns.Problem)
		case "personas", "users":
			for _, p := range ns.Personas {
				fmt.Printf("%s: %s\n", p.Name, strings.Join(p.Needs, ", "))
			}
		case "capabilities", "caps":
			for _, c := range ns.Capabilities {
				fmt.Printf("[%s/%s] %s\n", c.Timeline, c.Priority, c.Description)
			}
		case "risks":
			for _, r := range ns.Risks {
				fmt.Printf("[%s/%s] %s\n", r.Likelihood, r.Impact, r.Description)
			}
		case "requirements", "reqs":
			for _, r := range ns.Requirements {
				fmt.Printf("[%s] %s: %s\n", r.ID, r.Type, r.Description)
			}
		case "constraints":
			for _, c := range ns.Constraints {
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
	Long:  `Displays the Mangle facts generated from the northstar definition that are used by the kernel for reasoning.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ws := workspace
		if ws == "" {
			ws, _ = os.Getwd()
		}

		mgPath := filepath.Join(ws, ".nerd", "northstar.mg")
		data, err := os.ReadFile(mgPath)
		if err != nil {
			return fmt.Errorf("northstar.mg not found - run '/northstar' in interactive mode")
		}

		fmt.Print(string(data))
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

		ws := workspace
		if ws == "" {
			ws, _ = os.Getwd()
		}

		switch format {
		case "json":
			jsonPath := filepath.Join(ws, ".nerd", "northstar.json")
			data, err := os.ReadFile(jsonPath)
			if err != nil {
				return fmt.Errorf("northstar.json not found")
			}
			fmt.Print(string(data))

		case "markdown", "md":
			jsonPath := filepath.Join(ws, ".nerd", "northstar.json")
			data, err := os.ReadFile(jsonPath)
			if err != nil {
				return fmt.Errorf("northstar.json not found")
			}
			var ns NorthstarState
			if err := json.Unmarshal(data, &ns); err != nil {
				return fmt.Errorf("failed to parse: %w", err)
			}
			fmt.Print(generateNorthstarMarkdown(&ns))

		case "mangle", "mg":
			mgPath := filepath.Join(ws, ".nerd", "northstar.mg")
			data, err := os.ReadFile(mgPath)
			if err != nil {
				return fmt.Errorf("northstar.mg not found")
			}
			fmt.Print(string(data))

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
		ws := workspace
		if ws == "" {
			ws, _ = os.Getwd()
		}

		jsonPath := filepath.Join(ws, ".nerd", "northstar.json")
		data, err := os.ReadFile(jsonPath)
		if err != nil {
			return fmt.Errorf("northstar not defined")
		}

		var ns NorthstarState
		if err := json.Unmarshal(data, &ns); err != nil {
			return fmt.Errorf("failed to parse: %w", err)
		}

		fmt.Println("Northstar Statistics")
		fmt.Println(strings.Repeat("─", 40))

		fmt.Printf("Mission defined:      %v\n", ns.Mission != "")
		fmt.Printf("Vision defined:       %v\n", ns.Vision != "")
		fmt.Printf("Problem defined:      %v\n", ns.Problem != "")
		fmt.Printf("User Personas:        %d\n", len(ns.Personas))
		fmt.Printf("Capabilities:         %d\n", len(ns.Capabilities))
		fmt.Printf("Risks:                %d\n", len(ns.Risks))
		fmt.Printf("Requirements:         %d\n", len(ns.Requirements))
		fmt.Printf("Constraints:          %d\n", len(ns.Constraints))
		fmt.Printf("Research Documents:   %d\n", len(ns.ResearchDocs))
		fmt.Printf("Extracted Facts:      %d\n", len(ns.ExtractedFacts))

		// Capability breakdown by priority
		if len(ns.Capabilities) > 0 {
			fmt.Println()
			fmt.Println("Capabilities by Priority:")
			capsByPriority := map[string]int{}
			for _, c := range ns.Capabilities {
				capsByPriority[c.Priority]++
			}
			for p, count := range capsByPriority {
				fmt.Printf("  %s: %d\n", p, count)
			}
		}

		// Requirement breakdown by type
		if len(ns.Requirements) > 0 {
			fmt.Println()
			fmt.Println("Requirements by Type:")
			reqsByType := map[string]int{}
			for _, r := range ns.Requirements {
				reqsByType[r.Type]++
			}
			for t, count := range reqsByType {
				fmt.Printf("  %s: %d\n", t, count)
			}
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

		var ns NorthstarState
		if err := json.Unmarshal(data, &ns); err != nil {
			return fmt.Errorf("failed to parse %s: %w", args[0], err)
		}

		if ns.Mission == "" {
			return fmt.Errorf("invalid northstar: Mission must not be empty")
		}

		ws := workspace
		if ws == "" {
			ws, _ = os.Getwd()
		}

		nerdDir := filepath.Join(ws, ".nerd")
		if err := os.MkdirAll(nerdDir, 0755); err != nil {
			return fmt.Errorf("failed to create .nerd directory: %w", err)
		}

		// Prepare all representations before writing (parse and validate first, then write).
		out, err := json.MarshalIndent(ns, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal northstar: %w", err)
		}
		mgContent := generateNorthstarMangleFromState(&ns)
		vision := northstarStateToVision(&ns)

		jsonPath := filepath.Join(ws, ".nerd", "northstar.json")
		if err := os.WriteFile(jsonPath, out, 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", jsonPath, err)
		}
		fmt.Printf("Wrote %s\n", jsonPath)

		mgPath := filepath.Join(ws, ".nerd", "northstar.mg")
		if err := os.WriteFile(mgPath, []byte(mgContent), 0644); err != nil {
			return fmt.Errorf("failed to write %s: %w", mgPath, err)
		}
		fmt.Printf("Wrote %s\n", mgPath)

		// Store representation - report clearly on failure but leave JSON and .mg in place.
		store, err := northstar.NewStore(nerdDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to open northstar store: %v\n", err)
			fmt.Printf("Representations succeeded: json, mangle; store failed: %v\n", err)
			fmt.Printf("Mission: %s\n", ns.Mission)
			fmt.Printf("User Personas:        %d\n", len(ns.Personas))
			fmt.Printf("Capabilities:         %d\n", len(ns.Capabilities))
			fmt.Printf("Risks:                %d\n", len(ns.Risks))
			fmt.Printf("Requirements:         %d\n", len(ns.Requirements))
			fmt.Printf("Constraints:          %d\n", len(ns.Constraints))
			return fmt.Errorf("store write failed (json and mangle succeeded): %w", err)
		}
		defer store.Close()
		if err := store.SaveVision(vision); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to save vision to store: %v\n", err)
			fmt.Printf("Representations succeeded: json, mangle; store failed: %v\n", err)
			fmt.Printf("Mission: %s\n", ns.Mission)
			fmt.Printf("User Personas:        %d\n", len(ns.Personas))
			fmt.Printf("Capabilities:         %d\n", len(ns.Capabilities))
			fmt.Printf("Risks:                %d\n", len(ns.Risks))
			fmt.Printf("Requirements:         %d\n", len(ns.Requirements))
			fmt.Printf("Constraints:          %d\n", len(ns.Constraints))
			return fmt.Errorf("store write failed (json and mangle succeeded): %w", err)
		}
		fmt.Printf("Wrote %s\n", store.Path())

		fmt.Printf("Mission: %s\n", ns.Mission)
		fmt.Printf("User Personas:        %d\n", len(ns.Personas))
		fmt.Printf("Capabilities:         %d\n", len(ns.Capabilities))
		fmt.Printf("Risks:                %d\n", len(ns.Risks))
		fmt.Printf("Requirements:         %d\n", len(ns.Requirements))
		fmt.Printf("Constraints:          %d\n", len(ns.Constraints))
		fmt.Printf("Representations succeeded: json, mangle, store\n")

		return nil
	},
}

// =============================================================================
// TYPE DEFINITIONS (mirror chat package types)
// =============================================================================

// NorthstarState mirrors the NorthstarWizardState from chat package
type NorthstarState struct {
	Mission        string                 `json:"Mission"`
	Problem        string                 `json:"Problem"`
	Vision         string                 `json:"Vision"`
	Personas       []NorthstarPersona     `json:"Personas"`
	Capabilities   []NorthstarCapability  `json:"Capabilities"`
	Risks          []NorthstarRisk        `json:"Risks"`
	Requirements   []NorthstarRequirement `json:"Requirements"`
	Constraints    []string               `json:"Constraints"`
	ResearchDocs   []string               `json:"ResearchDocs"`
	ExtractedFacts []string               `json:"ExtractedFacts"`
}

// NorthstarPersona mirrors UserPersona
type NorthstarPersona struct {
	Name       string   `json:"name"`
	PainPoints []string `json:"pain_points"`
	Needs      []string `json:"needs"`
}

// NorthstarCapability mirrors Capability
type NorthstarCapability struct {
	Description string `json:"description"`
	Timeline    string `json:"timeline"`
	Priority    string `json:"priority"`
}

// NorthstarRisk mirrors Risk
type NorthstarRisk struct {
	Description string `json:"description"`
	Likelihood  string `json:"likelihood"`
	Impact      string `json:"impact"`
	Mitigation  string `json:"mitigation"`
}

// NorthstarRequirement mirrors NorthstarRequirement
type NorthstarRequirement struct {
	ID          string `json:"id"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Source      string `json:"source"`
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

// generateNorthstarMarkdown creates a markdown document from northstar state
func generateNorthstarMarkdown(ns *NorthstarState) string {
	var sb strings.Builder

	sb.WriteString("# Project Northstar\n\n")

	sb.WriteString("## Mission\n\n")
	sb.WriteString(ns.Mission + "\n\n")

	sb.WriteString("## Problem Statement\n\n")
	sb.WriteString(ns.Problem + "\n\n")

	sb.WriteString("## Vision\n\n")
	sb.WriteString(ns.Vision + "\n\n")

	if len(ns.Personas) > 0 {
		sb.WriteString("## Target Users\n\n")
		for _, p := range ns.Personas {
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

	if len(ns.Capabilities) > 0 {
		sb.WriteString("## Capabilities\n\n")
		sb.WriteString("| Description | Timeline | Priority |\n")
		sb.WriteString("|-------------|----------|----------|\n")
		for _, c := range ns.Capabilities {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", c.Description, c.Timeline, c.Priority))
		}
		sb.WriteString("\n")
	}

	if len(ns.Risks) > 0 {
		sb.WriteString("## Risks\n\n")
		sb.WriteString("| Description | Likelihood | Impact | Mitigation |\n")
		sb.WriteString("|-------------|------------|--------|------------|\n")
		for _, r := range ns.Risks {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", r.Description, r.Likelihood, r.Impact, r.Mitigation))
		}
		sb.WriteString("\n")
	}

	if len(ns.Requirements) > 0 {
		sb.WriteString("## Requirements\n\n")
		sb.WriteString("| ID | Type | Description | Priority |\n")
		sb.WriteString("|----|------|-------------|----------|\n")
		for _, r := range ns.Requirements {
			sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n", r.ID, r.Type, r.Description, r.Priority))
		}
		sb.WriteString("\n")
	}

	if len(ns.Constraints) > 0 {
		sb.WriteString("## Constraints\n\n")
		for _, c := range ns.Constraints {
			sb.WriteString(fmt.Sprintf("- %s\n", c))
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// generateNorthstarMangleFromState generates Mangle facts from NorthstarState.
// Equivalent to chat.generateNorthstarMangle but operating on the CLI's NorthstarState type.
func generateNorthstarMangleFromState(ns *NorthstarState) string {
	var sb strings.Builder

	sb.WriteString("# Northstar Vision Facts\n")
	sb.WriteString(fmt.Sprintf("# Generated: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString("# This file defines the project's north star and informs kernel reasoning.\n")
	sb.WriteString("# Schema declarations are in internal/core/defaults/schemas.mg\n\n")

	// Core Vision Facts - IDs are /string per Decl, use "global" as canonical ID (see internal/northstar/types.go ToFacts)
	sb.WriteString("# Core Vision Facts\n")
	sb.WriteString(fmt.Sprintf("northstar_mission(%q, %q).\n", "global", ns.Mission))
	sb.WriteString(fmt.Sprintf("northstar_problem(%q, %q).\n", "global", ns.Problem))
	sb.WriteString(fmt.Sprintf("northstar_vision(%q, %q).\n", "global", ns.Vision))
	sb.WriteString("\n")

	// Personas
	if len(ns.Personas) > 0 {
		sb.WriteString("# User Personas\n")
		for i, p := range ns.Personas {
			personaID := fmt.Sprintf("persona_%d", i+1)
			sb.WriteString(fmt.Sprintf("northstar_persona(%q, %q).\n", personaID, p.Name))
			for _, pain := range p.PainPoints {
				sb.WriteString(fmt.Sprintf("northstar_pain_point(%q, %q).\n", personaID, pain))
			}
			for _, need := range p.Needs {
				sb.WriteString(fmt.Sprintf("northstar_need(%q, %q).\n", personaID, need))
			}
		}
		sb.WriteString("\n")
	}

	// Capabilities - Decl bound [/string, /string, /name, /number]: ID string, Desc string, Timeline /name, Priority /number
	if len(ns.Capabilities) > 0 {
		sb.WriteString("# Capabilities\n")
		for i, c := range ns.Capabilities {
			capID := fmt.Sprintf("cap_%d", i+1)
			timeline := strings.ToLower(strings.ReplaceAll(c.Timeline, " ", "_"))
			priority := northstarPriorityToNumber(c.Priority)
			sb.WriteString(fmt.Sprintf("northstar_capability(%q, %q, /%s, %d).\n",
				capID, c.Description, timeline, priority))
		}
		sb.WriteString("\n")
	}

	// Risks - Decl bound [/string, /string, /name, /number]: ID string, Desc string, Likelihood /name, Impact /number
	if len(ns.Risks) > 0 {
		sb.WriteString("# Risks\n")
		for i, r := range ns.Risks {
			riskID := fmt.Sprintf("risk_%d", i+1)
			likelihood := strings.ToLower(r.Likelihood)
			impact := northstarRiskImpactToNumber(r.Impact)
			sb.WriteString(fmt.Sprintf("northstar_risk(%q, %q, /%s, %d).\n",
				riskID, r.Description, likelihood, impact))
			if r.Mitigation != "" && strings.ToLower(r.Mitigation) != "none" {
				// Decl northstar_mitigation(RiskID, Strategy) bound [/string, /name] - Strategy is /name atom
				sb.WriteString(fmt.Sprintf("northstar_mitigation(%q, /mitigation).\n", riskID))
			}
		}
		sb.WriteString("\n")
	}

	// Requirements - Decl bound [/string, /name, /string, /number]: ReqID string, Type /name, Description string, Priority /number
	if len(ns.Requirements) > 0 {
		sb.WriteString("# Requirements\n")
		for _, r := range ns.Requirements {
			reqID := strings.ToLower(r.ID)
			reqType := strings.ToLower(strings.ReplaceAll(r.Type, "-", "_"))
			reqType = strings.ReplaceAll(reqType, " ", "_")
			priority := northstarPriorityToNumber(r.Priority)
			sb.WriteString(fmt.Sprintf("northstar_requirement(%q, /%s, %q, %d).\n",
				reqID, reqType, r.Description, priority))
		}
		sb.WriteString("\n")
	}

	// Constraints - Decl bound [/string, /string]
	if len(ns.Constraints) > 0 {
		sb.WriteString("# Constraints\n")
		for i, c := range ns.Constraints {
			constraintID := fmt.Sprintf("constraint_%d", i+1)
			sb.WriteString(fmt.Sprintf("northstar_constraint(%q, %q).\n", constraintID, c))
		}
		sb.WriteString("\n")
	}

	sb.WriteString("northstar_defined().\n")

	return sb.String()
}

func northstarPriorityToNumber(p string) int {
	switch strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(p, "-", "_"), " ", "_")) {
	case "critical", "must_have":
		return 100
	case "high", "should_have":
		return 80
	case "medium":
		return 50
	case "low", "nice_to_have":
		return 20
	default:
		return 50
	}
}

func northstarRiskImpactToNumber(impact string) int {
	switch strings.ToLower(strings.TrimSpace(impact)) {
	case "high":
		return 100
	case "medium":
		return 50
	case "low":
		return 20
	default:
		return 50
	}
}

// northstarStateToVision converts a NorthstarState into a northstar.Vision for store persistence.
func northstarStateToVision(ns *NorthstarState) *northstar.Vision {
	v := &northstar.Vision{
		Mission:    ns.Mission,
		Problem:    ns.Problem,
		VisionStmt: ns.Vision,
		Constraints: ns.Constraints,
	}
	if ns.Constraints == nil {
		v.Constraints = []string{}
	}
	// Personas
	for _, p := range ns.Personas {
		v.Personas = append(v.Personas, northstar.Persona{
			Name:       p.Name,
			PainPoints: p.PainPoints,
			Needs:      p.Needs,
		})
	}
	if v.Personas == nil {
		v.Personas = []northstar.Persona{}
	}
	// Capabilities with generated IDs
	for i, c := range ns.Capabilities {
		v.Capabilities = append(v.Capabilities, northstar.Capability{
			ID:          fmt.Sprintf("cap_%d", i+1),
			Description: c.Description,
			Timeline:    c.Timeline,
			Priority:    c.Priority,
		})
	}
	if v.Capabilities == nil {
		v.Capabilities = []northstar.Capability{}
	}
	// Risks with generated IDs
	for i, r := range ns.Risks {
		v.Risks = append(v.Risks, northstar.Risk{
			ID:          fmt.Sprintf("risk_%d", i+1),
			Description: r.Description,
			Likelihood:  r.Likelihood,
			Impact:      r.Impact,
			Mitigation:  r.Mitigation,
		})
	}
	if v.Risks == nil {
		v.Risks = []northstar.Risk{}
	}
	// Requirements preserving original IDs lowercased
	for _, r := range ns.Requirements {
		v.Requirements = append(v.Requirements, northstar.Requirement{
			ID:          strings.ToLower(r.ID),
			Type:        r.Type,
			Description: r.Description,
			Priority:    r.Priority,
		})
	}
	if v.Requirements == nil {
		v.Requirements = []northstar.Requirement{}
	}
	return v
}

func init() {
	// Add subcommands
	northstarCmd.AddCommand(
		northstarShowCmd,
		northstarSummaryCmd,
		northstarQueryCmd,
		northstarFactsCmd,
		northstarExportCmd,
		northstarStatsCmd,
		northstarLoadCmd,
	)
}