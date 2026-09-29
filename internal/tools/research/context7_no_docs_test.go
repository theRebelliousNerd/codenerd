package research

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

// TestExecuteContext7_WhenNoDocsFound_ShouldReturnError is a regression test:
// executeContext7 must return a non-nil error (and empty result) when no
// llms.txt or fallback docs are found, so callers do not ingest the
// "no documentation" message as researched knowledge.
func TestExecuteContext7_WhenNoDocsFound_ShouldReturnError(t *testing.T) {
	t.Run("explicit repo all 404", func(t *testing.T) {
		pinLocalGitHub(t, func(w http.ResponseWriter, r *http.Request) {
			http.NotFound(w, r)
		})
		assertNoContext7Docs(t, map[string]any{
			"topic": "nonexistent-lib-xyz",
			"repo":  "owner/repo",
		}, "nonexistent-lib-xyz")
	})
	t.Run("unknown topic with no inferrable repo", func(t *testing.T) {
		assertNoContext7Docs(t, map[string]any{
			"topic": "unknown-topic-xyz-no-repo",
		}, "unknown-topic-xyz-no-repo")
	})
}

func assertNoContext7Docs(t *testing.T, args map[string]any, topic string) {
	t.Helper()
	result, err := executeContext7(context.Background(), args)
	if err == nil {
		t.Fatalf("executeContext7(%v) = %q, nil error; want non-nil error for no-documentation", args, result)
	}
	if result != "" {
		t.Errorf("executeContext7(%v) result = %q; want empty result on no-documentation error", args, result)
	}
	// Matched case-insensitively: the assertion is about the error
	// naming the no-documentation condition, not about its
	// capitalization, and the exact-case form broke when the message
	// was lowercased to follow Go's error-string convention.
	if !strings.Contains(strings.ToLower(err.Error()), "no llm-optimized documentation found") {
		t.Errorf("error %q should say no LLM-optimized documentation was found", err.Error())
	}
	if !strings.Contains(err.Error(), topic) {
		t.Errorf("error %q should mention topic %q", err.Error(), topic)
	}
}
