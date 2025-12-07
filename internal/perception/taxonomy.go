package perception

import (
	"codenerd/internal/mangle"
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
)

// TaxonomyEngine manages the verb taxonomy using Mangle.
type TaxonomyEngine struct {
	engine *mangle.Engine
	store  *TaxonomyStore
}

// SharedTaxonomy is the global instance loaded on init.
var SharedTaxonomy *TaxonomyEngine

func init() {
	var err error
	SharedTaxonomy, err = NewTaxonomyEngine()
	if err != nil {
		// In production, we might log this instead of panicking,
		// but for an embedded asset, panic ensures we catch invalid schemas early.
		// We use fmt.Printf because the logger might not be configured yet.
		fmt.Printf("CRITICAL: failed to init shared taxonomy: %v\n", err)
	}
}

// NewTaxonomyEngine creates a new taxonomy engine with the default corpus.
func NewTaxonomyEngine() (*TaxonomyEngine, error) {
	// Initialize a lightweight in-memory engine
	cfg := mangle.DefaultConfig()
	cfg.AutoEval = true // Auto-evaluate rules

	// No persistence needed for read-only taxonomy
	eng, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to init taxonomy engine: %w", err)
	}

	// Load the embedded taxonomy
	if err := eng.LoadSchemaString(DefaultTaxonomyMG); err != nil {
		return nil, fmt.Errorf("failed to load taxonomy schema: %w", err)
	}

	// Load the inference logic
	if err := eng.LoadSchemaString(InferenceLogicMG); err != nil {
		return nil, fmt.Errorf("failed to load inference logic: %w", err)
	}

	// Load learned rules if available (Best Effort)
	learnedPath := "internal/mangle/learned.mg"
	if _, err := os.Stat(learnedPath); err == nil {
		if err := eng.LoadSchema(learnedPath); err != nil {
			// Log but continue - don't break core taxonomy on corrupt learned file
			fmt.Printf("WARNING: Failed to load learned taxonomy: %v\n", err)
		}
	}

	return &TaxonomyEngine{engine: eng}, nil
}

// SetStore attaches a persistence store to the taxonomy engine.
func (t *TaxonomyEngine) SetStore(s *TaxonomyStore) {
	t.store = s
}

// HydrateFromDB loads all taxonomy facts from the database into the engine.
func (t *TaxonomyEngine) HydrateFromDB() error {
	if t.store == nil {
		return fmt.Errorf("no store configured")
	}
	return t.store.HydrateEngine(t.engine)
}

// EnsureDefaults populates the database with the default taxonomy if it's empty.
func (t *TaxonomyEngine) EnsureDefaults() error {
	if t.store == nil {
		return fmt.Errorf("no store configured")
	}

	// Check if we have any taxonomy facts
	facts, err := t.store.LoadAllTaxonomyFacts()
	if err != nil {
		return err
	}

	if len(facts) > 0 {
		return nil // Already populated
	}

	// Parse the DefaultTaxonomyMG string to extract facts
	// This is a bit hacky but avoids duplicating the data definitions.
	// A robust way is to query the engine (which has the defaults loaded) and store them.
	
	// Query all verb definitions
	res, err := t.engine.Query(context.Background(), "verb_def(Verb, Cat, Shard, Prio)")
	if err == nil {
		for _, row := range res.Bindings {
			verb := row["Verb"].(string)
			cat := row["Cat"].(string)
			shard := row["Shard"].(string)
			// Handle Mangle float/int conversion
			prioVal := row["Prio"]
			prio := 0
			if f, ok := prioVal.(float64); ok {
				prio = int(f)
			} else if i, ok := prioVal.(int); ok {
				prio = i
			} else if i64, ok := prioVal.(int64); ok {
				prio = int(i64)
			}
			
			t.store.StoreVerbDef(verb, cat, shard, prio)
		}
	}

	// Query synonyms
	res, err = t.engine.Query(context.Background(), "verb_synonym(Verb, Syn)")
	if err == nil {
		for _, row := range res.Bindings {
			t.store.StoreVerbSynonym(row["Verb"].(string), row["Syn"].(string))
		}
	}

	// Query patterns
	res, err = t.engine.Query(context.Background(), "verb_pattern(Verb, Pat)")
	if err == nil {
		for _, row := range res.Bindings {
			t.store.StoreVerbPattern(row["Verb"].(string), row["Pat"].(string))
		}
	}

	return nil
}

