package defaults

import (
	"os"
	"strings"
	"testing"
)

func TestPredicateCorpusAvailableMatchesDisk(t *testing.T) {
	expected := fileExists("predicate_corpus.db")
	if PredicateCorpusAvailable() != expected {
		t.Fatalf("predicate corpus availability mismatch: expected %v", expected)
	}
}

func TestPromptCorpusAvailableMatchesDisk(t *testing.T) {
	expected := fileExists("prompt_corpus.db")
	if PromptCorpusAvailable() != expected {
		t.Fatalf("prompt corpus availability mismatch: expected %v", expected)
	}
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestEvolutionEmbedsArePopulated(t *testing.T) {
	if EvolutionSchema == "" {
		t.Fatal("EvolutionSchema embed is empty")
	}
	if !strings.Contains(EvolutionSchema, "Decl evolution_cohort_member") {
		t.Fatalf("EvolutionSchema missing expected Decl, got %q", EvolutionSchema)
	}
	if EvolutionPolicy == "" {
		t.Fatal("EvolutionPolicy embed is empty")
	}
	if !strings.Contains(EvolutionPolicy, "evolution_cohort_member") {
		t.Fatalf("EvolutionPolicy missing cohort rule, got %q", EvolutionPolicy)
	}
	if !strings.Contains(EvolutionPolicy, "evolution_generation_allowed") {
		t.Fatalf("EvolutionPolicy missing generation rule, got %q", EvolutionPolicy)
	}
	if ConfigParamsPolicy == "" {
		t.Fatal("ConfigParamsPolicy embed is empty")
	}
	if !strings.Contains(ConfigParamsPolicy, "Decl config_param(") {
		t.Fatalf("ConfigParamsPolicy missing config_param Decl, got %q", ConfigParamsPolicy)
	}
	if !strings.Contains(ConfigParamsPolicy, "config_param_missing") {
		t.Fatalf("ConfigParamsPolicy missing missing-knob rule, got %q", ConfigParamsPolicy)
	}
}

func TestFileExists_PinsMissingAndPresent(t *testing.T) {
	if fileExists("definitely_missing_corpus_xyz_12345.db") {
		t.Fatal("fileExists returned true for a file that does not exist")
	}
	if fileExists("") {
		t.Fatal("fileExists returned true for empty path")
	}
	if !fileExists("corpus_test.go") {
		t.Fatal("fileExists returned false for corpus_test.go which exists beside this test")
	}
}

func TestEmbeddedCorpusFS_Readable(t *testing.T) {
	if _, err := IntentCorpusDB.ReadDir("."); err != nil {
		t.Fatalf("IntentCorpusDB.ReadDir failed: %v", err)
	}
	if _, err := PredicateCorpusDB.ReadDir("."); err != nil {
		t.Fatalf("PredicateCorpusDB.ReadDir failed: %v", err)
	}
	if _, err := PromptCorpusDB.ReadDir("."); err != nil {
		t.Fatalf("PromptCorpusDB.ReadDir failed: %v", err)
	}
	entries, err := PromptCorpusDB.ReadDir(".")
	if err != nil {
		t.Fatalf("PromptCorpusDB.ReadDir failed: %v", err)
	}
	foundPlaceholder := false
	foundDB := false
	for _, e := range entries {
		switch e.Name() {
		case "prompt_corpus.db.placeholder":
			foundPlaceholder = true
		case "prompt_corpus.db":
			foundDB = true
		}
	}
	if !foundPlaceholder && !foundDB {
		t.Fatalf("PromptCorpusDB embeds neither placeholder nor db, entries=%v", entries)
	}
	intentEntries, err := IntentCorpusDB.ReadDir(".")
	if err != nil {
		t.Fatalf("IntentCorpusDB.ReadDir failed: %v", err)
	}
	foundIntentPlaceholder := false
	foundIntentDB := false
	for _, e := range intentEntries {
		switch e.Name() {
		case "intent_corpus.db.placeholder":
			foundIntentPlaceholder = true
		case "intent_corpus.db":
			foundIntentDB = true
		}
	}
	if !foundIntentPlaceholder && !foundIntentDB {
		t.Fatalf("IntentCorpusDB embeds neither placeholder nor db, entries=%v", intentEntries)
	}
}

func TestCorpusAvailable_IsDeterministic(t *testing.T) {
	if PredicateCorpusAvailable() != PredicateCorpusAvailable() {
		t.Fatal("PredicateCorpusAvailable is nondeterministic")
	}
	if PromptCorpusAvailable() != PromptCorpusAvailable() {
		t.Fatal("PromptCorpusAvailable is nondeterministic")
	}
}
