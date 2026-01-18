package perception

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
)

// learnedPatternContext is used for embedding generation during pattern learning.
// We use a background context with timeout for this operation.
var learnedPatternContext = context.Background

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
			fact := response[start:end]
			if !strings.HasSuffix(fact, ".") {
				fact += "."
			}
			logging.PerceptionDebug("ExtractFactFromResponse: extracted fact: %s", fact)
			return fact
		}
	}
	logging.PerceptionDebug("ExtractFactFromResponse: malformed fact (no closing paren)")
	return ""
}

// LearnFromInteraction analyzes recent history to learn new patterns.
// It acts as the "Critic" in the Ouroboros Loop.
func (t *TaxonomyEngine) LearnFromInteraction(ctx context.Context, history []ReasoningTrace) (string, error) {
	timer := logging.StartTimer(logging.CategoryPerception, "LearnFromInteraction")
	defer timer.Stop()

	logging.Perception("LearnFromInteraction: analyzing %d history entries", len(history))

	if t.client == nil {
		logging.Get(logging.CategoryPerception).Error("LearnFromInteraction: no LLM client configured")
		return "", fmt.Errorf("no LLM client configured for taxonomy engine")
	}

	if len(history) == 0 {
		logging.PerceptionDebug("LearnFromInteraction: empty history, nothing to learn")
		return "", nil // Nothing to learn
	}

	// 1. Format the history for the Critic
	var transcript strings.Builder
	// Take last 5 turns to keep context focused
	startIdx := 0
	if len(history) > 5 {
		startIdx = len(history) - 5
		logging.PerceptionDebug("LearnFromInteraction: truncating history from %d to last 5 entries", len(history))
	}

	for i := startIdx; i < len(history); i++ {
		trace := history[i]
		transcript.WriteString(fmt.Sprintf("User: %s\nAgent Action: %s\nSuccess: %v\n---\n",
			trace.UserPrompt, trace.Response, trace.Success))
	}

	// 2. Ask the Critic to evaluate
	criticInput := fmt.Sprintf("Analyze this transcript for intent failures and user corrections:\n%s", transcript.String())
	logging.PerceptionDebug("LearnFromInteraction: calling Critic LLM (input: %d chars)", len(criticInput))

	llmTimer := logging.StartTimer(logging.CategoryPerception, "LearnFromInteraction-Critic-LLM")
	resp, err := t.client.CompleteWithSystem(ctx, CriticSystemPrompt, criticInput)
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

// PersistLearnedFact writes the new rule to the learned.mg file and reloads definitions.
func (t *TaxonomyEngine) PersistLearnedFact(fact string) error {
	timer := logging.StartTimer(logging.CategoryPerception, "PersistLearnedFact")
	defer timer.Stop()

	logging.Perception("PersistLearnedFact: persisting fact: %s", fact)

	// 0. Normalize the fact to use integer confidence (0-100)
	// LLM outputs use float (0.0-1.0), but Mangle rules expect integers for comparisons like "Conf > 80"
	normalizedFact, err := NormalizeLearnedFact(fact)
	if err != nil {
		// If normalization fails, use original fact but log warning
		logging.Get(logging.CategoryPerception).Warn("PersistLearnedFact: failed to normalize fact, using original: %v", err)
		normalizedFact = fact
	} else {
		logging.PerceptionDebug("PersistLearnedFact: normalized fact: %s", normalizedFact)
	}

	// 1. Add to running engine immediately (Hot Fix)
	logging.PerceptionDebug("PersistLearnedFact: hot-loading fact into running engine")
	if err := t.engine.LoadSchemaString(normalizedFact); err != nil {
		logging.Get(logging.CategoryPerception).Error("PersistLearnedFact: failed to hot-load fact: %v", err)
		return fmt.Errorf("failed to hot-load fact: %w", err)
	}
	logging.PerceptionDebug("PersistLearnedFact: hot-load successful")

	// 2. Parse and Persist to DB (Knowledge Graph)
	// We attempt this for robustness, but don't fail hard if it fails (the file is the source of truth for Mangle)
	pat, v, tgt, cons, conf, parseErr := ParseLearnedFact(fact) // Parse original to get float
	if parseErr != nil {
		logging.Get(logging.CategoryPerception).Warn("PersistLearnedFact: failed to parse fact: %v", parseErr)
	}

	if t.store != nil && parseErr == nil {
		logging.PerceptionDebug("PersistLearnedFact: persisting to knowledge graph DB")
		if err := t.store.StoreLearnedExemplar(pat, v, tgt, cons, conf); err != nil {
			logging.Get(logging.CategoryPerception).Warn("PersistLearnedFact: failed to persist to DB: %v", err)
		} else {
			logging.PerceptionDebug("PersistLearnedFact: DB persistence successful")
		}
	} else if t.store == nil {
		logging.PerceptionDebug("PersistLearnedFact: no store configured, skipping DB persistence")
	}

	// 2b. Add to SemanticClassifier for vector-based matching
	// This enables the learned pattern to be found via semantic similarity search,
	// not just exact string matching in Mangle rules.
	if SharedSemanticClassifier != nil && parseErr == nil {
		logging.PerceptionDebug("PersistLearnedFact: adding pattern to SemanticClassifier")
		ctx := learnedPatternContext()
		if err := SharedSemanticClassifier.AddLearnedPattern(ctx, pat, v, tgt, cons, conf); err != nil {
			// Non-fatal: Mangle-based matching still works
			logging.Get(logging.CategoryPerception).Warn("PersistLearnedFact: failed to add to SemanticClassifier: %v", err)
		} else {
			logging.PerceptionDebug("PersistLearnedFact: SemanticClassifier pattern added successfully")
		}
	}

	// 3. Append to persistent storage (Long-term Memory)
	// We separate "Schema" (learning.mg) from taxonomy data (learned_taxonomy.mg)
	// Write to user's workspace using explicit workspace root if set
	targetDir := t.nerdPath("mangle")
	logging.PerceptionDebug("PersistLearnedFact: ensuring mangle directory exists: %s", targetDir)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		logging.Get(logging.CategoryPerception).Error("PersistLearnedFact: failed to create directory %s: %v", targetDir, err)
		return fmt.Errorf("failed to create learning directory: %w", err)
	}
	path := filepath.Join(targetDir, "learned_taxonomy.mg")

	logging.PerceptionDebug("PersistLearnedFact: writing to file: %s", path)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logging.Get(logging.CategoryPerception).Error("PersistLearnedFact: failed to open file %s: %v", path, err)
		return fmt.Errorf("failed to open learning schema: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString("\n" + normalizedFact); err != nil {
		logging.Get(logging.CategoryPerception).Error("PersistLearnedFact: failed to write to file: %v", err)
		return fmt.Errorf("failed to write fact: %w", err)
	}

	logging.Perception("PersistLearnedFact: successfully persisted to %s", path)
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

	_, err = fmt.Sscanf(strings.TrimSpace(parts[4]), "%f", &confidence)
	if err != nil {
		logging.PerceptionDebug("ParseLearnedFact: failed to parse confidence: %v", err)
		return "", "", "", "", 0, err
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
