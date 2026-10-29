package perception

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
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
	promptAssembler   PromptAssembler
	kernel            RoutingKernel
	mu                sync.RWMutex
	lastUnderstanding *Understanding // GAP-018 FIX: Cache for debugging
	strategicContext  string         // Strategic knowledge about the codebase from /init
}

// NewUnderstandingTransducer creates a transducer using LLM-first classification.
func NewUnderstandingTransducer(client LLMClient) Transducer {
	// One transducer type serves every client. A client with thinking enabled
	// is classified through the thinking methods (LLMTransducer.methodPlan);
	// the capability is read there, once, so ParseIntent, the history-aware
	// method and the GCD adaptation cannot disagree about it. Until 2026-10-28
	// a separate wrapper type embedded this one, and the embedded GCD method
	// called the base context method, bypassing the wrapper's schema path for
	// the one caller (the perception firewall) that used GCD.
	if thinkingProvider, ok := client.(ThinkingProvider); ok && thinkingProvider.IsThinkingEnabled() {
		logging.Perception("[Transducer] thinking mode enabled (level=%s)", thinkingProvider.GetThinkingLevel())
	} else {
		logging.Perception("[Transducer] thinking mode NOT detected")
	}
	return &UnderstandingTransducer{client: client}
}

// SetPromptAssembler sets the JIT prompt assembler.
func (t *UnderstandingTransducer) SetPromptAssembler(pa PromptAssembler) {
	t.mu.Lock()
	t.promptAssembler = pa
	t.mu.Unlock()
}

// SetStrategicContext injects strategic knowledge about the codebase.
// This context is included in prompts to help answer conceptual questions.
// It takes the write lock: the setter runs live from session boot while
// in-flight Transduce/Understand calls read the field.
func (t *UnderstandingTransducer) SetStrategicContext(context string) {
	t.mu.Lock()
	t.strategicContext = context
	t.mu.Unlock()
}

// getStrategicContext returns the injected strategic context under the read
// lock, so a concurrent SetStrategicContext cannot race an in-flight
// Transduce/Understand that reads it.
func (t *UnderstandingTransducer) getStrategicContext() string {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.strategicContext
}

// SetKernel sets the Mangle kernel for routing queries.
func (t *UnderstandingTransducer) SetKernel(kernel *core.RealKernel) {
	if kernel != nil {
		// Create a routing kernel adapter that uses the RealKernel's Query method
		t.kernel = NewRealKernelRouter(kernel)
	}
}

// initialize lazily creates the LLMTransducer with the classification prompt.
// A prompt that cannot be compiled is returned as an error and nothing is
// cached, so a later turn compiles again once the JIT compiler is ready.
func (t *UnderstandingTransducer) initialize(ctx context.Context) error {
	t.mu.RLock()
	if t.llmTransducer != nil {
		t.mu.RUnlock()
		return nil
	}
	t.mu.RUnlock()

	t.mu.Lock()
	defer t.mu.Unlock()
	if t.llmTransducer != nil {
		return nil
	}

	prompt, err := getUnderstandingPrompt(ctx, t.promptAssembler)
	if err != nil {
		return err
	}
	t.llmTransducer = NewLLMTransducer(t.client, t.kernel, prompt)
	return nil
}

