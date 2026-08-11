package perception

import (
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
	items := metaInputFromHistory("", history, nil)
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
	items := metaInputFromHistory("", history, nil)
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
	items := metaInputFromHistory("", history, nil)
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
	items := metaInputFromHistory("", history, nil)
	if got := countItemsByType(items, "function_call"); got != 2 {
		t.Fatalf("expected 2 distinct function_call items (a,b), got %d", got)
	}
	if got := countItemsByType(items, "function_call_output"); got != 2 {
		t.Fatalf("expected 2 distinct function_call_output items (a,b), got %d", got)
	}
}
