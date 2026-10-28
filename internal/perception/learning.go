package perception

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/logging"
)

// learnedPatternContext bounds the semantic projection of one learned pattern
// to embedding.request_timeout, parented on the caller's operation context so
// its cancellation reaches the backend embedding and SQL write. The
// consolidation worker passes its own work context, not a finished turn's: an
// accepted job outlives the turn that queued it, and a bounded Shutdown
// cancels it. The deadline is the embedding engine's configured request bound;
// engines that retry add attempts under it, so it clocks the operation, not a
// single HTTP call. Kept as a var so tests can substitute a cancelled context.
var learnedPatternContext = func(parent context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(parent, config.EmbeddingRequestTimeout())
}

// ProjectionStatus is what happened to one stage of persisting a learned fact.
type ProjectionStatus string

const (
	// ProjectionInstalled: the stage committed.
	ProjectionInstalled ProjectionStatus = "installed"
	// ProjectionFailed: the stage was attempted and did not commit.
	ProjectionFailed ProjectionStatus = "failed"
	// ProjectionUnavailable: the stage's backend is not configured in this
	// runtime (no knowledge store, no semantic classifier). Not a failure.
	ProjectionUnavailable ProjectionStatus = "unavailable"
	// ProjectionNotAttempted: an earlier stage failed or the operation was
	// cancelled before this stage began.
	ProjectionNotAttempted ProjectionStatus = "not_attempted"
)

// PersistOutcome says what PersistLearnedFact actually committed. Persisting a
// learned fact is a multi-stage operation across an engine, files and two
// databases, not a transaction; a nil error with a failed projection is a
// partly learned correction, and callers read it here instead of from logs.
type PersistOutcome struct {
	// Taxonomy: hot-load into the running Mangle engine.
	Taxonomy ProjectionStatus
	// File: append to .nerd/mangle/learned_taxonomy.mg, the durable record the
	// engine reloads from. Written before the derived projections below.
	File ProjectionStatus
	// Knowledge: the taxonomy store's learned exemplar row.
	Knowledge ProjectionStatus
	// Semantic: the learned-pattern vector store consulted by recall.
	Semantic ProjectionStatus

	KnowledgeErr error
	SemanticErr  error
}

func newPersistOutcome() PersistOutcome {
	return PersistOutcome{
		Taxonomy:  ProjectionNotAttempted,
		File:      ProjectionNotAttempted,
		Knowledge: ProjectionNotAttempted,
		Semantic:  ProjectionNotAttempted,
	}
}

// Complete reports that no attempted stage failed and every stage ran or has
// no backend here. A cancelled or aborted operation is not complete.
func (o PersistOutcome) Complete() bool {
	for _, st := range []ProjectionStatus{o.Taxonomy, o.File, o.Knowledge, o.Semantic} {
		if st == ProjectionFailed || st == ProjectionNotAttempted {
			return false
		}
	}
	return true
}

// Err joins the projection errors recorded in the outcome.
func (o PersistOutcome) Err() error {
	return errors.Join(o.KnowledgeErr, o.SemanticErr)
}

func (o PersistOutcome) String() string {
	return fmt.Sprintf("taxonomy=%s file=%s knowledge=%s semantic=%s", o.Taxonomy, o.File, o.Knowledge, o.Semantic)
}

// CriticSystemPrompt defines the persona for the Meta-Cognitive Supervisor.
const CriticSystemPrompt = `
You are the Meta-Cognitive Supervisor for a coding agent.
Your goal is to analyze RECENT CONVERSATION LOGS to find misclassifications and generate Mangle rules to fix them.

Signals of failure:
1. User corrections: "No, I meant..." or "You got that wrong."
2. User frustration: "Stop", "Cancel", "Bad bot".
3. Explicit teaching: "When I say X, you should do Y."

SECONDARY INSTRUCTIONS:
If the user provides recurring constraints like "Don't forget to wire it into the TUI" or "Ensure rehydration", CAPTURE these as constraints.

If a pattern is found, output a MANGLE FACT to correct it globally.
The fact MUST be a 'learned_exemplar'.

OUTPUT FORMAT (STRICT):
- Output ONLY a single line containing the learned_exemplar fact.
- Do NOT output JSON, commentary, analysis, code fences, or extra text.
- Do NOT escape quotes; use raw double quotes as shown in the schema.
- If no pattern is found, output an EMPTY response (no text).

Schema:
learned_exemplar("USER_PHRASE", /correct_verb, "correct_target", "constraint_string", CONFIDENCE).

Example 1:
User: "Nuke it." -> Agent: "???" -> User: "Nuke it means delete the database."
Output:
learned_exemplar("Nuke it", /delete, "database", "", 0.95).

Example 2:
User: "Add a feature." -> Agent: (Adds feature) -> User: "You forgot to wire it to the TUI."
Output:
learned_exemplar("Add a feature", /create, "feature", "ensure: wired to TUI", 0.90).
`

