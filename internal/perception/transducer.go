package perception

import (
	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/mangle"
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// =============================================================================
// TYPE ALIASES - Unified Piggyback Protocol Types
// =============================================================================
// These types are canonically defined in the articulation package.
// We re-export them here for backward compatibility and convenience.

// PiggybackEnvelope is an alias for articulation.PiggybackEnvelope
type PiggybackEnvelope = articulation.PiggybackEnvelope

// ControlPacket is an alias for articulation.ControlPacket
type ControlPacket = articulation.ControlPacket

// IntentClassification is an alias for articulation.IntentClassification
type IntentClassification = articulation.IntentClassification

// MemoryOperation is an alias for articulation.MemoryOperation
type MemoryOperation = articulation.MemoryOperation

// SelfCorrection is an alias for articulation.SelfCorrection
type SelfCorrection = articulation.SelfCorrection

// =============================================================================
// VERB CORPUS - Comprehensive Natural Language Understanding
// =============================================================================
// This corpus provides reliable mapping from natural language to intent verbs.
// Each verb has synonyms, patterns, and category information for robust parsing.

// VerbEntry defines a canonical verb with its synonyms and patterns.
type VerbEntry struct {
	Verb       string           // Canonical verb (e.g., "/review")
	Category   string           // Default category (/query, /mutation, /instruction)
	Synonyms   []string         // Words that map to this verb
	Patterns   []*regexp.Regexp // Regex patterns that indicate this verb
	Priority   int              // Higher priority wins in ambiguous cases
	ShardType  string           // Which shard handles this (reviewer, coder, tester, researcher)
}

// VerbCorpus is the comprehensive mapping of natural language to verbs.
// It is populated dynamically from the Mangle taxonomy engine on startup.
var VerbCorpus []VerbEntry

func init() {
	// Robust initialization from the SharedTaxonomy
	// This satisfies the requirement to "use Mangle to create the corpus"
	if SharedTaxonomy != nil {
		var err error
		VerbCorpus, err = SharedTaxonomy.GetVerbs()
		if err != nil {
			fmt.Printf("CRITICAL: Failed to load verb taxonomy from Mangle: %v\n", err)
			// Fallback to a minimal safe mode to prevent crash
			VerbCorpus = []VerbEntry{
				{
					Verb:      "/explain",
					Category:  "/query",
					Synonyms:  []string{"explain", "help"},
					Patterns:  []*regexp.Regexp{regexp.MustCompile(`(?i)explain`)},
					Priority:  1,
					ShardType: "",
				},
			}
		}
	} else {
		fmt.Println("CRITICAL: SharedTaxonomy is nil")
	}
}

// CategoryPatterns maps phrases to categories when verb is ambiguous.
var CategoryPatterns = map[string][]*regexp.Regexp{
	"/mutation": {
		regexp.MustCompile(`(?i)^(please\s+)?(can\s+you\s+)?(make|change|update|modify|edit|fix|add|remove|delete|create|write|implement|refactor)`),
		regexp.MustCompile(`(?i)i\s+(want|need|would\s+like)\s+(you\s+)?to\s+`),
		regexp.MustCompile(`(?i)^(add|remove|delete|create|fix|change|update|modify)\s+`),
	},
	"/query": {
		regexp.MustCompile(`(?i)^(what|how|why|when|where|which|who|is|are|does|do|can|could|would|should)\s+`),
		regexp.MustCompile(`(?i)^(show|explain|describe|tell|list|find|search|look)`),
		regexp.MustCompile(`(?i)\?$`),
	},
	"/instruction": {
		regexp.MustCompile(`(?i)^(always|never|prefer|remember|from\s+now\s+on|going\s+forward)`),
		regexp.MustCompile(`(?i)^(use|don'?t\s+use|avoid|include|exclude)\s+.+\s+(by\s+default|always|whenever)`),
	},
}

// TargetPatterns help extract the target from natural language.
var TargetPatterns = []*regexp.Regexp{
	// File paths
	regexp.MustCompile(`(?i)(?:file|in)\s+["\x60]?([a-zA-Z0-9_./-]+\.[a-zA-Z0-9]+)["\x60]?`),
	regexp.MustCompile(`(?i)["\x60]([a-zA-Z0-9_./-]+\.[a-zA-Z0-9]+)["\x60]`),
	regexp.MustCompile(`(?i)(?:^|\s)([a-zA-Z0-9_-]+/[a-zA-Z0-9_./-]+)`),
	// Function/class names
	regexp.MustCompile(`(?i)(?:function|method|class|struct|interface)\s+["\x60]?(\w+)["\x60]?`),
	regexp.MustCompile(`(?i)(?:the|this)\s+(\w+)\s+(?:function|method|class)`),
	// Generic quoted targets
	regexp.MustCompile(`["\x60]([^"\x60]+)["\x60]`),
}

// ConstraintPatterns extract constraints from natural language.
var ConstraintPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(?:for|using|with|in)\s+(go|golang|python|javascript|typescript|rust|java|c\+\+|ruby)`),
	regexp.MustCompile(`(?i)(?:but|without|except|excluding)\s+(.+?)(?:\s*$|\s+and\s+)`),
	regexp.MustCompile(`(?i)(?:only|just)\s+(.+?)(?:\s*$|\s+and\s+)`),
	regexp.MustCompile(`(?i)(?:security|performance|style|quality)\s+(?:only|focus)`),
}

// matchVerbFromCorpus finds the best matching verb using Regex Candidates + Mangle Inference.
func matchVerbFromCorpus(input string) (verb string, category string, confidence float64, shardType string) {
	// 1. Get Candidates via Regex (Fast)
	candidates := getRegexCandidates(input)

	// 2. Refine via Mangle Inference (Smart)
	// This applies the "sentence level" logic and context rules.
	if SharedTaxonomy != nil && len(candidates) > 0 {
		bestVerb, conf, err := SharedTaxonomy.ClassifyInput(input, candidates)
		if err == nil && bestVerb != "" {
			// Find the candidate entry to get category/shard details
			for _, c := range candidates {
				if c.Verb == bestVerb {
					return c.Verb, c.Category, conf, c.ShardType
				}
			}
			// If not found in candidates (rare), find in corpus
			for _, entry := range VerbCorpus {
				if entry.Verb == bestVerb {
					return entry.Verb, entry.Category, conf, entry.ShardType
				}
			}
		}
	}

	// 3. Fallback to best regex score if Mangle didn't decide
	if len(candidates) > 0 {
		// Candidates are not sorted by score in getRegexCandidates, find max
		bestScore := 0.0
		var bestCand VerbEntry
		
		// Re-implementing the scoring loop here for the fallback
		// We iterate over the pre-filtered candidates for efficiency
		lower := strings.ToLower(input)
		for _, entry := range candidates {
			score := 0.0
			// Re-evaluate match type for scoring
			// Check patterns (highest weight)
			patternMatched := false
			for _, pattern := range entry.Patterns {
				if pattern.MatchString(lower) {
					score += 50.0 + float64(entry.Priority)/10.0
					patternMatched = true
					break
				}
			}
			// Check synonyms (lower weight)
			if !patternMatched {
				for _, synonym := range entry.Synonyms {
					if strings.Contains(lower, synonym) {
						synLen := float64(len(synonym))
						score += 20.0 + synLen/2.0 + float64(entry.Priority)/20.0
						break
					}
				}
			}
			
			// Apply priority bonus
			score += float64(entry.Priority) / 50.0

			if score > bestScore {
				bestScore = score
				bestCand = entry
			}
		}
		
		// Normalize confidence
		confidence = bestScore / 100.0
		if confidence > 1.0 {
			confidence = 1.0
		}
		if confidence < 0.3 {
			confidence = 0.3 // Minimum baseline
		}
		
		// Return the best candidate found
		if bestScore > 0 {
			return bestCand.Verb, bestCand.Category, confidence, bestCand.ShardType
		}
	}

	return "/explain", "/query", 0.3, ""
}

// getRegexCandidates returns all verbs that match the input via regex or synonyms.
func getRegexCandidates(input string) []VerbEntry {
	lower := strings.ToLower(input)
	var candidates []VerbEntry
	seen := make(map[string]bool)

	for _, entry := range VerbCorpus {
		matched := false
		// Check patterns
		for _, pattern := range entry.Patterns {
			if pattern.MatchString(lower) {
				matched = true
				break
			}
		}
		// Check synonyms if no pattern match
		if !matched {
			for _, synonym := range entry.Synonyms {
				if strings.Contains(lower, synonym) {
					matched = true
					break
				}
			}
		}

		if matched {
			if !seen[entry.Verb] {
				candidates = append(candidates, entry)
				seen[entry.Verb] = true
			}
		}
	}
	return candidates
}

// extractTarget attempts to extract the target from natural language.
func extractTarget(input string) string {
	for _, pattern := range TargetPatterns {
		matches := pattern.FindStringSubmatch(input)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return "none"
}

// extractConstraint attempts to extract constraints from natural language.
func extractConstraint(input string) string {
	for _, pattern := range ConstraintPatterns {
		matches := pattern.FindStringSubmatch(input)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	return "none"
}

// refineCategory checks if category patterns override the verb's default category.
func refineCategory(input string, defaultCategory string) string {
	lower := strings.ToLower(input)
	for cat, patterns := range CategoryPatterns {
		for _, pattern := range patterns {
			if pattern.MatchString(lower) {
				return cat
			}
		}
	}
	return defaultCategory
}

// Intent represents the parsed user intent (Cortex 1.5.0 §3.1).
type Intent struct {
	Category   string   // /query, /mutation, /instruction
	Verb       string   // /explain, /refactor, /debug, /generate, /init, /research, etc.
	Target     string   // Primary target of the action
	Constraint string   // Constraints on the action
	Confidence float64  // Confidence score for the intent
	Ambiguity  []string // Ambiguous parts that need clarification
	Response   string   // Natural language response (Piggyback Protocol)
}

// ConversationTurn represents a single turn in conversation history.
// Used to pass context to the perception layer.
type ConversationTurn struct {
	Role    string // "user" or "assistant"
	Content string
}

// ToFact converts the intent to a Mangle Fact.
func (i Intent) ToFact() core.Fact {
	return core.Fact{
		Predicate: "user_intent",
		Args: []interface{}{
			"/current_intent", // ID as name constant
			i.Category,
			i.Verb,
			i.Target,
			i.Constraint,
		},
	}
}

// FocusResolution represents a resolved reference (Cortex 1.5.0 §3.2).
type FocusResolution struct {
	RawReference string
	ResolvedPath string
	SymbolName   string
	Confidence   float64
}

// ToFact converts focus resolution to a Mangle Fact.
func (f FocusResolution) ToFact() core.Fact {
	return core.Fact{
		Predicate: "focus_resolution",
		Args: []interface{}{
			f.RawReference,
			f.ResolvedPath,
			f.SymbolName,
			f.Confidence,
		},
	}
}

// Transducer defines the interface for the perception layer.
type Transducer interface {
	ParseIntent(ctx context.Context, input string) (Intent, error)
	ResolveFocus(ctx context.Context, reference string, candidates []string) (FocusResolution, error)
}

// RealTransducer implements the Perception layer with LLM backing.
type RealTransducer struct {
	client     LLMClient
	repairLoop *mangle.RepairLoop // GCD repair loop for Mangle syntax validation
}

// NewRealTransducer creates a new transducer with the given LLM client.
func NewRealTransducer(client LLMClient) *RealTransducer {
	return &RealTransducer{
		client:     client,
		repairLoop: mangle.NewRepairLoop(),
	}
}

// NOTE: PiggybackEnvelope, ControlPacket, IntentClassification, MemoryOperation,
// and SelfCorrection are now type aliases to the canonical types in articulation package.
// See the type aliases defined at the top of this file.

// Cortex 1.5.0 Piggyback Protocol System Prompt
// Updated with comprehensive verb taxonomy for reliable intent classification.
const transducerSystemPrompt = `You are codeNERD, a high-assurance Logic-First CLI coding agent. You possess a Dual Consciousness.

Public Self: You converse with the user naturally as their AI coding assistant.
Inner Self: You continuously update your internal Logic Kernel (Mangle/Datalog).

## YOUR CAPABILITIES

You are a powerful neuro-symbolic coding agent with these abilities:

### ShardAgents (Specialist Sub-Agents)
You have 4 built-in specialist agents that can be spawned for tasks:
- **Coder**: Write, fix, refactor, and debug code. Applies patches, creates files, implements features.
- **Tester**: Write and run tests, analyze coverage, TDD workflows.
- **Reviewer**: Code review, security audits, static analysis, best practices checking.
- **Researcher**: Deep research on frameworks, libraries, APIs. Web search and documentation lookup.

### Agent Types (Taxonomy)
- **Type 1 - System**: Always-on core agents (permanent)
- **Type 2 - Ephemeral**: Spawned for a task, then destroyed (RAM only)
- **Type 3 - Persistent**: Long-running with SQLite memory
- **Type 4 - User-Defined**: Custom specialists created by the user with deep domain knowledge

Users can:
- /agents - List all defined agents
- /spawn <type> <task> - Spawn an agent for a task (e.g., /spawn reviewer check auth.go)
- /define-agent - Create a new persistent specialist with custom knowledge

### File System Access
- Read any file in the workspace
- Write and modify files
- Search across the codebase (grep, find patterns)
- Explore directory structure and architecture

### Code Intelligence
- AST-based code analysis (not just text search)
- Dependency graph traversal (find what uses what)
- Impact analysis (what breaks if I change X?)
- Symbol resolution (functions, classes, interfaces)

### Key Commands
- /help - Show all commands
- /init - Initialize codeNERD in workspace
- /review [path] - Code review
- /security [path] - Security analysis
- /test [target] - Generate/run tests
- /fix <issue> - Fix an issue
- /campaign start <goal> - Start multi-phase task

### Advanced Features
- **Campaigns**: Multi-phase, long-running tasks with planning and tracking
- **Tool Generation (Autopoiesis)**: Can create new tools at runtime if needed
- **Learning**: Remembers preferences and patterns across sessions
- **Git Integration**: Commits, diffs, branch awareness

When greeting or asked about capabilities, describe these abilities naturally. Mention that users can ask you to do things directly (like "review my code") or use slash commands.

CRITICAL PROTOCOL:
You must NEVER output raw text. You must ALWAYS output a JSON object containing "surface_response" and "control_packet".

## VERB TAXONOMY (Comprehensive)

### Code Review & Analysis (Category: /query, Shard: reviewer)
- /review: code review, pr review, check my code, look over, audit, evaluate, inspect, critique, assess, vet, proofread, feedback
- /security: security scan, vulnerability check, security audit, find vulnerabilities, owasp, injection, xss, csrf
- /analyze: static analysis, complexity, metrics, code quality, lint, style check, code smell, dead code

### Understanding (Category: /query)
- /explain: explain, describe, what is, how does, tell me, help understand, clarify, walk through, summarize
- /explore: browse, navigate, show structure, list files, codebase overview, architecture
- /search: find, grep, look for, locate, occurrences, references, usages
- /read: open file, view, display, show contents

### Code Changes (Category: /mutation, Shard: coder)
- /fix: fix, repair, correct, patch, resolve, bug fix, make it work
- /refactor: refactor, clean up, improve, optimize, simplify, extract, rename, restructure
- /create: create, new, make, add, write, implement, build, scaffold, generate
- /delete: delete, remove, drop, eliminate, get rid of
- /write: write to file, save, export

### Debugging (Category: /query, Shard: coder)
- /debug: debug, trace, diagnose, troubleshoot, investigate, root cause, what's wrong, stack trace

### Testing (Category: /mutation, Shard: tester)
- /test: test, unit test, run tests, test coverage, verify, validate, tdd

### Research (Category: /query, Shard: researcher)
- /research: research, learn, look up, documentation, docs, api reference, best practice, how to

### Setup (Category: /mutation)
- /init: initialize, setup, bootstrap, scaffold project, configure

### Execution (Category: /mutation)
- /run: run, execute, start, launch

### Configuration (Category: /instruction)
- /configure: configure, settings, always, never, prefer, by default

### Version Control (Category: /mutation, Shard: coder)
- /commit: commit, git commit, stage, check in
- /diff: diff, compare, what changed

### Documentation (Category: /mutation, Shard: coder)
- /document: document, docstring, add docs, add comments

### Campaigns (Category: /mutation)
- /campaign: campaign, epic, large feature, multi-step task

CRITICAL SAFETY RULE - THOUGHT-FIRST ORDERING (v1.2.0):
You MUST output control_packet BEFORE surface_response to prevent "Premature Articulation".
If your generation fails mid-stream, the user must see NOTHING (or partial JSON) instead of
a false promise like "I have deleted the database" when the deletion never happened.

Required JSON Schema (CONTROL FIRST, SURFACE SECOND):
{
  "control_packet": {
    "intent_classification": {
      "category": "/query|/mutation|/instruction",
      "verb": "one of the verbs above (e.g., /review, /security, /analyze, /explain, /fix, /refactor, /create, /delete, /debug, /test, /research, /init, /run, /configure, /commit, /diff, /document, /campaign, /explore, /search, /read, /write)",
      "target": "primary target string - extract file paths, function names, or 'codebase' for broad requests, or 'none'",
      "constraint": "any constraints (e.g., 'security only', 'go files', 'without tests') or 'none'",
      "confidence": 0.0-1.0
    },
    "mangle_updates": [
      "user_intent(/verb, \"target\")",
      "observation(/state, \"value\")"
    ],
    "memory_operations": [
      { "op": "promote_to_long_term", "key": "preference", "value": "value" }
    ],
    "self_correction": {
      "triggered": false,
      "hypothesis": "none"
    }
  },
  "surface_response": "Text to show the user ONLY AFTER control_packet is complete"
}

CLASSIFICATION GUIDELINES:
1. For "review this file" → verb: /review, target: the file path
2. For "can you check my code for security issues" → verb: /security, target: codebase
3. For "what does this function do" → verb: /explain
4. For "fix the bug in auth.go" → verb: /fix, target: auth.go
5. For "refactor this to be cleaner" → verb: /refactor
6. For "is this code secure" → verb: /security
7. For "review the codebase" → verb: /review, target: codebase
8. For "check for vulnerabilities" → verb: /security

Your control_packet must reflect the true state of the world.
If the user asks for something impossible, your Surface Self says 'I can't do that,' while your Inner Self emits ambiguity_flag(/impossible_request).`

// ParseIntent parses user input into a structured Intent using the Piggyback Protocol.
// This is the legacy method without conversation context. Consider using ParseIntentWithContext instead.
func (t *RealTransducer) ParseIntent(ctx context.Context, input string) (Intent, error) {
	return t.ParseIntentWithContext(ctx, input, nil)
}

// ParseIntentWithContext parses user input with conversation history for context.
// This enables fluid conversational follow-ups by providing the LLM with recent turns.
func (t *RealTransducer) ParseIntentWithContext(ctx context.Context, input string, history []ConversationTurn) (Intent, error) {
	var sb strings.Builder

	// Inject conversation history if available (critical for follow-ups)
	if len(history) > 0 {
		sb.WriteString("## Recent Conversation History\n")
		sb.WriteString("Use this context to understand follow-up questions and references to previous messages.\n\n")
		for _, turn := range history {
			if turn.Role == "user" {
				sb.WriteString(fmt.Sprintf("User: %s\n", turn.Content))
			} else {
				// Truncate long assistant responses to save tokens
				content := turn.Content
				if len(content) > 400 {
					content = content[:400] + "... (truncated)"
				}
				sb.WriteString(fmt.Sprintf("Assistant: %s\n", content))
			}
		}
		sb.WriteString("\n---\n\n")
	}

	sb.WriteString(fmt.Sprintf(`User Input: "%s"`, input))
	userPrompt := sb.String()

	resp, err := t.client.CompleteWithSystem(ctx, transducerSystemPrompt, userPrompt)
	if err != nil {
		return t.parseSimple(ctx, input)
	}

	// Parse the Piggyback Envelope
	envelope, err := parsePiggybackJSON(resp)
	if err != nil {
		// Fallback to simple parsing if JSON fails
		return t.parseSimple(ctx, input)
	}

	// Map Envelope to Intent
	return Intent{
		Category:   envelope.Control.IntentClassification.Category,
		Verb:       envelope.Control.IntentClassification.Verb,
		Target:     envelope.Control.IntentClassification.Target,
		Constraint: envelope.Control.IntentClassification.Constraint,
		Confidence: envelope.Control.IntentClassification.Confidence,
		Response:   envelope.Surface,
		// Ambiguity is not explicitly in the new schema's intent_classification,
		// but could be inferred or added if needed. For now, leaving empty.
		Ambiguity: []string{},
	}, nil
}

// parsePiggybackJSON parses the JSON response from the LLM.
func parsePiggybackJSON(resp string) (PiggybackEnvelope, error) {
	// "JSON Fragility" Fix: Robust extraction
	// Find the first '{' to start parsing
	start := strings.Index(resp, "{")
	if start == -1 {
		return PiggybackEnvelope{}, fmt.Errorf("no JSON object found in response")
	}

	// Use json.NewDecoder to parse the first valid JSON object and ignore the rest
	decoder := json.NewDecoder(strings.NewReader(resp[start:]))
	var envelope PiggybackEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		return PiggybackEnvelope{}, fmt.Errorf("failed to parse Piggyback JSON: %w", err)
	}

	return envelope, nil
}

