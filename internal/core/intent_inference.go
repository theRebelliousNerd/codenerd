package core

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/types"
)

// InferIntentFromTask derives a best-effort StructuredIntent from a free-form task string.
//
// This is primarily used for shard types that receive an unstructured "task" (e.g., user-defined
// specialist agents backed by the researcher base shard) so that JIT prompt selectors like
// intent_verbs can behave deterministically.
func InferIntentFromTask(task string) *types.StructuredIntent {
	trimmed := strings.TrimSpace(task)
	verb, remainder := inferVerbAndRemainder(trimmed)

	constraint := strings.TrimSpace(remainder)
	if constraint == "" {
		constraint = strings.TrimSpace(task)
	}

	intent := &types.StructuredIntent{
		ID:         fmt.Sprintf("inferred-intent-%d", time.Now().UnixNano()),
		Category:   inferCategoryFromVerb(verb),
		Verb:       verb,
		Target:     inferTargetFromText(constraint),
		Constraint: constraint,
	}

	return intent
}

func inferVerbAndRemainder(task string) (string, string) {
	if task == "" {
		return "/explain", ""
	}

	// Allow an explicit verb token at the start of the task: "/debug ...", "/explain ...", etc.
	if strings.HasPrefix(task, "/") {
		first, rest := splitFirstToken(task)
		if isSupportedVerb(first) {
			return first, strings.TrimSpace(rest)
		}
	}

	firstWord, rest := splitFirstToken(task)
	switch strings.ToLower(firstWord) {
	case "explain", "describe", "summarize", "overview", "teach":
		return "/explain", rest
	case "debug", "investigate", "triage", "diagnose":
		return "/debug", rest
	case "fix", "repair", "resolve":
		return "/fix", rest
	case "refactor", "rewrite", "cleanup", "restructure":
		return "/refactor", rest
	case "test", "tests", "pytest", "gotest":
		return "/test", rest
	case "review", "audit":
		return "/review", rest
	case "security", "secure", "harden":
		return "/security", rest
	case "research", "find", "lookup":
		return "/research", rest
	case "create", "build", "implement", "add":
		return "/create", rest
	default:
		// Default to /explain to ensure intent-based selectors can safely block non-matching atoms.
		return "/explain", task
	}
}

func splitFirstToken(s string) (string, string) {
	s = strings.TrimSpace(s)
	if s == "" {
		return "", ""
	}
	for i, r := range s {
		if r == ' ' || r == '\t' || r == '\n' || r == '\r' {
			return s[:i], strings.TrimSpace(s[i:])
		}
	}
	return s, ""
}

func isSupportedVerb(v string) bool {
	switch strings.TrimSpace(v) {
	case "/fix", "/debug", "/refactor", "/test", "/review", "/security", "/research", "/explain", "/create":
		return true
	default:
		return false
	}
}

func inferCategoryFromVerb(verb string) string {
	switch verb {
	case "/fix", "/debug", "/refactor", "/test", "/create":
		return "/mutation"
	case "/review", "/security", "/research", "/explain":
		return "/query"
	default:
		return "/query"
	}
}

func inferTargetFromText(text string) string {
	for _, tok := range strings.Fields(text) {
		tok = strings.Trim(tok, " \t\r\n,;:()[]{}<>\"'`")
		if tok == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(tok))
		switch ext {
		case ".go", ".rs", ".py", ".ts", ".tsx", ".js", ".jsx", ".java", ".md", ".yaml", ".yml", ".mg", ".gl":
			return tok
		}
	}
	return ""
}