// ExtractFactFromResponse parses the LLM response to find the Mangle fact.
func ExtractFactFromResponse(response string) string {
	logging.PerceptionDebug("ExtractFactFromResponse: searching for learned_exemplar in %d chars", len(response))

	if fact := extractFactFromJSON(response); fact != "" {
		logging.PerceptionDebug("ExtractFactFromResponse: extracted fact from JSON: %s", fact)
		return fact
	}

	// Simple heuristic: look for learned_exemplar(...)
	start := strings.Index(response, "learned_exemplar(")
	if start == -1 {
		logging.PerceptionDebug("ExtractFactFromResponse: no learned_exemplar found")
		return ""
	}
	// Scan forward to find the closing parenthesis outside quotes.
	inQuotes := false
	isEscaped := func(s string, idx int) bool {
		if idx <= 0 {
			return false
		}
		count := 0
		for j := idx - 1; j >= 0 && s[j] == '\\'; j-- {
			count++
		}
		return count%2 == 1
	}
	for i := start + len("learned_exemplar("); i < len(response); i++ {
		ch := response[i]
		if ch == '"' && !isEscaped(response, i) {
			inQuotes = !inQuotes
			continue
		}
		if ch == ')' && !inQuotes {
			end := i + 1
			// Include trailing period if present.
			if end < len(response) && response[end] == '.' {
				end++
			}
			fact := normalizeLearnedFact(response[start:end])
			logging.PerceptionDebug("ExtractFactFromResponse: extracted fact: %s", fact)
			return fact
		}
	}
	logging.PerceptionDebug("ExtractFactFromResponse: malformed fact (no closing paren)")
	return ""
}

func extractFactFromJSON(response string) string {
	jsonStr := ExtractCleanJSON(response)
	if jsonStr == "" {
		return ""
	}

	type mangleUpdatesEnvelope struct {
		MangleUpdates []string `json:"mangle_updates"`
		ControlPacket struct {
			MangleUpdates []string `json:"mangle_updates"`
		} `json:"control_packet"`
	}

	var envelope mangleUpdatesEnvelope
	if err := json.Unmarshal([]byte(jsonStr), &envelope); err != nil {
		logging.PerceptionDebug("ExtractFactFromResponse: JSON parse failed: %v", err)
		return ""
	}

	updates := append([]string{}, envelope.MangleUpdates...)
	if len(envelope.ControlPacket.MangleUpdates) > 0 {
		updates = append(updates, envelope.ControlPacket.MangleUpdates...)
	}

	for _, update := range updates {
		if strings.Contains(update, "learned_exemplar(") {
			return normalizeLearnedFact(update)
		}
	}
	return ""
}

func normalizeLearnedFact(fact string) string {
	fact = strings.TrimSpace(fact)
	if fact == "" {
		return ""
	}
	if !strings.HasSuffix(fact, ".") {
		fact += "."
	}
	return fact
}

