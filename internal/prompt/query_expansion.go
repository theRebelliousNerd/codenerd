package prompt

import (
	"strings"
	"unicode"
)

// stopWords represents common English words that should be excluded from semantic queries
// to improve retrieval quality by focusing on domain-specific terminology.
var stopWords = map[string]struct{}{
	"a": {}, "an": {}, "and": {}, "are": {}, "as": {}, "at": {}, "be": {}, "but": {}, "by": {},
	"for": {}, "if": {}, "in": {}, "into": {}, "is": {}, "it": {},
	"no": {}, "not": {}, "of": {}, "on": {}, "or": {}, "such": {},
	"that": {}, "the": {}, "their": {}, "then": {}, "there": {}, "these": {},
	"they": {}, "this": {}, "to": {}, "was": {}, "will": {}, "with": {},
}

// verbSynonyms expands intent verbs into broader semantic concepts.
var verbSynonyms = map[string][]string{
	"/fix":       {"fix", "debug", "error", "issue", "resolve", "bug"},
	"/implement": {"implement", "create", "build", "develop", "add", "new"},
	"/refactor":  {"refactor", "restructure", "clean", "improve", "optimize"},
	"/test":      {"test", "mock", "assert", "verify", "coverage"},
	"/document":  {"document", "comment", "docstring", "explain", "readme"},
	"/explain":   {"explain", "understand", "how", "why", "describe"},
	"/review":    {"review", "analyze", "check", "inspect", "audit"},
}

// targetSynonyms expands target types into related terminology.
var targetSynonyms = map[string][]string{
	"api":      {"api", "endpoint", "rest", "graphql", "grpc", "route"},
	"database": {"database", "db", "sql", "nosql", "query", "schema", "model"},
	"ui":       {"ui", "frontend", "component", "view", "interface"},
	"backend":  {"backend", "server", "service", "handler", "controller"},
	"auth":     {"auth", "authentication", "authorization", "login", "jwt", "session"},
	"security": {"security", "vulnerability", "safe", "protect", "encrypt"},
}

// languageSynonyms provides alternative names or related terms for programming languages.
var languageSynonyms = map[string][]string{
	"go":         {"go", "golang"},
	"javascript": {"javascript", "js", "ecmascript"},
	"typescript": {"typescript", "ts"},
	"python":     {"python", "py", "pytest", "django", "flask"},
	"ruby":       {"ruby", "rb", "rails"},
	"java":       {"java", "jvm", "spring"},
	"csharp":     {"csharp", "c#", ".net", "dotnet"},
	"cpp":        {"cpp", "c++", "cxx"},
}

