// Package init implements the "nerd init" cold-start initialization system.
// This file adds deep strategic knowledge generation using LLM analysis.
package init

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
	"codenerd/internal/world"
)

// StrategicKnowledge represents deep philosophical and architectural understanding
// of a codebase - the "soul" of the project that the main agent uses for reasoning.
type StrategicKnowledge struct {
	// Identity - What is this project at its core?
	ProjectVision     string   `json:"project_vision"`      // The "why" - purpose and goals
	CorePhilosophy    string   `json:"core_philosophy"`     // Guiding principles
	DesignPrinciples  []string `json:"design_principles"`   // Key architectural decisions

	// Architecture - How is it built?
	ArchitectureStyle string            `json:"architecture_style"`  // e.g., "neuro-symbolic", "microservices"
	KeyComponents     []ComponentInfo   `json:"key_components"`      // Major subsystems
	DataFlowPattern   string            `json:"data_flow_pattern"`   // How data moves through the system

	// Patterns - What patterns does it use?
	CorePatterns      []PatternInfo     `json:"core_patterns"`       // Key design patterns
	CommunicationFlow string            `json:"communication_flow"`  // How components communicate

	// Capabilities - What can it do?
	CoreCapabilities  []string          `json:"core_capabilities"`   // Main features
	ExtensionPoints   []string          `json:"extension_points"`    // Where it can be extended

	// Constraints - What are its boundaries?
	SafetyConstraints []string          `json:"safety_constraints"`  // Safety invariants
	Limitations       []string          `json:"limitations"`         // Known limitations

	// Evolution - How does it grow?
	LearningMechanisms []string         `json:"learning_mechanisms"` // How it adapts
	FutureDirections   []string         `json:"future_directions"`   // Planned evolution
}

// ComponentInfo describes a major subsystem.
type ComponentInfo struct {
	Name        string `json:"name"`
	Purpose     string `json:"purpose"`
	Location    string `json:"location"`     // Directory or package
	Interfaces  string `json:"interfaces"`   // How it exposes functionality
	DependsOn   []string `json:"depends_on"` // What it needs
}

// PatternInfo describes a design pattern used in the codebase.
type PatternInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	UsedIn      string `json:"used_in"`      // Where it's applied
	Why         string `json:"why"`          // Why this pattern was chosen
}

// generateStrategicKnowledge uses LLM to analyze the codebase deeply.
func (i *Initializer) generateStrategicKnowledge(ctx context.Context, profile ProjectProfile, scanResult *world.ScanResult) (*StrategicKnowledge, error) {
	if i.config.LLMClient == nil {
		return nil, fmt.Errorf("LLM client required for strategic knowledge generation")
	}

	// Gather context about the project
	codebaseContext := i.buildCodebaseContext(profile, scanResult)

	// Check for CLAUDE.md files which often contain architectural documentation
	claudeMDContent := i.findClaudeMDContent()
	if claudeMDContent != "" {
		codebaseContext += "\n\n## Existing Documentation (CLAUDE.md):\n" + claudeMDContent
	}

	prompt := fmt.Sprintf(`You are analyzing a software project to generate deep strategic knowledge.
This knowledge will be used by an AI coding agent to understand the project at a philosophical and architectural level.

## Project Context:
%s

## Task:
Generate a comprehensive strategic analysis of this codebase. Focus on:
1. The project's PURPOSE and PHILOSOPHY - why does it exist? what problem does it solve?
2. The ARCHITECTURE - how are the major components organized? what patterns are used?
3. The DATA FLOW - how does information move through the system?
4. The EXTENSION POINTS - where can the system be extended?
5. The SAFETY CONSTRAINTS - what invariants must be maintained?

Respond with a JSON object matching this structure:
{
  "project_vision": "string - the core purpose and goal of this project",
  "core_philosophy": "string - the guiding principles (e.g., 'Logic determines Reality; the Model merely describes it')",
  "design_principles": ["principle 1", "principle 2", ...],
  "architecture_style": "string - e.g., 'neuro-symbolic', 'microservices', 'monolith'",
  "key_components": [
    {"name": "Component", "purpose": "what it does", "location": "path", "interfaces": "how to use it", "depends_on": ["dep1"]}
  ],
  "data_flow_pattern": "string - how data flows through the system",
  "core_patterns": [
    {"name": "Pattern", "description": "what it is", "used_in": "where", "why": "why chosen"}
  ],
  "communication_flow": "string - how components communicate",
  "core_capabilities": ["capability 1", "capability 2", ...],
  "extension_points": ["extension 1", "extension 2", ...],
  "safety_constraints": ["constraint 1", "constraint 2", ...],
  "limitations": ["limitation 1", ...],
  "learning_mechanisms": ["mechanism 1", ...],
  "future_directions": ["direction 1", ...]
}

IMPORTANT: Be specific to THIS project, not generic. Extract real insights from the codebase structure.
`, codebaseContext)

	response, err := i.config.LLMClient.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("LLM analysis failed: %w", err)
	}

	// Parse JSON from response
	knowledge := &StrategicKnowledge{}

	// Extract JSON from response (may be wrapped in markdown code blocks)
	jsonStr := extractJSON(response)
	if err := json.Unmarshal([]byte(jsonStr), knowledge); err != nil {
		// If parsing fails, create minimal knowledge from profile
		logging.Get(logging.CategoryBoot).Warn("Failed to parse strategic knowledge JSON, using fallback: %v", err)
		knowledge = i.createFallbackStrategicKnowledge(profile)
	}

	return knowledge, nil
}

