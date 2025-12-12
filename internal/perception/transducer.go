package perception

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"codenerd/internal/articulation"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
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
	Verb      string           // Canonical verb (e.g., "/review")
	Category  string           // Default category (/query, /mutation, /instruction)
	Synonyms  []string         // Words that map to this verb
	Patterns  []*regexp.Regexp // Regex patterns that indicate this verb
	Priority  int              // Higher priority wins in ambiguous cases
	ShardType string           // Which shard handles this (reviewer, coder, tester, researcher)
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
			logging.Get(logging.CategoryPerception).Error("Failed to load verb taxonomy from Mangle: %v", err)
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
			logging.Perception("Initialized VerbCorpus with fallback (1 verb)")
		} else {
			logging.Perception("Initialized VerbCorpus from Mangle taxonomy (%d verbs)", len(VerbCorpus))
		}
	} else {
		logging.Get(logging.CategoryPerception).Error("SharedTaxonomy is nil - cannot load verb taxonomy")
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

// truncateForLog truncates a string for logging purposes.
func truncateForLog(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// matchVerbFromCorpus finds the best matching verb using Regex Candidates + Semantic + Mangle Inference.
// The classification flow is:
//  1. Get regex candidates (fast path)
//  2. Semantic classification via vector search (injects semantic_match facts into kernel)
//  3. Mangle inference (sees both regex candidates and semantic signals)
//  4. Fallback to best regex score
func matchVerbFromCorpus(ctx context.Context, input string) (verb string, category string, confidence float64, shardType string) {
	timer := logging.StartTimer(logging.CategoryPerception, "matchVerbFromCorpus")
	defer timer.Stop()

	logging.PerceptionDebug("Matching verb for input: %q", truncateForLog(input, 100))

	// 1. Get Candidates via Regex (Fast)
	candidates := getRegexCandidates(input)
	logging.PerceptionDebug("Regex candidates found: %d", len(candidates))

	// 2. Semantic classification - inject semantic_match facts into kernel
	// This step enriches Mangle's inference with vector-based similarity signals.
	// The SemanticClassifier.Classify() method asserts semantic_match facts that
	// the Mangle inference rules can use for boosting/overriding verb selection.
	if SharedSemanticClassifier != nil {
		matches, err := SharedSemanticClassifier.Classify(ctx, input)
		if err != nil {
			// Non-fatal: continue with regex-only classification
			logging.PerceptionDebug("Semantic classification error (non-fatal): %v", err)
		} else if len(matches) > 0 {
			logging.PerceptionDebug("Semantic matches found: %d (top: %s %.2f)",
				len(matches), matches[0].Verb, matches[0].Similarity)
		} else {
			logging.PerceptionDebug("Semantic classification returned no matches")
		}
		// Facts are now in kernel - Mangle inference will see them via semantic_match predicate
	}

	// 3. Refine via Mangle Inference (Smart)
	// This applies the "sentence level" logic and context rules.
	// Now includes semantic_match facts from the classifier above.
	if SharedTaxonomy != nil && len(candidates) > 0 {
		bestVerb, conf, err := SharedTaxonomy.ClassifyInput(input, candidates)
		if err == nil && bestVerb != "" {
			logging.PerceptionDebug("Mangle inference selected verb: %s (confidence: %.2f)", bestVerb, conf)
			// Find the candidate entry to get category/shard details
			for _, c := range candidates {
				if c.Verb == bestVerb {
					logging.Perception("Matched verb %s (category: %s, shard: %s, confidence: %.2f)", c.Verb, c.Category, c.ShardType, conf)
					return c.Verb, c.Category, conf, c.ShardType
				}
			}
			// If not found in candidates (rare), find in corpus
			for _, entry := range VerbCorpus {
				if entry.Verb == bestVerb {
					logging.Perception("Matched verb %s from corpus (category: %s, shard: %s, confidence: %.2f)", entry.Verb, entry.Category, entry.ShardType, conf)
					return entry.Verb, entry.Category, conf, entry.ShardType
				}
			}
		} else if err != nil {
			logging.PerceptionDebug("Mangle inference error: %v", err)
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
			logging.Perception("Regex fallback matched verb %s (category: %s, confidence: %.2f)", bestCand.Verb, bestCand.Category, confidence)
			return bestCand.Verb, bestCand.Category, confidence, bestCand.ShardType
		}
	}

	logging.PerceptionDebug("No verb match found, defaulting to /explain")
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
	Category         string            // /query, /mutation, /instruction
	Verb             string            // /explain, /refactor, /debug, /generate, /init, /research, /remember, etc.
	Target           string            // Primary target of the action
	Constraint       string            // Constraints on the action
	Confidence       float64           // Confidence score for the intent
	Ambiguity        []string          // Ambiguous parts that need clarification
	Response         string            // Natural language response (Piggyback Protocol)
	MemoryOperations []MemoryOperation // Memory operations for learning/forgetting (Cold Storage)
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
			core.MangleAtom("/current_intent"), // ID as name constant
			core.MangleAtom(i.Category),
			core.MangleAtom(i.Verb),
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
	// ConfidencePercent is 0-100 (integer) to keep Mangle numeric comparisons stable.
	// The kernel policy uses integer thresholds (e.g. Score < 85).
	ConfidencePercent int64
}

// ToFact converts focus resolution to a Mangle Fact.
func (f FocusResolution) ToFact() core.Fact {
	return core.Fact{
		Predicate: "focus_resolution",
		Args: []interface{}{
			f.RawReference,
			f.ResolvedPath,
			f.SymbolName,
			f.ConfidencePercent,
		},
	}
}

// Transducer defines the interface for the perception layer.
type Transducer interface {
	ParseIntent(ctx context.Context, input string) (Intent, error)
	ResolveFocus(ctx context.Context, reference string, candidates []string) (FocusResolution, error)
}

// RealTransducer implements the Perception layer (NL → Piggyback control_packet → intents).
//
// It supports JIT prompt compilation when provided an articulation.PromptAssembler, and
// falls back to the legacy static prompt when JIT is unavailable.
type RealTransducer struct {
	client          LLMClient
	repairLoop      *mangle.RepairLoop // GCD repair loop for Mangle syntax validation
	promptAssembler *articulation.PromptAssembler
}

// NewRealTransducer creates a new transducer with the given LLM client.
func NewRealTransducer(client LLMClient) *RealTransducer {
	logging.Perception("Initializing RealTransducer with LLM client")
	t := &RealTransducer{
		client:     client,
		repairLoop: mangle.NewRepairLoop(),
	}
	logging.PerceptionDebug("RealTransducer initialized with repair loop")
	return t
}

// SetPromptAssembler injects the PromptAssembler used for JIT prompt compilation.
// When unset or JIT is unavailable, the transducer uses the legacy static prompt.
func (t *RealTransducer) SetPromptAssembler(pa *articulation.PromptAssembler) {
	t.promptAssembler = pa
}

// withSystemContext creates a SystemLLMContext for this transducer's LLM calls.
// This ensures proper trace attribution for all transducer operations.
func (t *RealTransducer) withSystemContext(taskContext string) *SystemLLMContext {
	return NewSystemLLMContext(t.client, "transducer", taskContext)
}

func (t *RealTransducer) intentSystemPrompt(ctx context.Context) string {
	if t.promptAssembler == nil {
		return transducerSystemPrompt
	}

	// Prefer the JIT-compiled prompt when available. The assembler will fall back
	// internally, but for Perception we also keep the legacy prompt as a known-safe
	// fallback if assembly fails.
	pc := &articulation.PromptContext{
		ShardID:   "perception_transducer",
		ShardType: "perception",
	}
	if promptText, err := t.promptAssembler.AssembleSystemPrompt(ctx, pc); err == nil && strings.TrimSpace(promptText) != "" {
		return promptText
	}
	return transducerSystemPrompt
}

// Cortex 1.5.0 Piggyback Protocol System Prompt
// Updated with comprehensive verb taxonomy for reliable intent classification.
//
// NOTE: The content of this prompt has been migrated to YAML atoms in
// internal/prompt/atoms/perception/ for better organization and
// documentation. However, the transducer continues to use this static prompt
// due to import cycle constraints with the articulation package.
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
- /campaign assault [scope] [include...] - Start adversarial assault sweep

### Advanced Features
- **Campaigns**: Multi-phase, long-running tasks with planning and tracking
- **Tool Generation (Autopoiesis)**: Can create new tools at runtime if needed
- **Learning**: Remembers preferences and patterns across sessions
- **Git Integration**: Commits, diffs, branch awareness

When greeting or asked about capabilities, describe these abilities naturally. Mention that users can ask you to do things directly (like "review my code") or use slash commands.

CRITICAL PROTOCOL:
You must NEVER output raw text. You must ALWAYS output a JSON object containing "surface_response" and "control_packet".

## INTENT LIBRARY (Match User to Canonical Examples)
Instead of guessing verbs, match the user's request to the closest CANONICAL EXAMPLE.

| Canonical Request (The "Archetype") | Mangle Action (The "Logic") |
|-------------------------------------|-----------------------------|
| "Review this file for bugs." | {verb: "/review", target: "context_file", category: "/query"} |
| "Check my code for security issues." | {verb: "/security", target: "codebase", category: "/query"} |
| "Analyze this codebase structure." | {verb: "/analyze", target: "codebase", category: "/query"} |
| "Fix the compilation error." | {verb: "/fix", constraint: "compiler_error", category: "/mutation"} |
| "Refactor this function to be cleaner." | {verb: "/refactor", target: "focused_symbol", category: "/mutation"} |
| "What does this function do?" | {verb: "/explain", target: "focused_symbol", category: "/query"} |
| "Why is this test failing?" | {verb: "/debug", target: "test", category: "/query"} |
| "Run the tests." | {verb: "/test", target: "context_file", category: "/mutation"} |
| "Generate unit tests for this." | {verb: "/test", target: "context_file", category: "/mutation"} |
| "Build the project." | {verb: "/build", target: "project", category: "/mutation"} |
| "Run the application." | {verb: "/run", target: "application", category: "/mutation"} |
| "Deploy to production." | {verb: "/deploy", target: "production", category: "/mutation"} |
| "Research how to use X." | {verb: "/research", target: "X", category: "/query"} |
| "Create a new file called main.go." | {verb: "/create", target: "main.go", category: "/mutation"} |
| "Write this to config.json." | {verb: "/write", target: "config.json", category: "/mutation"} |
| "Read the contents of main.go." | {verb: "/read", target: "main.go", category: "/query"} |
| "Delete the database." | {verb: "/delete", target: "database", category: "/mutation"} |
| "Rename getUserById to fetchUser." | {verb: "/rename", target: "getUserById", category: "/mutation"} |
| "Move this function to utils.go." | {verb: "/move", target: "function", category: "/mutation"} |
| "Search for all TODO comments." | {verb: "/search", target: "TODO", category: "/query"} |
| "Find where this function is called." | {verb: "/search", target: "function_usages", category: "/query"} |
| "Explore the project structure." | {verb: "/explore", target: "project", category: "/query"} |
| "Format this file." | {verb: "/format", target: "context_file", category: "/mutation"} |
| "Lint the codebase." | {verb: "/lint", target: "codebase", category: "/query"} |
| "Install the dependencies." | {verb: "/install", target: "dependencies", category: "/mutation"} |
| "Update all packages." | {verb: "/update", target: "packages", category: "/mutation"} |
| "Commit these changes." | {verb: "/commit", target: "changes", category: "/mutation"} |
| "Show me the diff." | {verb: "/diff", target: "changes", category: "/query"} |
| "What's the git status?" | {verb: "/git", target: "status", category: "/query"} |
| "Push to origin." | {verb: "/git", target: "push", category: "/mutation"} |
| "Scaffold a new REST endpoint." | {verb: "/scaffold", target: "REST endpoint", category: "/mutation"} |
| "Generate boilerplate for a service." | {verb: "/scaffold", target: "service", category: "/mutation"} |
| "Document this function." | {verb: "/document", target: "function", category: "/mutation"} |
| "Add JSDoc comments to this file." | {verb: "/document", target: "file", category: "/mutation"} |
| "Start a campaign to rewrite auth." | {verb: "/campaign", target: "rewrite auth", category: "/mutation"} |
| "Run an assault campaign on internal/core." | {verb: "/assault", target: "internal/core", category: "/mutation"} |
| "Plan how to implement feature X." | {verb: "/plan", target: "feature X", category: "/query"} |
| "Summarize what this module does." | {verb: "/summarize", target: "module", category: "/query"} |
| "How many files are in the codebase?" | {verb: "/stats", target: "codebase", category: "/query"} |
| "What can you do?" | {verb: "/help", target: "capabilities", category: "/query"} |
| "Hello!" | {verb: "/greet", target: "none", category: "/query"} |
| "What do you remember about X?" | {verb: "/knowledge", target: "X", category: "/query"} |
| "What if I told you to recompile the binaries?" | {verb: "/dream", target: "recompile binaries", category: "/query"} |
| "Imagine you had to migrate the database." | {verb: "/dream", target: "migrate database", category: "/query"} |
| "Walk me through how you'd implement auth." | {verb: "/dream", target: "implement auth", category: "/query"} |
| "Think about what tools you'd need to deploy this." | {verb: "/dream", target: "deploy", category: "/query"} |
| "Hypothetically, how would you refactor this?" | {verb: "/dream", target: "refactor", category: "/query"} |
| "Configure the agent to be verbose." | {verb: "/configure", target: "verbosity", category: "/instruction"} |
| "Remember that X." | {verb: "/remember", target: "X", category: "/instruction"} + memory_operations |
| "Always do Y when Z." | {verb: "/remember", target: "preference", category: "/instruction"} + memory_operations |
| "Learn this: my agents are Coder, Tester, Reviewer, Researcher." | {verb: "/remember", target: "fact", category: "/instruction"} + memory_operations |
| "Stop what you're doing." | {verb: "/stop", target: "current_task", category: "/instruction"} |
| "Cancel that." | {verb: "/stop", target: "last_action", category: "/instruction"} |
| "Undo the last change." | {verb: "/undo", target: "last_change", category: "/instruction"} |

### Mangle Inference Rules
1. If the user's request matches a Canonical Example, use that example's Action.
2. If the user's request is ambiguous, output: ambiguity_flag(/ambiguous_intent).
3. If the user's request violates a safety rule, use the Mangle Kernel to validate it.

CRITICAL SAFETY RULE - THOUGHT-FIRST ORDERING (v1.2.0):
You MUST output control_packet BEFORE surface_response to prevent "Premature Articulation".
If your generation fails mid-stream, the user must see NOTHING (or partial JSON) instead of
a false promise like "I have deleted the database" when the deletion never happened.

Required JSON Schema (CONTROL FIRST, SURFACE SECOND):
{
  "control_packet": {
    "intent_classification": {
      "category": "/query|/mutation|/instruction",
      "verb": "/review|/security|/analyze|/fix|/refactor|/explain|/debug|/test|/build|/run|/deploy|/research|/create|/write|/read|/delete|/rename|/move|/search|/explore|/format|/lint|/install|/update|/commit|/diff|/git|/scaffold|/document|/campaign|/assault|/plan|/summarize|/stats|/help|/greet|/knowledge|/dream|/configure|/remember|/stop|/undo",
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

QUERIES (category: /query):
- "review", "check", "look at" → /review
- "secure?", "vulnerabilities", "security audit" → /security
- "analyze", "examine", "inspect" → /analyze
- "what does", "explain", "how does", "why" → /explain
- "debug", "troubleshoot", "why failing", "root cause" → /debug
- "find", "search", "grep", "where is" → /search
- "explore", "show structure", "what's in" → /explore
- "read", "show", "cat", "display contents" → /read
- "diff", "changes", "what changed" → /diff
- "lint", "style check", "code quality" → /lint
- "plan", "how should I", "design" → /plan
- "summarize", "tldr", "overview" → /summarize
- "stats", "count", "how many", "metrics" → /stats
- "help", "what can you", "capabilities" → /help
- "hello", "hi", "hey" → /greet
- "what do you remember", "your memory" → /knowledge
- "what if", "imagine", "hypothetically", "walk me through", "think about how", "dream" → /dream (simulation/learning mode)
- "research", "learn about", "documentation" → /research

MUTATIONS (category: /mutation):
- "fix", "repair", "patch", "resolve" → /fix
- "refactor", "clean up", "improve" → /refactor
- "create", "new", "add", "implement" → /create
- "write", "save", "output to" → /write
- "delete", "remove", "drop" → /delete
- "rename", "change name" → /rename
- "move", "relocate", "transfer" → /move
- "test", "run tests", "unit test" → /test
- "build", "compile", "make" → /build
- "run", "execute", "start" → /run
- "deploy", "ship", "release" → /deploy
- "format", "prettify", "indent" → /format
- "install", "add package", "npm install" → /install
- "update", "upgrade", "bump version" → /update
- "commit", "save changes", "check in" → /commit
- "git push", "git pull", "git checkout" → /git
- "scaffold", "boilerplate", "generate skeleton" → /scaffold
- "document", "add comments", "jsdoc" → /document
- "campaign", "epic", "multi-step project" → /campaign
- "assault", "assault campaign", "adversarial assault", "soak test", "stress test", "gauntlet" → /assault

INSTRUCTIONS (category: /instruction):
- "configure", "set", "change setting" → /configure
- "remember", "learn", "note that", "always", "never", "from now on" → /remember + memory_operations
- "stop", "cancel", "abort", "halt" → /stop
- "undo", "revert", "rollback" → /undo

DREAM STATE (/dream verb):
When the user asks "what if", "imagine", "hypothetically", or "walk me through":
- This is a SIMULATION/LEARNING mode - DO NOT execute anything
- Think through the hypothetical task step-by-step
- In your surface_response, provide:
  1. **Task Analysis**: What is being asked?
  2. **Shards I'd Consult**: Which agents (Coder/Tester/Reviewer/Researcher) would help?
  3. **Steps I'd Take**: Numbered action plan
  4. **Tools Required**: Existing tools I'd use
  5. **Tools I'd Need to Create**: Missing tools (Autopoiesis candidates)
  6. **Risks/Concerns**: What could go wrong?
  7. **Questions for You**: Clarifications I'd need
- End with: "This is a dry run. Correct me if my approach is wrong - I'll learn from your feedback."
- If the user provides corrections, treat them as /remember instructions

MEMORY OPERATIONS:
When the user asks you to "remember", "learn", "always", "never", or "from now on":
- Set verb: /remember, category: /instruction
- Add a memory_operations entry: { "op": "promote_to_long_term", "key": "<topic>", "value": "<what to remember>" }
- Respond with confirmation like "Got it, I'll remember that." or "Understood, I've noted that preference."

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
	timer := logging.StartTimer(logging.CategoryPerception, "ParseIntentWithContext")
	defer timer.Stop()

	logging.Perception("Parsing intent for input: %d chars", len(input))
	logging.PerceptionDebug("Raw input: %q", truncateForLog(input, 200))
	logging.PerceptionDebug("Conversation history: %d turns", len(history))

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

	logging.PerceptionDebug("Calling LLM for intent extraction (prompt: %d chars)", len(userPrompt))

	// Set system context for trace attribution
	sysCtx := t.withSystemContext("intent-extraction")
	defer sysCtx.Clear()

	llmTimer := logging.StartTimer(logging.CategoryPerception, "LLM-CompleteWithSystem")
	resp, err := sysCtx.CompleteWithSystem(ctx, t.intentSystemPrompt(ctx), userPrompt)
	llmTimer.Stop()

	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("LLM call failed, falling back to simple parsing: %v", err)
		return t.parseSimple(ctx, input)
	}

	logging.PerceptionDebug("LLM response received: %d chars", len(resp))

	processor := articulation.NewResponseProcessor()
	processor.RequireValidJSON = true
	result, err := processor.Process(resp)
	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("Failed to parse Piggyback JSON, falling back to simple parsing: %v", err)
		return t.parseSimple(ctx, input)
	}

	logging.Perception("Intent parsed: category=%s, verb=%s, target=%s, confidence=%.2f",
		result.Control.IntentClassification.Category,
		result.Control.IntentClassification.Verb,
		truncateForLog(result.Control.IntentClassification.Target, 50),
		result.Control.IntentClassification.Confidence)

	if len(result.Control.MemoryOperations) > 0 {
		logging.PerceptionDebug("Memory operations: %d", len(result.Control.MemoryOperations))
	}

	// Map Envelope to Intent
	return Intent{
		Category:         result.Control.IntentClassification.Category,
		Verb:             result.Control.IntentClassification.Verb,
		Target:           result.Control.IntentClassification.Target,
		Constraint:       result.Control.IntentClassification.Constraint,
		Confidence:       result.Control.IntentClassification.Confidence,
		Response:         result.Surface,
		MemoryOperations: result.Control.MemoryOperations,
		// Ambiguity is not explicitly in the new schema's intent_classification,
		// but could be inferred or added if needed. For now, leaving empty.
		Ambiguity: []string{},
	}, nil
}

// parsePiggybackJSON parses the JSON response from the LLM.
func parsePiggybackJSON(resp string) (PiggybackEnvelope, error) {
	logging.PerceptionDebug("Parsing Piggyback JSON from response")

	// "JSON Fragility" Fix: Robust extraction
	// Find the first '{' to start parsing
	start := strings.Index(resp, "{")
	if start == -1 {
		logging.Get(logging.CategoryPerception).Error("No JSON object found in LLM response")
		return PiggybackEnvelope{}, fmt.Errorf("no JSON object found in response")
	}

	if start > 0 {
		logging.PerceptionDebug("JSON starts at offset %d (skipping preamble)", start)
	}

	// Use json.NewDecoder to parse the first valid JSON object and ignore the rest
	decoder := json.NewDecoder(strings.NewReader(resp[start:]))
	var envelope PiggybackEnvelope
	if err := decoder.Decode(&envelope); err != nil {
		logging.Get(logging.CategoryPerception).Error("Failed to decode Piggyback JSON: %v", err)
		return PiggybackEnvelope{}, fmt.Errorf("failed to parse Piggyback JSON: %w", err)
	}

	logging.PerceptionDebug("Piggyback envelope parsed successfully")
	return envelope, nil
}

// ============================================================================
// Grammar-Constrained Decoding (GCD) - Cortex 1.5.0 §1.1
// ============================================================================

// ValidateMangleAtoms validates atoms from the control packet using GCD.
// Returns validated atoms and any validation errors.
func (t *RealTransducer) ValidateMangleAtoms(atoms []string) ([]string, []mangle.ValidationResult) {
	logging.PerceptionDebug("Validating %d Mangle atoms via GCD", len(atoms))

	if t.repairLoop == nil {
		logging.PerceptionDebug("Initializing repair loop (was nil)")
		t.repairLoop = mangle.NewRepairLoop()
	}

	validAtoms, _, _ := t.repairLoop.ValidateAndRepair(atoms)
	results := t.repairLoop.Validator.ValidateAtoms(atoms)

	validCount := 0
	for _, r := range results {
		if r.Valid {
			validCount++
		}
	}
	logging.PerceptionDebug("Mangle atom validation: %d/%d valid", validCount, len(atoms))

	return validAtoms, results
}

// ParseIntentWithGCD parses user input with Grammar-Constrained Decoding.
// This implements the repair loop described in S6.2 of the spec.
func (t *RealTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []ConversationTurn, maxRetries int) (Intent, []string, error) {
	timer := logging.StartTimer(logging.CategoryPerception, "ParseIntentWithGCD")
	defer timer.Stop()

	if maxRetries <= 0 {
		maxRetries = 3
	}

	logging.Perception("Parsing intent with GCD (maxRetries: %d, input: %d chars)", maxRetries, len(input))
	logging.PerceptionDebug("GCD input: %q", truncateForLog(input, 200))

	// Set system context for trace attribution
	sysCtx := t.withSystemContext("intent-extraction-gcd")
	defer sysCtx.Clear()

	var lastResult *articulation.ArticulationResult
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		logging.PerceptionDebug("GCD attempt %d/%d", attempt+1, maxRetries)

		var sb strings.Builder

		// Inject conversation history if available (critical for follow-ups)
		if len(history) > 0 {
			sb.WriteString("## Recent Conversation History\n")
			sb.WriteString("Use this context to understand follow-up questions and references to previous messages.\n\n")
			for _, turn := range history {
				if turn.Role == "user" {
					sb.WriteString(fmt.Sprintf("User: %s\n", turn.Content))
				} else {
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

		// Add repair context if this is a retry
		if attempt > 0 && lastErr != nil {
			logging.PerceptionDebug("GCD retry: injecting error context from previous attempt")
			userPrompt = fmt.Sprintf(`%s

PREVIOUS ATTEMPT FAILED - VALIDATION ERRORS DETECTED:
%s

Fix the issues and try again. Output valid Piggyback JSON with corrected mangle_updates.`, userPrompt, lastErr.Error())
		}

		systemPrompt := t.intentSystemPrompt(ctx)
		var resp string
		var streamedAtoms []string
		usedStreaming := false

		llmTimer := logging.StartTimer(logging.CategoryPerception, "GCD-LLM-Call")
		streamResp, streamAtoms, streamErr := t.completeWithGCDStreaming(ctx, sysCtx, systemPrompt, userPrompt, 4096)
		llmTimer.Stop()

		if streamErr == nil {
			resp = streamResp
			streamedAtoms = streamAtoms
			usedStreaming = true
		} else {
			var abortErr *gcdStreamAbortError
			if errors.As(streamErr, &abortErr) {
				logging.PerceptionDebug("GCD attempt %d: stream gate aborted early: %v", attempt+1, streamErr)
				lastErr = streamErr
				continue
			}
		}

		if streamErr != nil && errors.Is(streamErr, ErrStreamingNotSupported) {
			// Fallback to non-streaming if the active provider doesn't support streaming.
			llmTimer := logging.StartTimer(logging.CategoryPerception, "GCD-LLM-Call")
			raw, err := sysCtx.CompleteWithSystem(ctx, systemPrompt, userPrompt)
			llmTimer.Stop()
			if err != nil {
				logging.Get(logging.CategoryPerception).Warn("GCD LLM call failed, falling back to simple: %v", err)
				intent, fallbackErr := t.parseSimple(ctx, input)
				return intent, nil, fallbackErr
			}
			resp = raw
		} else if streamErr != nil {
			logging.Get(logging.CategoryPerception).Warn("GCD streaming call failed, falling back to simple: %v", streamErr)
			intent, fallbackErr := t.parseSimple(ctx, input)
			return intent, nil, fallbackErr
		}

		processor := articulation.NewResponseProcessor()
		processor.RequireValidJSON = true
		result, err := processor.Process(resp)
		if err != nil {
			logging.PerceptionDebug("GCD attempt %d: JSON parse failed: %v", attempt+1, err)
			lastErr = err
			continue
		}
		lastResult = result

		// Validate Mangle atoms using GCD
		if len(result.Control.MangleUpdates) > 0 {
			logging.PerceptionDebug("GCD validating %d Mangle updates", len(result.Control.MangleUpdates))
			if usedStreaming && streamedAtoms != nil {
				logging.Perception("GCD succeeded on attempt %d (stream-gated): verb=%s, %d valid atoms",
					attempt+1, result.Control.IntentClassification.Verb, len(streamedAtoms))
				return Intent{
					Category:         result.Control.IntentClassification.Category,
					Verb:             result.Control.IntentClassification.Verb,
					Target:           result.Control.IntentClassification.Target,
					Constraint:       result.Control.IntentClassification.Constraint,
					Confidence:       result.Control.IntentClassification.Confidence,
					Response:         result.Surface,
					Ambiguity:        []string{},
					MemoryOperations: result.Control.MemoryOperations,
				}, streamedAtoms, nil
			}

			validAtoms, results := t.ValidateMangleAtoms(result.Control.MangleUpdates)

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
				logging.Get(logging.CategoryPerception).Warn("GCD attempt %d: Mangle syntax errors: %s", attempt+1, strings.Join(errorMsgs, "; "))
				lastErr = fmt.Errorf("Invalid Mangle Syntax:\n%s", strings.Join(errorMsgs, "\n"))
				continue // Retry with error context
			}

			// All atoms valid - return success
			logging.Perception("GCD succeeded on attempt %d: verb=%s, %d valid atoms",
				attempt+1, result.Control.IntentClassification.Verb, len(validAtoms))
			return Intent{
				Category:         result.Control.IntentClassification.Category,
				Verb:             result.Control.IntentClassification.Verb,
				Target:           result.Control.IntentClassification.Target,
				Constraint:       result.Control.IntentClassification.Constraint,
				Confidence:       result.Control.IntentClassification.Confidence,
				Response:         result.Surface,
				Ambiguity:        []string{},
				MemoryOperations: result.Control.MemoryOperations,
			}, validAtoms, nil
		}

		// No mangle_updates to validate - return as-is
		logging.Perception("GCD succeeded on attempt %d (no Mangle updates): verb=%s",
			attempt+1, result.Control.IntentClassification.Verb)
		return Intent{
			Category:         result.Control.IntentClassification.Category,
			Verb:             result.Control.IntentClassification.Verb,
			Target:           result.Control.IntentClassification.Target,
			Constraint:       result.Control.IntentClassification.Constraint,
			Confidence:       result.Control.IntentClassification.Confidence,
			Response:         result.Surface,
			Ambiguity:        []string{},
			MemoryOperations: result.Control.MemoryOperations,
		}, nil, nil
	}

	// Max retries exceeded - return best effort from last envelope
	if lastResult != nil && lastResult.Surface != "" {
		logging.Get(logging.CategoryPerception).Warn("GCD max retries exceeded, returning degraded result (confidence halved)")
		return Intent{
			Category:         lastResult.Control.IntentClassification.Category,
			Verb:             lastResult.Control.IntentClassification.Verb,
			Target:           lastResult.Control.IntentClassification.Target,
			Constraint:       lastResult.Control.IntentClassification.Constraint,
			Confidence:       lastResult.Control.IntentClassification.Confidence * 0.5, // Reduce confidence
			Response:         lastResult.Surface,
			Ambiguity:        []string{"GCD validation failed after retries"},
			MemoryOperations: lastResult.Control.MemoryOperations,
		}, nil, fmt.Errorf("GCD validation failed after %d retries: %w", maxRetries, lastErr)
	}

	// Complete failure - fallback to simple parsing
	logging.Get(logging.CategoryPerception).Error("GCD complete failure after %d retries, falling back to simple parsing", maxRetries)
	intent, err := t.parseSimple(ctx, input)
	return intent, nil, err
}

var ErrStreamingNotSupported = errors.New("streaming not supported")

type gcdStreamAbortError struct {
	Reason string
}

func (e *gcdStreamAbortError) Error() string {
	if strings.TrimSpace(e.Reason) == "" {
		return "gcd stream aborted"
	}
	return "gcd stream aborted: " + e.Reason
}

type llmStreamingCallback interface {
	CompleteStreaming(ctx context.Context, systemPrompt, userPrompt string, callback StreamCallback) error
}

type llmStreamingChannels interface {
	CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error)
}

func (t *RealTransducer) completeWithGCDStreaming(ctx context.Context, sysCtx *SystemLLMContext, systemPrompt, userPrompt string, _ int) (string, []string, error) {
	if sysCtx == nil || sysCtx.client == nil {
		return "", nil, fmt.Errorf("nil sysCtx client")
	}

	// Prefer callback-based streaming (Codex/Claude CLI clients).
	if streamer, ok := sysCtx.client.(llmStreamingCallback); ok {
		var sb strings.Builder
		if err := streamer.CompleteStreaming(ctx, systemPrompt, userPrompt, func(chunk StreamChunk) error {
			if chunk.Error != "" {
				return fmt.Errorf("stream error: %s", chunk.Error)
			}
			if chunk.Text != "" {
				sb.WriteString(chunk.Text)
			} else if chunk.Content != "" {
				sb.WriteString(chunk.Content)
			}
			return nil
		}); err != nil {
			return "", nil, err
		}
		return sb.String(), []string{}, nil
	}

	// Fallback to channel-based streaming (ZAI).
	if streamer, ok := sysCtx.client.(llmStreamingChannels); ok {
		contentCh, errCh := streamer.CompleteWithStreaming(ctx, systemPrompt, userPrompt, false)
		var sb strings.Builder

		for {
			select {
			case <-ctx.Done():
				return "", nil, ctx.Err()
			case chunk, ok := <-contentCh:
				if !ok {
					// Drain any terminal error, if present.
					select {
					case err, ok := <-errCh:
						if ok && err != nil {
							return "", nil, err
						}
					default:
					}
					return sb.String(), []string{}, nil
				}
				sb.WriteString(chunk)
			case err, ok := <-errCh:
				if ok && err != nil {
					return "", nil, err
				}
			}
		}
	}

	return "", nil, ErrStreamingNotSupported
}

// parseSimple is a fallback parser using pipe-delimited format.
func (t *RealTransducer) parseSimple(ctx context.Context, input string) (Intent, error) {
	timer := logging.StartTimer(logging.CategoryPerception, "parseSimple")
	defer timer.Stop()

	// Set system context for trace attribution
	sysCtx := t.withSystemContext("simple-parse")
	defer sysCtx.Clear()

	logging.Perception("Using simple (pipe-delimited) fallback parser")
	logging.PerceptionDebug("Simple parse input: %q", truncateForLog(input, 100))

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

	llmTimer := logging.StartTimer(logging.CategoryPerception, "SimpleParser-LLM-Call")
	resp, err := sysCtx.Complete(ctx, prompt)
	llmTimer.Stop()

	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("Simple parser LLM call failed, using heuristic: %v", err)
		return t.heuristicParse(ctx, input), nil
	}

	parts := strings.Split(strings.TrimSpace(resp), "|")
	if len(parts) < 4 {
		logging.PerceptionDebug("Simple parser response malformed (%d parts), using heuristic", len(parts))
		return t.heuristicParse(ctx, input), nil
	}

	intent := Intent{
		Category:   strings.TrimSpace(parts[0]),
		Verb:       strings.TrimSpace(parts[1]),
		Target:     strings.TrimSpace(parts[2]),
		Constraint: strings.TrimSpace(parts[3]),
		Confidence: 0.7, // Lower confidence for fallback
	}

	logging.Perception("Simple parser result: category=%s, verb=%s, target=%s",
		intent.Category, intent.Verb, truncateForLog(intent.Target, 50))

	return intent, nil
}

// heuristicParse uses the comprehensive verb corpus for reliable offline parsing.
// This is the ultimate fallback when LLM is unavailable.
func (t *RealTransducer) heuristicParse(ctx context.Context, input string) Intent {
	timer := logging.StartTimer(logging.CategoryPerception, "heuristicParse")
	defer timer.Stop()

	logging.Perception("Using heuristic (offline) parser")
	logging.PerceptionDebug("Heuristic parse input: %q", truncateForLog(input, 100))

	// Use the comprehensive corpus matching (now includes semantic classification)
	verb, category, confidence, _ := matchVerbFromCorpus(ctx, input)

	// Refine category based on input patterns
	originalCategory := category
	category = refineCategory(input, category)
	if category != originalCategory {
		logging.PerceptionDebug("Category refined from %s to %s", originalCategory, category)
	}

	// Extract target from natural language
	target := extractTarget(input)
	if target == "none" {
		// Use input as target if no specific target found
		target = input
		logging.PerceptionDebug("No target extracted, using input as target")
	} else {
		logging.PerceptionDebug("Extracted target: %s", truncateForLog(target, 50))
	}

	// Extract constraint
	constraint := extractConstraint(input)
	if constraint != "none" {
		logging.PerceptionDebug("Extracted constraint: %s", constraint)
	}

	intent := Intent{
		Category:   category,
		Verb:       verb,
		Target:     target,
		Constraint: constraint,
		Confidence: confidence,
	}

	logging.Perception("Heuristic result: category=%s, verb=%s, confidence=%.2f",
		intent.Category, intent.Verb, intent.Confidence)

	return intent
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
	timer := logging.StartTimer(logging.CategoryPerception, "ResolveFocus")
	defer timer.Stop()

	// Set system context for trace attribution
	sysCtx := t.withSystemContext("focus-resolution")
	defer sysCtx.Clear()

	logging.Perception("Resolving focus for reference: %q (%d candidates)", truncateForLog(reference, 50), len(candidates))

	if len(candidates) == 0 {
		logging.PerceptionDebug("No candidates provided, returning empty resolution")
		return FocusResolution{
			RawReference:      reference,
			ConfidencePercent: 0,
		}, nil
	}

	if len(candidates) == 1 {
		logging.PerceptionDebug("Single candidate, auto-resolving to: %s", candidates[0])
		return FocusResolution{
			RawReference:      reference,
			ResolvedPath:      candidates[0],
			ConfidencePercent: 90,
		}, nil
	}

	logging.PerceptionDebug("Multiple candidates, using LLM for disambiguation")

	// Use LLM to disambiguate
	candidateList := strings.Join(candidates, "\n- ")
	prompt := fmt.Sprintf(`Given the reference "%s", which of these candidates is the best match?

Candidates:
- %s

Return JSON:
{
  "resolved_path": "best matching path",
  "symbol_name": "specific symbol if applicable or empty",
  "confidence": 0-100
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
	llmTimer := logging.StartTimer(logging.CategoryPerception, "ResolveFocus-LLM-Call")
	resp, err := sysCtx.CompleteWithSystem(ctx, focusSystemPrompt, prompt)
	llmTimer.Stop()

	if err != nil {
		logging.Get(logging.CategoryPerception).Warn("Focus resolution LLM call failed, using first candidate: %v", err)
		return FocusResolution{
			RawReference:      reference,
			ResolvedPath:      candidates[0],
			ConfidencePercent: 50,
		}, nil
	}

	// Parse JSON response
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```json")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	resp = strings.TrimSpace(resp)

	var parsed struct {
		ResolvedPath string `json:"resolved_path"`
		SymbolName   string `json:"symbol_name"`
		Confidence   int64  `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(resp), &parsed); err != nil {
		logging.Get(logging.CategoryPerception).Warn("Focus resolution JSON parse failed, using first candidate: %v", err)
		return FocusResolution{
			RawReference:      reference,
			ResolvedPath:      candidates[0],
			ConfidencePercent: 50,
		}, nil
	}

	conf := parsed.Confidence
	if conf < 0 {
		conf = 0
	} else if conf > 100 {
		conf = 100
	}

	logging.Perception("Focus resolved: %s -> %s (symbol: %s, confidence: %d)",
		truncateForLog(reference, 30), truncateForLog(parsed.ResolvedPath, 50), parsed.SymbolName, conf)

	return FocusResolution{
		RawReference:      reference,
		ResolvedPath:      parsed.ResolvedPath,
		SymbolName:        parsed.SymbolName,
		ConfidencePercent: conf,
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
	logging.Perception("Initializing DualPayloadTransducer")
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
	timer := logging.StartTimer(logging.CategoryPerception, "DualPayloadTransducer.Parse")
	defer timer.Stop()

	logging.Perception("Full transduction: input=%d chars, candidates=%d", len(input), len(fileCandidates))

	intent, err := t.ParseIntent(ctx, input)
	if err != nil {
		logging.Get(logging.CategoryPerception).Error("Transduction failed: %v", err)
		return TransducerOutput{}, err
	}

	output := TransducerOutput{
		Intent:      intent,
		MangleAtoms: []core.Fact{intent.ToFact()},
	}

	// Try to resolve focus if target looks like a file reference
	if intent.Target != "" && intent.Target != "none" {
		logging.PerceptionDebug("Attempting focus resolution for target: %s", truncateForLog(intent.Target, 50))
		focus, err := t.ResolveFocus(ctx, intent.Target, fileCandidates)
		if err == nil && focus.ConfidencePercent > 0 {
			output.Focus = append(output.Focus, focus)
			output.MangleAtoms = append(output.MangleAtoms, focus.ToFact())
			logging.PerceptionDebug("Focus resolved, added to output (total atoms: %d)", len(output.MangleAtoms))
		} else if err != nil {
			logging.PerceptionDebug("Focus resolution failed: %v", err)
		}
	}

	logging.Perception("Transduction complete: verb=%s, atoms=%d, focus=%d",
		intent.Verb, len(output.MangleAtoms), len(output.Focus))

	return output, nil
}

// =============================================================================
// PERCEPTION LAYER INITIALIZATION
// =============================================================================

// InitPerceptionLayer initializes all perception components.
// This should be called during session startup after the kernel and config are available.
//
// Components initialized:
//   - SemanticClassifier (vector-based intent classification)
//
// The function performs graceful degradation: if semantic classification cannot be
// initialized (e.g., no embedding config), the system continues with regex-only
// classification. This ensures the perception layer is always functional.
func InitPerceptionLayer(kernel core.Kernel, cfg *config.UserConfig) error {
	timer := logging.StartTimer(logging.CategoryPerception, "InitPerceptionLayer")
	defer timer.Stop()

	logging.Perception("Initializing perception layer components")

	// Initialize semantic classifier
	// GetEmbeddingConfig() returns defaults if not explicitly configured,
	// so we always attempt initialization and let it fail gracefully if
	// the embedding engine cannot connect (e.g., Ollama not running).
	embedCfg := cfg.GetEmbeddingConfig()
	logging.PerceptionDebug("Embedding config: provider=%s", embedCfg.Provider)

	if err := InitSemanticClassifier(kernel, cfg); err != nil {
		// Non-fatal: classification works without semantic layer
		logging.Get(logging.CategoryPerception).Warn("Semantic classifier init failed: %v (continuing with regex-only)", err)
	} else {
		logging.Perception("SemanticClassifier initialized successfully")
	}

	logging.Perception("Perception layer initialization complete")
	return nil
}

// ClosePerceptionLayer releases resources held by perception components.
// Should be called during graceful shutdown.
func ClosePerceptionLayer() error {
	logging.Perception("Closing perception layer components")

	if err := CloseSemanticClassifier(); err != nil {
		logging.Get(logging.CategoryPerception).Warn("Error closing SemanticClassifier: %v", err)
		return err
	}

	logging.Perception("Perception layer closed")
	return nil
}