// buildExpandedQuery generates a comprehensive semantic search query from a CompilationContext.
// It applies keyword extraction (removing stop words) and query expansion (adding synonyms)
// to produce a query optimized for vector embedding and retrieval.
func buildExpandedQuery(cc *CompilationContext) string {
	if cc == nil {
		return ""
	}

	// We use a map to deduplicate terms automatically while preserving order somewhat via slices later
	termSet := make(map[string]struct{})
	var orderedTerms []string

	addTerm := func(term string) {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			return
		}

		// Skip stop words
		if _, isStopWord := stopWords[term]; isStopWord {
			return
		}

		if _, exists := termSet[term]; !exists {
			termSet[term] = struct{}{}
			orderedTerms = append(orderedTerms, term)
		}
	}

	addTerms := func(terms []string) {
		for _, t := range terms {
			addTerm(t)
		}
	}

	// 1. Process Intent Verb
	if cc.IntentVerb != "" {
		// Try to expand the verb if it matches a known command
		if synonyms, ok := verbSynonyms[strings.ToLower(cc.IntentVerb)]; ok {
			addTerms(synonyms)
		} else {
			// Strip leading slash if present
			addTerm(strings.TrimPrefix(cc.IntentVerb, "/"))
		}
	}

	// 2. Process Intent Target with tokenization
	if cc.IntentTarget != "" {
		// Split target by spaces or punctuation to extract keywords
		tokens := tokenize(cc.IntentTarget)

		for _, token := range tokens {
			addTerm(token)

			// Also try to expand individual keywords
			if synonyms, ok := targetSynonyms[token]; ok {
				addTerms(synonyms)
			}
		}
	}

	// 3. Process Shard ID
	//
	// Tokenized, not added whole. Shard IDs are underscore-joined
	// ("tool_generator_count_mangle_predicates_given_a_directory_path_s"), and
	// a single 60-character term embeds as noise — the one word in it that
	// mattered, "mangle", was invisible to the search. Tokenizing costs
	// nothing for simple IDs: tokenize("coder") is still ["coder"].
	if cc.ShardID != "" {
		addTerms(tokenize(cc.ShardID))
	}

	// 3b. Process the explicit semantic query.
	//
	// SemanticQuery was consumed only by atom selection (selector.go), never by
	// the knowledge-base search this query feeds. So a caller that described its
	// task in prose — Ouroboros passes the tool's purpose — had that prose
	// dropped on the floor here, and the 29k-entry knowledge base was searched
	// with whatever the shard ID happened to contain.
	if cc.SemanticQuery != "" {
		for _, token := range tokenize(cc.SemanticQuery) {
			addTerm(token)
			if synonyms, ok := targetSynonyms[token]; ok {
				addTerms(synonyms)
			}
		}
	}

	// 4. Process Language. Canonical contexts carry "/go", not "go"; the
	// synonym table is keyed without slashes, so an unstripped lookup missed
	// every time and no language ever expanded in production.
	if cc.Language != "" {
		lang := strings.ToLower(strings.TrimPrefix(strings.TrimSpace(cc.Language), "/"))
		if synonyms, ok := languageSynonyms[lang]; ok {
			addTerms(synonyms)
		} else {
			addTerm(lang)
		}
	}

	// 5. Process Frameworks
	for _, fw := range cc.Frameworks {
		addTerm(fw)
	}

	return strings.Join(orderedTerms, " ")
}

// buildLexicalQuery is the term plan for bounded lexical knowledge search:
// the task's own terms -- the target's tokens, then the semantic query's --
// first, stopwords dropped and deduplicated, then the expanded query's terms.
//
// Lexical search keeps only the first few terms (knowledgeLexicalKeywords:
// eight), and the expanded query leads with generated verb and target
// synonyms: for /fix on database/checkpoint.go with the task "wal checkpoint
// stalls" those eight were "fix debug error issue resolve bug database db",
// and a row about "wal checkpoint" never reached SQL (audit
// project-knowledge-lexical-term-budget). The synonyms still fill what the
// task leaves. The semantic search keeps buildExpandedQuery unchanged.
func buildLexicalQuery(cc *CompilationContext) string {
	if cc == nil {
		return ""
	}
	seen := make(map[string]struct{})
	var terms []string
	add := func(term string) {
		term = strings.ToLower(strings.TrimSpace(term))
		if term == "" {
			return
		}
		if _, stop := stopWords[term]; stop {
			return
		}
		if _, dup := seen[term]; dup {
			return
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	for _, t := range tokenize(cc.IntentTarget) {
		add(t)
	}
	for _, t := range tokenize(cc.SemanticQuery) {
		add(t)
	}
	for _, t := range strings.Fields(buildExpandedQuery(cc)) {
		add(t)
	}
	return strings.Join(terms, " ")
}

// tokenize splits a string into lowercase alphanumeric keywords, filtering out punctuation.
func tokenize(text string) []string {
	var tokens []string
	var currentBuilder strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			currentBuilder.WriteRune(unicode.ToLower(r))
		} else if currentBuilder.Len() > 0 {
			tokens = append(tokens, currentBuilder.String())
			currentBuilder.Reset()
		}
	}

	if currentBuilder.Len() > 0 {
		tokens = append(tokens, currentBuilder.String())
	}

	return tokens
}
