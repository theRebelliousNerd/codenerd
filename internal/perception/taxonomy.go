package perception

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
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
	learnedPath   string // Absolute path of loaded .nerd/mangle/learned.mg (if any)
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
	t := &TaxonomyEngine{engine: eng}

	// Load Intent Definition Schemas (Modular) - must be loaded in order
	intentFiles := []string{
		"schema/intent_core.mg",         // Core Decl statements
		"schema/intent_qualifiers.mg",   // Interrogatives, modals, copular, negation
		"schema/intent_queries.mg",      // /query category intents
		"schema/intent_mutations.mg",    // /mutation category intents
		"schema/intent_instructions.mg", // /instruction category intents
		"schema/intent_campaign.mg",     // Campaign and multi-step intents
		"schema/intent_system.mg",       // System-level inference rules
	}

	for _, file := range intentFiles {
		content, err := core.GetDefaultContent(file)
		if err == nil {
			if err := eng.LoadSchemaString(content); err != nil {
				return nil, fmt.Errorf("failed to load %s: %w", file, err)
			}
		} else {
			// Fallback to disk (dev mode) or warn
			fmt.Printf("WARNING: %s not found in embedded defaults: %v\n", file, err)
		}
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

	// Load learned rules if available (workspace-aware; SetWorkspace() will re-attempt).
	t.tryLoadLearned()
	return t, nil
}

// SetStore attaches a persistence store to the taxonomy engine.
func (t *TaxonomyEngine) SetStore(s *TaxonomyStore) {
	t.store = s
}

// SetWorkspace sets the explicit workspace root path for .nerd directory resolution.
// This MUST be called to ensure learned facts are persisted in the correct location.
func (t *TaxonomyEngine) SetWorkspace(root string) {
	t.workspaceRoot = root
	t.tryLoadLearned()
}

// HasWorkspace returns true if an explicit workspace root has been set.
func (t *TaxonomyEngine) HasWorkspace() bool {
	return t.workspaceRoot != ""
}

// nerdPath returns the correct path for a .nerd subdirectory.
// Uses workspaceRoot if set, otherwise returns relative path (legacy behavior).
func (t *TaxonomyEngine) nerdPath(subpath string) string {
	if t.workspaceRoot != "" {
		return filepath.Join(t.workspaceRoot, ".nerd", subpath)
	}
	return filepath.Join(".nerd", subpath)
}

func (t *TaxonomyEngine) tryLoadLearned() {
	// Taxonomy learns in its own schema file; do not load the kernel's learned.mg
	// (which contains unrelated system rules and can break schema analysis here).
	learnedPath := filepath.Join(t.nerdPath("mangle"), "learned_taxonomy.mg")
	if abs, err := filepath.Abs(learnedPath); err == nil {
		learnedPath = abs
	}
	if learnedPath == t.learnedPath {
		return
	}
	if _, err := os.Stat(learnedPath); err != nil {
		return
	}
	if err := t.engine.LoadSchema(learnedPath); err != nil {
		fmt.Printf("WARNING: Failed to load learned taxonomy from %s: %v\n", learnedPath, err)
		return
	}
	t.learnedPath = learnedPath
}

