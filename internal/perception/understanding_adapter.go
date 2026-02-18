package perception

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// ThinkingProvider is an optional interface for clients that support thinking.
type ThinkingProvider interface {
	IsThinkingEnabled() bool
	GetThinkingLevel() string
}

// UnderstandingTransducer implements the Transducer interface using LLM-first classification.
// It provides deep semantic understanding of user intent through structured LLM output parsing.
//
// This is the canonical transducer implementation for codeNERD's perception layer.
type UnderstandingTransducer struct {
	llmTransducer     *LLMTransducer
	client            LLMClient
	promptAssembler   *articulation.PromptAssembler
	kernel            RoutingKernel
	mu                sync.RWMutex
	lastUnderstanding *Understanding // GAP-018 FIX: Cache for debugging
	strategicContext  string         // Strategic knowledge about the codebase from /init
}

// NewUnderstandingTransducer creates a transducer using LLM-first classification.
func NewUnderstandingTransducer(client LLMClient) Transducer {
	base := &UnderstandingTransducer{
		client: client,
	}

	// Check if this is a Gemini client with thinking enabled
	if thinkingProvider, ok := client.(ThinkingProvider); ok && thinkingProvider.IsThinkingEnabled() {
		logging.Perception("[Transducer] Using GeminiThinkingTransducer (thinking mode enabled, level=%s)",
			thinkingProvider.GetThinkingLevel())
		// Return the specialized transducer for Gemini Thinking Mode
		return NewGeminiThinkingTransducer(base)
	}

	logging.Perception("[Transducer] Using UnderstandingTransducer (thinking mode NOT detected)")
	return base
}

// SetPromptAssembler sets the prompt assembler for JIT compilation.
func (t *UnderstandingTransducer) SetPromptAssembler(pa *articulation.PromptAssembler) {
	t.promptAssembler = pa
}

// SetStrategicContext injects strategic knowledge about the codebase.
// This context is included in prompts to help answer conceptual questions.
func (t *UnderstandingTransducer) SetStrategicContext(context string) {
	t.strategicContext = context
}

// SetKernel sets the Mangle kernel for routing queries.
func (t *UnderstandingTransducer) SetKernel(kernel *core.RealKernel) {
	if kernel != nil {
		// Create a routing kernel adapter that uses the RealKernel's Query method
		t.kernel = NewRealKernelRouter(kernel)
	}
}

// initialize lazily creates the LLMTransducer with the classification prompt.
func (t *UnderstandingTransducer) initialize(ctx context.Context) {
	if t.llmTransducer != nil {
		return
	}

	// Load the classification prompt (JIT if available, otherwise embedded)
	prompt := getUnderstandingPrompt(ctx, t.promptAssembler)
	t.llmTransducer = NewLLMTransducer(t.client, t.kernel, prompt)
}

// getUnderstandingPrompt returns the system prompt for LLM classification.
// Uses JIT compilation if available, otherwise falls back to embedded prompt.
func getUnderstandingPrompt(ctx context.Context, pa *articulation.PromptAssembler) string {
	// Check if JIT is available
	if pa == nil || !pa.JITReady() {
		return understandingSystemPrompt
	}

	// Build prompt context for perception layer
	pc := &articulation.PromptContext{
		ShardID:   "perception-transducer",
		ShardType: "perception",
	}

	// Attempt JIT compilation
	prompt, err := pa.AssembleSystemPrompt(ctx, pc)
	if err != nil {
		// Log the error and fall back to embedded prompt
		return understandingSystemPrompt
	}

	// Validate the compiled prompt has reasonable content
	if len(strings.TrimSpace(prompt)) < 100 {
		return understandingSystemPrompt
	}

	return prompt
}

// ParseIntent parses user input into an Intent using LLM-first classification.
// This is the main entry point implementing the Transducer interface.
func (t *UnderstandingTransducer) ParseIntent(ctx context.Context, input string) (Intent, error) {
	return t.ParseIntentWithContext(ctx, input, nil)
}

// ParseIntentWithContext parses user input with conversation history.
// This is the primary method called by the chat loop.
func (t *UnderstandingTransducer) ParseIntentWithContext(ctx context.Context, input string, history []ConversationTurn) (Intent, error) {
	t.initialize(ctx)

	// GAP-006 FIX: Run semantic classification to inject semantic_match facts into kernel
	// This provides neuro-symbolic grounding even in LLM-first mode
	if SharedSemanticClassifier != nil {
		matches, err := SharedSemanticClassifier.Classify(ctx, input)
		if err != nil {
			// Graceful degradation - log but continue with LLM-only classification
			// Semantic classification is optional enhancement, not required
			_ = matches // matches already injected into kernel by Classify()
		}
		// Note: semantic_match facts are automatically asserted by Classify()
	}

	// Convert history to Turn format
	var turns []Turn
	for _, h := range history {
		turns = append(turns, Turn{
			Role:    h.Role,
			Content: h.Content,
		})
	}

	// Get Understanding from LLM
	understanding, err := t.llmTransducer.Understand(ctx, input, turns)
	if err != nil {
		return Intent{}, fmt.Errorf("LLM classification failed: %w", err)
	}

	// GAP-018 FIX: Cache understanding for debugging
	t.mu.Lock()
	t.lastUnderstanding = understanding
	t.mu.Unlock()

	// Convert Understanding to Intent for backward compatibility
	intent := t.understandingToIntent(understanding)

	return intent, nil
}

