package main

import (
	"strings"
	"testing"
)

// containsToken reports whether want is present in toks (case-insensitive exact match).
func containsToken(toks []string, want string) bool {
	want = strings.ToLower(want)
	for _, tok := range toks {
		if tok == want {
			return true
		}
	}
	return false
}

func TestDistinctiveTokensDropsInterrogatives(t *testing.T) {
	// Core question-shaped example from the spec: the interrogative is not
	// evidence, the subject words are.
	toks := distinctiveTokens("which prompt atom categories consume the most tokens")
	if containsToken(toks, "which") {
		t.Fatalf("distinctiveTokens should drop interrogative 'which', got %v", toks)
	}
	for _, want := range []string{"prompt", "atom", "categories", "tokens"} {
		if !containsToken(toks, want) {
			t.Errorf("distinctiveTokens %v missing expected token %q", toks, want)
		}
	}

	interrogatives := []string{
		"which", "what", "why", "how", "where",
		"who", "whom", "whose", "them", "they",
		"their", "there", "here", "each", "any",
		"all", "some", "both", "other", "others",
	}
	for _, word := range interrogatives {
		t.Run(word, func(t *testing.T) {
			// Leading position as specified in the task.
			input := word + " the kernel schema"
			got := distinctiveTokens(input)
			if containsToken(got, word) {
				t.Errorf("distinctiveTokens(%q) should drop interrogative %q, got %v", input, word, got)
			}
			for _, want := range []string{"kernel", "schema"} {
				if !containsToken(got, want) {
					t.Errorf("distinctiveTokens(%q) missing expected token %q, got %v", input, want, got)
				}
			}
			// Embedded position: verifies the drop is via the stopword list, not
			// merely the leading-imperative exclusion. Words like "which"/"what"
			// are hidden when leading even without a stopword entry; mid-sentence
			// they are only hidden if the stopword entry exists, so this is the
			// assertion that fails if the entry is removed.
			embeddedInput := "the kernel " + word + " schema evaluation"
			embedded := distinctiveTokens(embeddedInput)
			if containsToken(embedded, word) {
				t.Errorf("distinctiveTokens should drop interrogative %q even when not leading (input %q), got %v", word, embeddedInput, embedded)
			}
			for _, want := range []string{"kernel", "schema"} {
				if !containsToken(embedded, want) {
					t.Errorf("distinctiveTokens(%q) missing expected token %q, got %v", embeddedInput, want, embedded)
				}
			}
		})
	}
}

func TestDistinctiveTokensDropsReportingVerbs(t *testing.T) {
	toks := distinctiveTokens("list the failing predicates")
	if containsToken(toks, "list") {
		t.Fatalf("distinctiveTokens should drop reporting verb 'list', got %v", toks)
	}
	for _, want := range []string{"failing", "predicates"} {
		if !containsToken(toks, want) {
			t.Errorf("distinctiveTokens %v missing expected token %q", toks, want)
		}
	}

	reportingVerbs := []string{
		"state", "list", "name", "explain", "describe",
		"show", "tell", "identify", "report", "summarize",
		"summarise", "provide", "give", "return", "output",
		"print", "display", "mention", "note", "detail",
		"specify",
	}
	for _, word := range reportingVerbs {
		t.Run(word, func(t *testing.T) {
			input := word + " the failing predicates"
			got := distinctiveTokens(input)
			if containsToken(got, word) {
				t.Errorf("distinctiveTokens(%q) should drop reporting verb %q, got %v", input, word, got)
			}
			for _, want := range []string{"failing", "predicates"} {
				if !containsToken(got, want) {
					t.Errorf("distinctiveTokens(%q) missing expected token %q, got %v", input, want, got)
				}
			}
			// Embedded position: leading-verb exclusion hides every reporting
			// verb when it is first, so the spec's leading check alone would not
			// fail if the stopword entry were removed. The embedded check is the
			// one that fails on removal.
			embeddedInput := "the kernel " + word + " predicates evaluation"
			embedded := distinctiveTokens(embeddedInput)
			if containsToken(embedded, word) {
				t.Errorf("distinctiveTokens should drop reporting verb %q even when not leading (input %q), got %v", word, embeddedInput, embedded)
			}
			for _, want := range []string{"kernel", "predicates"} {
				if !containsToken(embedded, want) {
					t.Errorf("distinctiveTokens(%q) missing expected token %q, got %v", embeddedInput, want, embedded)
				}
			}
		})
	}
}

func TestDistinctiveTokensKeepsDomainWordsThatLookGeneric(t *testing.T) {
	toks := distinctiveTokens("state the kernel fixpoint evaluation order")
	if len(toks) == 0 {
		t.Fatalf("distinctiveTokens should keep domain words for 'state the kernel fixpoint evaluation order', got empty (input filtered to nothing)")
	}
	for _, want := range []string{"kernel", "fixpoint", "evaluation"} {
		if !containsToken(toks, want) {
			t.Errorf("distinctiveTokens %v missing expected domain token %q", toks, want)
		}
	}
	// Document the deliberate trade-off: "state" is a stop word even though it
	// can be domain content. The guard prefers to keep the distinctive domain
	// tokens rather than require the verb to be echoed.
	if containsToken(toks, "state") {
		t.Errorf("distinctiveTokens should drop reporting verb 'state' even in domain-heavy instruction, got %v", toks)
	}
}