// GetVerbs returns all defined verbs with their metadata.
func (t *TaxonomyEngine) GetVerbs() ([]VerbEntry, error) {
	// Query: verb_def(Verb, Category, Shard, Priority)
	result, err := t.engine.Query(context.Background(), "verb_def(Verb, Category, Shard, Priority)")
	if err != nil {
		return nil, err
	}

	var verbs []VerbEntry
	for _, row := range result.Bindings {
		v := VerbEntry{
			Verb:      row["Verb"].(string),
			Category:  row["Category"].(string),
			ShardType: row["Shard"].(string),
			Priority:  int(row["Priority"].(float64)), // Mangle numbers are floats in bindings usually
		}
		if v.ShardType == "/none" {
			v.ShardType = ""
		}
		
		// Populate synonyms and patterns
		syns, _ := t.getSynonyms(v.Verb)
		v.Synonyms = syns
		
		// For patterns, we need to recompile regexes
		patterns, _ := t.getPatterns(v.Verb)
		for _, p := range patterns {
			if re, err := regexp.Compile(p); err == nil {
				v.Patterns = append(v.Patterns, re)
			}
		}

		verbs = append(verbs, v)
	}

	// Sort by priority desc
	sort.Slice(verbs, func(i, j int) bool {
		return verbs[i].Priority > verbs[j].Priority
	})

	return verbs, nil
}

func (t *TaxonomyEngine) getSynonyms(verb string) ([]string, error) {
	// verb_synonym(Verb, Synonym)
	// We need to query by specific verb.
	query := fmt.Sprintf("verb_synonym(%s, Syn)", verb)
	if !strings.HasPrefix(verb, "/") {
		query = fmt.Sprintf("verb_synonym(/%s, Syn)", strings.TrimPrefix(verb, "/"))
	}
	
	result, err := t.engine.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}
	
	var syns []string
	for _, row := range result.Bindings {
		syns = append(syns, row["Syn"].(string))
	}
	return syns, nil
}

func (t *TaxonomyEngine) getPatterns(verb string) ([]string, error) {
	// verb_pattern(Verb, Regex)
	query := fmt.Sprintf("verb_pattern(%s, Regex)", verb)
	if !strings.HasPrefix(verb, "/") {
		query = fmt.Sprintf("verb_pattern(/%s, Regex)", strings.TrimPrefix(verb, "/"))
	}

	result, err := t.engine.Query(context.Background(), query)
	if err != nil {
		return nil, err
	}

	var pats []string
	for _, row := range result.Bindings {
		pats = append(pats, row["Regex"].(string))
	}
	return pats, nil
}

// ClassifyInput uses advanced Mangle inference to determine the best intent.
// This combines regex-based candidates with contextual logic rules.
func (t *TaxonomyEngine) ClassifyInput(input string, candidates []VerbEntry) (bestVerb string, bestConf float64, err error) {
	// 1. Clear previous transient state (if we had sessions, we'd use a fresh one)
	t.engine.Clear() // Drastic, but safe for stateless classification.
	
	// Re-hydrate base facts from DB/Schema if Clear() wiped them
	// Clear() in engine.go resets baseStore. We need to ensure static facts are preserved.
	// Actually, for this shared engine, Clear() is dangerous if we lose the loaded schema facts.
	// Instead of Clear(), we should use a separate "session" store for transient facts or 
	// re-insert the static facts.
	// Better approach for POC: Re-load defaults/DB after clear.
	// Or rely on the fact that schema facts (rules) are in programInfo, but EDB facts (verb_def) 
	// are in the store.
	// So after Clear(), we MUST re-hydrate.
	if t.store != nil {
		t.HydrateFromDB()
	} else {
		// Fallback to embedded defaults
		t.engine.LoadSchemaString(DefaultTaxonomyMG)
	}

	// 2. Insert Context Facts
	facts := []mangle.Fact{}

	// Tokenize input (simple split for POC)
	tokens := strings.Fields(strings.ToLower(input))
	for _, token := range tokens {
		facts = append(facts, mangle.Fact{Predicate: "context_token", Args: []interface{}{token}})
	}

	// Insert Candidates (from regex match)
	// We reuse the existing regex logic to generate candidates, then let Mangle refine them.
	if len(candidates) == 0 {
		return "", 0, nil
	}

	for _, cand := range candidates {
		// Base score is roughly Priority. We normalize it later.
		baseScore := float64(cand.Priority)
		facts = append(facts, mangle.Fact{
			Predicate: "candidate_intent",
			Args:      []interface{}{cand.Verb, baseScore},
		})
	}

	if err := t.engine.AddFacts(facts); err != nil {
		return "", 0, fmt.Errorf("failed to add facts: %w", err)
	}

	// 3. Run Inference
	// Query: selected_verb(Verb)
	result, err := t.engine.Query(context.Background(), "selected_verb(Verb)")
	if err != nil {
		return "", 0, fmt.Errorf("inference query failed: %w", err)
	}

	if len(result.Bindings) > 0 {
		verb := result.Bindings[0]["Verb"].(string)
		// Return 1.0 confidence if logic selected it
		return verb, 1.0, nil
	}

	// Fallback: no logic selection, stick to regex winner
	return "", 0, nil
}

