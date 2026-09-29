// Package init implements the "nerd init" cold-start initialization system.
// This file adds deep strategic knowledge generation using LLM analysis.
package init

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/store"
)

// StrategicKnowledge represents deep philosophical and architectural understanding
// of a codebase - the "soul" of the project that the main agent uses for reasoning.
type StrategicKnowledge struct {
	// Identity - What is this project at its core?
	ProjectVision    string   `json:"project_vision"`    // The "why" - purpose and goals
	CorePhilosophy   string   `json:"core_philosophy"`   // Guiding principles
	DesignPrinciples []string `json:"design_principles"` // Key architectural decisions

	// Architecture - How is it built?
	ArchitectureStyle string          `json:"architecture_style"` // e.g., "neuro-symbolic", "microservices"
	KeyComponents     []ComponentInfo `json:"key_components"`     // Major subsystems
	DataFlowPattern   string          `json:"data_flow_pattern"`  // How data moves through the system

	// Patterns - What patterns does it use?
	CorePatterns      []PatternInfo `json:"core_patterns"`      // Key design patterns
	CommunicationFlow string        `json:"communication_flow"` // How components communicate

	// Capabilities - What can it do?
	CoreCapabilities []string `json:"core_capabilities"` // Main features
	ExtensionPoints  []string `json:"extension_points"`  // Where it can be extended

	// Constraints - What are its boundaries?
	SafetyConstraints []string `json:"safety_constraints"` // Safety invariants
	Limitations       []string `json:"limitations"`        // Known limitations

	// Evolution - How does it grow?
	LearningMechanisms []string `json:"learning_mechanisms"` // How it adapts
	FutureDirections   []string `json:"future_directions"`   // Planned evolution
}

// ComponentInfo describes a major subsystem.
type ComponentInfo struct {
	Name       string   `json:"name"`
	Purpose    string   `json:"purpose"`
	Location   string   `json:"location"`   // Directory or package
	Interfaces string   `json:"interfaces"` // How it exposes functionality
	DependsOn  []string `json:"depends_on"` // What it needs
}

// PatternInfo describes a design pattern used in the codebase.
type PatternInfo struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	UsedIn      string `json:"used_in"` // Where it's applied
	Why         string `json:"why"`     // Why this pattern was chosen
}

// PersistStrategicKnowledge saves the knowledge to the main knowledge.db.
// Uses embedding-enabled storage for semantic search capability.
func (i *Initializer) PersistStrategicKnowledge(ctx context.Context, knowledge *StrategicKnowledge, db *store.LocalStore) (int, error) {
	atomCount := 0

	// Helper to store with embedding for semantic search
	storeAtom := func(concept, content string, confidence float64) {
		if content == "" {
			return
		}
		if err := db.StoreKnowledgeAtomWithEmbedding(ctx, concept, content, confidence); err == nil {
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

	if value := extractBalancedJSON(s); value != "" {
		return value
	}
	return s
}

// extractBalancedJSON returns the first complete JSON object or array in s.
//
// The previous version only looked for '{' and counted braces without regard
// for string literals, which broke both callers of this function:
//
//   - Document extraction may return a JSON array of structured atoms. An
//     unfenced `[{...},{...}]` reply made this return just the first object,
//     the unmarshal into []RelevanceResult failed, and the whole batch silently
//     fell back to the priority heuristic — so on any provider that answers
//     without a code fence, LLM document filtering never actually ran.
//   - Structured knowledge can contain prose-valued JSON objects. A
//     '}' inside any string value (a Mangle snippet, a brace in a description)
//     closed the object early and the parse failed, discarding the analysis in
//     favour of the profile-only fallback.
func extractBalancedJSON(s string) string {
	start := -1
	var openCh, closeCh byte
	for idx := 0; idx < len(s); idx++ {
		if s[idx] == '{' {
			start, openCh, closeCh = idx, '{', '}'
			break
		}
		if s[idx] == '[' {
			start, openCh, closeCh = idx, '[', ']'
			break
		}
	}
	if start == -1 {
		return ""
	}

	depth := 0
	inString := false
	escaped := false
	for idx := start; idx < len(s); idx++ {
		c := s[idx]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case openCh:
			depth++
		case closeCh:
			depth--
			if depth == 0 {
				return s[start : idx+1]
			}
		}
	}
	return ""
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

// keysFromMap extracts keys from a map for display.
func keysFromMap(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
