package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHybridLoader_Parse(t *testing.T) {
	// Create temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.mg")

	content := `
# Valid Mangle
Decl foo(Name).
foo("bar").

# Hybrid Directives
INTENT: "fix things" -> /fix "all"
PROMPT: /sys_prompt [system] -> "You are generic."
TAXONOMY: animal > mammal > dog
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp file: %v", err)
	}

	result, err := LoadHybridMangleFile(path)
	if err != nil {
		t.Fatalf("LoadHybridMangleFile failed: %v", err)
	}

	// 1. Check Logic (comments preserved, directives removed or mostly preserved? Impl strips them?)
	// Looking at impl: DIRECTIVES are processed and `continue` matches, so NOT added to Logic string.
	// Only normal lines added to Logic.

	// 2. Check Intents
	if len(result.Intents) != 1 {
		t.Errorf("Expected 1 intent, got %d", len(result.Intents))
	} else {
		if result.Intents[0].Verb != "/fix" {
			t.Errorf("Expected verb /fix, got %s", result.Intents[0].Verb)
		}
	}

	// 3. Check Prompts
	if len(result.Prompts) != 1 {
		t.Errorf("Expected 1 prompt, got %d", len(result.Prompts))
	} else {
		if result.Prompts[0].ID != "sys_prompt" {
			t.Errorf("Expected prompt ID sys_prompt, got %s", result.Prompts[0].ID)
		}
	}

	// 4. Check Taxonomy (converted to subclass_of facts)
	// animal > mammal > dog produces:
	// subclass_of(dog, mammal)
	// subclass_of(mammal, animal)
	foundSubclass := 0
	for _, f := range result.Facts {
		if f.Predicate == "subclass_of" {
			foundSubclass++
		}
	}
	if foundSubclass != 2 {
		t.Errorf("Expected 2 subclass_of facts, got %d", foundSubclass)
	}
}