// getUnderstandingPrompt compiles the classification system prompt from the
// perception atoms (atoms/perception/understanding.yaml and
// atoms/system/perception.yaml). There is no embedded copy: an unwired or
// unready compiler, a failed compile, or a compiled prompt without a valid
// UnderstandingEnvelope contract is an error wrapping
// types.ErrRolePromptUnavailable.
func getUnderstandingPrompt(ctx context.Context, pa PromptAssembler) (string, error) {
	if pa == nil || !pa.JITReady() {
		return "", fmt.Errorf("%w: perception: the JIT prompt compiler is not ready", types.ErrRolePromptUnavailable)
	}
	prompt, err := pa.AssembleSystemPrompt(ctx, "perception-transducer", "perception")
	if err != nil {
		return "", fmt.Errorf("%w: perception: compile the classification prompt: %w", types.ErrRolePromptUnavailable, err)
	}
	if strings.TrimSpace(prompt) == "" {
		return "", fmt.Errorf("%w: perception: the classification prompt compiled empty", types.ErrRolePromptUnavailable)
	}
	if reason := understandingPromptContractReason(prompt); reason != "" {
		return "", fmt.Errorf("%w: perception: the compiled classification prompt has a %s UnderstandingEnvelope contract", types.ErrRolePromptUnavailable, reason)
	}
	logging.PerceptionDebug("Perception prompt: source=jit contract=UnderstandingEnvelope")
	return prompt, nil
}

// understandingPromptContractReason reports why a compiled perception prompt
// cannot be used ("" when it can): getUnderstandingPrompt refuses the prompt
// on any reason, so a JIT prompt that teaches a conflicting or malformed
// output shape never reaches the classifier.
func understandingPromptContractReason(prompt string) string {
	found := false
	for offset := 0; offset < len(prompt); {
		start := strings.IndexByte(prompt[offset:], '{')
		if start < 0 {
			break
		}
		start += offset
		end := understandingObjectEnd(prompt, start)
		if end < 0 {
			if mentionsUnderstandingObject(prompt[start:]) {
				return "malformed_contract"
			}
			break
		}
		object := prompt[start:end]
		offset = end
		var shape map[string]any
		if err := json.Unmarshal([]byte(normalizeUnderstandingTemplate(object)), &shape); err != nil {
			if mentionsUnderstandingObject(object) {
				return "malformed_contract"
			}
			continue
		}
		if _, exists := shape["control_packet"]; exists {
			return "conflicting_contract"
		}
		_, category := shape["category"]
		_, verb := shape["verb"]
		if category && verb {
			return "conflicting_contract"
		}
		if _, exists := shape["understanding"]; exists {
			if !isUnderstandingEnvelopeShape(shape) {
				return "malformed_contract"
			}
			found = true
		} else if _, exists := shape["surface_response"]; exists {
			return "conflicting_contract"
		}
	}
	if !found {
		return "missing_contract"
	}
	return ""
}

func mentionsUnderstandingObject(object string) bool {
	return strings.Contains(object, `"understanding"`) || strings.Contains(object, `"control_packet"`)
}

func isUnderstandingEnvelopeShape(shape map[string]any) bool {
	// understanding and surface_response, and optionally the running brief.
	switch len(shape) {
	case 2:
	case 3:
		if _, ok := shape["brief"].(map[string]any); !ok {
			return false
		}
	default:
		return false
	}
	if _, ok := shape["surface_response"].(string); !ok {
		return false
	}
	understandingShape, ok := shape["understanding"].(map[string]any)
	if !ok {
		return false
	}
	for _, key := range []string{"primary_intent", "semantic_type", "action_type", "domain"} {
		if _, ok := understandingShape[key].(string); !ok {
			return false
		}
	}
	scope, ok := understandingShape["scope"].(map[string]any)
	if !ok {
		return false
	}
	for _, key := range []string{"level", "target"} {
		if _, ok := scope[key].(string); !ok {
			return false
		}
	}
	_, ok = understandingShape["suggested_approach"].(map[string]any)
	return ok
}

func understandingObjectEnd(text string, start int) int {
	depth, quoted, escaped := 0, false, false
	for index := start; index < len(text); index++ {
		if quoted {
			if escaped {
				escaped = false
			} else if text[index] == '\\' {
				escaped = true
			} else if text[index] == '"' {
				quoted = false
			}
			continue
		}
		switch text[index] {
		case '"':
			quoted = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return index + 1
			}
		}
	}
	return -1
}

