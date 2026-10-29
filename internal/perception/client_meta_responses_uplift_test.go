package perception

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/types"
)

// The Chat path validates Meta's payload limits before sending; the Responses
// path never did, so a 65-character call_id sailed to the vendor and died as
// an opaque HTTP 400. The rejection must happen client-side, before the wire.
func TestMetaResponses_RejectsOversizeCallIDBeforeWire(t *testing.T) {
	var seen int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&seen, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metaResponsesOKReply))
	}))
	defer srv.Close()

	history := []types.Message{{
		Role: "assistant",
		ToolCalls: []types.ToolCall{
			{ID: strings.Repeat("c", 65), Name: "read_file"},
		},
	}}
	_, err := newTestCompatClient(t, ProviderMeta, srv.URL).CompleteWithToolResults(
		context.Background(), "sys", history, nil)
	if err == nil {
		t.Fatal("expected a validation error for the 65-char call_id")
	}
	if !strings.Contains(err.Error(), "1-64") {
		t.Errorf("error = %q, want Meta's 1-64 constraint named", err)
	}
	if got := atomic.LoadInt32(&seen); got != 0 {
		t.Errorf("server saw %d requests, want 0 (rejected before the wire)", got)
	}
}

// Tool names with more than one dot are rejected by Meta with HTTP 400 on
// this surface too — same client-side bar as the Chat path.
func TestMetaResponses_RejectsBadToolNameBeforeWire(t *testing.T) {
	var seen int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&seen, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(metaResponsesOKReply))
	}))
	defer srv.Close()

	tools := []ToolDefinition{{Name: "a.b.c", Description: "bad"}}
	_, err := newTestCompatClient(t, ProviderMeta, srv.URL).CompleteWithToolResults(
		context.Background(), "sys", metaResponsesHistory(), tools)
	if err == nil {
		t.Fatal("expected a validation error for the dotted tool name")
	}
	if got := atomic.LoadInt32(&seen); got != 0 {
		t.Errorf("server saw %d requests, want 0 (rejected before the wire)", got)
	}
}

// The validator is Meta-scoped: other vendors sharing the compat client must
// not inherit Meta's payload limits.
func TestMetaResponses_ValidationIsNoOpForOtherVendors(t *testing.T) {
	c := newTestCompatClient(t, ProviderDashScope, "http://127.0.0.1:1")
	history := []types.Message{{
		Role: "assistant",
		ToolCalls: []types.ToolCall{
			{ID: strings.Repeat("c", 65), Name: "read_file"},
		},
	}}
	if err := c.validateMetaHistory([]ToolDefinition{{Name: "a.b.c"}}, history); err != nil {
		t.Errorf("non-Meta vendor rejected: %v, want no-op", err)
	}
}

// A failed run carries no usable output; returning it as success would hand
// the executor an empty turn it mistakes for a final answer. The vendor's
// own message must survive in the error.
func TestMetaResponses_FailedStatusIsError(t *testing.T) {
	// With a vendor error object, the transport layer already surfaces the
	// vendor's message; either way the turn must fail, never read as empty.
	bodies := []string{
		`{"id":"r1","status":"failed","model":"m","output":[],` +
			`"error":{"type":"server_error","code":"x","message":"backend blew up"}}`,
		`{"id":"r1","status":"failed","model":"m","output":[]}`,
	}
	for _, body := range bodies {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
		}))
		_, err := newTestCompatClient(t, ProviderMeta, srv.URL).CompleteWithToolResults(
			context.Background(), "sys", metaResponsesHistory(), nil)
		srv.Close()
		if err == nil {
			t.Fatalf("body %s: expected an error for status failed", body)
		}
		if !strings.Contains(body, "backend blew up") && !strings.Contains(err.Error(), "failed") {
			t.Errorf("body %s: error = %q, want the failed status named", body, err)
		}
		if strings.Contains(body, "backend blew up") && !strings.Contains(err.Error(), "backend blew up") {
			t.Errorf("error = %q, want the vendor message", err)
		}
	}
}

func TestMetaResponses_CancelledStatusIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"r1","status":"cancelled","model":"m","output":[]}`))
	}))
	defer srv.Close()

	_, err := newTestCompatClient(t, ProviderMeta, srv.URL).CompleteWithToolResults(
		context.Background(), "sys", metaResponsesHistory(), nil)
	if err == nil {
		t.Fatal("expected an error for status cancelled")
	}
	if !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("error = %q, want the cancelled status named", err)
	}
}

// Every other client reports how its turn ended; the Responses path left the
// stop reason empty. Text turns end, tool turns continue — the same
// vocabulary the Chat path normalizes to.
func TestMetaResponses_StopReasonParity(t *testing.T) {
	text := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(metaResponsesOKReply))
	}))
	defer text.Close()

	resp, err := newTestCompatClient(t, ProviderMeta, text.URL).CompleteWithToolResults(
		context.Background(), "sys", metaResponsesHistory(), nil)
	if err != nil {
		t.Fatalf("text turn: %v", err)
	}
	if resp.StopReason != "end_turn" {
		t.Errorf("text turn StopReason = %q, want end_turn", resp.StopReason)
	}

	calls := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"r2","status":"completed","model":"m","output":[` +
			`{"id":"fc1","type":"function_call","status":"completed",` +
			`"name":"read_file","arguments":"{\"path\":\"a.go\"}","call_id":"call_1"}]}`))
	}))
	defer calls.Close()

	resp, err = newTestCompatClient(t, ProviderMeta, calls.URL).CompleteWithToolResults(
		context.Background(), "sys", metaResponsesHistory(),
		[]ToolDefinition{{Name: "read_file", Description: "r"}})
	if err != nil {
		t.Fatalf("tool turn: %v", err)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("tool turn StopReason = %q, want tool_use", resp.StopReason)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Input["path"] != "a.go" {
		t.Errorf("ToolCalls = %+v, want the parsed read_file call", resp.ToolCalls)
	}
}
