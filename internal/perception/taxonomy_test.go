package perception

import (
	"context"
	"regexp"
	"testing"
)

func TestMatchVerbFromCorpus_Integration(t *testing.T) {
	// Initialize the shared taxonomy engine
	var err error
	SharedTaxonomy, err = NewTaxonomyEngine()
	if err != nil {
		t.Fatalf("Failed to initialize taxonomy engine: %v", err)
	}

	// Initialize VerbCorpus manually since we can't easily trigger the init() again
	VerbCorpus, err = SharedTaxonomy.GetVerbs()
	if err != nil {
		t.Fatalf("Failed to get verbs from taxonomy: %v", err)
	}

	// DEBUG: Check if intent_qualifiers.mg loaded
	facts, err := SharedTaxonomy.engine.GetFacts("interrogative_type")
	if err != nil {
		t.Fatalf("Failed to query interrogative_type: %v", err)
	}
	if len(facts) == 0 {
		t.Fatal("CRITICAL: interrogative_type facts missing! intent_qualifiers.mg not loaded.")
	}
	t.Logf("Found %d interrogative_type definitions", len(facts))

	tests := []struct {
		name         string
		input        string
		expectedVerb string
		expectedCat  string
	}{
		{
			name:         "Direct review match",
			input:        "review code",
			expectedVerb: "/review",
			expectedCat:  "/query",
		},
		{
			name:         "Why is failing (Causality + Error)",
			input:        "Why is this test failing?",
			expectedVerb: "/debug", // Should map to debug due to 'why' + 'failing'
			expectedCat:  "/query",
		},
		{
			name:         "What is this (Definition)",
			input:        "What is this function?",
			expectedVerb: "/explain", // Should map to explain due to 'what is'
			expectedCat:  "/query",
		},
		{
			name:         "Where is (Spatial)",
			input:        "Where is the config file?",
			expectedVerb: "/search", // Should map to search due to 'where is'
			expectedCat:  "/query",
		},
		{
			name:         "Hypothetical (Modal)",
			input:        "What if I deleted this?",
			expectedVerb: "/dream", // Should map to dream due to 'what if'
			expectedCat:  "/query",
		},
		{
			name:         "Polite request (Strip Modal)",
			input:        "Can you please review this?",
			expectedVerb: "/review", // Should map to review, stripping 'Can you please'
			expectedCat:  "/query",
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verb, category, conf, _ := matchVerbFromCorpus(ctx, tt.input)
			
			if verb != tt.expectedVerb {
				t.Errorf("Expected verb %s, got %s (conf: %.2f)", tt.expectedVerb, verb, conf)
			}
			if category != tt.expectedCat {
				t.Errorf("Expected category %s, got %s", tt.expectedCat, category)
			}
		})
	}
}

func TestGetRegexCandidates(t *testing.T) {
	// Setup a controlled corpus for testing
	originalCorpus := VerbCorpus
	defer func() { VerbCorpus = originalCorpus }()
	
	VerbCorpus = []VerbEntry{
		{
			Verb:     "/review",
			Synonyms: []string{"review", "audit"},
			Patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)review.*code`)},
		},
		{
			Verb:     "/fix",
			Synonyms: []string{"fix", "repair"},
			Patterns: []*regexp.Regexp{regexp.MustCompile(`(?i)fix.*bug`)},
		},
	}

	tests := []struct {
		input       string
		expectVerbs []string
	}{
		{"review code", []string{"/review"}},
		{"audit this", []string{"/review"}},
		{"fix bug", []string{"/fix"}},
		{"repair it", []string{"/fix"}},
		{"unknown command", []string{}},
	}

	for _, tt := range tests {
		candidates := getRegexCandidates(tt.input)
		if len(candidates) != len(tt.expectVerbs) {
			t.Errorf("Input '%s': expected %d candidates, got %d", tt.input, len(tt.expectVerbs), len(candidates))
		}
		
		for _, expect := range tt.expectVerbs {
			found := false
			for _, c := range candidates {
				if c.Verb == expect {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("Input '%s': expected candidate %s not found", tt.input, expect)
			}
		}
	}
}