func normalizeUnderstandingTemplate(object string) string {
	var out strings.Builder
	quoted, escaped := false, false
	for index := 0; index < len(object); index++ {
		ch := object[index]
		if quoted {
			out.WriteByte(ch)
			if escaped {
				escaped = false
			} else if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				quoted = false
			}
			continue
		}
		if ch == '"' {
			quoted = true
		} else if ch == '<' {
			if end := strings.IndexByte(object[index:], '>'); end >= 0 {
				out.WriteString("null")
				index += end
				continue
			}
		} else if strings.HasPrefix(object[index:], "0.0-1.0") {
			out.WriteByte('0')
			index += len("0.0-1.0") - 1
			continue
		}
		out.WriteByte(ch)
	}
	return out.String()
}

// Compile-time proof that the canonical transducer exposes the kernel port
// the factory wires for Mangle-derived routing.
var _ TransducerWithKernel = (*UnderstandingTransducer)(nil)

// emptyInputFallback returns the fail-closed intent for blank input.
// Shared by the base and Gemini transducers so both honor the same
// contract: blank input classifies as /explain without calling the LLM.
func emptyInputFallback(input string) (Intent, bool) {
	if strings.TrimSpace(input) == "" {
		return Intent{Verb: "/explain", Category: "/query", Response: "Input is empty"}, true
	}
	return Intent{}, false
}

// maxClassificationInputChars caps raw classification input. ParseIntent
// truncates here, and BuildPrompt caps every other section, so the
// classification call stays bounded no matter what was pasted.
const maxClassificationInputChars = 50000

// truncateClassificationInput bounds a giant paste for the classification
// call: head and tail, line-snapped, rune-safe, with the marker naming the
// cut. Head-only dropped the end of a large paste, which is where the
// instruction ("...now fix this") usually is.
func truncateClassificationInput(input string) (string, bool) {
	if len(input) <= maxClassificationInputChars {
		return input, false
	}
	return types.ClampText(input, maxClassificationInputChars, "user input"), true
}

// degradedClassificationIntent builds the fail-soft result for a failed
// classification (a provider error or an unreadable answer). Callers do not
// expect ParseIntentWithContext to error here, so this is returned with a nil
// error; the typed cause rides on Intent.Outcome, where the perception
// firewall and the chat publish it as classification_outcome and the kernel
// derives what the turn does (policy/perception_outcome.mg). The Intent itself
// is a compatibility projection -- a zero-confidence /explain -- and is not an
// interpretation of the user.
func degradedClassificationIntent(outcome ClassificationOutcome) Intent {
	// The raw error carries the provider's response body, and a bad answer
	// carries the model's output; the user gets the class, the status and the
	// safe code instead (ClassificationOutcome.SafeMessage).
	degraded := Intent{
		Verb:     "/explain",
		Category: "/query",
		Response: fmt.Sprintf("I had trouble understanding that: %s", outcome.SafeMessage()),
		Outcome:  outcome,
	}
	if outcome.Retryable() && outcome.Disposition == ClassificationProviderFailed {
		// On the live interactive path the canonical wording comes from the
		// kernel (perception_outcome.mg). The same text is set here only for a
		// caller that reads Intent.Response directly, so the user never sees the
		// misleading "I had trouble understanding that" framing for an outage.
		degraded.Response = "I couldn't reach the language model just now - it may be briefly overloaded. Please try again in a moment."
	}
	return degraded
}

// ParseIntent parses user input into an Intent using LLM-first classification.
// This is the main entry point implementing the Transducer interface.
func (t *UnderstandingTransducer) ParseIntent(ctx context.Context, input string) (Intent, error) {
	return t.ParseIntentWithContext(ctx, input, nil)
}

