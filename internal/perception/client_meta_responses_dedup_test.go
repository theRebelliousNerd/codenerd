package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"codenerd/internal/types"
)

// countItemsByTypeAndCallID counts how many items of given type carry a given call_id.
// Items are map[string]any with keys "type" and "call_id" as produced by metaFunctionCallItem / metaFunctionOutputItem.
func countItemsByTypeAndCallID(items []any, wantType, callID string) int {
	n := 0
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		t, _ := m["type"].(string)
		if t != wantType {
			continue
		}
		cid, _ := m["call_id"].(string)
		if cid == callID {
			n++
		}
	}
	return n
}

func countItemsByType(items []any, wantType string) int {
	n := 0
	for _, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if m["type"] == wantType {
			n++
		}
	}
	return n
}

// Same ToolUseID appearing in two separate messages must be emitted only once.
// Before the wire-boundary de-duplication fix this produced two function_call_output items
// with identical call_id, which Meta rejects with HTTP 400 Duplicate function_call_output.
func TestMetaInputFromHistory_DeduplicatesFunctionCallOutput(t *testing.T) {
	history := []types.Message{
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "call_1", Name: "read_file", Input: map[string]any{"path": "a.go"}}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "req_vet", Content: "ok"}}},
		{Role: "assistant", Text: "second turn"},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "req_vet", Content: "ok"}}},
	}
	items := metaInputFromHistory("", history)
	got := countItemsByTypeAndCallID(items, "function_call_output", "req_vet")
	if got != 1 {
		t.Fatalf("expected exactly 1 function_call_output for call_id req_vet, got %d; items=%#v", got, items)
	}
}

// Two distinct call_ids must both survive — de-duplication must not over-filter.
func TestMetaInputFromHistory_DistinctOutputsBothSurvive(t *testing.T) {
	history := []types.Message{
		{Role: "assistant", ToolCalls: []types.ToolCall{
			{ID: "call_1", Name: "read_file"},
			{ID: "call_2", Name: "write_file"},
		}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "call_1", Content: "a"}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "call_2", Content: "b"}}},
	}
	items := metaInputFromHistory("", history)
	if got := countItemsByTypeAndCallID(items, "function_call_output", "call_1"); got != 1 {
		t.Fatalf("expected 1 output for call_1, got %d", got)
	}
	if got := countItemsByTypeAndCallID(items, "function_call_output", "call_2"); got != 1 {
		t.Fatalf("expected 1 output for call_2, got %d", got)
	}
	if total := countItemsByType(items, "function_call_output"); total != 2 {
		t.Fatalf("expected 2 total function_call_output items, got %d", total)
	}
}

// Duplicated assistant function_call ids must also be de-duplicated — Meta requires the pair to be one-to-one.
func TestMetaInputFromHistory_DeduplicatesFunctionCall(t *testing.T) {
	history := []types.Message{
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "dup_call", Name: "read_file", Input: map[string]any{"path": "x"}}}},
		{Role: "user", Text: "intermediate"},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "dup_call", Name: "read_file", Input: map[string]any{"path": "x"}}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "dup_call", Content: "ok"}}},
	}
	items := metaInputFromHistory("", history)
	got := countItemsByTypeAndCallID(items, "function_call", "dup_call")
	if got != 1 {
		t.Fatalf("expected exactly 1 function_call for call_id dup_call, got %d; items=%#v", got, items)
	}
}

// Ordering for non-duplicates must be preserved and duplicates must not affect distinct ids interleaved.
func TestMetaInputFromHistory_DedupPreservesOrderAndDistinct(t *testing.T) {
	history := []types.Message{
		{Role: "assistant", ToolCalls: []types.ToolCall{
			{ID: "a", Name: "tool_a"},
			{ID: "b", Name: "tool_b"},
		}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "a", Content: "out a"}}},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "a", Name: "tool_a"}}}, // duplicate of a
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "b", Content: "out b"}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "a", Content: "duplicate out a"}}}, // duplicate output
	}
	items := metaInputFromHistory("", history)
	if got := countItemsByType(items, "function_call"); got != 2 {
		t.Fatalf("expected 2 distinct function_call items (a,b), got %d", got)
	}
	if got := countItemsByType(items, "function_call_output"); got != 2 {
		t.Fatalf("expected 2 distinct function_call_output items (a,b), got %d", got)
	}
}

// scriptedMetaReply writes one Responses reply whose output array is exactly
// the caller-supplied items (already-encoded JSON objects).
func scriptedMetaReply(t *testing.T, w http.ResponseWriter, id, items string) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write([]byte(`{"id":"` + id + `","status":"completed","model":"muse-spark","output":[` + items + `],"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}`))
}

func metaReasoningItemJSON(id, enc string) string {
	return `{"id":"` + id + `","type":"reasoning","summary":[],"encrypted_content":"` + enc + `"}`
}

func metaCallItemJSON(callID string) string {
	return `{"type":"function_call","call_id":"` + callID + `","name":"read_file","arguments":"{}"}`
}

// duplicateReasoningID reports the first reasoning id repeated inside one
// Responses request body, or "" when every reasoning id is unique.
func duplicateReasoningID(body map[string]any) string {
	raw, _ := body["input"].([]any)
	seen := make(map[string]struct{})
	for _, item := range raw {
		m, _ := item.(map[string]any)
		if m == nil || m["type"] != "reasoning" {
			continue
		}
		id, _ := m["id"].(string)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			return id
		}
		seen[id] = struct{}{}
	}
	return ""
}

