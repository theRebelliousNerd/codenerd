package perception

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"codenerd/internal/types"
)

// A completion the vendor stopped with finish_reason "length" is not returned
// as text. It is a typed truncation report carrying the partial for
// diagnostics, so the broker can send the request back to be restated and
// nothing downstream can mistake the fragment for an answer.
func TestCompleteWithSystem_LengthFinishIsATruncationReportNotAnAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"m","choices":[{"message":{"role":"assistant","content":"the first half of a long"},"finish_reason":"length"}],"usage":{"prompt_tokens":10,"completion_tokens":16384}}`))
	}))
	defer srv.Close()

	c := newCompatClientAt(t, ProviderMeta, srv.URL)
	out, err := c.CompleteWithSystem(context.Background(), "sys", "user")
	if out != "" {
		t.Fatalf("a cut completion leaked as text: %q", out)
	}
	report, ok := types.AsOutputTruncated(err)
	if !ok {
		t.Fatalf("err = %v, want a truncation report", err)
	}
	if report.Reason != "length" || report.OutputTokens != 16384 || report.Partial != "the first half of a long" {
		t.Fatalf("report = %+v, want the vendor's finish reason, billed output and the partial", report)
	}
	if report.LimitTokens <= 0 {
		t.Fatalf("report.LimitTokens = %d, want the client's configured ceiling", report.LimitTokens)
	}
}

// A tool response cut at the ceiling is refused whole: a function call whose
// arguments were cut would otherwise be dispatched with whatever parsed.
func TestCompleteWithTools_LengthFinishIsRefusedWhole(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"m","choices":[{"message":{"role":"assistant","content":"","tool_calls":[{"id":"c1","type":"function","function":{"name":"read_file","arguments":"{\"path\":\"a.go\"}"}}]},"finish_reason":"length"}],"usage":{"prompt_tokens":10,"completion_tokens":99}}`))
	}))
	defer srv.Close()

	c := newCompatClientAt(t, ProviderDashScope, srv.URL)
	resp, err := c.CompleteWithTools(context.Background(), "sys", "user", []ToolDefinition{{Name: "read_file"}})
	if resp != nil {
		t.Fatalf("a cut tool response leaked: %+v", resp)
	}
	if _, ok := types.AsOutputTruncated(err); !ok {
		t.Fatalf("err = %v, want a truncation report", err)
	}
}

// A normal stop is untouched: the report exists for one finish reason only.
func TestCompleteWithSystem_StopFinishIsAnAnswer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"model":"m","choices":[{"message":{"role":"assistant","content":"whole"},"finish_reason":"stop"}],"usage":{"completion_tokens":2}}`))
	}))
	defer srv.Close()

	c := newCompatClientAt(t, ProviderMeta, srv.URL)
	out, err := c.CompleteWithSystem(context.Background(), "sys", "user")
	if err != nil || out != "whole" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