// ParseIntentWithContext parses user input with conversation history.
// This is the primary method called by the chat loop.
func (t *UnderstandingTransducer) ParseIntentWithContext(ctx context.Context, input string, history []ConversationTurn) (Intent, error) {
	pipelineStart := time.Now()
	truncatedInput := input
	if len(truncatedInput) > 80 {
		truncatedInput = truncatedInput[:80] + "..."
	}
	logging.Perception("[ParseIntentWithContext] START input=%q len=%d historyTurns=%d",
		truncatedInput, len(input), len(history))

	// Empty input asks the model nothing, so it needs no classification prompt.
	if intent, done := emptyInputFallback(input); done {
		logging.Perception("[ParseIntentWithContext] empty input, returning /explain fallback")
		return intent, nil
	}

	initStart := time.Now()
	if err := t.initialize(ctx); err != nil {
		return Intent{}, err
	}
	logging.Perception("[ParseIntentWithContext] initialized: %dms", time.Since(initStart).Milliseconds())

	// Truncate extremely large input to prevent OOM/Payload Too Large
	if truncated, didTruncate := truncateClassificationInput(input); didTruncate {
		logging.Perception("[ParseIntentWithContext] input truncated from %d to %d", len(input), len(truncated))
		input = truncated
	}

	// The stability-bypass filter that used to live here (reuse the PRIOR
	// turn's understanding when recent verbs were stable and no topic-shift
	// keyword appeared) has been removed deliberately. It routinely
	// misclassified short conversational turns — "thanks" or "hello there"
	// after a run of /fix turns inherited the previous mutation intent and
	// spawned a coder shard. With classification on the fast model tier the
	// bypass saves nothing worth that failure mode: every turn is classified
	// fresh.

	// GAP-006 FIX: Run semantic classification to inject semantic_match facts into kernel
	// This provides neuro-symbolic grounding even in LLM-first mode
	var semanticMatches []SemanticMatch
	semanticStart := time.Now()
	if SharedSemanticClassifier != nil {
		logging.Perception("[ParseIntentWithContext] running semantic classification (embedding lookup)...")
		matches, err := SharedSemanticClassifier.Classify(ctx, input)
		if err != nil {
			// Graceful degradation - log but continue with LLM-only classification
			// Semantic classification is optional enhancement, not required
			_ = matches // matches already injected into kernel by Classify()
			logging.Perception("[ParseIntentWithContext] semantic classification FAILED: %v (%dms) | continuing with LLM-only",
				err, time.Since(semanticStart).Milliseconds())
		} else {
			semanticMatches = matches
			// Log match details so we can see if embedding retrieval is finding useful exemplars
			for i, sm := range semanticMatches {
				if i < 3 { // top 3 only
					logging.Perception("[ParseIntentWithContext] semantic match[%d]: sim=%.3f verb=%s target=%q text=%q",
						i, sm.Similarity, sm.Verb, sm.Target, truncateForLog(sm.TextContent, 60))
				}
			}
			logging.Perception("[ParseIntentWithContext] semantic classification: %dms | %d matches (above-threshold for prompt injection)",
				time.Since(semanticStart).Milliseconds(), len(semanticMatches))
		}
		// Note: semantic_match facts are automatically asserted by Classify()
	} else {
		logging.Perception("[ParseIntentWithContext] semantic classifier not initialized (SharedSemanticClassifier=nil) | no embedding lookup")
	}

	// Get Understanding from LLM
	llmStart := time.Now()
	strategicContext := t.getStrategicContext()
	hasStrategicCtx := len(strategicContext) > 0
	logging.Perception("[ParseIntentWithContext] calling LLMTransducer.Understand (hasStrategicContext=%v, strategicLen=%d)...",
		hasStrategicCtx, len(strategicContext))
	sessionCtx := types.GetSessionContext(ctx)
	understanding, outcome := t.llmTransducer.Understand(ctx, input, history, semanticMatches, sessionCtx, strategicContext)
	if outcome.Failed() {
		logging.Perception("[ParseIntentWithContext] LLM understanding FAILED after %dms: disposition=%s stage=%s class=%s method=%s code=%q",
			time.Since(llmStart).Milliseconds(), outcome.Disposition, outcome.Stage, outcome.Failure, outcome.Method, outcome.Code)
		logging.Get(logging.CategoryPerception).Warn("LLM classification failed: %s", outcome.SafeMessage())
		return degradedClassificationIntent(outcome), nil
	}
	logging.Perception("[ParseIntentWithContext] LLM understanding: %dms | method=%s attempts=%d",
		time.Since(llmStart).Milliseconds(), outcome.Method, outcome.Attempts)

	// GAP-018 FIX: Cache understanding for debugging
	cacheStart := time.Now()
	t.mu.Lock()
	t.lastUnderstanding = understanding
	t.mu.Unlock()

	// Convert Understanding to Intent for backward compatibility
	intent := t.understandingToIntent(understanding)
	intent.Outcome = outcome
	hasSurface := len(intent.Response) > 0
	surfacePreview := intent.Response
	if len(surfacePreview) > 100 {
		surfacePreview = surfacePreview[:100] + "..."
	}
	logging.Perception("[ParseIntentWithContext] intent conversion: %dms | mapping: action=%q→verb=%s semantic=%q→category=%s | target=%q confidence=%.2f hasSurface=%v surfacePreview=%q",
		time.Since(cacheStart).Milliseconds(),
		understanding.ActionType, intent.Verb,
		understanding.SemanticType, intent.Category,
		intent.Target, intent.Confidence,
		hasSurface, surfacePreview)

	llmDurationMs := time.Since(llmStart).Milliseconds()
	semanticDurationMs := max(time.Since(semanticStart).Milliseconds()-llmDurationMs, 0)
	logging.Perception("[ParseIntentWithContext] COMPLETE: %dms total | init=%dms semantic=%dms llm=%dms convert=%dms | result: %s%s → %q (%.0f%%)",
		time.Since(pipelineStart).Milliseconds(),
		time.Since(initStart).Milliseconds(),
		semanticDurationMs,
		llmDurationMs,
		time.Since(cacheStart).Milliseconds(),
		intent.Category, intent.Verb, intent.Target, intent.Confidence*100)

	return intent, nil
}

