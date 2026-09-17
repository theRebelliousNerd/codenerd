package session

import (
	"strings"
	"testing"
)

func TestTurnFactsForFinalAnswer_NamesWrittenFiles(t *testing.T) {
	t.Run("names written files", func(t *testing.T) {
		result := &ExecutionResult{
			WrittenPaths:         []string{"a.go", "b.go"},
			SuccessfulWriteTools: 2,
		}
		got := turnFactsForFinalAnswer(result)
		if !strings.Contains(got, "a.go") {
			t.Errorf("expected facts to contain %q, got %q", "a.go", got)
		}
		if !strings.Contains(got, "b.go") {
			t.Errorf("expected facts to contain %q, got %q", "b.go", got)
		}
		if !strings.Contains(got, "2") {
			t.Errorf("expected facts to contain %q, got %q", "2", got)
		}
		if !strings.Contains(got, "must agree with these facts") {
			t.Errorf("expected facts to contain agreement sentence, got %q", got)
		}
	})

	t.Run("empty result names none", func(t *testing.T) {
		result := &ExecutionResult{}
		got := turnFactsForFinalAnswer(result)
		if !strings.Contains(got, "none") {
			t.Errorf("expected facts to contain %q, got %q", "none", got)
		}
	})

	t.Run("nil result empty", func(t *testing.T) {
		if got := turnFactsForFinalAnswer(nil); got != "" {
			t.Errorf("expected empty string for nil result, got %q", got)
		}
	})
}
