package main

import (
	"testing"

	"codenerd/internal/core"
)

func TestCleanRegexForEmbedding(t *testing.T) {
	pattern := "(?i)review.*code|audit"
	cleaned := cleanRegexForEmbedding(pattern)
	if cleaned != "review code or audit" {
		t.Fatalf("unexpected cleaned pattern: %q", cleaned)
	}
}

func TestFactToCorpusEntryIntentDefinition(t *testing.T) {
	fact := core.Fact{
		Predicate: "intent_definition",
		Args: []interface{}{
			"How many files?",
			core.MangleAtom("/stats"),
			"count",
		},
	}

	entry, ok := factToCorpusEntry(fact, "schema.mg")
	if !ok {
		t.Fatalf("expected intent_definition to be extractable")
	}
	if entry.TextContent != "How many files?" {
		t.Fatalf("unexpected text content: %q", entry.TextContent)
	}
	if entry.Verb != "/stats" {
		t.Fatalf("unexpected verb: %q", entry.Verb)
	}
	if entry.Target != "count" {
		t.Fatalf("unexpected target: %q", entry.Target)
	}
}

func TestArgToStringBool(t *testing.T) {
	if got := argToString(true); got != "true" {
		t.Fatalf("expected true, got %q", got)
	}
	if got := argToString(false); got != "false" {
		t.Fatalf("expected false, got %q", got)
	}
}