// abbreviatesFile reports whether target is a shortened spelling of file: the
// file's slash-normalized path ends with it.
func abbreviatesFile(target, file string) bool {
	t := strings.TrimSpace(strings.ReplaceAll(target, "\\", "/"))
	f := strings.ReplaceAll(file, "\\", "/")
	return t != "" && t != f && strings.HasSuffix(f, t)
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

	// Build target from scope. A target the file's path ends with is that
	// file abbreviated, and the file is the name a tool can open: a delegated
	// "research problems.log" sent read_file after a file that does not exist,
	// while scope.file held the run's real .nerd/logs/<run>_problems.log
	// (problems log 2026-10-28).
	target := u.Scope.Target
	if file := u.Scope.File; file != "" && (target == "" || abbreviatesFile(target, file)) {
		target = file
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
		Category:    category,
		Verb:        verb,
		Target:      target,
		Constraint:  constraint,
		Confidence:  u.Confidence,
		Response:    u.SurfaceResponse,
		IsQuestion:  u.Signals.IsQuestion || isBackupQuestionSignal(u.SemanticType, u.ActionType),
		ScopeLevel:  u.Scope.Level,
		ScopeFile:   u.Scope.File,
		ToolsNeeded: u.SuggestedApproach.ToolsNeeded,
		Brief:       u.Brief,
		Restriction: RestrictionFrom(u),

		// Preserve Understanding in Ambiguity for debugging
		Ambiguity: ambiguityInfo,

		// Map memory operations from constraints
		MemoryOperations: t.extractMemoryOperations(u),
	}
}

