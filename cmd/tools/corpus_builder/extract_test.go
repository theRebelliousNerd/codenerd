package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corpus.mg")
	content := `intent_definition("How many files are there?", /stats, "count").
intent_category("How many files are there?", /query).
verb_synonym(/review, "audit").
some_unrelated_predicate("ignored", 1).
`
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	entries, err := extractFromFile(path)
	if err != nil {
		t.Fatalf("extractFromFile: %v", err)
	}

	byPred := map[string]CorpusEntry{}
	for _, e := range entries {
		byPred[e.Predicate] = e
	}

	def, ok := byPred["intent_definition"]
	if !ok {
		t.Fatal("expected an intent_definition corpus entry")
	}
	if def.TextContent != "How many files are there?" || def.Verb != "/stats" || def.Target != "count" {
		t.Errorf("intent_definition entry wrong: %+v", def)
	}

	cat, ok := byPred["intent_category"]
	if !ok || cat.Category != "/query" {
		t.Errorf("intent_category entry wrong: %+v (ok=%v)", cat, ok)
	}

	syn, ok := byPred["verb_synonym"]
	if !ok || syn.Verb != "/review" || syn.TextContent != "audit" {
		t.Errorf("verb_synonym entry wrong: %+v (ok=%v)", syn, ok)
	}

	// Unrecognized predicates must not produce corpus entries.
	if _, ok := byPred["some_unrelated_predicate"]; ok {
		t.Error("unrelated predicates should be skipped")
	}

	// Source file is the basename.
	if def.SourceFile != "corpus.mg" {
		t.Errorf("SourceFile=%q, want corpus.mg", def.SourceFile)
	}
}

func TestFindMGFilesAndContains(t *testing.T) {
	files, err := findMGFiles()
	if err != nil {
		t.Fatalf("findMGFiles: %v", err)
	}
	// The known files are always included regardless of the filesystem.
	if !contains(files, "internal/core/defaults/taxonomy.mg") {
		t.Errorf("findMGFiles should always include taxonomy.mg: %v", files)
	}
	if contains(files, "nonexistent.mg") {
		t.Error("contains returned true for a missing entry")
	}
}

func TestGetAPIKey_FromEnv(t *testing.T) {
	t.Setenv("GEMINI_API_KEY", "env-key-123")
	if got := getAPIKey(); got != "env-key-123" {
		t.Errorf("getAPIKey()=%q, want env-key-123", got)
	}
}