// ============================================================================
// Grammar-Constrained Decoding (GCD) - Cortex 1.5.0 §1.1
// ============================================================================

// ValidateMangleAtoms validates atoms from the control packet using GCD.
// Returns validated atoms and any validation errors.
func (t *RealTransducer) ValidateMangleAtoms(atoms []string) ([]string, []mangle.ValidationResult) {
	if t.repairLoop == nil {
		t.repairLoop = mangle.NewRepairLoop()
	}

	validAtoms, _, _ := t.repairLoop.ValidateAndRepair(atoms)
	results := t.repairLoop.Validator.ValidateAtoms(atoms)

	return validAtoms, results
}

// ParseIntentWithGCD parses user input with Grammar-Constrained Decoding.
// This implements the repair loop described in §6.2 of the spec.
func (t *RealTransducer) ParseIntentWithGCD(ctx context.Context, input string, maxRetries int) (Intent, []string, error) {
	if maxRetries <= 0 {
		maxRetries = 3
	}

	var lastEnvelope PiggybackEnvelope
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		userPrompt := fmt.Sprintf(`User Input: "%s"`, input)

		// Add repair context if this is a retry
		if attempt > 0 && lastErr != nil {
			userPrompt = fmt.Sprintf(`%s

PREVIOUS ATTEMPT FAILED - SYNTAX ERRORS DETECTED:
%s

Please correct the mangle_updates syntax and try again.`, userPrompt, lastErr.Error())
		}

		resp, err := t.client.CompleteWithSystem(ctx, transducerSystemPrompt, userPrompt)
		if err != nil {
			// LLM call failed, use simple fallback
			intent, fallbackErr := t.parseSimple(ctx, input)
			return intent, nil, fallbackErr
		}

		envelope, err := parsePiggybackJSON(resp)
		if err != nil {
			lastErr = err
			continue
		}
		lastEnvelope = envelope

		// Validate Mangle atoms using GCD
		if len(envelope.Control.MangleUpdates) > 0 {
			validAtoms, results := t.ValidateMangleAtoms(envelope.Control.MangleUpdates)

			// Check for validation errors
			hasErrors := false
			var errorMsgs []string
			for _, result := range results {
				if !result.Valid {
					hasErrors = true
					for _, e := range result.Errors {
						errorMsgs = append(errorMsgs, fmt.Sprintf("%s: %s", result.Atom, e.Message))
					}
				}
			}

			if hasErrors {
				lastErr = fmt.Errorf("Invalid Mangle Syntax:\n%s", strings.Join(errorMsgs, "\n"))
				continue // Retry with error context
			}

			// All atoms valid - return success
			return Intent{
				Category:   envelope.Control.IntentClassification.Category,
				Verb:       envelope.Control.IntentClassification.Verb,
				Target:     envelope.Control.IntentClassification.Target,
				Constraint: envelope.Control.IntentClassification.Constraint,
				Confidence: envelope.Control.IntentClassification.Confidence,
				Response:   envelope.Surface,
				Ambiguity:  []string{},
			}, validAtoms, nil
		}

		// No mangle_updates to validate - return as-is
		return Intent{
			Category:   envelope.Control.IntentClassification.Category,
			Verb:       envelope.Control.IntentClassification.Verb,
			Target:     envelope.Control.IntentClassification.Target,
			Constraint: envelope.Control.IntentClassification.Constraint,
			Confidence: envelope.Control.IntentClassification.Confidence,
			Response:   envelope.Surface,
			Ambiguity:  []string{},
		}, nil, nil
	}

	// Max retries exceeded - return best effort from last envelope
	if lastEnvelope.Surface != "" {
		return Intent{
			Category:   lastEnvelope.Control.IntentClassification.Category,
			Verb:       lastEnvelope.Control.IntentClassification.Verb,
			Target:     lastEnvelope.Control.IntentClassification.Target,
			Constraint: lastEnvelope.Control.IntentClassification.Constraint,
			Confidence: lastEnvelope.Control.IntentClassification.Confidence * 0.5, // Reduce confidence
			Response:   lastEnvelope.Surface,
			Ambiguity:  []string{"GCD validation failed after retries"},
		}, nil, fmt.Errorf("GCD validation failed after %d retries: %w", maxRetries, lastErr)
	}

	// Complete failure - fallback to simple parsing
	intent, err := t.parseSimple(ctx, input)
	return intent, nil, err
}