// isInterrogativeSemanticType reports whether the semantic type describes a
// question form. The set mirrors the semantic-type table in the
// perception_understanding atom: "recommendation" ("Should I ...?") and "state"
// ("Is X ...?") are question forms. "hypothetical" is deliberately
// excluded: what-if turns carry mode=dream through SuggestedApproach and
// route by that mode, not by question-ness. "instruction" is excluded
// because directives ("always ...") are commands, not questions.
func isInterrogativeSemanticType(semanticType string) bool {
	switch strings.ToLower(strings.TrimSpace(semanticType)) {
	case "definition", "causation", "mechanism", "location", "temporal",
		"attribution", "selection", "existence", "quantification",
		"recommendation", "state":
		return true
	default:
		return false
	}
}

// isMutationAction reports whether the action type performs code, file, or
// environment effects (as opposed to read-only inspection or conversation).
// It is the single source of truth for the /mutation filing in
// mapSemanticToCategory and for the IsQuestion dominance rule below.
func isMutationAction(actionType string) bool {
	switch strings.ToLower(strings.TrimSpace(actionType)) {
	case "implement", "modify", "refactor", "attack", "revert", "configure",
		"migrate", "optimize", "document", "scaffold", "format", "deploy":
		return true
	default:
		return false
	}
}

// isBackupQuestionSignal re-derives IsQuestion when the model forgets to set
// signals.is_question but correctly classifies the semantic type. The WHAT
// (a mutation action) dominates the HOW (an interrogative semantic form):
// "is this fast?" about code the user asked to optimize is still an
// optimize request, not a question turn.
func isBackupQuestionSignal(semanticType, actionType string) bool {
	return isInterrogativeSemanticType(semanticType) && !isMutationAction(actionType)
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
	case "diagnose_self":
		// codeNERD's own failures: the kernel's self-diagnosis chain.
		return "/self_diagnose"
	case "approve":
		// The go-ahead on a proposal the assistant made. Whether a proposal
		// is pending is the kernel's question (approve_lane,
		// routing_arbitration.mg), not this mapping's.
		return "/approve"
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

	// Actions that modify code are mutations. This set must stay aligned with
	// the taxonomy corpus categories (DefaultTaxonomyData) and the preset
	// mirror (categoryForIntentVerb): codedom_edit.mg derives next_action(
	// /edit_element) only for /mutation, so a file-writing action left as
	// /query can never edit through the CodeDOM path. verify stays /query
	// deliberately: running tests is read-only even though presets coarsely
	// bucket /test as /mutation.
	if isMutationAction(actionType) {
		return "/mutation"
	}
	switch actionType {
	case "verify":
		return "/query"
	case "remember", "forget":
		return "/instruction"
	}

	// Everything else is a query
	return "/query"
}

// extractMemoryOperations extracts memory operations from Understanding.
func (t *UnderstandingTransducer) extractMemoryOperations(u *Understanding) []articulation.MemoryOperation {
	var ops []articulation.MemoryOperation

	switch strings.ToLower(strings.TrimSpace(u.ActionType)) {
	case "remember":
		// "Remember that X" -> store X
		if u.Scope.Target != "" {
			ops = append(ops, articulation.MemoryOperation{
				Op:    "promote_to_long_term",
				Key:   "preference",
				Value: u.Scope.Target,
			})
		}
	case "forget":
		// "Forget X" -> remove X
		if u.Scope.Target != "" {
			// The person's own words: the host, not a model packet, names the
			// saved long-term memory as the domain.
			ops = append(ops, articulation.MemoryOperation{
				Op:     "forget",
				Key:    u.Scope.Target,
				Domain: articulation.MemoryDomainUserMemory,
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
// It is the same classification operation as ParseIntentWithContext -- one
// type, one method plan, so the schema-enforced attempt a schema-capable
// thinking client gets is the same through every public entry point. The
// retry limit is unused: this transducer's attempts are its method plan, not a
// Mangle-syntax validation loop. The returned mangle updates are always empty
// as LLM-first doesn't generate raw Mangle.
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
