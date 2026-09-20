package context

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// =============================================================================
// Fact Serialization
// =============================================================================
// Serializes facts to Mangle notation for LLM context injection.

// FactSerializer handles serialization of facts to various formats.
type FactSerializer struct {
	// Options
	includeComments  bool
	maxLineLength    int
	groupByPredicate bool

	// Corpus-based serialization order (loaded from predicate_corpus.db)
	// Takes precedence over the hardcoded predicateSortOrder() function
	corpusOrder map[string]int
}

// NewFactSerializer creates a new serializer with default options.
func NewFactSerializer() *FactSerializer {
	return &FactSerializer{
		includeComments:  true,
		maxLineLength:    120,
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

// SetCorpusOrder sets the serialization order from corpus priorities.
// This takes precedence over the hardcoded predicateSortOrder() function.
func (fs *FactSerializer) SetCorpusOrder(order map[string]int) *FactSerializer {
	fs.corpusOrder = order
	return fs
}

// LoadSerializationOrderFromCorpus loads serialization order from a PredicateCorpus.
// Returns the serializer for chaining.
func (fs *FactSerializer) LoadSerializationOrderFromCorpus(corpus *core.PredicateCorpus) *FactSerializer {
	if corpus == nil {
		return fs
	}
	order, err := corpus.GetSerializationOrder()
	if err != nil {
		// Fall back to the hardcoded order, but say so: a corpus that fails
		// to load is a broken single source of truth, not a preference.
		logging.ContextDebug("LoadSerializationOrderFromCorpus: falling back to hardcoded order: %v", err)
		return fs
	}
	fs.corpusOrder = order
	return fs
}

// getSortOrder returns the sort order for a predicate.
// Checks corpus order first, then falls back to hardcoded predicateSortOrder().
func (fs *FactSerializer) getSortOrder(pred string) int {
	if fs.corpusOrder != nil {
		if order, ok := fs.corpusOrder[pred]; ok {
			return order
		}
	}
	return predicateSortOrder(pred)
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
//
// The maxLineLength cut used to live only in serializeGrouped, so flipping
// WithGrouping(false) silently removed the only per-fact length bound in the
// context block. A fact's arguments are arbitrary strings written by whatever
// asserted them — a tool result, a file path, an error message — so the bound
// belongs on both paths.
func (fs *FactSerializer) serializeFlat(facts []core.Fact) string {
	var sb strings.Builder
	for _, f := range facts {
		sb.WriteString(fs.renderFact(f))
		sb.WriteString("\n")
	}
	return sb.String()
}

// renderFact renders one fact, collapsing over-long arguments.
func (fs *FactSerializer) renderFact(f core.Fact) string {
	factStr := f.String()
	if fs.maxLineLength > 0 && len(factStr) > fs.maxLineLength {
		return fs.truncateFact(f)
	}
	return factStr
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
	// Uses corpus-based order if available, otherwise hardcoded fallback
	sort.SliceStable(predicateOrder, func(i, j int) bool {
		return fs.getSortOrder(predicateOrder[i]) < fs.getSortOrder(predicateOrder[j])
	})

	var sb strings.Builder

	for _, pred := range predicateOrder {
		predFacts := groups[pred]

		if fs.includeComments && len(predFacts) > 1 {
			sb.WriteString(fmt.Sprintf("# %s (%d facts)\n", pred, len(predFacts)))
		}

		for _, f := range predFacts {
			sb.WriteString(fs.renderFact(f))
			sb.WriteString("\n")
		}

		if fs.includeComments {
			sb.WriteString("\n")
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// maxFactArgChars bounds one rendered fact argument in the context block.
const maxFactArgChars = 47

// truncateFact creates a truncated version of a fact for display.
//
// The per-argument cut used to append a bare "...", which says something is
// missing but not how much and is indistinguishable from an ellipsis the
// author of the fact wrote. A fact argument is arbitrary text asserted by
// whatever produced it — a tool result, an error message, a file path — and a
// path cut to "internal/core/defaults/policy/dele..." reads as a real path.
// The marker names the count, so the model can tell a shortened argument from
// a short one and ask for the fact instead of acting on the prefix.
func (fs *FactSerializer) truncateFact(f core.Fact) string {
	var args []string
	for _, arg := range f.Args {
		argStr := formatArg(arg)
		// Cut on a rune boundary: a byte cut can split a multi-byte rune and
		// inject invalid UTF-8 into the context block. ClampInline measures in
		// bytes, so the rune-safe prefix is taken first and its byte length is
		// what ClampInline is given.
		//
		// The marker costs more than the "..." it replaces, so a cut that does
		// not actually shorten the argument is not made: dropping forty
		// characters to add fifty leaves the block bigger AND the fact less
		// complete, which is the worst of both.
		if runes := []rune(argStr); len(runes) > maxFactArgChars {
			if cut := types.ClampInline(argStr, len(string(runes[:maxFactArgChars])), "fact arg"); len(cut) < len(argStr) {
				argStr = cut
			}
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
		sb.WriteString(fs.renderFact(sf.Fact))
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
	if ctx == nil {
		return ""
	}
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

	return types.ClampText(sb.String(), maxContextBlockChars, "mangle context block")
}

// Bounds on the injected context block.
//
// Nothing downstream re-checks this. ContextBlockBuilder.Build MEASURES the
// block (TokenUsage) but never enforces anything, and recalcBudget overwrites
// the budget counters wholesale via SetUsage, so the budget is a report, not
// a gate. CheckTotalBudget then runs at the START of the next BuildContext,
// which means an over-budget block is always shipped once before anything
// notices.
const (
	// maxContextBlockChars caps the serialized Mangle context block
	// (~16k tokens). Core facts alone are unbounded: getCoreFacts collects
	// every `permitted` row the kernel holds, and a long session accumulates
	// one per (action, resource) pair it has evaluated.
	maxContextBlockChars = 64 * 1024
)

// =============================================================================
// Control Packet Extraction
// =============================================================================

// ExtractAtomsFromControlPacket extracts Mangle atoms from a control packet.
func ExtractAtomsFromControlPacket(packet *articulation.ControlPacket) ([]core.Fact, error) {
	if packet == nil {
		return nil, nil
	}

	var facts []core.Fact

	// Extract intent classification as a fact
	if packet.IntentClassification.Category != "" {
		facts = append(facts, core.Fact{
			Predicate: "user_intent",
			Args: []any{
				"/current_intent",
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
			// Skip malformed atoms, but name them: a control packet whose
			// updates never parse is a broken producer, not an empty turn.
			logging.Get(logging.CategoryContext).Warn("Skipping malformed mangle update %q: %v", update, err)
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
	args, err := parseArgs(argsStr)
	if err != nil {
		return core.Fact{}, err
	}

	return core.Fact{
		Predicate: predicate,
		Args:      args,
	}, nil
}

// parseArgs parses a comma-separated argument list.
func parseArgs(argsStr string) ([]any, error) {
	var args []any

	if strings.TrimSpace(argsStr) == "" {
		return args, nil
	}

	parts, err := splitArgs(argsStr)
	if err != nil {
		return nil, err
	}
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		args = append(args, parseArgValue(part))
	}

	return args, nil
}

// splitArgs splits arguments respecting quoted strings and nested parentheses.
// A backslash inside quotes escapes the next rune, so an embedded quote does
// not end the string early. Unbalanced quotes or parentheses are an error:
// mis-splitting them produced wrong facts silently.
func splitArgs(s string) ([]string, error) {
	var result []string
	var current strings.Builder
	depth := 0
	inQuotes := false
	quoteChar := rune(0)

	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]
		switch {
		case ch == '\\' && inQuotes && i+1 < len(runes):
			current.WriteRune(ch)
			i++
			current.WriteRune(runes[i])
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
			if depth < 0 {
				return nil, fmt.Errorf("unbalanced parenthesis in %q", s)
			}
			current.WriteRune(ch)
		case ch == ',' && !inQuotes && depth == 0:
			result = append(result, current.String())
			current.Reset()
		default:
			current.WriteRune(ch)
		}
	}
	if inQuotes {
		return nil, fmt.Errorf("unterminated string in %q", s)
	}
	if depth != 0 {
		return nil, fmt.Errorf("unbalanced parenthesis in %q", s)
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return result, nil
}

// parseArgValue converts a string argument to the appropriate Go type.
func parseArgValue(s string) any {
	s = strings.TrimSpace(s)

	// Name constant (starts with /)
	if strings.HasPrefix(s, "/") {
		return s
	}

	// Quoted string. The length guard matters: a lone quote character used to
	// reach s[1:0] and panic.
	if len(s) >= 2 && ((strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"")) ||
		(strings.HasPrefix(s, "'") && strings.HasSuffix(s, "'"))) {
		return unescapeQuoted(s[1 : len(s)-1])
	}

	// Boolean
	if s == "true" || s == "/true" {
		return true
	}
	if s == "false" || s == "/false" {
		return false
	}

	// Numbers must parse whole: fmt.Sscanf's %d accepts a numeric prefix, so
	// it read "1.5" as int64(1) — every float silently lost its fraction and
	// the %f branch below it was dead code — and "123abc" as int64(123).
	if intVal, err := strconv.ParseInt(s, 10, 64); err == nil {
		return intVal
	}
	if floatVal, err := strconv.ParseFloat(s, 64); err == nil {
		return floatVal
	}

	// Default to string
	return s
}

// unescapeQuoted resolves backslash escapes inside a quoted argument, so the
// string splitArgs protected with escape handling arrives without its armor.
func unescapeQuoted(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var sb strings.Builder
	sb.Grow(len(s))
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			i++
			sb.WriteRune(runes[i])
			continue
		}
		sb.WriteRune(runes[i])
	}
	return sb.String()
}

// formatArg formats an argument for serialization.
func formatArg(arg any) string {
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

// fallbackPredicateOrder is the hardcoded predicate sort order used when the
// corpus order is unavailable. It lives at package level because the lookup
// runs once per sort comparison — rebuilding the map on every call turned
// every serialization into a stream of ashamed allocations.
var fallbackPredicateOrder = map[string]int{
	"user_intent":      1,
	"focus_resolution": 2,
	"active_goal":      3,
	"diagnostic":       10,
	"test_state":       11,
	"file_topology":    20,
	"modified":         21,
	"symbol_graph":     22,
	"dependency_link":  23,
	"campaign":         30,
	"campaign_phase":   31,
	"campaign_task":    32,
	"issue_text":       33,
	"issue_keyword":    34,
	"delegate_task":    40,
	"permitted":        50,
	"activation":       60,
	"context_atom":     61,
}

// predicateSortOrder returns a hardcoded sort order for predicates.
// Lower numbers appear first.
//
// DEPRECATED: This is the fallback when corpus order is not available.
// Prefer using FactSerializer.LoadSerializationOrderFromCorpus() which
// loads order from predicate_corpus.db for a single source of truth.
func predicateSortOrder(pred string) int {
	if o, ok := fallbackPredicateOrder[pred]; ok {
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
