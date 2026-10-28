// Package init implements the "nerd init" cold-start initialization system.
// This file adds deep strategic knowledge generation using LLM analysis.
package init

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"

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

// strategicConcepts are the knowledge.db concepts PersistStrategicKnowledge
// owns. Each write replaces all of them, so the set always describes one
// derivation. strategic/source/* belongs to persistOrientationKnowledge.
var strategicConcepts = []string{
	"strategic/vision", "strategic/philosophy", "strategic/architecture_style",
	"strategic/data_flow", "strategic/communication", "strategic/principle",
	"strategic/component", "strategic/pattern", "strategic/capability",
	"strategic/extension_point", "strategic/safety_constraint", "strategic/limitation",
	"strategic/learning", "strategic/future", "strategic/full_knowledge",
}

// knowledgeAtom is one atom PersistStrategicKnowledge writes.
type knowledgeAtom struct {
	concept    string
	content    string
	confidence float64
}

// atoms renders the knowledge as strategic atoms. Empty values and the
// profile's "unknown" placeholder are not knowledge and are not written.
func (knowledge *StrategicKnowledge) atoms() ([]knowledgeAtom, error) {
	var out []knowledgeAtom
	add := func(concept, content string, confidence float64) {
		content = strings.TrimSpace(content)
		if content == "" || strings.EqualFold(content, "unknown") {
			return
		}
		out = append(out, knowledgeAtom{concept: concept, content: content, confidence: confidence})
	}
	add("strategic/vision", knowledge.ProjectVision, 1.0)
	add("strategic/philosophy", knowledge.CorePhilosophy, 1.0)
	add("strategic/architecture_style", knowledge.ArchitectureStyle, 0.95)
	add("strategic/data_flow", knowledge.DataFlowPattern, 0.95)
	add("strategic/communication", knowledge.CommunicationFlow, 0.95)
	for _, principle := range knowledge.DesignPrinciples {
		add("strategic/principle", principle, 0.9)
	}
	for _, comp := range knowledge.KeyComponents {
		add("strategic/component", fmt.Sprintf("%s: %s (location: %s, interfaces: %s)",
			comp.Name, comp.Purpose, comp.Location, comp.Interfaces), 0.9)
	}
	for _, pattern := range knowledge.CorePatterns {
		add("strategic/pattern", fmt.Sprintf("%s: %s. Used in: %s. Why: %s",
			pattern.Name, pattern.Description, pattern.UsedIn, pattern.Why), 0.9)
	}
	for _, capability := range knowledge.CoreCapabilities {
		add("strategic/capability", capability, 0.85)
	}
	for _, ext := range knowledge.ExtensionPoints {
		add("strategic/extension_point", ext, 0.85)
	}
	for _, constraint := range knowledge.SafetyConstraints {
		add("strategic/safety_constraint", constraint, 0.95)
	}
	for _, limit := range knowledge.Limitations {
		add("strategic/limitation", limit, 0.8)
	}
	for _, mech := range knowledge.LearningMechanisms {
		add("strategic/learning", mech, 0.85)
	}
	for _, dir := range knowledge.FutureDirections {
		add("strategic/future", dir, 0.7)
	}
	if len(out) == 0 {
		return nil, nil
	}
	full, err := json.MarshalIndent(knowledge, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode strategic knowledge: %w", err)
	}
	add("strategic/full_knowledge", string(full), 1.0)
	return out, nil
}

// PersistStrategicKnowledge replaces the strategic atoms in knowledge.db with
// knowledge, embedding each for semantic search, and returns how many it
// stored. The atoms already stored are compared first: an identical set is
// left alone rather than re-embedded. Knowledge with nothing in it writes
// nothing and keeps what is stored.
func (i *Initializer) PersistStrategicKnowledge(ctx context.Context, knowledge *StrategicKnowledge, db *store.LocalStore) (int, error) {
	if db == nil {
		return 0, fmt.Errorf("persist strategic knowledge: no knowledge database")
	}
	if knowledge == nil {
		return 0, nil
	}
	atoms, err := knowledge.atoms()
	if err != nil {
		return 0, err
	}
	if len(atoms) == 0 {
		return 0, nil
	}
	existing, err := db.GetKnowledgeAtomsByPrefix("strategic/")
	if err != nil {
		return 0, fmt.Errorf("read stored strategic knowledge: %w", err)
	}
	if sameAtomSet(atoms, ownedStrategicAtoms(existing)) {
		return len(atoms), nil
	}
	if _, err := db.DeleteKnowledgeAtomsByConcept(strategicConcepts...); err != nil {
		return 0, fmt.Errorf("replace strategic knowledge: %w", err)
	}
	stored := 0
	var errs []error
	for _, atom := range atoms {
		if err := db.StoreKnowledgeAtomWithEmbedding(ctx, atom.concept, atom.content, atom.confidence); err != nil {
			errs = append(errs, fmt.Errorf("store %s: %w", atom.concept, err))
			continue
		}
		stored++
	}
	if len(errs) > 0 {
		return stored, fmt.Errorf("strategic knowledge partly stored (%d/%d): %w", stored, len(atoms), errors.Join(errs...))
	}
	return stored, nil
}

// ownedStrategicAtoms keeps the stored atoms PersistStrategicKnowledge owns.
func ownedStrategicAtoms(stored []store.KnowledgeAtom) []knowledgeAtom {
	var out []knowledgeAtom
	for _, atom := range stored {
		if slices.Contains(strategicConcepts, atom.Concept) {
			out = append(out, knowledgeAtom{concept: atom.Concept, content: atom.Content})
		}
	}
	return out
}

// sameAtomSet compares two atom lists as multisets of concept and content.
func sameAtomSet(a, b []knowledgeAtom) bool {
	if len(a) != len(b) {
		return false
	}
	key := func(atoms []knowledgeAtom) []string {
		keys := make([]string, len(atoms))
		for index, atom := range atoms {
			keys[index] = atom.concept + "\x00" + atom.content
		}
		sort.Strings(keys)
		return keys
	}
	return slices.Equal(key(a), key(b))
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

// keysFromMap extracts keys from a map for display.
func keysFromMap(m map[string][]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