// HydrateFromDB loads all taxonomy facts from the database into the engine.
func (t *TaxonomyEngine) HydrateFromDB() error {
	if t.store == nil {
		return fmt.Errorf("no store configured")
	}
	if err := t.store.HydrateEngine(t.engine); err != nil {
		return err
	}

	// Keep the package-level VerbCorpus in sync after hydration so parsing reflects
	// any newly learned verbs/synonyms/patterns persisted in SQLite.
	if verbs, err := t.GetVerbs(); err == nil && len(verbs) > 0 {
		VerbCorpus = verbs
	}

	return nil
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
	// We MUST Reset() to clear schema fragments too, otherwise reloading schemas creates duplicate Decls.
	t.engine.Reset()

	// 1. Reload Intent Schemas (Modular) - ALWAYS REQUIRED for inference
	// These contain critical facts like interrogative_type, modal_type, etc.
	intentFiles := core.DefaultIntentSchemaFiles()
	intentFiles = append(intentFiles, "schema/learning.mg") // CRITICAL: Required by InferenceLogicMG
	for _, file := range intentFiles {
		content, err := core.GetDefaultContent(file)
		if err == nil {
			if err := t.engine.LoadSchemaString(content); err != nil {
				logging.PerceptionDebug("Failed to reload schema %s: %v", file, err)
			}
		} else {
			logging.PerceptionDebug("Failed to get content for %s: %v", file, err)
		}
	}

	// 2. Reload Inference Logic - ALWAYS REQUIRED
	if err := t.engine.LoadSchemaString(InferenceLogicMG); err != nil {
		logging.PerceptionDebug("Failed to reload InferenceLogicMG: %v", err)
	}

	// 3. Re-hydrate Verb Taxonomy (EDB facts)
	if t.store != nil {
		t.HydrateFromDB()
	} else {
		// Re-add defaults
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

	// Use GetFacts to get all derived potential scores and aggregate in Go
	// This avoids "predicate potential_score has no modes declared" errors.
	factsFromStore, err := t.engine.GetFacts("potential_score")
	if err != nil {
		return "", 0, fmt.Errorf("failed to get potential_score facts: %w", err)
	}

	var bestScore float64 = -1.0

	for _, fact := range factsFromStore {
		if len(fact.Args) != 2 {
			continue
		}
		verb, ok := fact.Args[0].(string)
		if !ok {
			continue
		}

		// Handle score which might be int64 or float64 depending on Mangle internal representation
		var score float64
		switch s := fact.Args[1].(type) {
		case float64:
			score = s
		case int64:
			score = float64(s)
		case int:
			score = float64(s)
		default:
			continue
		}

		if score > bestScore {
			bestScore = score
			bestVerb = verb
		}
	}

	if bestVerb != "" {
		return bestVerb, 1.0, nil
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
		Verb: "/assault", Category: "/mutation", ShardType: "/none", Priority: 99,
		Synonyms: []string{"assault", "assault campaign", "adversarial assault", "adversarial campaign", "gauntlet", "soak test", "stress test", "torture test", "adversarial sweep"},
		Patterns: []string{
			"(?i)\\bassault\\b",
			"(?i)\\badversarial\\s+(assault|campaign|sweep)\\b",
			"(?i)\\b(soak|stress|torture)\\s+test\\b",
			"(?i)\\b(run|launch|start)\\s+(an\\s+)?assault\\b",
			"(?i)\\bgauntlet\\b",
		},
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
# Convert float score to int for calculation if needed, but here we just pass it.
# candidate_intent RawScore is already scaled to int64 by engine.go if it was > 1.0.
potential_score(Verb, Score) :- candidate_intent(Verb, Score).

# Learned Pattern Override (Highest Priority)
# If the input matches a learned pattern, give it a massive boost.
potential_score(Verb, 100) :-
    user_input_string(Input),
    learned_exemplar(Pattern, Verb, _, _, _),
    Input = Pattern.

# 2. Boosted Scores (Rule-based)
# Use integer arithmetic for scores (0-100 scale).
potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /security,
    context_token("security"),
    NewScore = fn:plus(Base, 30).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /security,
    context_token("vulnerability"),
    NewScore = fn:plus(Base, 30).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /test,
    context_token("coverage"),
    NewScore = fn:plus(Base, 20).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /debug,
    context_token("panic"),
    NewScore = fn:plus(Base, 15).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /debug,
    context_token("stacktrace"),
    NewScore = fn:plus(Base, 15).

potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    Verb = /test,
    context_token("verify"),
    NewScore = fn:plus(Base, 20).

# Note: Aggregation (finding max score) is now handled in Go code to avoid
# "no modes declared" errors caused by complex negation in Mangle.

# =============================================================================
# SEMANTIC MATCHING INFERENCE
# =============================================================================
# These rules use semantic_match facts to influence verb selection.

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
potential_score(Verb, 100) :-
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
    NewScore = fn:plus(Base, 30).

# LOW-CONFIDENCE SEMANTIC BOOST
# Rank 1-5 with similarity 60-69 get +15 boost
potential_score(Verb, NewScore) :-
    candidate_intent(Verb, Base),
    semantic_match(_, _, Verb, _, Rank, Similarity),
    Rank <= 5,
    Similarity >= 60,
    Similarity < 70,
    NewScore = fn:plus(Base, 15).

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
    NewScore = fn:plus(Base, 40).

# =============================================================================
# INTENT QUALIFIER INFERENCE
# =============================================================================
# These rules use the intent qualifiers (interrogatives, modals, copular states,
# negation) to enhance verb selection beyond simple pattern matching.

# --- Derived predicates for qualifier detection ---
Decl detected_interrogative(Word, SemanticType, DefaultVerb, Priority).
Decl detected_modal(Word, ModalMeaning, Transformation, Priority).
Decl detected_state_adj(Adjective, ImpliedVerb, StateCategory, Priority).
Decl detected_negation(Word, NegationType, Priority).
Decl detected_existence(Pattern, DefaultVerb, Priority).
Decl has_negation(Flag).
Decl has_polite_modal(Flag).
Decl has_hypothetical_modal(Flag).

# --- Detect interrogatives from context tokens ---
# Single-word tokens are often atomized if they are identifiers (like 'where', 'is')
detected_interrogative(Word, SemanticType, DefaultVerb, Priority) :-
    context_token(Word),
    interrogative_type(Word, SemanticType, DefaultVerb, Priority).

# Two-word interrogatives (check for both tokens present)
detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/what),
    context_token(/is),
    interrogative_type("what is", SemanticType, DefaultVerb, Priority),
    Phrase = "what is".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/what),
    context_token(/if),
    interrogative_type("what if", SemanticType, DefaultVerb, Priority),
    Phrase = "what if".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/why),
    context_token(/is),
    interrogative_type("why is", SemanticType, DefaultVerb, Priority),
    Phrase = "why is".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/why),
    context_token(/does),
    interrogative_type("why does", SemanticType, DefaultVerb, Priority),
    Phrase = "why does".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/how),
    context_token(/do),
    context_token(/i),
    interrogative_type("how do i", SemanticType, DefaultVerb, Priority),
    Phrase = "how do i".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/how),
    context_token(/can),
    context_token(/i),
    interrogative_type("how can i", SemanticType, DefaultVerb, Priority),
    Phrase = "how can i".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/where),
    context_token(/is),
    interrogative_type("where is", SemanticType, DefaultVerb, Priority),
    Phrase = "where is".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/who),
    context_token(/wrote),
    interrogative_type("who wrote", SemanticType, DefaultVerb, Priority),
    Phrase = "who wrote".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/which),
    context_token(/file),
    interrogative_type("which file", SemanticType, DefaultVerb, Priority),
    Phrase = "which file".

