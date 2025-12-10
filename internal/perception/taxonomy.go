package perception

import (
	"codenerd/internal/core"
	"codenerd/internal/mangle"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// TaxonomyEngine manages the verb taxonomy using Mangle.
type TaxonomyEngine struct {
	engine        *mangle.Engine
	store         *TaxonomyStore
	client        LLMClient
	workspaceRoot string // Explicit workspace root (for .nerd paths)
}

// SharedTaxonomy is the global instance loaded on init.
var SharedTaxonomy *TaxonomyEngine

func init() {
	var err error
	SharedTaxonomy, err = NewTaxonomyEngine()
	if err != nil {
		fmt.Printf("CRITICAL: failed to init shared taxonomy: %v\n", err)
	}
}

// NewTaxonomyEngine creates a new taxonomy engine with the default corpus.
func NewTaxonomyEngine() (*TaxonomyEngine, error) {
	cfg := mangle.DefaultConfig()
	cfg.AutoEval = true

	eng, err := mangle.NewEngine(cfg, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to init taxonomy engine: %w", err)
	}

	// Load Intent Definition Schema (Canonical Examples)
	intentContent, err := core.GetDefaultContent("schema/intent.mg")
	if err == nil {
		if err := eng.LoadSchemaString(intentContent); err != nil {
			return nil, fmt.Errorf("failed to load intent schema: %w", err)
		}
	} else {
		// Fallback to disk (dev mode) or warn
		fmt.Printf("WARNING: intent.mg not found in embedded defaults: %v\n", err)
	}

	// Load Learning Schema (Ouroboros)
	learningContent, err := core.GetDefaultContent("schema/learning.mg")
	if err == nil {
		if err := eng.LoadSchemaString(learningContent); err != nil {
			return nil, fmt.Errorf("failed to load learning schema: %w", err)
		}
	} else {
		fmt.Printf("WARNING: learning.mg not found in embedded defaults: %v. Using fallback declaration.\n", err)
		// Fallback declaration if file missing, to satisfy InferenceLogicMG
		if err := eng.LoadSchemaString("Decl learned_exemplar(Pattern, Verb, Target, Constraint, Confidence)."); err != nil {
			return nil, fmt.Errorf("failed to define fallback learned_exemplar: %w", err)
		}
	}

	// Load declarations and logic (Must be loaded AFTER learning.mg)
	if err := eng.LoadSchemaString(InferenceLogicMG); err != nil {
		return nil, fmt.Errorf("failed to load inference logic: %w", err)
	}

	// Populate default data (robustly via Go)
	for _, entry := range DefaultTaxonomyData {
		// verb_def
		if err := eng.AddFact("verb_def", entry.Verb, entry.Category, entry.ShardType, entry.Priority); err != nil {
			return nil, fmt.Errorf("failed to add verb_def %s: %w", entry.Verb, err)
		}
		// synonyms
		for _, syn := range entry.Synonyms {
			if err := eng.AddFact("verb_synonym", entry.Verb, syn); err != nil {
				return nil, fmt.Errorf("failed to add synonym %s: %w", syn, err)
			}
		}
		// patterns
		for _, pat := range entry.Patterns {
			if err := eng.AddFact("verb_pattern", entry.Verb, pat); err != nil {
				return nil, fmt.Errorf("failed to add pattern %s: %w", pat, err)
			}
		}
	}

	// Load learned rules if available
	learnedPath := "internal/mangle/learned.mg"
	if _, err := os.Stat(learnedPath); err == nil {
		// Only load if it's valid
		if err := eng.LoadSchema(learnedPath); err != nil {
			fmt.Printf("WARNING: Failed to load learned taxonomy: %v\n", err)
		}
	}

	return &TaxonomyEngine{engine: eng}, nil
}

// SetStore attaches a persistence store to the taxonomy engine.
func (t *TaxonomyEngine) SetStore(s *TaxonomyStore) {
	t.store = s
}

// SetWorkspace sets the explicit workspace root path for .nerd directory resolution.
// This MUST be called to ensure learned facts are persisted in the correct location.
func (t *TaxonomyEngine) SetWorkspace(root string) {
	t.workspaceRoot = root
}

// nerdPath returns the correct path for a .nerd subdirectory.
// Uses workspaceRoot if set, otherwise returns relative path (legacy behavior).
func (t *TaxonomyEngine) nerdPath(subpath string) string {
	if t.workspaceRoot != "" {
		return filepath.Join(t.workspaceRoot, ".nerd", subpath)
	}
	return filepath.Join(".nerd", subpath)
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

	facts, err := t.store.LoadAllTaxonomyFacts()
	if err != nil {
		return err
	}

	if len(facts) > 0 {
		return nil
	}

	// Use defaults from Go struct
	for _, entry := range DefaultTaxonomyData {
		t.store.StoreVerbDef(entry.Verb, entry.Category, entry.ShardType, entry.Priority)
		for _, syn := range entry.Synonyms {
			t.store.StoreVerbSynonym(entry.Verb, syn)
		}
		for _, pat := range entry.Patterns {
			t.store.StoreVerbPattern(entry.Verb, pat)
		}
	}

	return nil
}

// GetVerbs returns all defined verbs with their metadata.
func (t *TaxonomyEngine) GetVerbs() ([]VerbEntry, error) {
	// Use GetFacts to access EDB directly
	facts, err := t.engine.GetFacts("verb_def")
	if err != nil {
		return nil, err
	}

	var verbs []VerbEntry
	for _, fact := range facts {
		if len(fact.Args) != 4 {
			continue
		}
		v := VerbEntry{
			Verb:      fact.Args[0].(string),
			Category:  fact.Args[1].(string),
			ShardType: fact.Args[2].(string),
			Priority:  toInt(fact.Args[3]),
		}
		if v.ShardType == "/none" {
			v.ShardType = ""
		}

		syns, _ := t.getSynonyms(v.Verb)
		v.Synonyms = syns

		patterns, _ := t.getPatterns(v.Verb)
		for _, p := range patterns {
			if re, err := regexp.Compile(p); err == nil {
				v.Patterns = append(v.Patterns, re)
			}
		}

		verbs = append(verbs, v)
	}

	sort.Slice(verbs, func(i, j int) bool {
		return verbs[i].Priority > verbs[j].Priority
	})

	return verbs, nil
}

func (t *TaxonomyEngine) getSynonyms(verb string) ([]string, error) {
	facts, err := t.engine.GetFacts("verb_synonym")
	if err != nil {
		return nil, err
	}

	targetVerb := verb
	if !strings.HasPrefix(targetVerb, "/") {
		targetVerb = "/" + targetVerb
	}

	var syns []string
	for _, fact := range facts {
		if len(fact.Args) == 2 {
			v := fact.Args[0].(string)
			if v == targetVerb || v == verb {
				syns = append(syns, fact.Args[1].(string))
			}
		}
	}
	return syns, nil
}

func (t *TaxonomyEngine) getPatterns(verb string) ([]string, error) {
	facts, err := t.engine.GetFacts("verb_pattern")
	if err != nil {
		return nil, err
	}

	targetVerb := verb
	if !strings.HasPrefix(targetVerb, "/") {
		targetVerb = "/" + targetVerb
	}

	var pats []string
	for _, fact := range facts {
		if len(fact.Args) == 2 {
			v := fact.Args[0].(string)
			if v == targetVerb || v == verb {
				pats = append(pats, fact.Args[1].(string))
			}
		}
	}
	return pats, nil
}

func toInt(val interface{}) int {
	if i, ok := val.(int); ok {
		return i
	}
	if i64, ok := val.(int64); ok {
		return int(i64)
	}
	if f, ok := val.(float64); ok {
		return int(f)
	}
	return 0
}

// ClassifyInput uses advanced Mangle inference to determine the best intent.
func (t *TaxonomyEngine) ClassifyInput(input string, candidates []VerbEntry) (bestVerb string, bestConf float64, err error) {
	// Note: We don't Clear() because we want to keep the static facts.
	// But we need to clear transient facts. Mangle Engine wrapper needs improvement for sessions.
	// For now, we add transient facts, query, and then maybe remove them?
	// Or just accept that memory grows (it's small for now).

	// Actually, if we don't Clear(), we accumulate context_token.
	// This is bad.
	// We MUST Clear() but then we lose the static facts.
	// So we MUST re-add static facts.
	// Since we have them in DefaultTaxonomyData, we can re-add them fast.
	// Or relying on the previous fix where we loaded them.

	t.engine.Clear()

	// Re-hydrate
	if t.store != nil {
		t.HydrateFromDB()
	} else {
		// Re-add defaults
		// Note: We must re-load the schema (declarations) AND the facts.
		t.engine.LoadSchemaString(InferenceLogicMG)
		for _, entry := range DefaultTaxonomyData {
			t.engine.AddFact("verb_def", entry.Verb, entry.Category, entry.ShardType, entry.Priority)
			for _, syn := range entry.Synonyms {
				t.engine.AddFact("verb_synonym", entry.Verb, syn)
			}
			for _, pat := range entry.Patterns {
				t.engine.AddFact("verb_pattern", entry.Verb, pat)
			}
		}
	}

	facts := []mangle.Fact{}
	tokens := strings.Fields(strings.ToLower(input))
	for _, token := range tokens {
		facts = append(facts, mangle.Fact{Predicate: "context_token", Args: []interface{}{token}})
	}
	// Inject full input string for exact/fuzzy matching against learned patterns
	facts = append(facts, mangle.Fact{Predicate: "user_input_string", Args: []interface{}{input}})

	if len(candidates) == 0 {
		return "", 0, nil
	}

	for _, cand := range candidates {
		baseScore := float64(cand.Priority)
		facts = append(facts, mangle.Fact{
			Predicate: "candidate_intent",
			Args:      []interface{}{cand.Verb, baseScore},
		})
	}

	if err := t.engine.AddFacts(facts); err != nil {
		return "", 0, fmt.Errorf("failed to add facts: %w", err)
	}

	// Use Query for inference
	result, err := t.engine.Query(context.Background(), "selected_verb(Verb)")
	if err != nil {
		return "", 0, fmt.Errorf("inference query failed: %w", err)
	}

	if len(result.Bindings) > 0 {
		verb := result.Bindings[0]["Verb"].(string)
		return verb, 1.0, nil
	}

	return "", 0, nil
}

// SetClient provides the taxonomy engine with an LLM client for the "Critic" loop.
func (t *TaxonomyEngine) SetClient(client LLMClient) {
	t.client = client
}

// GenerateSystemPromptSection generates the "VERB TAXONOMY" section.
func (t *TaxonomyEngine) GenerateSystemPromptSection() (string, error) {
	verbs, err := t.GetVerbs()
	if err != nil {
		return "", err
	}

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
			sb.WriteString(fmt.Sprintf("- %s: %s\n", v.Verb, strings.Join(v.Synonyms, ", ")))
		}
		sb.WriteString("\n")
	}

	// Inject usage of Learned Patterns
	results, _ := t.engine.Query(context.Background(), "learned_exemplar(P, V, T, C, _)")
	if len(results.Bindings) > 0 {
		sb.WriteString("### LEARNED USER PATTERNS (High Priority)\n")
		sb.WriteString("| User Phrase | Mapped Action | Constraint |\n")
		sb.WriteString("|-------------|---------------|------------|\n")
		for _, row := range results.Bindings {
			// Row is map[string]interface{}. Need to extract.
			p, _ := row["P"].(string)
			v, _ := row["V"].(string)
			t, _ := row["T"].(string)
			c, _ := row["C"].(string) // Constraint
			sb.WriteString(fmt.Sprintf("| %q | {verb: %s, target: %q} | %s |\n", p, v, t, c))
		}
		sb.WriteString("\n")
	}

	// Inject Canonical Examples (Phase 1 from user feedback)
	// Query intent_definition(Sentence, Verb, Target)
	// We need to check if intent_definition exists first to avoid query error if schema not loaded
	// But if we fail, we just ignore.
	canonResults, _ := t.engine.Query(context.Background(), "intent_definition(S, V, T)")
	if len(canonResults.Bindings) > 0 {
		sb.WriteString("### INTENT LIBRARY (Canonical Examples)\n")
		sb.WriteString("| Canonical Request | Mangle Action |\n")
		sb.WriteString("|-------------------|---------------|\n")
		for _, row := range canonResults.Bindings {
			s, _ := row["S"].(string)
			v, _ := row["V"].(string)
			t, _ := row["T"].(string)
			sb.WriteString(fmt.Sprintf("| %q | {verb: %s, target: %q} |\n", s, v, t))
		}
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// DefaultTaxonomyData defines the corpus in Go structures to avoid parsing fragility.
type TaxonomyDef struct {
	Verb      string
	Category  string
	ShardType string
	Priority  int
	Synonyms  []string
	Patterns  []string
}

var DefaultTaxonomyData = []TaxonomyDef{
	{
		Verb: "/review", Category: "/query", ShardType: "/reviewer", Priority: 100,
		Synonyms: []string{"review", "code review", "audit", "evaluate", "critique", "check code"},
		Patterns: []string{"(?i)review.*(file|code|changes)", "(?i)check.*code"},
	},
	{
		Verb: "/review_enhance", Category: "/query", ShardType: "/reviewer", Priority: 102,
		Synonyms: []string{"review enhance", "review and enhance", "creative review", "suggest improvements", "enhance"},
		Patterns: []string{"(?i)review.*enhance", "(?i)review.*suggest", "(?i)improve.*code", "(?i)creative.*feedback", "(?i)how.*make.*better"},
	},
	{
		Verb: "/security", Category: "/query", ShardType: "/reviewer", Priority: 105,
		Synonyms: []string{"security", "vulnerability", "injection", "xss", "scan"},
		Patterns: []string{"(?i)security.*scan", "(?i)check.*for.*vuln", "(?i)find.*vuln"},
	},
	{
		Verb: "/analyze", Category: "/query", ShardType: "/reviewer", Priority: 95,
		Synonyms: []string{"analyze", "complexity", "lint", "code smell"},
		Patterns: []string{"(?i)analy[sz]e.*code"},
	},
	{
		Verb: "/explain", Category: "/query", ShardType: "/none", Priority: 80,
		Synonyms: []string{"explain", "describe", "what is", "how does"},
		Patterns: []string{"(?i)explain.*this", "(?i)tell.*me.*about"},
	},
	{
		Verb: "/explore", Category: "/query", ShardType: "/researcher", Priority: 75,
		Synonyms: []string{"explore", "browse", "structure", "list files"},
		Patterns: []string{"(?i)show.*structure"},
	},
	{
		Verb: "/search", Category: "/query", ShardType: "/researcher", Priority: 85,
		Synonyms: []string{"search", "find", "grep", "occurrences"},
		Patterns: []string{"(?i)search.*for", "(?i)find.*all"},
	},
	{
		Verb: "/fix", Category: "/mutation", ShardType: "/coder", Priority: 90,
		Synonyms: []string{"fix", "repair", "patch", "resolve", "bug fix"},
		Patterns: []string{"(?i)fix.*bug", "(?i)resolve.*issue"},
	},
	{
		Verb: "/refactor", Category: "/mutation", ShardType: "/coder", Priority: 88,
		Synonyms: []string{"refactor", "clean up", "improve", "optimize"},
		Patterns: []string{"(?i)refactor", "(?i)clean.*up"},
	},
	{
		Verb: "/create", Category: "/mutation", ShardType: "/coder", Priority: 85,
		Synonyms: []string{"create", "new", "add", "implement", "generate"},
		Patterns: []string{"(?i)create.*new", "(?i)add.*new", "(?i)implement"},
	},
	{
		Verb: "/write", Category: "/mutation", ShardType: "/coder", Priority: 70,
		Synonyms: []string{"write", "save", "export"},
		Patterns: []string{"(?i)write.*to"},
	},
	{
		Verb: "/delete", Category: "/mutation", ShardType: "/coder", Priority: 85,
		Synonyms: []string{"delete", "remove", "drop"},
		Patterns: []string{"(?i)delete", "(?i)remove"},
	},
	{
		Verb: "/debug", Category: "/query", ShardType: "/coder", Priority: 92,
		Synonyms: []string{"debug", "troubleshoot", "diagnose", "root cause"},
		Patterns: []string{"(?i)debug", "(?i)why.*fail"},
	},
	{
		Verb: "/test", Category: "/mutation", ShardType: "/tester", Priority: 88,
		Synonyms: []string{"test", "unit test", "run tests", "coverage"},
		Patterns: []string{"(?i)run.*test", "(?i)test.*coverage"},
	},
	{
		Verb: "/research", Category: "/query", ShardType: "/researcher", Priority: 75,
		Synonyms: []string{"research", "learn", "docs", "documentation"},
		Patterns: []string{"(?i)research", "(?i)learn.*about"},
	},
	{
		Verb: "/init", Category: "/mutation", ShardType: "/researcher", Priority: 70,
		Synonyms: []string{"init", "setup", "bootstrap"},
		Patterns: []string{"(?i)^init"},
	},
	{
		Verb: "/configure", Category: "/instruction", ShardType: "/none", Priority: 65,
		Synonyms: []string{"configure", "config", "settings"},
		Patterns: []string{"(?i)configure"},
	},
	{
		Verb: "/campaign", Category: "/mutation", ShardType: "/coder", Priority: 95,
		Synonyms: []string{"campaign", "epic", "feature"},
		Patterns: []string{"(?i)start.*campaign"},
	},
	{
		Verb: "/generate_tool", Category: "/mutation", ShardType: "/tool_generator", Priority: 95,
		Synonyms: []string{"generate tool", "create tool", "need a tool", "build tool", "make tool", "implement tool"},
		Patterns: []string{"(?i)create.*tool", "(?i)build.*tool", "(?i)make.*tool", "(?i)need.*tool", "(?i)tool.*for"},
	},
	// --- NEW DIRECT-RESPONSE VERBS ---
	{
		Verb: "/stats", Category: "/query", ShardType: "/none", Priority: 85,
		Synonyms: []string{"stats", "statistics", "count", "breakdown", "how many", "file types", "totals", "metrics"},
		Patterns: []string{"(?i)how many", "(?i)breakdown", "(?i)file types", "(?i)count.*files", "(?i)statistics", "(?i)codebase.*size"},
	},
	{
		Verb: "/help", Category: "/query", ShardType: "/none", Priority: 60,
		Synonyms: []string{"help", "capabilities", "can you", "what can", "features", "commands", "abilities"},
		Patterns: []string{"(?i)what.*can.*you", "(?i)help.*me", "(?i)your.*capabilities", "(?i)available.*commands"},
	},
	{
		Verb: "/greet", Category: "/query", ShardType: "/none", Priority: 50,
		Synonyms: []string{"hello", "hi", "hey", "greetings", "good morning", "good evening"},
		Patterns: []string{"(?i)^hello", "(?i)^hi$", "(?i)^hey", "(?i)good morning", "(?i)good evening"},
	},
	{
		Verb: "/knowledge", Category: "/query", ShardType: "/none", Priority: 70,
		Synonyms: []string{"knowledge", "memory", "remember", "learned", "preferences", "facts"},
		Patterns: []string{"(?i)what.*remember", "(?i)what.*know", "(?i)your.*memory", "(?i)learned.*from"},
	},
	{
		Verb: "/dream", Category: "/query", ShardType: "/none", Priority: 85,
		Synonyms: []string{"dream", "what if", "imagine", "hypothetically", "walk me through", "think about", "how would you"},
		Patterns: []string{"(?i)what.*if.*told", "(?i)imagine.*had.*to", "(?i)walk.*through", "(?i)think.*about.*how", "(?i)hypothetically", "(?i)how.*would.*you"},
	},
	{
		Verb: "/shadow", Category: "/query", ShardType: "/none", Priority: 75,
		Synonyms: []string{"shadow", "simulate", "dry run", "preview"},
		Patterns: []string{"(?i)simulate", "(?i)dry.*run", "(?i)preview.*change", "(?i)would.*happen"},
	},
	{
		Verb: "/git", Category: "/mutation", ShardType: "/coder", Priority: 90,
		Synonyms: []string{"git", "commit", "push", "pull", "branch", "merge", "status", "github"},
		Patterns: []string{"(?i)git.*status", "(?i)commit.*changes", "(?i)push.*to", "(?i)create.*branch", "(?i)push.*github", "(?i)push.*remote"},
	},
	{
		Verb: "/read", Category: "/query", ShardType: "/none", Priority: 82,
		Synonyms: []string{"read", "show", "display", "open", "view", "cat"},
		Patterns: []string{"(?i)read.*file", "(?i)show.*contents", "(?i)display.*file", "(?i)open.*file"},
	},
}

// InferenceLogicMG contains the advanced patterns for context-aware classification.
const InferenceLogicMG = `
# Inference Logic for Intent Refinement
# This module takes raw intent candidates (from regex/LLM) and refines them
# using contextual logic and safety constraints.

Decl candidate_intent(Verb, RawScore).
Decl context_token(Token).
Decl user_input_string(Input).

# Import learned patterns
# Decl learned_exemplar imported from schema/learning.mg

Decl boost(Verb, Amount).
Decl penalty(Verb, Amount).

# EDB Declarations for data loaded from Go
Decl verb_def(Verb, Category, Shard, Priority).
Decl verb_synonym(Verb, Synonym).
Decl verb_pattern(Verb, Regex).

# Intermediate score generation
Decl potential_score(Verb, Score).

# 1. Base Score
# 1. Base Score
potential_score(Verb, Score) :- candidate_intent(Verb, Score).

# Learned Pattern Override (Highest Priority)
# If the input matches a learned pattern, give it a massive boost.
potential_score(Verb, 100.0) :-
    user_input_string(Input),
    learned_exemplar(Pattern, Verb, _, _, _),
    # Simple case-insensitive exact match for now. 
    # Mangle doesn't have robust fuzzy matching built-in yet, relying on LLM for fuzzy part.
    # But this handles exact recurrence.
    Input = Pattern.

# 2. Boosted Scores (Rule-based)
potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /security,
    context_token("security"),
    NewScore = fn:plus(Base, 0.3).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /security,
    context_token("vulnerability"),
    NewScore = fn:plus(Base, 0.3).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /test,
    context_token("coverage"),
    NewScore = fn:plus(Base, 0.2).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /debug,
    context_token("panic"),
    NewScore = fn:plus(Base, 0.15).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /debug,
    context_token("stacktrace"),
    NewScore = fn:plus(Base, 0.15).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /test,
    context_token("verify"),
    NewScore = fn:plus(Base, 0.2).

# 3. Relational Max Score Selection
# Define a predicate that finds scores that are NOT max
Decl has_greater_score(Score).
has_greater_score(S) :-
    potential_score(_, S),
    potential_score(_, Other),
    Other > S.

# Define max score as one that has no greater score
Decl best_score(MaxScore).
best_score(S) :-
    potential_score(_, S),
    !has_greater_score(S).

# Select verb matching the max score
Decl selected_verb(Verb).
selected_verb(Verb) :-
    potential_score(Verb, S),
    best_score(Max),
    S = Max.

# =============================================================================
# SEMANTIC MATCHING INFERENCE
# =============================================================================
# These rules use semantic_match facts to influence verb selection.
# They work alongside existing token-based boosting.

# EDB declarations for semantic matching (facts asserted by SemanticClassifier)
Decl semantic_match(UserInput, CanonicalSentence, Verb, Target, Rank, Similarity).
Decl verb_composition(Verb1, Verb2, ComposedAction, Priority).

# Derived predicates for semantic matching
Decl semantic_suggested_verb(Verb, Similarity).
Decl compound_suggestion(Verb1, Verb2).

# Derive suggested verbs from semantic matches (top 3 only, similarity >= 60)
semantic_suggested_verb(Verb, Similarity) :-
    semantic_match(_, _, Verb, _, Rank, Similarity),
    Rank <= 3,
    Similarity >= 60.

# HIGH-CONFIDENCE SEMANTIC OVERRIDE
# If rank 1 match has similarity >= 85, override to max score
potential_score(Verb, 100.0) :-
    semantic_match(_, _, Verb, _, 1, Similarity),
    Similarity >= 85.

# MEDIUM-CONFIDENCE SEMANTIC BOOST
# Rank 1-3 with similarity 70-84 get +30 boost
potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    semantic_match(_, _, Verb, _, Rank, Similarity),
    Rank <= 3,
    Similarity >= 70,
    Similarity < 85,
    NewScore = fn:plus(Base, 30.0).

# LOW-CONFIDENCE SEMANTIC BOOST
# Rank 1-5 with similarity 60-69 get +15 boost
potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    semantic_match(_, _, Verb, _, Rank, Similarity),
    Rank <= 5,
    Similarity >= 60,
    Similarity < 70,
    NewScore = fn:plus(Base, 15.0).

# VERB COMPOSITION FROM MULTIPLE MATCHES
# If two different verbs both have high similarity, suggest composition
compound_suggestion(V1, V2) :-
    semantic_suggested_verb(V1, S1),
    semantic_suggested_verb(V2, S2),
    V1 != V2,
    S1 >= 65,
    S2 >= 65,
    verb_composition(V1, V2, _, Priority),
    Priority >= 80.

# LEARNED PATTERN PRIORITY
# Semantic matches from learned patterns (detected by constraint presence)
# get additional boost - these represent user-specific preferences
potential_score(Verb, NewScore) :-
    semantic_match(_, Sentence, Verb, _, 1, Similarity),
    Similarity >= 70,
    learned_exemplar(Sentence, Verb, _, _, _),
    candidate_intent(Verb, Base),
    NewScore = fn:plus(Base, 40.0).
`
