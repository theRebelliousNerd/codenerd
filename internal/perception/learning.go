package perception

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

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
	// Simple heuristic: look for learned_exemplar(...)
	start := strings.Index(response, "learned_exemplar(")
	if start == -1 {
		return ""
	}
	// Find the closing parenthesis
	// This is a naive extraction, assuming the LLM outputs it on one line or cleanly.
	rest := response[start:]
	end := strings.Index(rest, ").")
	if end == -1 {
		// Try just )
		end = strings.Index(rest, ")")
	}

	if end != -1 {
		return rest[:end+1] + "." // Include closing param and dot
	}
	return ""
}

// LearnFromInteraction analyzes recent history to learn new patterns.
// It acts as the "Critic" in the Ouroboros Loop.
func (t *TaxonomyEngine) LearnFromInteraction(ctx context.Context, history []ReasoningTrace) (string, error) {
	if t.client == nil {
		return "", fmt.Errorf("no LLM client configured for taxonomy engine")
	}

	if len(history) == 0 {
		return "", nil // Nothing to learn
	}

	// 1. Format the history for the Critic
	var transcript strings.Builder
	// Take last 5 turns to keep context focused
	startIdx := 0
	if len(history) > 5 {
		startIdx = len(history) - 5
	}

	for i := startIdx; i < len(history); i++ {
		trace := history[i]
		transcript.WriteString(fmt.Sprintf("User: %s\nAgent Action: %s\nSuccess: %v\n---\n",
			trace.UserPrompt, trace.Response, trace.Success))
	}

	// 2. Ask the Critic to evaluate
	criticInput := fmt.Sprintf("Analyze this transcript for intent failures and user corrections:\n%s", transcript.String())

	resp, err := t.client.CompleteWithSystem(ctx, CriticSystemPrompt, criticInput)
	if err != nil {
		return "", fmt.Errorf("critic failed: %w", err)
	}

	// 3. Extract Mangle Fact
	fact := ExtractFactFromResponse(resp)
	if fact == "" {
		return "", nil // No pattern found
	}

	// 4. Persist the Lesson
	if err := t.PersistLearnedFact(fact); err != nil {
		return "", fmt.Errorf("failed to persist fact: %w", err)
	}

	return fact, nil
}

// PersistLearnedFact writes the new rule to the learned.mg file and reloads definitions.
func (t *TaxonomyEngine) PersistLearnedFact(fact string) error {
	// 1. Add to running engine immediately (Hot Fix)
	if err := t.engine.LoadSchemaString(fact); err != nil {
		return fmt.Errorf("failed to hot-load fact: %w", err)
	}

	// 2. Parse and Persist to DB (Knowledge Graph)
	// We attempt this for robustness, but don't fail hard if it fails (the file is the source of truth for Mangle)
	if t.store != nil {
		pat, v, tgt, cons, conf, err := ParseLearnedFact(fact)
		if err == nil {
			if err := t.store.StoreLearnedExemplar(pat, v, tgt, cons, conf); err != nil {
				fmt.Printf("WARNING: Failed to persist learned fact to DB: %v\n", err)
			}
		} else {
			fmt.Printf("WARNING: Failed to parse learned fact for DB: %v\n", err)
		}
	}

	// 3. Append to persistent storage (Long-term Memory)
	// We separate "Schema" (learning.mg) from "Data" (learned.mg)
	// Write to user's workspace
	targetDir := filepath.Join(".nerd", "mangle")
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create learning directory: %w", err)
	}
	path := filepath.Join(targetDir, "learned.mg")

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open learning schema: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString("\n" + fact); err != nil {
		return fmt.Errorf("failed to write fact: %w", err)
	}

	return nil
}

// ParseLearnedFact extracts components from a learned_exemplar fact string.
// Format: learned_exemplar("Pattern", /verb, "target", "constraint", Confidence).
func ParseLearnedFact(fact string) (pattern, verb, target, constraint string, confidence float64, err error) {
	// learned_exemplar("Pattern", /verb, "Target", "Constraint", 0.95).
	// Strip outer
	s := strings.TrimSpace(fact)
	if !strings.HasPrefix(s, "learned_exemplar(") {
		return "", "", "", "", 0, fmt.Errorf("invalid predicate")
	}
	s = strings.TrimPrefix(s, "learned_exemplar(")
	s = strings.TrimSuffix(s, ".")
	s = strings.TrimSuffix(s, ")")

	// Split by comma
	parts := strings.Split(s, ",")
	if len(parts) != 5 {
		return "", "", "", "", 0, fmt.Errorf("expected 5 args, got %d", len(parts))
	}

	// Helper to clean quotes
	clean := func(in string) string {
		return strings.Trim(strings.TrimSpace(in), "\"")
	}

	pattern = clean(parts[0])
	verb = strings.TrimSpace(parts[1])
	target = clean(parts[2])
	constraint = clean(parts[3])

	_, err = fmt.Sscanf(strings.TrimSpace(parts[4]), "%f", &confidence)
	if err != nil {
		return "", "", "", "", 0, err
	}

	return pattern, verb, target, constraint, confidence, nil
}