// understandingToIntent converts the new Understanding to legacy Intent.
// This enables gradual migration without breaking existing code.
func (t *UnderstandingTransducer) understandingToIntent(u *Understanding) Intent {
	// Guard against nil understanding
	if u == nil {
		logging.Get(logging.CategoryPerception).Warn("understandingToIntent called with nil understanding, falling back to /explain")
		return Intent{
			Verb:     "/explain",
			Category: "/query",
			Response: "Internal error: understanding is nil",
		}
	}

	// Map action_type to verb
	verb := t.mapActionToVerb(u.ActionType, u.Domain)

	// Map semantic_type to category
	category := t.mapSemanticToCategory(u.SemanticType, u.ActionType)

	// Build target from scope
	target := u.Scope.Target
	if target == "" && u.Scope.File != "" {
		target = u.Scope.File
	}
	if target == "" && u.Scope.Symbol != "" {
		target = u.Scope.Symbol
	}

	// Build constraint from user constraints
	constraint := strings.Join(u.UserConstraints, "; ")

	// Build ambiguity info including shard routing for debugging
	ambiguityInfo := []string{
		fmt.Sprintf("semantic_type=%s", u.SemanticType),
		fmt.Sprintf("action_type=%s", u.ActionType),
		fmt.Sprintf("domain=%s", u.Domain),
	}
	if u.Routing != nil && u.Routing.PrimaryShard != "" {
		ambiguityInfo = append(ambiguityInfo, fmt.Sprintf("shard=%s", u.Routing.PrimaryShard))
	}

	return Intent{
		Category:   category,
		Verb:       verb,
		Target:     target,
		Constraint: constraint,
		Confidence: u.Confidence,
		Response:   u.SurfaceResponse,

		// Preserve Understanding in Ambiguity for debugging
		Ambiguity: ambiguityInfo,

		// Map memory operations from constraints
		MemoryOperations: t.extractMemoryOperations(u),
	}
}

// mapActionToVerb converts action_type to the legacy verb format.
func (t *UnderstandingTransducer) mapActionToVerb(actionType, domain string) string {
	actionType = strings.ToLower(strings.TrimSpace(actionType))
	domain = strings.ToLower(strings.TrimSpace(domain))

	// Direct mappings
	switch actionType {
	case "investigate":
		if domain == "testing" {
			return "/debug"
		}
		return "/analyze"
	case "implement":
		return "/create"
	case "modify":
		return "/fix"
	case "refactor":
		return "/refactor"
	case "verify":
		return "/test"
	case "explain":
		return "/explain"
	case "research":
		return "/research"
	case "configure":
		return "/configure"
	case "attack":
		return "/assault"
	case "revert":
		return "/git" // or /revert if we add it
	case "review":
		if domain == "security" {
			return "/security"
		}
		return "/review"
	case "remember":
		return "/remember"
	case "forget":
		return "/forget"
	case "chat":
		return "/converse"
	case "deploy":
		return "/deploy"
	case "migrate":
		return "/migrate"
	case "optimize":
		return "/optimize"
	case "document":
		return "/document"
	case "benchmark":
		return "/benchmark"
	case "profile":
		return "/profile"
	case "audit":
		return "/audit"
	case "scaffold":
		return "/scaffold"
	case "lint":
		return "/lint"
	case "format":
		return "/format"
	default:
		logging.Get(logging.CategoryPerception).Warn("Unknown action_type %q (domain=%q) fell back to /explain", actionType, domain)
		return "/explain" // Safe fallback
	}
}

// mapSemanticToCategory converts semantic_type to legacy category.
func (t *UnderstandingTransducer) mapSemanticToCategory(semanticType, actionType string) string {
	semanticType = strings.ToLower(strings.TrimSpace(semanticType))
	actionType = strings.ToLower(strings.TrimSpace(actionType))

	// Use semantic type to refine category
	if semanticType == "instruction" {
		return "/instruction"
	}

	// Actions that modify code are mutations
	switch actionType {
	case "implement", "modify", "refactor", "attack", "revert", "configure":
		return "/mutation"
	case "verify":
		return "/query"
	case "remember", "forget":
		return "/instruction"
	}

	// Everything else is a query
	return "/query"
}

