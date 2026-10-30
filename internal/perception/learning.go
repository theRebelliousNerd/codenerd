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
	"codenerd/internal/mangle"
	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/symbols"
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

// learningCriticRole is the role whose atoms
// (atoms/perception/learning_critic.yaml) compile the critic's system prompt.
// Its reply is one learned_exemplar fact or nothing, parsed by
// ExtractFactFromResponse; the "judge" intent keeps worker intent atoms out.
const (
	learningCriticRole   = "learning_critic"
	learningCriticIntent = "judge"
)

// SetRolePrompts wires the compiler of the learning critic's system prompt.
// Without one LearnFromInteraction reports types.ErrRolePromptUnavailable.
func (t *TaxonomyEngine) SetRolePrompts(rolePrompts types.RolePromptCompiler) {
	t.mu.Lock()
	t.rolePrompts = rolePrompts
	t.mu.Unlock()
}

// criticSystemPrompt compiles the learning critic's system prompt from its
// atoms. There is no embedded copy: no compiler, or a failed compile, is an
// error wrapping types.ErrRolePromptUnavailable.
func (t *TaxonomyEngine) criticSystemPrompt(ctx context.Context) (string, error) {
	t.mu.Lock()
	rolePrompts := t.rolePrompts
	t.mu.Unlock()
	if rolePrompts == nil {
		return "", fmt.Errorf("%w: no compiler for the %s prompt", types.ErrRolePromptUnavailable, learningCriticRole)
	}
	system, err := rolePrompts.CompileRolePrompt(ctx, types.RolePromptRequest{Role: learningCriticRole, Intent: learningCriticIntent})
	if err != nil {
		if errors.Is(err, types.ErrRolePromptUnavailable) {
			return "", err
		}
		return "", fmt.Errorf("%w: %s: %w", types.ErrRolePromptUnavailable, learningCriticRole, err)
	}
	if strings.TrimSpace(system) == "" {
		return "", fmt.Errorf("%w: the %s prompt compiled empty", types.ErrRolePromptUnavailable, learningCriticRole)
	}
	return system, nil
}

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
		action := truncateForCritic(visibleTaskOutput(trace.Response), maxTraceFieldChars)
		if strings.TrimSpace(trace.Response) == "" {
			action = "(no reply yet)"
		}
		success := fmt.Sprintf("%v", trace.Success)
		if trace.OutcomeUnknown {
			success = "unknown"
		}
		transcript.WriteString(fmt.Sprintf("User: %s\nAgent Action: %s\nSuccess: %s\n---\n",
			truncateForCritic(trace.UserPrompt, maxTraceFieldChars), action, success))
	}

	// 2. Ask the Critic to evaluate
	criticSystem, err := t.criticSystemPrompt(ctx)
	if err != nil {
		logging.Get(logging.CategoryPerception).Error("LearnFromInteraction: %v", err)
		return "", fmt.Errorf("critic failed: %w", err)
	}
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
	resp, err := t.client.CompleteWithSystem(WithAllowEmptyCompletion(callCtx), criticSystem, criticInput)
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
	exemplar, err := parseLearnedExemplar(fact)
	if err != nil {
		// Normalization RECONSTRUCTS the fact from parsed components, which is
		// what makes the hot-load below safe. A critic payload that fails to
		// parse (trailing clauses, wrong arity) is refused here instead of
		// reaching the engine as unvalidated source.
		logging.Get(logging.CategoryPerception).Warn("PersistLearnedFact: refusing unparseable fact: %v", err)
		return outcome, fmt.Errorf("refusing to persist unparseable learned fact: %w", err)
	}
	normalizedFact := exemplar.source()
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

	// 3. The derived projections, from the same descriptor (float confidence).
	pat, v, tgt, cons, conf := exemplar.Pattern, exemplar.Verb, exemplar.Target, exemplar.Constraint, exemplar.Confidence

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

// learnedExemplar is one learned_exemplar fact as typed values: the
// descriptor the hot-load source, the taxonomy file and the projections are
// all derived from.
type learnedExemplar struct {
	Pattern, Verb, Target, Constraint string
	Confidence                        float64
}

