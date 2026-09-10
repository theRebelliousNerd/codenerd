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
	tests := []struct {
		name string
		args map[string]any
		want string
	}{
		{
			name: "explicit repo all 404",
			args: map[string]any{
				"topic": "nonexistent-lib-xyz",
				"repo":  "owner/repo",
			},
			want: "nonexistent-lib-xyz",
		},
		{
			name: "unknown topic with no inferrable repo",
			args: map[string]any{
				"topic": "unknown-topic-xyz-no-repo",
			},
			want: "unknown-topic-xyz-no-repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockTransport()
			// No responders registered — all requests 404.

			oldTransport := http.DefaultClient.Transport
			http.DefaultClient.Transport = mock
			defer func() { http.DefaultClient.Transport = oldTransport }()

			result, err := executeContext7(context.Background(), tt.args)
			if err == nil {
				t.Fatalf("executeContext7(%v) = %q, nil error; want non-nil error for no-documentation", tt.args, result)
			}
			if result != "" {
				t.Errorf("executeContext7(%v) result = %q; want empty result on no-documentation error", tt.args, result)
			}
			// Matched case-insensitively: the assertion is about the error
			// naming the no-documentation condition, not about its
			// capitalization, and the exact-case form broke when the message
			// was lowercased to follow Go's error-string convention.
			if !strings.Contains(strings.ToLower(err.Error()), "no llm-optimized documentation found") {
				t.Errorf("error %q should say no LLM-optimized documentation was found", err.Error())
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error %q should mention topic %q", err.Error(), tt.want)
			}
		})
	}
}