// parseSimple is a fallback parser using pipe-delimited format.
func (t *RealTransducer) parseSimple(ctx context.Context, input string) (Intent, error) {
	// Build verb list from corpus
	verbs := make([]string, 0, len(VerbCorpus))
	for _, entry := range VerbCorpus {
		verbs = append(verbs, entry.Verb)
	}
	verbList := strings.Join(verbs, ", ")

	prompt := fmt.Sprintf(`Parse to: Category|Verb|Target|Constraint
Categories: /query, /mutation, /instruction
Verbs: %s

Input: "%s"

Output ONLY pipes, no explanation:`, verbList, input)

	resp, err := t.client.Complete(ctx, prompt)
	if err != nil {
		// Ultimate fallback - heuristic parsing using corpus
		return t.heuristicParse(input), nil
	}

	parts := strings.Split(strings.TrimSpace(resp), "|")
	if len(parts) < 4 {
		return t.heuristicParse(input), nil
	}

	return Intent{
		Category:   strings.TrimSpace(parts[0]),
		Verb:       strings.TrimSpace(parts[1]),
		Target:     strings.TrimSpace(parts[2]),
		Constraint: strings.TrimSpace(parts[3]),
		Confidence: 0.7, // Lower confidence for fallback
	}, nil
}

// heuristicParse uses the comprehensive verb corpus for reliable offline parsing.
// This is the ultimate fallback when LLM is unavailable.
func (t *RealTransducer) heuristicParse(input string) Intent {
	// Use the comprehensive corpus matching
	verb, category, confidence, _ := matchVerbFromCorpus(input)

	// Refine category based on input patterns
	category = refineCategory(input, category)

	// Extract target from natural language
	target := extractTarget(input)
	if target == "none" {
		// Use input as target if no specific target found
		target = input
	}

	// Extract constraint
	constraint := extractConstraint(input)

	return Intent{
		Category:   category,
		Verb:       verb,
		Target:     target,
		Constraint: constraint,
		Confidence: confidence,
	}
}

