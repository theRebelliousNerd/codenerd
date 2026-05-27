package perception

import (
	"codenerd/internal/core"
	"context"
	"strings"
	"testing"

	"codenerd/internal/mangle"
)

func BenchmarkMatchVerbFromCorpus(b *testing.B) {
	// Setup a dummy corpus
	SetVerbCorpus([]VerbEntry{
		{Verb: "/explain", Category: "/query", ShardType: "general"},
		{Verb: "/test", Category: "/action", ShardType: "test"},
		{Verb: "/run", Category: "/action", ShardType: "run"},
	})
	ctx := context.Background()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		matchVerbFromCorpus(ctx, "test input that might match something")
	}
}

func BenchmarkGetRegexCandidates(b *testing.B) {
	SetVerbCorpus([]VerbEntry{
		{Verb: "/explain", Category: "/query", ShardType: "general"},
		{Verb: "/test", Category: "/action", ShardType: "test"},
		{Verb: "/run", Category: "/action", ShardType: "run"},
	})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		getRegexCandidates("test input", GetVerbCorpus())
	}
}

// BenchmarkTaxonomy_ClassifyInput exercises the hot path that bug #18
// addresses: a freshly constructed engine should NOT reload schemas on every
// ClassifyInput. After the fix, each iteration only Clear()s the EDB and
// re-hydrates verb facts. Use -benchtime=Nx (or default time-based) to
// observe steady-state cost; the first iteration includes one-shot schema
// load amortized across the loop.
func BenchmarkTaxonomy_ClassifyInput(b *testing.B) {
	engine, err := NewTaxonomyEngine()
	if err != nil {
		b.Skipf("skipping: NewTaxonomyEngine failed: %v", err)
	}
	defer engine.StopWorker()

	candidates := []VerbEntry{
		{Verb: "/fix", Priority: 90},
		{Verb: "/review", Priority: 100},
		{Verb: "/explain", Priority: 80},
		{Verb: "/search", Priority: 85},
	}

	inputs := []string{
		"fix the bug in the parser",
		"review my recent changes",
		"explain how the kernel works",
		"search the codebase for TODOs",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = engine.ClassifyInput(inputs[i%len(inputs)], candidates)
	}
}

// BenchmarkTaxonomy_ClassifyInput_PreFix replicates the pre-bug-#18 behavior:
// Reset() the engine and reload every embedded schema/logic file on each call.
// This bench exists ONLY to give a measurable before/after comparison against
// BenchmarkTaxonomy_ClassifyInput; it does not exercise production code.
func BenchmarkTaxonomy_ClassifyInput_PreFix(b *testing.B) {
	engine, err := NewTaxonomyEngine()
	if err != nil {
		b.Skipf("skipping: NewTaxonomyEngine failed: %v", err)
	}
	defer engine.StopWorker()

	candidates := []VerbEntry{
		{Verb: "/fix", Priority: 90},
		{Verb: "/review", Priority: 100},
		{Verb: "/explain", Priority: 80},
		{Verb: "/search", Priority: 85},
	}

	inputs := []string{
		"fix the bug in the parser",
		"review my recent changes",
		"explain how the kernel works",
		"search the codebase for TODOs",
	}

	intentFiles := []string{"schemas_intent.mg"}
	intentFiles = append(intentFiles, core.DefaultIntentSchemaFiles()...)
	intentFiles = append(intentFiles, "schemas_learning.mg")
	intentFiles = append(intentFiles, "policy/taxonomy_qualifiers.mg")
	intentFiles = append(intentFiles, "policy/taxonomy_inference.mg")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate the old hot path: Reset + reload schemas + re-add default facts.
		engine.engine.Reset()
		for _, file := range intentFiles {
			content, gerr := core.GetDefaultContent(file)
			if gerr != nil {
				continue
			}
			_ = engine.engine.LoadSchemaString(content)
		}

		facts := []mangle.Fact{}
		for _, entry := range DefaultTaxonomyData {
			facts = append(facts, mangle.Fact{Predicate: "verb_def", Args: []interface{}{entry.Verb, entry.Category, entry.ShardType, entry.Priority}})
			for _, syn := range entry.Synonyms {
				facts = append(facts, mangle.Fact{Predicate: "verb_synonym", Args: []interface{}{entry.Verb, syn}})
			}
			for _, pat := range entry.Patterns {
				facts = append(facts, mangle.Fact{Predicate: "verb_pattern", Args: []interface{}{entry.Verb, pat}})
			}
		}

		input := inputs[i%len(inputs)]
		for _, token := range strings.Fields(strings.ToLower(input)) {
			token = strings.Trim(token, ".,!?;:\"'()[]{}<>")
			if token == "" {
				continue
			}
			facts = append(facts, mangle.Fact{Predicate: "context_token", Args: []interface{}{token}})
		}
		facts = append(facts, mangle.Fact{Predicate: "user_input_string", Args: []interface{}{input}})
		for _, cand := range candidates {
			facts = append(facts, mangle.Fact{
				Predicate: "candidate_intent",
				Args:      []interface{}{cand.Verb, int64(cand.Priority)},
			})
		}
		_ = engine.engine.AddFacts(facts)
		_, _ = engine.engine.GetFacts("potential_score")
	}
}