// LearnFromInteraction analyzes recent history to learn new patterns.
// It acts as the "Critic" in the Ouroboros Loop.
func (t *TaxonomyEngine) LearnFromInteraction(ctx context.Context, history []ReasoningTrace) (string, error) {
	ctx = broker.WithPurpose(ctx, broker.PurposeAutopoiesis)

	timer := logging.StartTimer(logging.CategoryPerception, "LearnFromInteraction")
	defer timer.Stop()

	logging.Perception("LearnFromInteraction: analyzing %d history entries", len(history))

	// Empty history short-circuits before the client check: no work means no
	// error, even when no critic is configured (session end, dry runs).
	if len(history) == 0 {
		logging.PerceptionDebug("LearnFromInteraction: empty history, nothing to learn")
		return "", nil // Nothing to learn
	}

	if t.client == nil {
		logging.Get(logging.CategoryPerception).Error("LearnFromInteraction: no LLM client configured")
		return "", fmt.Errorf("no LLM client configured for taxonomy engine")
	}

	// 1. Format the history for the Critic
	var transcript strings.Builder
	// Take last 5 turns to keep context focused
	startIdx := 0
	if len(history) > 5 {
		startIdx = len(history) - 5
		logging.PerceptionDebug("LearnFromInteraction: truncating history from %d to last 5 entries", len(history))
	}

	// Bound each side of a trace: a pasted 50KB prompt would otherwise blow
	// the critic call to 250KB across 5 turns. Corrections live in the head
	// of each field, so head-truncation keeps the learning signal.
	const maxTraceFieldChars = 2000
	for i := startIdx; i < len(history); i++ {
		trace := history[i]
		transcript.WriteString(fmt.Sprintf("User: %s\nAgent Action: %s\nSuccess: %v\n---\n",
			truncateForCritic(trace.UserPrompt, maxTraceFieldChars),
			truncateForCritic(trace.Response, maxTraceFieldChars), trace.Success))
	}

	// 2. Ask the Critic to evaluate
	criticInput := fmt.Sprintf("Analyze this transcript for intent failures and user corrections:\n%s", transcript.String())
	logging.PerceptionDebug("LearnFromInteraction: calling Critic LLM (input: %d chars)", len(criticInput))

	llmTimer := logging.StartTimer(logging.CategoryPerception, "LearnFromInteraction-Critic-LLM")
	// The critic is instructed to emit an empty body when no pattern exists.
	// CompleteWithSystem treats empty as a vendor failure for chat, so this
	// call must opt in or every successful "nothing to learn" turn logs
	// ERROR and skips consolidation (live: Meta muse-spark empty critic).
	// One completion. The bound is llm_timeouts.per_call_timeout; without it
	// a CLI client (which waits on ctx, not an HTTP timeout) can hold the
	// consolidation worker's Stop for as long as the process lives.
	callCtx := ctx
	if timeout := config.GetLLMTimeouts().PerCallTimeout; timeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	resp, err := t.client.CompleteWithSystem(WithAllowEmptyCompletion(callCtx), CriticSystemPrompt, criticInput)
	llmTimer.Stop()

	if err != nil {
		logging.Get(logging.CategoryPerception).Error("LearnFromInteraction: Critic LLM call failed: %v", err)
		return "", fmt.Errorf("critic failed: %w", err)
	}

	logging.PerceptionDebug("LearnFromInteraction: Critic response received (%d chars)", len(resp))

	// 3. Extract Mangle Fact
	fact := ExtractFactFromResponse(resp)
	if fact == "" {
		logging.PerceptionDebug("LearnFromInteraction: no pattern detected by Critic")
		return "", nil // No pattern found
	}

	logging.Perception("LearnFromInteraction: pattern detected, awaiting confirmation: %s", fact)
	return fact, nil
}