// extractMemoryOperations extracts memory operations from Understanding.
func (t *UnderstandingTransducer) extractMemoryOperations(u *Understanding) []MemoryOperation {
	var ops []MemoryOperation

	switch strings.ToLower(strings.TrimSpace(u.ActionType)) {
	case "remember":
		// "Remember that X" -> store X
		if u.Scope.Target != "" {
			ops = append(ops, MemoryOperation{
				Op:    "promote_to_long_term",
				Key:   "preference",
				Value: u.Scope.Target,
			})
		}
	case "forget":
		// "Forget X" -> remove X
		if u.Scope.Target != "" {
			ops = append(ops, MemoryOperation{
				Op:  "forget",
				Key: u.Scope.Target,
			})
		}
	}

	return ops
}

// ResolveFocus resolves ambiguous references to specific candidates.
// For the LLM-first transducer, this is a simple pass-through since
// the LLM handles disambiguation during classification.
func (t *UnderstandingTransducer) ResolveFocus(ctx context.Context, reference string, candidates []string) (FocusResolution, error) {
	// The LLM-first approach handles focus resolution during classification.
	// For now, return the first candidate as a simple fallback.
	if len(candidates) > 0 {
		return FocusResolution{
			RawReference:      reference,
			ResolvedPath:      candidates[0],
			ConfidencePercent: 50,
		}, nil
	}
	return FocusResolution{
		RawReference:      reference,
		ResolvedPath:      reference,
		ConfidencePercent: 30,
	}, nil
}

// ParseIntentWithGCD implements the Transducer interface for Grammar-Constrained Decoding.
// For UnderstandingTransducer, this is equivalent to ParseIntentWithContext since
// the LLM-first approach uses structured JSON output instead of Mangle syntax validation.
// The returned mangle updates are always empty as LLM-first doesn't generate raw Mangle.
func (t *UnderstandingTransducer) ParseIntentWithGCD(ctx context.Context, input string, history []ConversationTurn, maxRetries int) (Intent, []string, error) {
	intent, err := t.ParseIntentWithContext(ctx, input, history)
	if err != nil {
		return Intent{}, nil, err
	}
	// LLM-first transducer doesn't produce raw Mangle updates;
	// the Understanding struct is already validated JSON
	return intent, nil, nil
}

// GetLastUnderstanding returns the last Understanding for debugging.
// This allows inspection of the full LLM classification.
// GAP-018 FIX: Returns cached understanding from last ParseIntentWithContext call
func (t *UnderstandingTransducer) GetLastUnderstanding() *Understanding {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.lastUnderstanding
}

