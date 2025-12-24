package defaults

import (
	"os"
	"testing"
)

func TestIntentCorpusAvailableMatchesDisk(t *testing.T) {
	expected := fileExists("intent_corpus.db")
	if IntentCorpusAvailable() != expected {
		t.Fatalf("intent corpus availability mismatch: expected %v", expected)
	}
}

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
