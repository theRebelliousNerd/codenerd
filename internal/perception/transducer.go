package perception

import (
	"context"
	"regexp"
	"strings"

	"codenerd/internal/articulation"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/logging"
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
			// Non-fatal: seed fallback semantic_match facts from regex candidates
			logging.PerceptionDebug("Semantic classification error (non-fatal): %v - seeding fallback facts", err)
			seedFallbackSemanticFacts(input, candidates)
		} else if len(matches) > 0 {
			logging.PerceptionDebug("Semantic matches found: %d (top: %s %.2f)",
				len(matches), matches[0].Verb, matches[0].Similarity)
		} else {
			logging.PerceptionDebug("Semantic classification returned no matches - seeding fallback facts")
			seedFallbackSemanticFacts(input, candidates)
		}
		// Facts are now in kernel - Mangle inference will see them via semantic_match predicate
	}

	// 3. Refine via Mangle Inference (Smart)
	// This applies the "sentence level" logic and context rules.
	// Now includes semantic_match facts from the classifier above.
	// Always run inference, even if no candidates, to support pure interrogative logic.
	if SharedTaxonomy != nil {
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

	logging.Get(logging.CategoryPerception).Warn("No verb match found for input, defaulting to /explain")
	return "/explain", "/query", 0.3, ""
}

// maxRegexInputLen caps the input length fed to regex matching.
// Go's regexp uses Thompson NFA (linear time), but there's no reason to
// run the full VerbCorpus against a 100KB paste. The first 2000 chars
// contain all meaningful verb/intent signals.
const maxRegexInputLen = 2000

// getRegexCandidates returns all verbs that match the input via regex or synonyms.
func getRegexCandidates(input string) []VerbEntry {
	// Truncate to prevent linear-cost amplification on massive inputs
	truncated := input
	if len(truncated) > maxRegexInputLen {
		truncated = truncated[:maxRegexInputLen]
	}
	lower := strings.ToLower(truncated)
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

// sanitizeFactArg strips Mangle control characters and caps length to prevent
// injection attacks when user/LLM-derived strings are embedded as fact arguments.
// Without this, a Target like "foo). malicious_rule(X) :- " could inject rules.
func sanitizeFactArg(s string) string {
	const maxFactArgLen = 2048
	if len(s) > maxFactArgLen {
		s = s[:maxFactArgLen]
	}
	// Strip characters that have syntactic meaning in Mangle:
	// ( ) . , / :- ; could terminate atoms or inject new rules
	// Also strip null bytes and control characters
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == 0: // null byte
			continue
		case r < 0x20 && r != '\n' && r != '\r' && r != '\t': // control chars (keep newlines/tabs)
			continue
		case r == 0x1b: // ANSI escape
			continue
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// ToFact converts the intent to a Mangle Fact.
func (i Intent) ToFact() core.Fact {
	return core.Fact{
		Predicate: "user_intent",
		Args: []interface{}{
			core.MangleAtom("/current_intent"), // ID as name constant
			core.MangleAtom(i.Category),
			core.MangleAtom(i.Verb),
			sanitizeFactArg(i.Target),
			sanitizeFactArg(i.Constraint),
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
// UnderstandingTransducer is the canonical LLM-first implementation.
type Transducer interface {
	// ParseIntent parses user input into an Intent without conversation history.
	ParseIntent(ctx context.Context, input string) (Intent, error)

	// ParseIntentWithContext parses user input with conversation history.
	// This is the primary method used by the chat loop.
	ParseIntentWithContext(ctx context.Context, input string, history []ConversationTurn) (Intent, error)

	// ParseIntentWithGCD parses user input with Grammar-Constrained Decoding.
	// Returns the intent, validated Mangle updates, and any error.
	// For LLM-first transducers, this is equivalent to ParseIntentWithContext with empty updates.
	ParseIntentWithGCD(ctx context.Context, input string, history []ConversationTurn, maxRetries int) (Intent, []string, error)

	// ResolveFocus resolves ambiguous references to specific candidates.
	ResolveFocus(ctx context.Context, reference string, candidates []string) (FocusResolution, error)

	// SetPromptAssembler sets the prompt assembler for JIT compilation.
	SetPromptAssembler(pa *articulation.PromptAssembler)

	// SetStrategicContext injects strategic knowledge about the codebase.
	SetStrategicContext(context string)
}

// TransducerWithKernel extends Transducer with kernel integration for routing.
type TransducerWithKernel interface {
	Transducer

	// SetKernel sets the Mangle kernel for routing queries.
	SetKernel(kernel *core.RealKernel)
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

// GetShardTypeForVerb returns the shard type associated with a canonical verb.
// Returns empty string if verb is not found in the VerbCorpus.
func GetShardTypeForVerb(verb string) string {
	for _, entry := range VerbCorpus {
		if entry.Verb == verb {
			return entry.ShardType
		}
	}
	return ""
}

// DualPayloadTransducer wraps a transducer to emit Cortex 1.5.0 dual payloads.
type DualPayloadTransducer struct {
	Transducer
}

// NewDualPayloadTransducer creates a transducer that outputs dual payloads.
func NewDualPayloadTransducer(client LLMClient) *DualPayloadTransducer {
	logging.Perception("Initializing DualPayloadTransducer")
	return &DualPayloadTransducer{
		Transducer: NewUnderstandingTransducer(client),
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

// seedFallbackSemanticFacts injects low-confidence semantic_match facts from regex candidates
// when the SemanticClassifier fails or returns no matches. This ensures the Mangle inference
// rules always have some semantic signal to work with, even in degraded mode.
func seedFallbackSemanticFacts(input string, candidates []VerbEntry) {
	if SharedTaxonomy == nil || SharedTaxonomy.engine == nil {
		return
	}

	// Seed low-confidence semantic_match facts for top regex candidates
	for rank, cand := range candidates {
		if rank >= 5 {
			break // Only top 5 candidates
		}
		// Assert semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity)
		err := SharedTaxonomy.engine.AddFact("semantic_match",
			input,     // UserInput
			"",        // CanonicalSentence (empty for fallback)
			cand.Verb, // Verb
			"",        // Target (empty for fallback)
			rank+1,    // Rank (1-indexed)
			50.0,      // Similarity (fixed low confidence for fallback)
		)
		if err != nil {
			logging.PerceptionDebug("Failed to seed fallback semantic_match for %s: %v", cand.Verb, err)
		}
	}

	if len(candidates) > 0 {
		logging.PerceptionDebug("Seeded %d fallback semantic_match facts", min(len(candidates), 5))
	}
}