// buildCodebaseContext creates a rich context string for LLM analysis.
func (i *Initializer) buildCodebaseContext(profile ProjectProfile, scanResult *world.ScanResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Project: %s\n", profile.Name))
	sb.WriteString(fmt.Sprintf("Description: %s\n", profile.Description))
	sb.WriteString(fmt.Sprintf("Language: %s\n", profile.Language))
	if profile.Framework != "" {
		sb.WriteString(fmt.Sprintf("Framework: %s\n", profile.Framework))
	}
	if profile.Architecture != "" {
		sb.WriteString(fmt.Sprintf("Architecture: %s\n", profile.Architecture))
	}

	// Add directory structure (extract from facts)
	sb.WriteString("\n## Directory Structure:\n")
	if scanResult != nil && len(scanResult.Facts) > 0 {
		dirs := extractDirectoriesFromFacts(scanResult.Facts)
		for _, dir := range dirs[:min(30, len(dirs))] {
			sb.WriteString(fmt.Sprintf("- %s\n", dir))
		}
	}

	// Add entry points
	if len(profile.EntryPoints) > 0 {
		sb.WriteString("\n## Entry Points:\n")
		for _, ep := range profile.EntryPoints {
			sb.WriteString(fmt.Sprintf("- %s\n", ep))
		}
	}

	// Add dependencies
	if len(profile.Dependencies) > 0 {
		sb.WriteString("\n## Key Dependencies:\n")
		for _, dep := range profile.Dependencies[:min(20, len(profile.Dependencies))] {
			sb.WriteString(fmt.Sprintf("- %s (%s)\n", dep.Name, dep.Type))
		}
	}

	// Add patterns if detected
	if len(profile.Patterns) > 0 {
		sb.WriteString("\n## Detected Patterns:\n")
		for _, pattern := range profile.Patterns {
			sb.WriteString(fmt.Sprintf("- %s\n", pattern))
		}
	}

	return sb.String()
}

