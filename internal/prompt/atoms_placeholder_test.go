package prompt

import (
	"strings"
	"testing"
)

// TestNoPlaceholderParrotStrings guards against prose-shaped schema examples
// that the model copies verbatim. No atom may contain the historical
// placeholder surface or the old copyable refusal sentence.
func TestNoPlaceholderParrotStrings(t *testing.T) {
	corpus, err := LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus() error = %v", err)
	}
	for _, atom := range corpus.All() {
		content := atom.Content
		if strings.Contains(content, "Human-readable response to the user") || strings.Contains(content, "I cannot delete that directory because it's protected") {
			t.Errorf("atom %q contains parroted placeholder text", atom.ID)
		}
	}
}