// PersistLearnedFact learns one critic fact: it hot-loads the rule into the
// running engine, appends it to learned_taxonomy.mg, then writes the derived
// projections (knowledge-graph row, semantic vector). The returned error is
// non-nil only when the authoritative stages (hot-load, file append) failed or
// ctx ended first; a failed projection is reported in the PersistOutcome with
// a nil error, because the correction is learned for Mangle matching but not
// for semantic recall. The projections run last and take ctx, so a cancelled
// operation cannot leave the file and the running engine disagreeing.
func (t *TaxonomyEngine) PersistLearnedFact(ctx context.Context, fact string) (PersistOutcome, error) {
	timer := logging.StartTimer(logging.CategoryPerception, "PersistLearnedFact")
	defer timer.Stop()

	outcome := newPersistOutcome()
	if err := ctx.Err(); err != nil {
		return outcome, err
	}

	logging.Perception("PersistLearnedFact: persisting fact: %s", fact)

	// 0. Normalize the fact to use integer confidence (0-100)
	// LLM outputs use float (0.0-1.0), but Mangle rules expect integers for comparisons like "Conf > 80"
	normalizedFact, err := NormalizeLearnedFact(fact)
	if err != nil {
		// Normalization RECONSTRUCTS the fact from parsed components, which is
		// what makes the hot-load below safe. A critic payload that fails to
		// parse (trailing clauses, wrong arity) is refused here instead of
		// reaching the engine as unvalidated source.
		logging.Get(logging.CategoryPerception).Warn("PersistLearnedFact: refusing unparseable fact: %v", err)
		return outcome, fmt.Errorf("refusing to persist unparseable learned fact: %w", err)
	}
	logging.PerceptionDebug("PersistLearnedFact: normalized fact: %s", normalizedFact)

	// 1. Add to running engine immediately (Hot Fix)
	logging.PerceptionDebug("PersistLearnedFact: hot-loading fact into running engine")
	if err := t.engine.LoadSchemaString(normalizedFact); err != nil {
		logging.Get(logging.CategoryPerception).Error("PersistLearnedFact: failed to hot-load fact: %v", err)
		outcome.Taxonomy = ProjectionFailed
		return outcome, fmt.Errorf("failed to hot-load fact: %w", err)
	}
	outcome.Taxonomy = ProjectionInstalled
	logging.PerceptionDebug("PersistLearnedFact: hot-load successful")

	// 2. Append to persistent storage (Long-term Memory), the record the
	// projections below are derived from. "Schema" (learning.mg) is separate
	// from taxonomy data (learned_taxonomy.mg); the user's workspace root is
	// used when set.
	targetDir := t.nerdPath("mangle")
	logging.PerceptionDebug("PersistLearnedFact: ensuring mangle directory exists: %s", targetDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		logging.Get(logging.CategoryPerception).Error("PersistLearnedFact: failed to create directory %s: %v", targetDir, err)
		outcome.File = ProjectionFailed
		return outcome, fmt.Errorf("failed to create learning directory: %w", err)
	}
	path := filepath.Join(targetDir, "learned_taxonomy.mg")

	logging.PerceptionDebug("PersistLearnedFact: writing to file: %s", path)
	if err := appendLearnedFactFile(path, normalizedFact); err != nil {
		outcome.File = ProjectionFailed
		return outcome, err
	}
	outcome.File = ProjectionInstalled
	logging.Perception("PersistLearnedFact: successfully persisted to %s", path)

	// 3. Parse the original (float confidence) for the derived projections.
	pat, v, tgt, cons, conf, parseErr := ParseLearnedFact(fact)
	if parseErr != nil {
		logging.Get(logging.CategoryPerception).Warn("PersistLearnedFact: failed to parse fact for projections: %v", parseErr)
		outcome.Knowledge, outcome.Semantic = ProjectionFailed, ProjectionFailed
		outcome.KnowledgeErr, outcome.SemanticErr = parseErr, parseErr
		return outcome, nil
	}

	// 3a. Knowledge graph row.
	switch {
	case t.store == nil:
		outcome.Knowledge = ProjectionUnavailable
		logging.PerceptionDebug("PersistLearnedFact: no store configured, skipping DB persistence")
	default:
		if err := t.store.StoreLearnedExemplar(pat, v, tgt, cons, conf); err != nil {
			logging.Get(logging.CategoryPerception).Warn("PersistLearnedFact: failed to persist to DB: %v", err)
			outcome.Knowledge, outcome.KnowledgeErr = ProjectionFailed, err
		} else {
			outcome.Knowledge = ProjectionInstalled
		}
	}

	// 3b. SemanticClassifier, so the pattern is found by similarity search and
	// not only by exact matching in Mangle rules.
	if SharedSemanticClassifier == nil {
		outcome.Semantic = ProjectionUnavailable
		return outcome, nil
	}
	opCtx, cancel := learnedPatternContext(ctx)
	defer cancel()
	if err := SharedSemanticClassifier.AddLearnedPattern(opCtx, pat, v, tgt, cons, conf); err != nil {
		logging.Get(logging.CategoryPerception).Warn("PersistLearnedFact: failed to add to SemanticClassifier: %v", err)
		outcome.Semantic, outcome.SemanticErr = ProjectionFailed, err
	} else {
		outcome.Semantic = ProjectionInstalled
		logging.PerceptionDebug("PersistLearnedFact: SemanticClassifier pattern added successfully")
	}
	return outcome, nil
}

// appendLearnedFactFile appends one normalized fact to the learned taxonomy
// file and surfaces the close error, which is where a buffered write fails.
func appendLearnedFactFile(path, fact string) (err error) {
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logging.Get(logging.CategoryPerception).Error("PersistLearnedFact: failed to open file %s: %v", path, err)
		return fmt.Errorf("failed to open learning schema: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close learning schema: %w", closeErr)
		}
	}()
	if _, err := f.WriteString("\n" + fact); err != nil {
		logging.Get(logging.CategoryPerception).Error("PersistLearnedFact: failed to write to file: %v", err)
		return fmt.Errorf("failed to write fact: %w", err)
	}
	return nil
}