detected_interrogative(Phrase, SemanticType, DefaultVerb, Priority) :-
    context_token(/which),
    context_token(/files),
    interrogative_type("which files", SemanticType, DefaultVerb, Priority),
    Phrase = "which files".

# --- Detect modals from context tokens ---
detected_modal(Word, ModalMeaning, Transformation, Priority) :-
    context_token(Word),
    modal_type(Word, ModalMeaning, Transformation, Priority).

# Two-word modals
detected_modal(Phrase, ModalMeaning, Transformation, Priority) :-
    context_token(/can),
    context_token(/you),
    modal_type("can you", ModalMeaning, Transformation, Priority),
    Phrase = "can you".

detected_modal(Phrase, ModalMeaning, Transformation, Priority) :-
    context_token(/could),
    context_token(/you),
    modal_type("could you", ModalMeaning, Transformation, Priority),
    Phrase = "could you".

detected_modal(Phrase, ModalMeaning, Transformation, Priority) :-
    context_token(/would),
    context_token(/you),
    modal_type("would you", ModalMeaning, Transformation, Priority),
    Phrase = "would you".

detected_modal(Phrase, ModalMeaning, Transformation, Priority) :-
    context_token(/help),
    context_token(/me),
    modal_type("help me", ModalMeaning, Transformation, Priority),
    Phrase = "help me".

detected_modal(Phrase, ModalMeaning, Transformation, Priority) :-
    context_token(/what),
    context_token(/if),
    modal_type("what if", ModalMeaning, Transformation, Priority),
    Phrase = "what if".

# --- Detect state adjectives from context tokens ---
detected_state_adj(Adjective, ImpliedVerb, StateCategory, Priority) :-
    context_token(Adjective),
    state_adjective(Adjective, ImpliedVerb, StateCategory, Priority).

# --- Detect negation from context tokens ---
detected_negation(Word, NegationType, Priority) :-
    context_token(Word),
    negation_marker(Word, NegationType, Priority).

# Flag if any negation is present (use /true sentinel for boolean)
has_negation(/true) :-
    detected_negation(_, _, _).

# Flag if polite modal is present (use /true sentinel for boolean)
has_polite_modal(/true) :-
    detected_modal(_, /polite_request, _, _).

# Flag if hypothetical modal is present (use /true sentinel for boolean)
has_hypothetical_modal(/true) :-
    detected_modal(_, /hypothetical, _, _).

# --- Detect existence patterns ---
detected_existence(Pattern, DefaultVerb, Priority) :-
    context_token(/is),
    context_token(/there),
    existence_pattern("is there", _, DefaultVerb, Priority),
    Pattern = "is there".

detected_existence(Pattern, DefaultVerb, Priority) :-
    context_token(/are),
    context_token(/there),
    existence_pattern("are there", _, DefaultVerb, Priority),
    Pattern = "are there".

detected_existence(Pattern, DefaultVerb, Priority) :-
    context_token(/do),
    context_token(/we),
    context_token(/have),
    existence_pattern("do we have", _, DefaultVerb, Priority),
    Pattern = "do we have".