// findAllMarkdownContent scans for all markdown files in the workspace.
// Prioritizes CLAUDE.md and README.md, then includes other docs.
func (i *Initializer) findClaudeMDContent() string {
	var sb strings.Builder
	totalChars := 0
	const maxTotalChars = 50000 // Cap total markdown content

	// Priority files to read first (in order)
	priorityFiles := []string{
		filepath.Join(i.config.Workspace, "CLAUDE.md"),
		filepath.Join(i.config.Workspace, ".claude", "CLAUDE.md"),
		filepath.Join(i.config.Workspace, "README.md"),
		filepath.Join(i.config.Workspace, "ARCHITECTURE.md"),
		filepath.Join(i.config.Workspace, "CONTRIBUTING.md"),
		filepath.Join(i.config.Workspace, "DESIGN.md"),
	}

	seen := make(map[string]bool)

	// Read priority files first
	for _, path := range priorityFiles {
		if totalChars >= maxTotalChars {
			break
		}
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}
		if seen[absPath] {
			continue
		}
		if content, err := os.ReadFile(path); err == nil {
			seen[absPath] = true
			filename := filepath.Base(path)
			sb.WriteString(fmt.Sprintf("\n### %s\n\n", filename))
			s := string(content)
			if totalChars+len(s) > maxTotalChars {
				s = s[:maxTotalChars-totalChars] + "\n...[truncated]"
			}
			sb.WriteString(s)
			sb.WriteString("\n")
			totalChars += len(s)
		}
	}

	// Scan for additional markdown files (up to 3 levels deep)
	additionalDirs := []string{
		i.config.Workspace,
		filepath.Join(i.config.Workspace, "docs"),
		filepath.Join(i.config.Workspace, "doc"),
		filepath.Join(i.config.Workspace, ".github"),
	}

	for _, dir := range additionalDirs {
		if totalChars >= maxTotalChars {
			break
		}
		i.scanMarkdownDir(dir, seen, &sb, &totalChars, maxTotalChars, 0, 2)
	}

	result := sb.String()
	if result == "" {
		return ""
	}

	logging.Get(logging.CategoryBoot).Debug("Found %d chars of markdown documentation", totalChars)
	return result
}

// scanMarkdownDir recursively scans a directory for markdown files.
func (i *Initializer) scanMarkdownDir(dir string, seen map[string]bool, sb *strings.Builder, totalChars *int, maxChars int, depth int, maxDepth int) {
	if depth > maxDepth || *totalChars >= maxChars {
		return
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if *totalChars >= maxChars {
			break
		}

		path := filepath.Join(dir, entry.Name())
		absPath, err := filepath.Abs(path)
		if err != nil {
			continue
		}

		if entry.IsDir() {
			// Skip common non-doc directories
			name := entry.Name()
			if name == "node_modules" || name == "vendor" || name == ".git" ||
				name == "dist" || name == "build" || name == "__pycache__" ||
				name == ".nerd" || name == "target" {
				continue
			}
			i.scanMarkdownDir(path, seen, sb, totalChars, maxChars, depth+1, maxDepth)
		} else if strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			if seen[absPath] {
				continue
			}
			content, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			seen[absPath] = true

			// Get relative path for display
			relPath, _ := filepath.Rel(i.config.Workspace, path)
			if relPath == "" {
				relPath = entry.Name()
			}

			sb.WriteString(fmt.Sprintf("\n### %s\n\n", relPath))
			s := string(content)
			if *totalChars+len(s) > maxChars {
				s = s[:maxChars-*totalChars] + "\n...[truncated]"
			}
			sb.WriteString(s)
			sb.WriteString("\n")
			*totalChars += len(s)
		}
	}
}

// createFallbackStrategicKnowledge creates minimal knowledge when LLM fails.
func (i *Initializer) createFallbackStrategicKnowledge(profile ProjectProfile) *StrategicKnowledge {
	return &StrategicKnowledge{
		ProjectVision:    profile.Description,
		CorePhilosophy:   fmt.Sprintf("A %s project built with %s.", profile.Language, profile.Framework),
		DesignPrinciples: profile.Patterns,
		ArchitectureStyle: profile.Architecture,
		KeyComponents:    []ComponentInfo{},
		DataFlowPattern:  "Standard request-response flow",
		CorePatterns:     []PatternInfo{},
		CoreCapabilities: []string{},
		SafetyConstraints: []string{},
		Limitations:      []string{},
	}
}