// parseLearnedExemplar reads exactly one ground
// learned_exemplar("Pattern", /verb, "Target", "Constraint", Confidence)
// through the serialized Mangle parser: strings are Mangle string constants,
// the verb a name constant, the confidence a number. Anything else -- a
// second clause, a rule, a variable, another type, an unfinished literal --
// is refused.
//
// The bespoke parser this replaced split on commas, stripped the field
// quotes and decoded \" to ", and the normalizer pasted the decoded text
// back between quotes: a correction whose text held an escaped quote became
// malformed source and was refused, or read as different text (audit
// learned-example-literal-fidelity).
func parseLearnedExemplar(fact string) (learnedExemplar, error) {
	src := strings.TrimSpace(fact)
	if !strings.HasPrefix(src, "learned_exemplar(") {
		return learnedExemplar{}, fmt.Errorf("invalid predicate")
	}
	if !strings.HasSuffix(src, ".") {
		src += "."
	}
	unit, err := mangle.ParseUnit(strings.NewReader(src))
	if err != nil {
		return learnedExemplar{}, fmt.Errorf("not one Mangle fact: %w", err)
	}
	// The parser adds the unit's implicit Package declaration; any other is
	// source the fact must not carry.
	decls := 0
	for _, d := range unit.Decls {
		if d.DeclaredAtom.Predicate != symbols.Package {
			decls++
		}
	}
	if decls > 0 || len(unit.Clauses) != 1 {
		return learnedExemplar{}, fmt.Errorf("want exactly one learned_exemplar fact, got %d clause(s) and %d declaration(s)", len(unit.Clauses), decls)
	}
	clause := unit.Clauses[0]
	if len(clause.Premises) > 0 || clause.Transform != nil {
		return learnedExemplar{}, fmt.Errorf("learned_exemplar must be a fact, not a rule")
	}
	if clause.Head.Predicate.Symbol != "learned_exemplar" {
		return learnedExemplar{}, fmt.Errorf("invalid predicate %s", clause.Head.Predicate.Symbol)
	}
	args := clause.Head.Args
	if len(args) != 5 {
		return learnedExemplar{}, fmt.Errorf("expected 5 args, got %d", len(args))
	}
	constant := func(i int, want ast.ConstantType, what string) (ast.Constant, error) {
		c, ok := args[i].(ast.Constant)
		if !ok || c.Type != want {
			return ast.Constant{}, fmt.Errorf("argument %d must be %s, got %v", i+1, what, args[i])
		}
		return c, nil
	}
	var e learnedExemplar
	for i, dst := range map[int]*string{0: &e.Pattern, 2: &e.Target, 3: &e.Constraint} {
		c, err := constant(i, ast.StringType, "a string")
		if err != nil {
			return learnedExemplar{}, err
		}
		*dst = c.Symbol
	}
	verb, err := constant(1, ast.NameType, "a /verb name")
	if err != nil {
		return learnedExemplar{}, err
	}
	e.Verb = verb.Symbol
	conf, ok := args[4].(ast.Constant)
	switch {
	case ok && conf.Type == ast.NumberType:
		e.Confidence = float64(conf.NumValue)
	case ok && conf.Type == ast.Float64Type:
		f, ferr := conf.Float64Value()
		if ferr != nil {
			return learnedExemplar{}, fmt.Errorf("malformed confidence %v: %w", args[4], ferr)
		}
		e.Confidence = f
	default:
		return learnedExemplar{}, fmt.Errorf("malformed confidence %v", args[4])
	}
	return e, nil
}

// source is the exemplar as normalized Mangle source, its confidence an
// integer 0-100 (Mangle rules compare `Conf > 80`): a float 0.0-1.0 is
// scaled, anything above 1 is taken as already scaled. Every literal is
// written by Mangle's own quoting, so the text the engine and the file hold
// is the text that was parsed.
func (e learnedExemplar) source() string {
	confInt := int(e.Confidence)
	if e.Confidence <= 1.0 {
		confInt = int(e.Confidence * 100)
	}
	verb, err := ast.Name(e.Verb)
	verbText := e.Verb
	if err == nil {
		verbText = verb.String()
	}
	return fmt.Sprintf("learned_exemplar(%s, %s, %s, %s, %d).",
		ast.String(e.Pattern).String(), verbText, ast.String(e.Target).String(), ast.String(e.Constraint).String(), confInt)
}

// FormatLearnedFact writes a learned_exemplar from its parts, normalized as
// NormalizeLearnedFact writes it: the proposal of a producer that has a
// mapping but no critic fact (an unmapped verb the executive saw) is stored
// and confirmed as exactly this text. verb must be a /name.
func FormatLearnedFact(pattern, verb, target, constraint string, confidence float64) (string, error) {
	if _, err := ast.Name(verb); err != nil {
		return "", fmt.Errorf("learned fact verb %q is not a name: %w", verb, err)
	}
	return learnedExemplar{Pattern: pattern, Verb: verb, Target: target, Constraint: constraint, Confidence: confidence}.source(), nil
}

// ParseLearnedFact extracts components from a learned_exemplar fact string.
// Format: learned_exemplar("Pattern", /verb, "target", "constraint", Confidence).
// See parseLearnedExemplar.
func ParseLearnedFact(fact string) (pattern, verb, target, constraint string, confidence float64, err error) {
	e, err := parseLearnedExemplar(fact)
	if err != nil {
		logging.PerceptionDebug("ParseLearnedFact: refusing %q: %v", fact, err)
		return "", "", "", "", 0, err
	}
	return e.Pattern, e.Verb, e.Target, e.Constraint, e.Confidence, nil
}

// NormalizeLearnedFact rewrites a learned_exemplar fact with integer
// confidence (learnedExemplar.source). On a parse failure it returns the
// original text with the error.
func NormalizeLearnedFact(fact string) (string, error) {
	e, err := parseLearnedExemplar(fact)
	if err != nil {
		logging.PerceptionDebug("NormalizeLearnedFact: parse failed, returning original: %v", err)
		return fact, err
	}
	normalized := e.source()
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
