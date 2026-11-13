package session

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/broker"
	"codenerd/internal/types"
)

// A working request is measured by part, and the parts add up to the
// request: the payload of the attempt in flight whole, the one it
// supersedes as its pointer, the tool results, the task, and the harness's
// views and notes apart from them.
func TestRequestComposition_AttributesEveryByteOfAWorkingRequest(t *testing.T) {
	t.Parallel()
	e, ctx, history, body := oneWorkingRound(t, 200000)
	loop := activeWorkingLoop(ctx)

	first := "The tests fail:\n\n```\n" + strings.Repeat("first failure line\n", 300) + "```\n"
	second := "The tests fail:\n\n```\n" + strings.Repeat("second failure line\n", 300) + "```\n"
	if err := loop.notePayload(ctx, first); err != nil {
		t.Fatal(err)
	}
	call := types.ToolCall{ID: "call-2", Name: "read_file", Input: map[string]any{"path": "target.go", "start_line": 2}}
	if err := e.recordWorkingResult(ctx, call, "line two", nil); err != nil {
		t.Fatal(err)
	}
	history = append(history,
		types.Message{Role: "user", Text: first},
		types.Message{Role: "assistant", Text: "reading", ToolCalls: []types.ToolCall{call}},
		types.Message{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: call.ID, Content: "line two"}}},
	)
	if err := loop.notePayload(ctx, second); err != nil {
		t.Fatal(err)
	}
	history = append(history, types.Message{Role: "user", Text: second})

	const system = "SYSTEM PROMPT"
	messages, err := e.prepareWorkingRequest(ctx, system, history, nil)
	if err != nil {
		t.Fatal(err)
	}
	c := loop.composition(system, history, messages, nil)
	t.Logf("%s", c)
	if c.System != len(system) {
		t.Fatalf("system = %d, want %d", c.System, len(system))
	}
	if c.RepairPayloads != 1 || c.RetiredPayloads != 1 {
		t.Fatalf("payloads carried/retired = %d/%d, want 1/1", c.RepairPayloads, c.RetiredPayloads)
	}
	if c.RepairPayload < len(second) || c.RepairPayload >= len(second)+len(first) {
		t.Fatalf("repair payload = %d bytes: want the second (%d) whole and the first (%d) as its pointer", c.RepairPayload, len(second), len(first))
	}
	if c.ToolResults != len(body)+len("line two") {
		t.Fatalf("tool results = %d, want %d", c.ToolResults, len(body)+len("line two"))
	}
	if c.Assistant == 0 || c.Task == 0 {
		t.Fatalf("assistant %d, task %d: a part went unmeasured", c.Assistant, c.Task)
	}
	if got, want := c.Total()-c.System-c.Catalog, requestBytes(messages); got != want {
		t.Fatalf("the parts add up to %d bytes and the request is %d: %s", got, want, c)
	}
}

// compositionProvider records the composition each request's context carried
// to the provider, where the broker reads it onto the receipt.
type compositionProvider struct {
	got []broker.Composition
}

func (p *compositionProvider) CompleteWithToolResults(ctx context.Context, _ string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if c, ok := broker.CompositionFromContext(ctx); ok {
		p.got = append(p.got, c)
	}
	return &types.LLMToolResponse{Text: "done"}, nil
}

// Every working request reaches its provider -- the broker, in production --
// with its composition on the context.
func TestRequestComposition_RidesEveryWorkingRequestToTheProvider(t *testing.T) {
	t.Parallel()
	e, ctx, history, body := oneWorkingRound(t, 200000)
	p := &compositionProvider{}
	if _, err := e.completeWithWorkingContext(ctx, p, "SYSTEM", history, nil); err != nil {
		t.Fatal(err)
	}
	if len(p.got) != 1 || p.got[0].ToolResults != len(body) || p.got[0].System != len("SYSTEM") {
		t.Fatalf("compositions at the provider = %+v, want one with the round's %d result bytes", p.got, len(body))
	}
}