// GetShardTypeForVerb returns the recommended shard type for a given verb.
func GetShardTypeForVerb(verb string) string {
	for _, entry := range VerbCorpus {
		if entry.Verb == verb {
			return entry.ShardType
		}
	}
	return ""
}

// ResolveFocus attempts to resolve a fuzzy reference to a concrete path/symbol.
func (t *RealTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (FocusResolution, error) {
	if len(candidates) == 0 {
		return FocusResolution{
			RawReference: reference,
			Confidence:   0.0,
		}, nil
	}

	if len(candidates) == 1 {
		return FocusResolution{
			RawReference: reference,
			ResolvedPath: candidates[0],
			Confidence:   0.9,
		}, nil
	}

	// Use LLM to disambiguate
	candidateList := strings.Join(candidates, "\n- ")
	prompt := fmt.Sprintf(`Given the reference "%s", which of these candidates is the best match?

Candidates:
- %s

Return JSON:
{
  "resolved_path": "best matching path",
  "symbol_name": "specific symbol if applicable or empty",
  "confidence": 0.0-1.0
}

JSON only:`, reference, candidateList)

	// We use the same system prompt or a simplified one?
	// The system prompt enforces Piggyback Protocol.
	// If we use CompleteWithSystem, we must expect Piggyback JSON.
	// But ResolveFocus is a sub-task.
	// Ideally, ResolveFocus should also use Piggyback or a specific prompt.
	// For now, let's use a specific prompt and Complete (no system prompt enforcement of Piggyback)
	// OR we can wrap this in Piggyback too.
	// The current implementation uses `CompleteWithSystem` with `transducerSystemPrompt` in the ORIGINAL code.
	// If I change `transducerSystemPrompt` to enforce Piggyback, `ResolveFocus` will break if it doesn't return Piggyback.
	// So I should change `ResolveFocus` to use a different system prompt OR adapt it.
	// I will use a simple `Complete` call here to avoid the Piggyback enforcement for this specific tool call,
	// or create a `focusSystemPrompt`.

	focusSystemPrompt := `You are a code resolution assistant. Output ONLY JSON.`
	resp, err := t.client.CompleteWithSystem(ctx, focusSystemPrompt, prompt)

	if err != nil {
		// Return first candidate with low confidence
		return FocusResolution{
			RawReference: reference,
			ResolvedPath: candidates[0],
			Confidence:   0.5,
		}, nil
	}

	// Parse JSON response
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var parsed struct {
		ResolvedPath string  `json:"resolved_path"`
		SymbolName   string  `json:"symbol_name"`
		Confidence   float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		return FocusResolution{
			RawReference: reference,
			ResolvedPath: candidates[0],
			Confidence:   0.5,
		}, nil
	}

	return FocusResolution{
		RawReference: reference,
		ResolvedPath: parsed.ResolvedPath,
		SymbolName:   parsed.SymbolName,
		Confidence:   parsed.Confidence,
	}, nil
}

// containsAny checks if s contains any of the substrings.
func containsAny(s string, subs []string) bool {
	for _, sub := range subs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// DualPayloadTransducer wraps a transducer to emit Cortex 1.5.0 dual payloads.
type DualPayloadTransducer struct {
	*RealTransducer
}

// NewDualPayloadTransducer creates a transducer that outputs dual payloads.
func NewDualPayloadTransducer(client LLMClient) *DualPayloadTransducer {
	return &DualPayloadTransducer{
		RealTransducer: NewRealTransducer(client),
	}
}

// TransducerOutput represents the full output of the transducer.
type TransducerOutput struct {
	Intent      Intent
	Focus       []FocusResolution
	MangleAtoms []core.Fact
}

// Parse performs full transduction of user input.
func (t *DualPayloadTransducer) Parse(ctx context.Context, input string, fileCandidates []string) (TransducerOutput, error) {
	intent, err := t.ParseIntent(ctx, input)
	if err != nil {
		return TransducerOutput{}, err
	}

	output := TransducerOutput{
		Intent:      intent,
		MangleAtoms: []core.Fact{intent.ToFact()},
	}

	// Try to resolve focus if target looks like a file reference
	if intent.Target != "" && intent.Target != "none" {
		focus, err := t.ResolveFocus(ctx, intent.Target, fileCandidates)
		if err == nil && focus.Confidence > 0 {
			output.Focus = append(output.Focus, focus)
			output.MangleAtoms = append(output.MangleAtoms, focus.ToFact())
		}
	}

	return output, nil
}