func reasoningIDsIn(body map[string]any) []string {
	var out []string
	raw, _ := body["input"].([]any)
	for _, item := range raw {
		m, _ := item.(map[string]any)
		if m == nil || m["type"] != "reasoning" {
			continue
		}
		id, _ := m["id"].(string)
		out = append(out, id)
	}
	return out
}

// TestMetaResponses_SlidingWindowDoesNotDuplicateReasoning replays F-META-1:
// the working context re-windows the transcript every round, so a block-built
// assistant turn whose reply carried no reasoning used to pull the previous
// turn's reasoning out of the index-keyed side cache and send it twice,
// which Meta rejects with HTTP 400 "Duplicate item found with id rs_...".
// The scripted server answers 400 on any repeated reasoning id, exactly like
// the vendor, so all four turns must succeed for this to pass.
func TestMetaResponses_SlidingWindowDoesNotDuplicateReasoning(t *testing.T) {
	var step atomic.Int64
	var requests []map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode responses request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		requests = append(requests, body)
		if dup := duplicateReasoningID(body); dup != "" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte("{\"message\":\"Duplicate item found with id `" + dup + "`. Remove duplicate items from your input and try again.\"}"))
			return
		}
		switch n := step.Add(1); n {
		case 1:
			scriptedMetaReply(t, w, "resp-0", metaReasoningItemJSON("rs_X0", "enc_X0")+","+metaCallItemJSON("call_0"))
		case 2:
			scriptedMetaReply(t, w, "resp-1", metaReasoningItemJSON("rs_X1", "enc_X1")+","+metaCallItemJSON("call_1"))
		case 3:
			scriptedMetaReply(t, w, "resp-2", metaCallItemJSON("call_2"))
		case 4:
			scriptedMetaReply(t, w, "resp-3", metaReasoningItemJSON("rs_X3", "enc_X3")+","+metaCallItemJSON("call_3"))
		default:
			t.Errorf("unexpected request %d", n)
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer srv.Close()

	client := newTestCompatClient(t, ProviderMeta, srv.URL)
	ctx := context.Background()
	anchor := types.NewUserMessage(types.TextBlock("anchor"))

	r0, err := client.CompleteWithToolResults(ctx, "A", []types.Message{anchor}, nil)
	if err != nil {
		t.Fatalf("call 1: %v", err)
	}
	if len(r0.ToolCalls) != 1 || r0.ToolCalls[0].ID != "call_0" {
		t.Fatalf("call 1 reply calls = %+v, want [call_0]", r0.ToolCalls)
	}
	res0 := types.NewUserMessage(types.ToolResultBlock("call_0", "ok", false))

	r1, err := client.CompleteWithToolResults(ctx, "A", []types.Message{anchor, types.AssistantMessageFrom(r0), res0}, nil)
	if err != nil {
		t.Fatalf("call 2: %v", err)
	}
	if len(r1.ToolCalls) != 1 || r1.ToolCalls[0].ID != "call_1" {
		t.Fatalf("call 2 reply calls = %+v, want [call_1]", r1.ToolCalls)
	}
	res1 := types.NewUserMessage(types.ToolResultBlock("call_1", "ok", false))

	r2, err := client.CompleteWithToolResults(ctx, "B", []types.Message{anchor, types.AssistantMessageFrom(r0), res0, types.AssistantMessageFrom(r1), res1}, nil)
	if err != nil {
		t.Fatalf("call 3: %v", err)
	}
	if len(r2.ToolCalls) != 1 || r2.ToolCalls[0].ID != "call_2" {
		t.Fatalf("call 3 reply calls = %+v, want [call_2]", r2.ToolCalls)
	}
	res2 := types.NewUserMessage(types.ToolResultBlock("call_2", "ok", false))

	// The slid window: the anchor plus only the last two turns, so the
	// reasoning-less r2 sits at the index that used to address r1's cache
	// slot and must NOT pull r1's rs_X1 out of the side cache.
	r3, err := client.CompleteWithToolResults(ctx, "A", []types.Message{
		anchor,
		types.AssistantMessageFrom(r1),
		res1,
		types.AssistantMessageFrom(r2),
		res2,
	}, nil)
	if err != nil {
		t.Fatalf("call 4 (slid window): %v", err)
	}
	if len(r3.ToolCalls) != 1 || r3.ToolCalls[0].ID != "call_3" {
		t.Fatalf("call 4 reply calls = %+v, want [call_3]", r3.ToolCalls)
	}

	if len(requests) != 4 {
		t.Fatalf("server saw %d requests, want 4", len(requests))
	}
	if got := reasoningIDsIn(requests[3]); len(got) != 1 || got[0] != "rs_X1" {
		t.Fatalf("4th request reasoning ids = %v, want exactly [rs_X1]", got)
	}
}

// The same block-built assistant turn (with a signed thinking block)
// appearing twice in history must yield one reasoning item — Meta rejects
// a repeated reasoning id with HTTP 400 Duplicate item.
func TestMetaInputFromHistory_DeduplicatesReasoningID(t *testing.T) {
	thinking := types.ThinkingBlock("thinking", "enc_dup")
	thinking.ID = "rs_dup"
	turn := types.NewAssistantMessage(thinking, types.ToolUseBlock("call_dup", "read_file", map[string]any{"path": "x"}))
	history := []types.Message{turn, turn}
	items := metaInputFromHistory("", history)
	if got := countItemsByType(items, "reasoning"); got != 1 {
		t.Fatalf("expected exactly 1 reasoning item for rs_dup, got %d; items=%#v", got, items)
	}
}
