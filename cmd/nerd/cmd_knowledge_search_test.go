package main

import (
	"strings"
	"testing"
)

// `nerd knowledge search` is the command's whole output to the user: a long
// atom prints whole, never sliced for display.
func TestFormatKnowledgeSearchHit_PrintsAtomWhole(t *testing.T) {
	content := strings.Repeat("ab", 500) // 1000 chars: twice the old 500-char cut
	got := formatKnowledgeSearchHit(2, "concept", content)
	if !strings.Contains(got, content) {
		t.Fatalf("hit of %d chars does not contain the whole atom", len(content))
	}
	if strings.HasSuffix(got, "...") {
		t.Error("hit ends in an ellipsis: the atom was cut")
	}
}

// The extraction kept the rendering byte-identical: header, separator, body.
func TestFormatKnowledgeSearchHit_ExactRendering(t *testing.T) {
	got := formatKnowledgeSearchHit(1, "C", "body")
	want := "\n### 1. C\n" + strings.Repeat("─", 40) + "\nbody"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
