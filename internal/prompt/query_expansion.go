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
	"api":       {"api", "endpoint", "rest", "graphql", "grpc", "route"},
	"database":  {"database", "db", "sql", "nosql", "query", "schema", "model"},
	"ui":        {"ui", "frontend", "component", "view", "interface"},
	"backend":   {"backend", "server", "service", "handler", "controller"},
	"auth":      {"auth", "authentication", "authorization", "login", "jwt", "session"},
	"security":  {"security", "vulnerability", "safe", "protect", "encrypt"},
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
	if cc.ShardID != "" {
		addTerm(cc.ShardID)
	}

	// 4. Process Language
	if cc.Language != "" {
		lang := strings.ToLower(cc.Language)
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