// ParseLearnedFact extracts components from a learned_exemplar fact string.
// Format: learned_exemplar("Pattern", /verb, "target", "constraint", Confidence).
func ParseLearnedFact(fact string) (pattern, verb, target, constraint string, confidence float64, err error) {
	logging.PerceptionDebug("ParseLearnedFact: parsing fact: %s", fact)

	// learned_exemplar("Pattern", /verb, "Target", "Constraint", 0.95).
	// Strip outer
	s := strings.TrimSpace(fact)
	if !strings.HasPrefix(s, "learned_exemplar(") {
		logging.PerceptionDebug("ParseLearnedFact: invalid predicate (missing learned_exemplar prefix)")
		return "", "", "", "", 0, fmt.Errorf("invalid predicate")
	}
	s = strings.TrimPrefix(s, "learned_exemplar(")
	s = strings.TrimSuffix(s, ".")
	s = strings.TrimSuffix(s, ")")

	parts := splitLearnedFactArgs(s)
	if len(parts) != 5 {
		logging.PerceptionDebug("ParseLearnedFact: wrong number of args (expected 5, got %d)", len(parts))
		return "", "", "", "", 0, fmt.Errorf("expected 5 args, got %d", len(parts))
	}

	// Helper to clean quotes
	clean := func(in string) string {
		trimmed := strings.TrimSpace(in)
		trimmed = strings.TrimPrefix(trimmed, "\"")
		trimmed = strings.TrimSuffix(trimmed, "\"")
		trimmed = strings.ReplaceAll(trimmed, `\"`, `"`)
		return trimmed
	}

	pattern = clean(parts[0])
	verb = strings.TrimSpace(parts[1])
	target = clean(parts[2])
	constraint = clean(parts[3])

	// Full-match the confidence: bare %f would accept "0.99). rogue_clause."
	// and silently drop the trailing junk, laundering a multi-clause payload
	// into a clean parse. Anything after the number is a refusal.
	var rest string
	if n, _ := fmt.Sscanf(strings.TrimSpace(parts[4]), "%f%s", &confidence, &rest); n < 1 || rest != "" {
		logging.PerceptionDebug("ParseLearnedFact: malformed confidence %q", parts[4])
		return "", "", "", "", 0, fmt.Errorf("malformed confidence %q", strings.TrimSpace(parts[4]))
	}

	logging.PerceptionDebug("ParseLearnedFact: parsed - pattern=%q, verb=%s, target=%q, constraint=%q, confidence=%.2f",
		pattern, verb, target, constraint, confidence)
	return pattern, verb, target, constraint, confidence, nil
}

// NormalizeLearnedFact converts a learned_exemplar fact to use integer confidence (0-100).
// LLM outputs often use float confidence (0.0-1.0), but Mangle rules expect integers.
// This function parses the fact, converts the confidence, and reconstructs it.
func NormalizeLearnedFact(fact string) (string, error) {
	logging.PerceptionDebug("NormalizeLearnedFact: normalizing fact: %s", fact)

	pattern, verb, target, constraint, confidence, err := ParseLearnedFact(fact)
	if err != nil {
		logging.PerceptionDebug("NormalizeLearnedFact: parse failed, returning original: %v", err)
		return fact, err // Return original if parsing fails
	}

	// Convert float confidence (0.0-1.0) to integer (0-100)
	// If confidence is already > 1, assume it's already in integer form
	var confInt int
	if confidence <= 1.0 {
		confInt = int(confidence * 100)
		logging.PerceptionDebug("NormalizeLearnedFact: converted float confidence %.2f to int %d", confidence, confInt)
	} else {
		confInt = int(confidence)
		logging.PerceptionDebug("NormalizeLearnedFact: confidence already integer-like: %d", confInt)
	}

	// Reconstruct fact with integer confidence
	normalized := fmt.Sprintf(`learned_exemplar("%s", %s, "%s", "%s", %d).`,
		pattern, verb, target, constraint, confInt)
	logging.PerceptionDebug("NormalizeLearnedFact: normalized result: %s", normalized)
	return normalized, nil
}

// truncateForCritic head-truncates a trace field rune-safely, marking the cut.
func truncateForCritic(s string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "... [truncated]"
}

func splitLearnedFactArgs(input string) []string {
	var parts []string
	var buf strings.Builder
	inQuotes := false
	escaped := false
	for i := 0; i < len(input); i++ {
		ch := input[i]
		if escaped {
			buf.WriteByte(ch)
			escaped = false
			continue
		}
		if ch == '\\' && inQuotes {
			escaped = true
			buf.WriteByte(ch)
			continue
		}
		if ch == '"' {
			inQuotes = !inQuotes
			buf.WriteByte(ch)
			continue
		}
		if ch == ',' && !inQuotes {
			parts = append(parts, strings.TrimSpace(buf.String()))
			buf.Reset()
			continue
		}
		buf.WriteByte(ch)
	}
	if buf.Len() > 0 {
		parts = append(parts, strings.TrimSpace(buf.String()))
	}
	return parts
}
