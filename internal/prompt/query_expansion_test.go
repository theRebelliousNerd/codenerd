package prompt

import (
	"strings"
	"testing"
)

func TestQueryExpansion(t *testing.T) {
	tests := []struct {
		name     string
		context  *CompilationContext
		expected string // We expect these space-separated terms
	}{
		{
			name:     "nil context",
			context:  nil,
			expected: "",
		},
		{
			name:     "empty context",
			context:  &CompilationContext{},
			expected: "",
		},
		{
			name: "only intent verb",
			context: &CompilationContext{
				IntentVerb: "/fix",
			},
			expected: "fix debug error issue resolve bug",
		},
		{
			name: "unknown intent verb",
			context: &CompilationContext{
				IntentVerb: "/unknown",
			},
			expected: "unknown",
		},
		{
			name: "intent target tokenization and synonyms",
			context: &CompilationContext{
				IntentTarget: "auth service",
			},
			expected: "auth authentication authorization login jwt session service",
		},
		{
			name: "target tokenization dropping punctuation",
			context: &CompilationContext{
				IntentTarget: "hello, world! auth-service",
			},
			expected: "hello world auth authentication authorization login jwt session service",
		},
		{
			name: "stop word removal",
			context: &CompilationContext{
				IntentTarget: "fix the database for a user",
			},
			expected: "fix database db sql nosql query schema model user",
		},
		{
			name: "language synonyms",
			context: &CompilationContext{
				Language: "go",
			},
			expected: "go golang",
		},
		{
			name: "language unknown synonym",
			context: &CompilationContext{
				Language: "rust",
			},
			expected: "rust",
		},
		{
			name: "full context combined",
			context: &CompilationContext{
				IntentVerb:   "/fix",
				IntentTarget: "auth module",
				ShardID:      "coder",
				Language:     "go",
				Frameworks:   []string{"gin", "cobra"},
			},
			expected: "fix debug error issue resolve bug auth authentication authorization login jwt session module coder go golang gin cobra",
		},
		{
			name: "deduplication check",
			context: &CompilationContext{
				IntentVerb:   "/fix",
				IntentTarget: "fix the auth for go",
				Language:     "go",
			},
			expected: "fix debug error issue resolve bug auth authentication authorization login jwt session go golang",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildExpandedQuery(tt.context)

			// Order is semi-deterministic based on order of processing
			// so we just split both and ensure they have the same set of words

			expectedTerms := strings.Fields(tt.expected)
			resultTerms := strings.Fields(result)

			if len(expectedTerms) != len(resultTerms) {
				t.Errorf("expected %d terms (%q), got %d terms (%q)",
					len(expectedTerms), tt.expected,
					len(resultTerms), result)
			}

			// build map for checking
			expectedMap := make(map[string]bool)
			for _, term := range expectedTerms {
				expectedMap[term] = true
			}

			for _, term := range resultTerms {
				if !expectedMap[term] {
					t.Errorf("unexpected term in result: %q", term)
				}
			}
		})
	}
}
