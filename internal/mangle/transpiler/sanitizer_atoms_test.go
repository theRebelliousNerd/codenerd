package transpiler

import (
	"strings"
	"testing"
)

func TestSanitizeAtomsDirect(t *testing.T) {
	s := NewSanitizer()

	// A well-formed clause round-trips through parse -> transform -> serialize
	// with the predicate preserved.
	out, err := s.SanitizeAtoms(`research_topic("gemini", "testing", "pending").`)
	if err != nil {
		t.Fatalf("SanitizeAtoms: %v", err)
	}
	if !strings.Contains(out, "research_topic") {
		t.Errorf("expected research_topic predicate in output, got %q", out)
	}

	// Malformed input surfaces a parse error rather than panicking.
	if _, err := s.SanitizeAtoms("@@@ not valid mangle @@@"); err == nil {
		t.Error("expected a parse error for malformed input")
	}
}