// LearnSynonym persists a new synonym for a verb, enabling self-improvement (Autopoiesis).
func (t *TaxonomyEngine) LearnSynonym(verb, synonym string) error {
	// 1. Validate verb exists
	valid := false
	verbs, _ := t.GetVerbs()
	for _, v := range verbs {
		if v.Verb == verb {
			valid = true
			break
		}
	}
	if !valid {
		return fmt.Errorf("unknown verb: %s", verb)
	}

	// 2. Update DB if available (Primary Persistence)
	if t.store != nil {
		if err := t.store.StoreVerbSynonym(verb, synonym); err != nil {
			return fmt.Errorf("failed to persist synonym to DB: %w", err)
		}
	}

	// 3. Append to learned.mg (Backup / Human Readable)
	fact := fmt.Sprintf("\nverb_synonym(%s, \"%s\").", verb, synonym)
	path := "internal/mangle/learned.mg"
	
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		defer f.Close()
		f.WriteString(fact)
	} else {
		fmt.Printf("WARNING: Failed to write to learned.mg: %v\n", err)
	}

	// 4. Reload schema/engine to apply immediately
	// If we have a store, re-hydration handles it. If not, LoadSchemaString.
	if t.store != nil {
		return t.HydrateFromDB()
	}
	return t.engine.LoadSchemaString(fact)
}