// persistStrategicKnowledge saves the knowledge to the main knowledge.db.
func (i *Initializer) persistStrategicKnowledge(ctx context.Context, knowledge *StrategicKnowledge, db *store.LocalStore) (int, error) {
	atomCount := 0

	// Helper to store with error handling
	storeAtom := func(concept, content string, confidence float64) {
		if content == "" {
			return
		}
		if err := db.StoreKnowledgeAtom(concept, content, confidence); err == nil {
			atomCount++
		} else {
			logging.Get(logging.CategoryBoot).Debug("Failed to store atom %s: %v", concept, err)
		}
	}

	// Store core identity (highest confidence)
	storeAtom("strategic/vision", knowledge.ProjectVision, 1.0)
	storeAtom("strategic/philosophy", knowledge.CorePhilosophy, 1.0)
	storeAtom("strategic/architecture_style", knowledge.ArchitectureStyle, 0.95)
	storeAtom("strategic/data_flow", knowledge.DataFlowPattern, 0.95)
	storeAtom("strategic/communication", knowledge.CommunicationFlow, 0.95)

	// Store design principles
	for _, principle := range knowledge.DesignPrinciples {
		storeAtom("strategic/principle", principle, 0.9)
	}

	// Store components
	for _, comp := range knowledge.KeyComponents {
		content := fmt.Sprintf("%s: %s (location: %s, interfaces: %s)",
			comp.Name, comp.Purpose, comp.Location, comp.Interfaces)
		storeAtom("strategic/component", content, 0.9)
	}

	// Store patterns
	for _, pattern := range knowledge.CorePatterns {
		content := fmt.Sprintf("%s: %s. Used in: %s. Why: %s",
			pattern.Name, pattern.Description, pattern.UsedIn, pattern.Why)
		storeAtom("strategic/pattern", content, 0.9)
	}

	// Store capabilities
	for _, cap := range knowledge.CoreCapabilities {
		storeAtom("strategic/capability", cap, 0.85)
	}

	// Store extension points
	for _, ext := range knowledge.ExtensionPoints {
		storeAtom("strategic/extension_point", ext, 0.85)
	}

	// Store safety constraints (high confidence - these are critical)
	for _, constraint := range knowledge.SafetyConstraints {
		storeAtom("strategic/safety_constraint", constraint, 0.95)
	}

	// Store limitations
	for _, limit := range knowledge.Limitations {
		storeAtom("strategic/limitation", limit, 0.8)
	}

	// Store learning mechanisms
	for _, mech := range knowledge.LearningMechanisms {
		storeAtom("strategic/learning", mech, 0.85)
	}

	// Store future directions
	for _, dir := range knowledge.FutureDirections {
		storeAtom("strategic/future", dir, 0.7)
	}

	// Also persist as JSON for easy loading
	jsonBytes, _ := json.MarshalIndent(knowledge, "", "  ")
	storeAtom("strategic/full_knowledge", string(jsonBytes), 1.0)

	return atomCount, nil
}

// extractJSON extracts JSON from a string that might have markdown code blocks.
func extractJSON(s string) string {
	// Try to find JSON in code blocks first
	if idx := strings.Index(s, "```json"); idx != -1 {
		start := idx + 7
		if end := strings.Index(s[start:], "```"); end != -1 {
			return strings.TrimSpace(s[start : start+end])
		}
	}
	if idx := strings.Index(s, "```"); idx != -1 {
		start := idx + 3
		// Skip optional language identifier
		if nlIdx := strings.Index(s[start:], "\n"); nlIdx != -1 {
			start += nlIdx + 1
		}
		if end := strings.Index(s[start:], "```"); end != -1 {
			return strings.TrimSpace(s[start : start+end])
		}
	}

	// Try to find raw JSON object
	if start := strings.Index(s, "{"); start != -1 {
		depth := 0
		for i := start; i < len(s); i++ {
			switch s[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return s[start : i+1]
				}
			}
		}
	}

	return s
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// extractDirectoriesFromFacts extracts directory paths from file_topology facts.
func extractDirectoriesFromFacts(facts []core.Fact) []string {
	seen := make(map[string]bool)
	var dirs []string

	for _, f := range facts {
		if f.Predicate == "file_topology" && len(f.Args) >= 2 {
			// file_topology(path, type) where type is /directory
			if typeArg, ok := f.Args[1].(string); ok && typeArg == "/directory" {
				if path, ok := f.Args[0].(string); ok && !seen[path] {
					seen[path] = true
					dirs = append(dirs, path)
				}
			}
		}
	}
	return dirs
}

// truncateString truncates a string to maxLen characters, adding "..." if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}
