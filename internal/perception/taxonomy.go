package perception

import (
	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/mangle"
	"codenerd/internal/types"
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
		"schemas_intent.mg",             // Core intent declarations (moved from schema/intent_core.mg)
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
	learningContent, err := core.GetDefaultContent("schemas_learning.mg")
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

	// Load Qualifier Logic (Must be loaded BEFORE inference logic)
	qualifierLogic, err := core.GetDefaultContent("policy/taxonomy_qualifiers.mg")
	if err != nil {
		return nil, fmt.Errorf("failed to get qualifier logic content: %w", err)
	}
	if err := eng.LoadSchemaString(qualifierLogic); err != nil {
		return nil, fmt.Errorf("failed to load qualifier logic: %w", err)
	}

	// Load declarations and logic (Must be loaded AFTER learning.mg)
	inferenceContent, err := core.GetDefaultContent("policy/taxonomy_inference.mg")
	if err != nil {
		return nil, fmt.Errorf("failed to get inference logic content: %w", err)
	}
	if err := eng.LoadSchemaString(inferenceContent); err != nil {
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
		SetVerbCorpus(verbs)
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
			Verb:      types.ExtractString(fact.Args[0]),
			Category:  types.ExtractString(fact.Args[1]),
			ShardType: types.ExtractString(fact.Args[2]),
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
			v := types.ExtractString(fact.Args[0])
			if v == targetVerb || v == verb {
				syn := types.ExtractString(fact.Args[1])
				// Defense-in-depth: strip leading "/" from synonyms that were
				// incorrectly stored as NameType atoms instead of strings.
				// This handles legacy data from when verb_synonym was declared
				// bound [/name, /name] instead of bound [/name, /string].
				syn = strings.TrimPrefix(syn, "/")
				syns = append(syns, syn)
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
			v := types.ExtractString(fact.Args[0])
			if v == targetVerb || v == verb {
				pats = append(pats, types.ExtractString(fact.Args[1]))
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
	// Must include schemas_intent.mg FIRST as it declares context_token, candidate_intent, etc.
	intentFiles := []string{"schemas_intent.mg"}
	intentFiles = append(intentFiles, core.DefaultIntentSchemaFiles()...)
	intentFiles = append(intentFiles, "schemas_learning.mg") // CRITICAL: Required by InferenceLogicMG
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
	inferenceContent, err := core.GetDefaultContent("policy/taxonomy_inference.mg")
	if err != nil {
		logging.PerceptionDebug("Failed to get inference logic content: %v", err)
	} else if err := t.engine.LoadSchemaString(inferenceContent); err != nil {
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
	rawTokens := strings.Fields(strings.ToLower(input))
	for _, token := range rawTokens {
		// Keep tokenization simple and stable: trim common punctuation and add a naive singular form.
		token = strings.Trim(token, ".,!?;:\"'()[]{}<>")
		if token == "" {
			continue
		}
		facts = append(facts, mangle.Fact{Predicate: "context_token", Args: []interface{}{token}})
		if strings.HasSuffix(token, "s") && len(token) > 3 {
			facts = append(facts, mangle.Fact{Predicate: "context_token", Args: []interface{}{strings.TrimSuffix(token, "s")}})
		}
	}
	// Inject full input string for exact/fuzzy matching against learned patterns
	facts = append(facts, mangle.Fact{Predicate: "user_input_string", Args: []interface{}{input}})

	for _, cand := range candidates {
		facts = append(facts, mangle.Fact{
			Predicate: "candidate_intent",
			Args:      []interface{}{cand.Verb, int64(cand.Priority)},
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
	if t.engine != nil {
		results, err := t.engine.Query(context.Background(), "learned_exemplar(P, V, T, C, _)")
		if err == nil && results != nil && len(results.Bindings) > 0 {
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
	}

	// Inject Canonical Examples (Phase 1 from user feedback)
	// Query intent_definition(Sentence, Verb, Target)
	// We need to check if intent_definition exists first to avoid query error if schema not loaded
	// But if we fail, we just ignore.
	if t.engine != nil {
		canonResults, err := t.engine.Query(context.Background(), "intent_definition(S, V, T)")
		if err == nil && canonResults != nil && len(canonResults.Bindings) > 0 {
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
	{
		Verb: "/migrate", Category: "/mutation", ShardType: "/coder", Priority: 85,
		Synonyms: []string{"migrate", "migration", "upgrade", "port", "convert", "transition"},
		Patterns: []string{"(?i)migrate", "(?i)upgrade.*to", "(?i)port.*to", "(?i)convert.*from.*to"},
	},
	{
		Verb: "/optimize", Category: "/mutation", ShardType: "/coder", Priority: 86,
		Synonyms: []string{"optimize", "optimise", "speed up", "make faster", "improve performance"},
		Patterns: []string{"(?i)optimize", "(?i)optimise", "(?i)speed.*up", "(?i)make.*faster", "(?i)improve.*perf"},
	},
	{
		Verb: "/document", Category: "/mutation", ShardType: "/coder", Priority: 72,
		Synonyms: []string{"document", "write docs", "add comments", "add documentation", "write readme"},
		Patterns: []string{"(?i)document", "(?i)write.*doc", "(?i)add.*comment", "(?i)add.*doc", "(?i)write.*readme"},
	},
	{
		Verb: "/benchmark", Category: "/query", ShardType: "/tester", Priority: 80,
		Synonyms: []string{"benchmark", "bench", "perf test", "performance test", "load test"},
		Patterns: []string{"(?i)benchmark", "(?i)bench\\b", "(?i)perf.*test", "(?i)load.*test"},
	},
	{
		Verb: "/profile", Category: "/query", ShardType: "/tester", Priority: 78,
		Synonyms: []string{"profile", "profiling", "cpu profile", "memory profile", "pprof", "trace"},
		Patterns: []string{"(?i)profile", "(?i)profil", "(?i)pprof", "(?i)cpu.*usage", "(?i)memory.*usage"},
	},
	{
		Verb: "/audit", Category: "/query", ShardType: "/reviewer", Priority: 90,
		Synonyms: []string{"audit", "compliance", "security audit", "code audit"},
		Patterns: []string{"(?i)audit", "(?i)compliance", "(?i)security.*check", "(?i)code.*audit"},
	},
	{
		Verb: "/scaffold", Category: "/mutation", ShardType: "/coder", Priority: 75,
		Synonyms: []string{"scaffold", "scaffolding", "boilerplate", "bootstrap", "generate project", "create project"},
		Patterns: []string{"(?i)scaffold", "(?i)boilerplate", "(?i)bootstrap.*project", "(?i)generate.*project", "(?i)create.*project"},
	},
	{
		Verb: "/lint", Category: "/query", ShardType: "/reviewer", Priority: 83,
		Synonyms: []string{"lint", "linter", "static analysis", "check style", "code quality"},
		Patterns: []string{"(?i)\\blint\\b", "(?i)linter", "(?i)static.*analysis", "(?i)check.*style", "(?i)code.*quality"},
	},
	{
		Verb: "/format", Category: "/mutation", ShardType: "/coder", Priority: 70,
		Synonyms: []string{"format", "fmt", "gofmt", "prettier", "auto-format", "fix formatting"},
		Patterns: []string{"(?i)\\bformat\\b", "(?i)\\bfmt\\b", "(?i)gofmt", "(?i)prettier", "(?i)fix.*format"},
	},
}
