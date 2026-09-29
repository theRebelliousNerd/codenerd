// limits_cleanup_test.go pins the LIMITS CLEANUP behavior changes in package
// init: document content and stored atoms reach the LLM whole instead of
// being cut at hardcoded char bounds.
package init

import (
	"strings"
	"testing"

	"codenerd/internal/store"
)

// Stored atoms feed the synthesis model whole: a 200-char cut truncated the
// context for every long insight.
func TestBuildAtomsSummary_LongAtomContentReturnedWhole(t *testing.T) {
	long := strings.Repeat("insight detail ", 30) + "TAIL-PAST-200"
	if len(long) <= 200 {
		t.Fatalf("fixture too short to exercise the old 200-char cut: %d", len(long))
	}
	atoms := []store.KnowledgeAtom{
		{Concept: "doc/DESIGN.md/architecture/layering", Content: long, Confidence: 0.9},
		{Concept: "doc/README.md/philosophy/simplicity", Content: "short", Confidence: 0.8},
	}
	summary, categories := buildAtomsSummary(atoms)
	if !strings.Contains(summary, long) {
		t.Error("summary cut atom content at 200 chars")
	}
	if !strings.Contains(summary, "TAIL-PAST-200") {
		t.Error("summary lost the tail past char 200")
	}
	if len(categories) != 2 {
		t.Errorf("got %d categories, want 2", len(categories))
	}
}