# =============================================================================
# QUALIFIER-ENHANCED VERB SCORING
# =============================================================================

# --- NEGATION BLOCKING (Highest Priority) ---
# If negation + verb detected, DO NOT select that verb
# Instead, convert to an instruction intent
Decl negated_verb(Verb).
negated_verb(Verb) :-
    has_negation(/true),
    context_token(VerbWord),
    verb_synonym(Verb, VerbWord).

# Negated verbs get negative score (effectively blocked)
potential_score(Verb, -100) :-
    negated_verb(Verb).

# When negation present, boost /instruction or /explain instead
potential_score(/explain, 85) :-
    has_negation(/true),
    negated_verb(_).

# --- MODAL STRIPPING (High Priority) ---
# "Can you review this?" -> strip modal, boost /review
# This fires when polite modal + verb synonym detected
potential_score(Verb, 95) :-
    has_polite_modal(/true),
    context_token(VerbWord),
    verb_synonym(Verb, VerbWord),
    !negated_verb(Verb).

# --- HYPOTHETICAL MODE (High Priority) ---
# "What if I deleted this?" -> boost /dream
potential_score(/dream, 92) :-
    has_hypothetical_modal(/true).

# --- COPULAR + STATE ADJECTIVE (High Priority) ---
# "Is this code secure?" -> /security
# Requires copular verb + state adjective in context
Decl copular_state_intent(ImpliedVerb, Priority).
copular_state_intent(ImpliedVerb, Priority) :-
    context_token(Copular),
    copular_verb(Copular, _, _),
    detected_state_adj(_, ImpliedVerb, _, Priority).

# Helper predicates for safe negation (wildcards in negated atoms cause safety violations)
Decl has_copular_state_intent(Flag).
has_copular_state_intent(/true) :- copular_state_intent(_, _).

Decl has_candidate_intent(Flag).
has_candidate_intent(/true) :- candidate_intent(_, _).

potential_score(Verb, Score) :-
    copular_state_intent(Verb, BasePriority),
    !has_negation(/true),
    Score = fn:plus(BasePriority, 5).

# --- INTERROGATIVE + STATE COMBINATION (Very High Priority) ---
# "Why is this failing?" -> causation + error_state -> /debug
Decl interrogative_state_combo(CombinedVerb, Priority).
interrogative_state_combo(CombinedVerb, Priority) :-
    detected_interrogative(_, InterrogType, _, _),
    detected_state_adj(_, _, StateCategory, _),
    interrogative_state_signal(InterrogType, StateCategory, CombinedVerb, Priority).

Decl has_interrogative_state_combo(Flag).
has_interrogative_state_combo(/true) :- interrogative_state_combo(_, _).

potential_score(Verb, Score) :-
    interrogative_state_combo(Verb, Priority),
    !has_negation(/true),
    Score = fn:plus(Priority, 2).

# --- PURE INTERROGATIVE FALLBACK (Medium Priority) ---
# If interrogative detected but no verb match, use interrogative's default verb
Decl pure_interrogative_intent(DefaultVerb, Priority).
pure_interrogative_intent(DefaultVerb, Priority) :-
    detected_interrogative(_, _, DefaultVerb, Priority),
    !has_polite_modal(/true),
    !has_copular_state_intent(/true),
    !has_interrogative_state_combo(/true).

potential_score(Verb, Score) :-
    pure_interrogative_intent(Verb, Priority),
    !has_candidate_intent(/true),
    !has_negation(/true),
    Score = Priority.

# --- EXISTENCE QUERIES (Medium Priority) ---
# "Is there a config file?" -> /search
potential_score(DefaultVerb, Score) :-
    detected_existence(_, DefaultVerb, Priority),
    !has_negation(/true),
    Score = Priority.

# =============================================================================
# INTENT METADATA DERIVATION
# =============================================================================
# Derive additional metadata about the intent for routing decisions.
# Note: Mangle requires at least one argument per predicate; use /true sentinel for booleans.

Decl intent_is_question(Flag).
Decl intent_is_hypothetical(Flag).
Decl intent_is_negated(Flag).
Decl intent_semantic_type(Type).
Decl intent_state_category(Category).

intent_is_question(/true) :-
    detected_interrogative(_, _, _, _).

intent_is_hypothetical(/true) :-
    has_hypothetical_modal(/true).

intent_is_negated(/true) :-
    has_negation(/true).

intent_semantic_type(Type) :-
    detected_interrogative(_, Type, _, _).

intent_state_category(Category) :-
    detected_state_adj(_, _, Category, _).
`