// GenerateSystemPromptSection generates the "VERB TAXONOMY" section for the LLM prompt.
func (t *TaxonomyEngine) GenerateSystemPromptSection() (string, error) {
	verbs, err := t.GetVerbs()
	if err != nil {
		return "", err
	}

	// Group by category/shard for readability
	type Group struct {
		Name  string
		Verbs []VerbEntry
	}
	groups := make(map[string]*Group)
	var groupOrder []string

	for _, v := range verbs {
		key := fmt.Sprintf("%s (%s)", v.Category, v.ShardType)
		if v.ShardType == "" {
			key = fmt.Sprintf("%s (General)", v.Category)
		}
		
		if _, exists := groups[key]; !exists {
			groups[key] = &Group{Name: key}
			groupOrder = append(groupOrder, key)
		}
		groups[key].Verbs = append(groups[key].Verbs, v)
	}
	sort.Strings(groupOrder)

	var sb strings.Builder
	sb.WriteString("## VERB TAXONOMY (Comprehensive)\n\n")

	for _, key := range groupOrder {
		g := groups[key]
		sb.WriteString(fmt.Sprintf("### %s\n", key))
		for _, v := range g.Verbs {
			// Format: - /verb: synonym1, synonym2, ...
			sb.WriteString(fmt.Sprintf("- %s: %s\n", v.Verb, strings.Join(v.Synonyms, ", ")))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// DefaultTaxonomyMG contains the Mangle definitions for the verb corpus.
const DefaultTaxonomyMG = `
Decl verb_def(Verb.Type<n>, Category.Type<n>, Shard.Type<n>, Priority.Type<int>).
Decl verb_synonym(Verb.Type<n>, Synonym.Type<string>).
Decl verb_pattern(Verb.Type<n>, Regex.Type<string>).

# =========================================================================
# CODE REVIEW & ANALYSIS (Reviewer)
# =========================================================================

# /review
verb_def(/review, /query, /reviewer, 100).
verb_synonym(/review, "review").
verb_synonym(/review, "code review").
verb_synonym(/review, "pr review").
verb_synonym(/review, "check code").
verb_synonym(/review, "audit").
verb_synonym(/review, "evaluate").
verb_synonym(/review, "critique").
verb_pattern(/review, "(?i)review\s+(this|the|my|our)?\s*(file|code|changes?|diff|pr|pull\s*request)?").
verb_pattern(/review, "(?i)can\s+you\s+review").
verb_pattern(/review, "(?i)check\s+(this|the|my)?\s*(code|file)").

# /security
verb_def(/security, /query, /reviewer, 105).
verb_synonym(/security, "security").
verb_synonym(/security, "security scan").
verb_synonym(/security, "vulnerability").
verb_synonym(/security, "injection").
verb_synonym(/security, "xss").
verb_pattern(/security, "(?i)security\s+(scan|check|audit|review|analysis)").
verb_pattern(/security, "(?i)check\s+(for\s+)?(security|vulnerabilities|vulns)").
verb_pattern(/security, "(?i)find\s+(security\s+)?(vulnerabilities|issues|bugs)").

# /analyze
verb_def(/analyze, /query, /reviewer, 95).
verb_synonym(/analyze, "analyze").
verb_synonym(/analyze, "complexity").
verb_synonym(/analyze, "metrics").
verb_synonym(/analyze, "lint").
verb_synonym(/analyze, "code smell").
verb_pattern(/analyze, "(?i)analy[sz]e\s+(this|the|my)?\s*(code|file|codebase)?").
verb_pattern(/analyze, "(?i)(code\s+)?(complexity|metrics|quality)").
verb_pattern(/analyze, "(?i)static\s+analysis").

# =========================================================================
# UNDERSTANDING (Researcher/None)
# =========================================================================

# /explain
verb_def(/explain, /query, /none, 80).
verb_synonym(/explain, "explain").
verb_synonym(/explain, "describe").
verb_synonym(/explain, "what is").
verb_synonym(/explain, "how does").
verb_synonym(/explain, "help me understand").
verb_pattern(/explain, "(?i)explain\s+(this|the|how|what|why)?").
verb_pattern(/explain, "(?i)tell\s+me\s+(about|how|what|why)").
verb_pattern(/explain, "(?i)help\s+me\s+understand").

# /explore
verb_def(/explore, /query, /researcher, 75).
verb_synonym(/explore, "explore").
verb_synonym(/explore, "browse").
verb_synonym(/explore, "show structure").
verb_synonym(/explore, "list files").
verb_pattern(/explore, "(?i)show\s+(me\s+)?(the\s+)?(structure|architecture|layout|files?)").
verb_pattern(/explore, "(?i)explore\s+(the\s+)?(codebase|project|code)?").

# /search
verb_def(/search, /query, /researcher, 85).
verb_synonym(/search, "search").
verb_synonym(/search, "find").
verb_synonym(/search, "grep").
verb_synonym(/search, "occurrences").
verb_pattern(/search, "(?i)search\s+(for\s+)?").
verb_pattern(/search, "(?i)find\s+(all\s+)?(occurrences?|references?|usages?|uses?)").
verb_pattern(/search, "(?i)grep\s+").

# =========================================================================
# MUTATION (Coder)
# =========================================================================

# /fix
verb_def(/fix, /mutation, /coder, 90).
verb_synonym(/fix, "fix").
verb_synonym(/fix, "repair").
verb_synonym(/fix, "patch").
verb_synonym(/fix, "resolve").
verb_synonym(/fix, "bug fix").
verb_pattern(/fix, "(?i)fix\s+(this|the|my|that|a)?\s*(bug|error|issue|problem)?").
verb_pattern(/fix, "(?i)repair\s+").
verb_pattern(/fix, "(?i)resolve\s+(this|the)?\s*(issue|error|bug)?").

# /refactor
verb_def(/refactor, /mutation, /coder, 88).
verb_synonym(/refactor, "refactor").
verb_synonym(/refactor, "clean up").
verb_synonym(/refactor, "improve").
verb_synonym(/refactor, "optimize").
verb_synonym(/refactor, "simplify").
verb_pattern(/refactor, "(?i)refactor\s+").
verb_pattern(/refactor, "(?i)clean\s*up\s+").
verb_pattern(/refactor, "(?i)improve\s+(the\s+)?(code|quality|readability|performance)").

# /create
verb_def(/create, /mutation, /coder, 85).
verb_synonym(/create, "create").
verb_synonym(/create, "new").
verb_synonym(/create, "add").
verb_synonym(/create, "implement").
verb_synonym(/create, "generate").
verb_pattern(/create, "(?i)create\s+(a\s+)?(new\s+)?").
verb_pattern(/create, "(?i)add\s+(a\s+)?(new\s+)?").
verb_pattern(/create, "(?i)implement\s+").

# /write
verb_def(/write, /mutation, /coder, 70).
verb_synonym(/write, "write").
verb_synonym(/write, "save").
verb_synonym(/write, "export").
verb_pattern(/write, "(?i)write\s+(to\s+)?(file|disk)?").
verb_pattern(/write, "(?i)save\s+(to\s+)?").

# /delete
verb_def(/delete, /mutation, /coder, 85).
verb_synonym(/delete, "delete").
verb_synonym(/delete, "remove").
verb_synonym(/delete, "drop").
verb_pattern(/delete, "(?i)delete\s+").
verb_pattern(/delete, "(?i)remove\s+").

# =========================================================================
# DEBUGGING (Coder)
# =========================================================================

# /debug
verb_def(/debug, /query, /coder, 92).
verb_synonym(/debug, "debug").
verb_synonym(/debug, "troubleshoot").
verb_synonym(/debug, "diagnose").
verb_synonym(/debug, "root cause").
verb_pattern(/debug, "(?i)debug\s+").
verb_pattern(/debug, "(?i)troubleshoot\s+").
verb_pattern(/debug, "(?i)why\s+(is|does|did)\s+(this|it)\s+(fail|error|crash|break)").

# =========================================================================
# TESTING (Tester)
# =========================================================================

# /test
verb_def(/test, /mutation, /tester, 88).
verb_synonym(/test, "test").
verb_synonym(/test, "unit test").
verb_synonym(/test, "run tests").
verb_synonym(/test, "coverage").
verb_pattern(/test, "(?i)(write|add|create)\s+(a\s+)?(unit\s+)?tests?").
verb_pattern(/test, "(?i)run\s+(the\s+)?tests?").
verb_pattern(/test, "(?i)test\s+(this|the|coverage)?").

# =========================================================================
# RESEARCH (Researcher)
# =========================================================================

# /research
verb_def(/research, /query, /researcher, 75).
verb_synonym(/research, "research").
verb_synonym(/research, "learn").
verb_synonym(/research, "docs").
verb_synonym(/research, "documentation").
verb_pattern(/research, "(?i)research\s+").
verb_pattern(/research, "(?i)learn\s+(about|how)").
verb_pattern(/research, "(?i)(show|find)\s+(me\s+)?(the\s+)?docs").

# =========================================================================
# SETUP & CONFIG
# =========================================================================

# /init
verb_def(/init, /mutation, /researcher, 70).
verb_synonym(/init, "init").
verb_synonym(/init, "setup").
verb_synonym(/init, "bootstrap").
verb_pattern(/init, "(?i)^init(iali[sz]e)?$").
verb_pattern(/init, "(?i)set\s*up\s+").

# /configure
verb_def(/configure, /instruction, /none, 65).
verb_synonym(/configure, "configure").
verb_synonym(/configure, "config").
verb_synonym(/configure, "settings").
verb_pattern(/configure, "(?i)configure\s+").
verb_pattern(/configure, "(?i)change\s+(the\s+)?setting").

# =========================================================================
# CAMPAIGN
# =========================================================================

# /campaign
verb_def(/campaign, /mutation, /coder, 95).
verb_synonym(/campaign, "campaign").
verb_synonym(/campaign, "epic").
verb_synonym(/campaign, "feature").
verb_pattern(/campaign, "(?i)start\s+(a\s+)?campaign").
verb_pattern(/campaign, "(?i)implement\s+(a\s+)?(full|complete|entire)\s+").

# =========================================================================
# AUTOPOIESIS (Tool Generation)
# =========================================================================

# /generate_tool
verb_def(/generate_tool, /mutation, /tool_generator, 95).
verb_synonym(/generate_tool, "generate tool").
verb_synonym(/generate_tool, "create tool").
verb_synonym(/generate_tool, "need a tool").
verb_pattern(/generate_tool, "(?i)(create|make|generate|build)\s+(a\s+)?tool\s+(for|to|that)").
verb_pattern(/generate_tool, "(?i)i\s+need\s+(a\s+)?tool\s+(for|to)").
`

// InferenceLogicMG contains the advanced patterns for context-aware classification.
const InferenceLogicMG = `
# Inference Logic for Intent Refinement
# This module takes raw intent candidates (from regex/LLM) and refines them
# using contextual logic and safety constraints.

Decl candidate_intent(Verb.Type<n>, RawScore.Type<float>).
Decl context_token(Token.Type<string>).
# Decl system_state(Key.Type<string>, Value.Type<string>).

# Output: Refined Score
Decl refined_score(Verb.Type<n>, Score.Type<float>).

# Base score from candidate
refined_score(Verb, Score) :-
    candidate_intent(Verb, Score).

# -----------------------------------------------------------------------------
# CONTEXTUAL BOOSTING
# -----------------------------------------------------------------------------

# Security Boost: If "security" or "vuln" appears, boost /security
# Even if regex matched /review, context implies /security is better.
boost(Verb, 0.3) :-
    candidate_intent(Verb, _),
    Verb == /security,
    context_token("security").

boost(Verb, 0.3) :-
    candidate_intent(Verb, _),
    Verb == /security,
    context_token("vulnerability").

# Testing Boost: If "coverage" appears, prefer /test over /review
boost(Verb, 0.2) :-
    candidate_intent(Verb, _),
    Verb == /test,
    context_token("coverage").

# Debugging Boost: If "error" or "panic" appears, prefer /debug over /fix
# fixing is the goal, but debugging is the immediate action.
boost(Verb, 0.15) :-
    candidate_intent(Verb, _),
    Verb == /debug,
    context_token("panic").

boost(Verb, 0.15) :-
    candidate_intent(Verb, _),
    Verb == /debug,
    context_token("stacktrace").

# -----------------------------------------------------------------------------
# SAFETY CONSTRAINTS (Penalties)
# -----------------------------------------------------------------------------

# Safety: Don't /delete if we are in a "learning" mode or context implies "safe"
penalty(Verb, 0.5) :-
    candidate_intent(Verb, _),
    Verb == /delete,
    context_token("safe").

# Ambiguity: If "fix" and "test" both appear, "fix" usually dominates,
# but if "verify" is present, "test" should win.
boost(Verb, 0.2) :-
    candidate_intent(Verb, _),
    Verb == /test,
    context_token("verify").

# -----------------------------------------------------------------------------
# FINAL AGGREGATION
# -----------------------------------------------------------------------------

Decl final_adjustment(Verb.Type<n>, Delta.Type<float>).

final_adjustment(Verb, D) :-
    boost(Verb, Amount) |>
    do fn:group_by(Verb),
    let D = fn:Sum(Amount).

final_adjustment(Verb, D) :-
    penalty(Verb, Amount) |>
    do fn:group_by(Verb),
    let P = fn:Sum(Amount) |>
    let D = fn:negate(P). # Negative delta

# Calculate Final Score
Decl final_intent_score(Verb.Type<n>, Score.Type<float>).

final_intent_score(Verb, Final) :-
    candidate_intent(Verb, Base),
    !final_adjustment(Verb, _) |> # No adjustments
    let Final = Base.

final_intent_score(Verb, Final) :-
    candidate_intent(Verb, Base),
    final_adjustment(Verb, Delta) |> 
    let Final = fn:plus(Base, Delta).

# Select Best (Max)
Decl best_intent_score(MaxScore.Type<float>).
best_intent_score(M) :-
    final_intent_score(_, S) |>
    do fn:group_by(),
    let M = fn:Max(S).

Decl selected_verb(Verb.Type<n>).
selected_verb(Verb) :-
    final_intent_score(Verb, Score),
    best_intent_score(Max),
    Score == Max.
`