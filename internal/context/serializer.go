package context

import (
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

// =============================================================================
// Fact Serialization
// =============================================================================
// Serializes facts to Mangle notation for LLM context injection.

// FactSerializer handles serialization of facts to various formats.
type FactSerializer struct {
	// Options
	includeComments bool
	maxLineLength   int
	groupByPredicate bool
}

// NewFactSerializer creates a new serializer with default options.
func NewFactSerializer() *FactSerializer {
	return &FactSerializer{
		includeComments: true,
		maxLineLength:   120,
		groupByPredicate: true,
	}
}

// WithComments enables/disables comment generation.
func (fs *FactSerializer) WithComments(include bool) *FactSerializer {
	fs.includeComments = include
	return fs
}

// WithGrouping enables/disables grouping facts by predicate.
func (fs *FactSerializer) WithGrouping(group bool) *FactSerializer {
	fs.groupByPredicate = group
	return fs
}

// SerializeFacts converts a slice of facts to Mangle notation.
func (fs *FactSerializer) SerializeFacts(facts []core.Fact) string {
	if len(facts) == 0 {
		return ""
	}

	if fs.groupByPredicate {
		return fs.serializeGrouped(facts)
	}
	return fs.serializeFlat(facts)
}

// serializeFlat serializes facts in order without grouping.
func (fs *FactSerializer) serializeFlat(facts []core.Fact) string {
	var sb strings.Builder
	for _, f := range facts {
		sb.WriteString(f.String())
		sb.WriteString("\n")
	}
	return sb.String()
}

// serializeGrouped serializes facts grouped by predicate.
func (fs *FactSerializer) serializeGrouped(facts []core.Fact) string {
	// Group by predicate
	groups := make(map[string][]core.Fact)
	var predicateOrder []string

	for _, f := range facts {
		if _, exists := groups[f.Predicate]; !exists {
			predicateOrder = append(predicateOrder, f.Predicate)
		}
		groups[f.Predicate] = append(groups[f.Predicate], f)
	}

	// Sort predicates by importance (based on predicate priority order)
	sort.SliceStable(predicateOrder, func(i, j int) bool {
		return predicateSortOrder(predicateOrder[i]) < predicateSortOrder(predicateOrder[j])
	})

	var sb strings.Builder

	for _, pred := range predicateOrder {
		predFacts := groups[pred]

		if fs.includeComments && len(predFacts) > 1 {
			sb.WriteString(fmt.Sprintf("# %s (%d facts)\n", pred, len(predFacts)))
		}

		for _, f := range predFacts {
			factStr := f.String()
			if len(factStr) > fs.maxLineLength && fs.maxLineLength > 0 {
				// Truncate long strings in arguments
				factStr = fs.truncateFact(f)
			}
			sb.WriteString(factStr)
			sb.WriteString("\n")
		}

		if fs.includeComments {
			sb.WriteString("\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// truncateFact creates a truncated version of a fact for display.
func (fs *FactSerializer) truncateFact(f core.Fact) string {
	var args []string
	for _, arg := range f.Args {
		argStr := formatArg(arg)
		if len(argStr) > 50 {
			argStr = argStr[:47] + "..."
		}
		args = append(args, argStr)
	}
	return fmt.Sprintf("%s(%s).", f.Predicate, strings.Join(args, ", "))
}

// SerializeScoredFacts serializes scored facts with optional score annotations.
func (fs *FactSerializer) SerializeScoredFacts(facts []ScoredFact, includeScores bool) string {
	var sb strings.Builder

	for _, sf := range facts {
		if includeScores && fs.includeComments {
			sb.WriteString(fmt.Sprintf("# score: %.1f\n", sf.Score))
		}
		sb.WriteString(sf.Fact.String())
		sb.WriteString("\n")
	}

	return sb.String()
}

// SerializeCompressedTurn serializes a compressed turn to a structured format.
func (fs *FactSerializer) SerializeCompressedTurn(turn CompressedTurn) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("# Turn %d (%s) @ %s\n",
		turn.TurnNumber,
		turn.Role,
		turn.Timestamp.Format(time.RFC3339)))

	if turn.IntentAtom != nil {
		sb.WriteString(turn.IntentAtom.String())
		sb.WriteString("\n")
	}

	for _, f := range turn.FocusAtoms {
		sb.WriteString(f.String())
		sb.WriteString("\n")
	}

	for _, f := range turn.ActionAtoms {
		sb.WriteString(f.String())
		sb.WriteString("\n")
	}

	for _, f := range turn.ResultAtoms {
		sb.WriteString(f.String())
		sb.WriteString("\n")
	}

	return sb.String()
}

// SerializeCompressedContext creates the full context block for LLM injection.
func (fs *FactSerializer) SerializeCompressedContext(ctx *CompressedContext) string {
	var sb strings.Builder

	// Header
	sb.WriteString("# ═══════════════════════════════════════════════════════════\n")
	sb.WriteString("# MANGLE CONTEXT BLOCK (Compressed Logical State)\n")
	sb.WriteString(fmt.Sprintf("# Turn: %d | Generated: %s\n", ctx.TurnNumber, ctx.GeneratedAt.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("# Tokens: %d used / %d available\n", ctx.TokenUsage.Total, ctx.TokenUsage.Total+ctx.TokenUsage.Available))
	sb.WriteString("# ═══════════════════════════════════════════════════════════\n\n")

	// Core facts (always present)
	if ctx.CoreFacts != "" {
		sb.WriteString("# ─── CONSTITUTIONAL FACTS ───\n")
		sb.WriteString(ctx.CoreFacts)
		sb.WriteString("\n\n")
	}

	// High-activation context atoms
	if ctx.ContextAtoms != "" {
		sb.WriteString("# ─── ACTIVE CONTEXT ───\n")
		sb.WriteString(ctx.ContextAtoms)
		sb.WriteString("\n\n")
	}

	// Compressed history
	if ctx.HistorySummary != "" {
		sb.WriteString("# ─── COMPRESSED HISTORY ───\n")
		sb.WriteString("# (Surface text discarded, logical state retained)\n")
		sb.WriteString(ctx.HistorySummary)
		sb.WriteString("\n\n")
	}

	// Recent turns
	if len(ctx.RecentTurns) > 0 {
		sb.WriteString("# ─── RECENT TURNS ───\n")
		for _, turn := range ctx.RecentTurns {
			sb.WriteString(fs.SerializeCompressedTurn(turn))
			sb.WriteString("\n")
		}
	}

	sb.WriteString("# ═══════════════════════════════════════════════════════════\n")

	return sb.String()
}

// =============================================================================
// Control Packet Extraction
// =============================================================================

// ExtractAtomsFromControlPacket extracts Mangle atoms from a control packet.
func ExtractAtomsFromControlPacket(packet *perception.ControlPacket) ([]core.Fact, error) {
	if packet == nil {
		return nil, nil
	}

	var facts []core.Fact

	// Extract intent classification as a fact
	if packet.IntentClassification.Category != "" {
		facts = append(facts, core.Fact{
			Predicate: "user_intent",
			Args: []interface{}{
				"/current",
				packet.IntentClassification.Category,
				packet.IntentClassification.Verb,
				packet.IntentClassification.Target,
				packet.IntentClassification.Constraint,
			},
		})
	}

	// Parse mangle_updates strings into facts
	for _, update := range packet.MangleUpdates {
		fact, err := ParseMangleAtom(update)
		if err != nil {
			// Skip malformed atoms but log
			continue
		}
		facts = append(facts, fact)
	}

	return facts, nil
}

// ParseMangleAtom parses a Mangle atom string into a Fact.
// Format: predicate(arg1, arg2, ...).
func ParseMangleAtom(atom string) (core.Fact, error) {
	atom = strings.TrimSpace(atom)
	atom = strings.TrimSuffix(atom, ".")

	// Find predicate
	parenIdx := strings.Index(atom, "(")
	if parenIdx == -1 {
		return core.Fact{}, fmt.Errorf("invalid atom: no opening parenthesis")
	}

	predicate := strings.TrimSpace(atom[:parenIdx])
	if predicate == "" {
		return core.Fact{}, fmt.Errorf("invalid atom: empty predicate")
	}

	// Find arguments
	closeIdx := strings.LastIndex(atom, ")")
	if closeIdx == -1 || closeIdx <= parenIdx {
		return core.Fact{}, fmt.Errorf("invalid atom: no closing parenthesis")
	}

	argsStr := atom[parenIdx+1 : closeIdx]
	args := parseArgs(argsStr)

	return core.Fact{
		Predicate: predicate,
		Args:      args,
	}, nil
}

// parseArgs parses a comma-separated argument list.
func parseArgs(argsStr string) []interface{} {
	var args []interface{}

	if strings.TrimSpace(argsStr) == "" {
		return args
	}

	parts := splitArgs(argsStr)
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		args = append(args, parseArgValue(part))
	}

	return args
}

// splitArgs splits arguments respecting quoted strings and nested parentheses.
func splitArgs(s string) []string {
	var result []string
	var current strings.Builder
	depth := 0
	inQuotes := false
	quoteChar := rune(0)

	for _, ch := range s {
		switch {
		case (ch == '"' || ch == '\'') && !inQuotes:
			inQuotes = true
			quoteChar = ch
			current.WriteRune(ch)
		case ch == quoteChar && inQuotes:
			inQuotes = false
			current.WriteRune(ch)
		case ch == '(' && !inQuotes:
			depth++
			current.WriteRune(ch)
		case ch == ')' && !inQuotes:
			depth--
			current.WriteRune(ch)
		case ch == ',' && !inQuotes && depth == 0:
			result = append(result, current.String())
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result
}

// parseArgValue converts a string argument to the appropriate Go type.
func parseArgValue(s string) interface{} {
	s = strings.TrimSpace(s)

	// Name constant (starts with /)
	if strings.HasPrefix(s, "/") {
		return s
	}

	// Quoted string
	if (strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'")) {
		return s[1 : len(s)-1]
	}

	// Boolean
	if s == "true" || s == "/true" {
		return true
	}
	if s == "false" || s == "/false" {
		return false
	}

	// Integer
	var intVal int64
	if _, err := fmt.Sscanf(s, "%d", &intVal); err == nil {
		return intVal
	}

	// Float
	var floatVal float64
	if _, err := fmt.Sscanf(s, "%f", &floatVal); err == nil {
		return floatVal
	}

	// Default to string
	return s
}

// formatArg formats an argument for serialization.
func formatArg(arg interface{}) string {
	switch v := arg.(type) {
	case string:
		if strings.HasPrefix(v, "/") {
			return v // Name constant
		}
		return fmt.Sprintf("%q", v)
	case int, int64:
		return fmt.Sprintf("%d", v)
	case float64:
		return fmt.Sprintf("%.2f", v)
	case bool:
		if v {
			return "/true"
		}
		return "/false"
	default:
		return fmt.Sprintf("%v", v)
	}
}

// predicateSortOrder returns a sort order for predicates.
// Lower numbers appear first.
func predicateSortOrder(pred string) int {
	order := map[string]int{
		"user_intent":       1,
		"focus_resolution":  2,
		"active_goal":       3,
		"diagnostic":        10,
		"test_state":        11,
		"file_topology":     20,
		"modified":          21,
		"symbol_graph":      22,
		"dependency_link":   23,
		"campaign":          30,
		"campaign_phase":    31,
		"campaign_task":     32,
		"delegate_task":     40,
		"permitted":         50,
		"activation":        60,
		"context_atom":      61,
	}

	if o, ok := order[pred]; ok {
		return o
	}
	return 100 // Default order for unknown predicates
}

// =============================================================================
// JSON Serialization (for persistence)
// =============================================================================

// MarshalCompressedState serializes compressed state to JSON.
func MarshalCompressedState(state *CompressedState) ([]byte, error) {
	return json.MarshalIndent(state, "", "  ")
}

// UnmarshalCompressedState deserializes compressed state from JSON.
func UnmarshalCompressedState(data []byte) (*CompressedState, error) {
	var state CompressedState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}

// =============================================================================
// Context Block Builder
// =============================================================================

// ContextBlockBuilder builds the context block for LLM injection.
type ContextBlockBuilder struct {
	serializer *FactSerializer
	counter    *TokenCounter
}

// NewContextBlockBuilder creates a new context block builder.
func NewContextBlockBuilder() *ContextBlockBuilder {
	return &ContextBlockBuilder{
		serializer: NewFactSerializer(),
		counter:    NewTokenCounter(),
	}
}

// Build creates a CompressedContext from the given components.
func (cbb *ContextBlockBuilder) Build(
	coreFacts []core.Fact,
	contextAtoms []ScoredFact,
	historySummary string,
	recentTurns []CompressedTurn,
	turnNumber int,
) *CompressedContext {
	now := time.Now()

	// Serialize components
	coreStr := cbb.serializer.SerializeFacts(coreFacts)
	atomsStr := cbb.serializer.SerializeScoredFacts(contextAtoms, false)

	// Calculate token usage
	usage := TokenUsage{
		Core:    cbb.counter.CountString(coreStr),
		Atoms:   cbb.counter.CountString(atomsStr),
		History: cbb.counter.CountString(historySummary),
		Recent:  cbb.counter.CountTurns(recentTurns),
	}
	usage.Total = usage.Core + usage.Atoms + usage.History + usage.Recent

	return &CompressedContext{
		ContextAtoms:   atomsStr,
		CoreFacts:      coreStr,
		HistorySummary: historySummary,
		RecentTurns:    recentTurns,
		TokenUsage:     usage,
		GeneratedAt:    now,
		TurnNumber:     turnNumber,
		CompressionID:  fmt.Sprintf("ctx_%d_%d", turnNumber, now.Unix()),
	}
}