// understandingSystemPrompt is the comprehensive prompt for LLM classification.
const understandingSystemPrompt = `## Your Role: Perception Layer

You are the perception layer of a coding agent. Your job is to deeply understand what the user wants and express that understanding in structured form. The harness will use your understanding to route to the right tools, context, and agents.

**Philosophy**: You describe what the user wants. The harness determines how to fulfill it.

## What You Must Understand

### 1. Semantic Type (HOW is the user asking?)

| Type | Trigger Patterns | Example |
|------|------------------|---------|
| definition | "what is", "what are", "explain" | "What is this function?" |
| causation | "why", "what causes", "root cause" | "Why is this failing?" |
| mechanism | "how does", "how do I", "how to" | "How does auth work?" |
| location | "where is", "find", "locate" | "Where is the config?" |
| temporal | "when", "history", "last changed" | "When was this added?" |
| attribution | "who wrote", "who owns", "author" | "Who wrote this?" |
| selection | "which", "compare", "best" | "Which approach is better?" |
| hypothetical | "what if", "imagine", "suppose" | "What if I deleted this?" |
| recommendation | "should I", "best practice", "advice" | "Should I use channels here?" |
| existence | "is there", "does X exist", "any" | "Is there a retry mechanism?" |
| quantification | "how many", "how much", "count" | "How many tests are failing?" |
| state | "is X working", "is X safe", status | "Is this code secure?" |
| instruction | "always", "never", "remember" | "Always use context.Context" |

### 2. Action Type (WHAT does the user want done?)

| Action | Meaning | Modifies Code? |
|--------|---------|----------------|
| investigate | Analyze, debug, find root cause | No |
| implement | Create new functionality | Yes |
| modify | Change existing code | Yes |
| refactor | Restructure without behavior change | Yes |
| verify | Test, validate, check correctness | Maybe (writes tests) |
| explain | Describe, document, teach | No |
| research | Gather information, find docs | No |
| configure | Setup, initialize, adjust settings | Maybe |
| attack | Adversarial testing, security probe | Maybe |
| revert | Undo, rollback, restore | Yes |
| review | Audit, critique, assess quality | No |
| remember | Store preference for future | No (memory only) |
| forget | Remove stored preference | No (memory only) |
| chat | Social interaction, greeting | No |
| migrate | Move code between frameworks/versions | Yes |
| optimize | Improve performance, reduce resource usage | Yes |
| document | Write docs, comments, READMEs | Yes |
| benchmark | Measure performance, run benchmarks | Maybe |
| profile | Profile CPU/memory, find bottlenecks | No |
| audit | Security/compliance/quality audit | No |
| scaffold | Generate boilerplate, project structure | Yes |
| lint | Run linters, fix lint issues | Maybe |
| format | Format code, fix style issues | Yes |

### 3. Domain (WHAT AREA is this about?)

- testing - Tests, coverage, fixtures, mocks
- security - Vulnerabilities, auth, encryption, validation
- performance - Speed, memory, optimization, profiling
- documentation - Comments, READMEs, API docs
- architecture - Design patterns, structure, dependencies
- git - Version control, commits, branches, history
- dependencies - Packages, imports, versions
- configuration - Settings, env vars, build config
- error_handling - Exceptions, panics, recovery, logging
- concurrency - Goroutines, threads, races, deadlocks
- general - No specific domain

### 4. Scope (HOW MUCH is involved?)

line < block < function < type < file < package < module < codebase

### 5. Signals (Boolean flags)

- is_question: User asking a question vs requesting action
- is_hypothetical: "What if" / simulation request
- is_multi_step: Requires multiple distinct phases
- is_negated: User said NOT to do something ("don't delete")
- requires_confirmation: User wants approval before action
- urgency: low | normal | high | critical

### 6. Constraints (Explicit limitations)

Look for phrases like:
- "don't break tests" → preserve test behavior
- "keep it simple" → minimal changes
- "don't change the API" → preserve interfaces
- "just explain, don't fix" → read-only
- "without external dependencies" → no new deps

### 7. Implicit Assumptions

What is the user assuming that they didn't say?
- "This was working before" (implies recent regression)
- "Following our patterns" (implies existing conventions)
- "Like the other endpoints" (implies reference implementation)

## Output Format

You MUST output valid JSON in this exact structure:

{
  "understanding": {
    "primary_intent": "<one word summary>",
    "semantic_type": "<from table above>",
    "action_type": "<from table above>",
    "domain": "<from list above>",
    "scope": {
      "level": "<line|block|function|type|file|package|module|codebase>",
      "target": "<specific target name>",
      "file": "<file path if known>",
      "symbol": "<symbol name if known>"
    },
    "user_constraints": ["<explicit constraints>"],
    "implicit_assumptions": ["<unstated assumptions>"],
    "confidence": <0.0 to 1.0>,
    "signals": {
      "is_question": <true|false>,
      "is_hypothetical": <true|false>,
      "is_multi_step": <true|false>,
      "is_negated": <true|false>,
      "requires_confirmation": <true|false>,
      "urgency": "<low|normal|high|critical>"
    },
    "suggested_approach": {
      "mode": "<normal|tdd|dream|debug|security_audit|campaign|research|assault>",
      "primary_shard": "<coder|tester|reviewer|researcher|nemesis>",
      "supporting_shards": ["<additional agents if needed>"],
      "tools_needed": ["<tools you expect to use>"],
      "context_needed": ["<what context would help>"]
    }
  },
  "surface_response": "<natural language response to user>"
}

## Important Rules

1. **Be specific**: If you can identify the file or symbol, include it.
2. **Be honest about confidence**: Lower confidence if the request is ambiguous.
3. **Capture constraints**: User limitations are critical for correct execution.
4. **Surface assumptions**: Making assumptions explicit helps the user correct misunderstandings.
5. **Match urgency to context**: "ASAP", "urgent", "blocking" = high/critical.
6. **Read-only by default**: Questions and investigations don't modify code.
7. **Multi-step detection**: "First X, then Y" or "X and then Y" = multi-step.

## Examples

### Example: Greeting
**User**: "Hello world"

{
  "understanding": {
    "primary_intent": "greeting",
    "semantic_type": "state",
    "action_type": "chat",
    "domain": "general",
    "scope": {
      "level": "codebase",
      "target": "user",
      "file": "",
      "symbol": ""
    },
    "user_constraints": [],
    "implicit_assumptions": [],
    "confidence": 0.99,
    "signals": {
      "is_question": false,
      "is_hypothetical": false,
      "is_multi_step": false,
      "is_negated": false,
      "requires_confirmation": false,
      "urgency": "normal"
    },
    "suggested_approach": {
      "mode": "normal",
      "primary_shard": "coder",
      "supporting_shards": [],
      "tools_needed": [],
      "context_needed": []
    }
  },
  "surface_response": "Hello! How can I help you code today?"
}
`
