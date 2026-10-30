package deliberation

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/sessionlog"
	"codenerd/internal/tools"
	"codenerd/internal/types"
)

type stubPrompts struct{}

func (stubPrompts) CompileRolePrompt(context.Context, types.RolePromptRequest) (string, error) {
	return "role prompt", nil
}

// scriptedClient answers CompleteWithTools from a script and records what each
// call offered and sent.
type scriptedClient struct {
	types.LLMClient
	replies []*types.LLMToolResponse
	offers  [][]types.ToolDefinition
	inputs  []string
}

func (c *scriptedClient) CompleteWithTools(_ context.Context, _, user string, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	c.offers = append(c.offers, defs)
	c.inputs = append(c.inputs, user)
	r := c.replies[0]
	c.replies = c.replies[1:]
	return r, nil
}

func agentFixture(t *testing.T, budget int64, output string) (types.Kernel, ReadTools, *int) {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(types.Fact{Predicate: "config_param", Args: []any{types.MangleAtom("/deliberation_tool_calls"), budget}}); err != nil {
		t.Fatal(err)
	}
	runs := 0
	reg := tools.NewRegistry()
	if err := reg.Register(&tools.Tool{Name: "read_file", Description: "read", Schema: tools.ToolSchema{Properties: map[string]tools.Property{"path": {Type: "string"}}},
		Execute: func(context.Context, map[string]any) (string, error) { runs++; return output, nil }}); err != nil {
		t.Fatal(err)
	}
	return k, ReadTools{Kernel: k, Registry: reg, ContextBytes: 4000}, &runs
}

func readCalls(n int) []types.ToolCall {
	calls := make([]types.ToolCall, n)
	for i := range calls {
		calls[i] = types.ToolCall{ID: string(rune('a' + i)), Name: "read_file", Input: map[string]any{"path": "x.go"}}
	}
	return calls
}

// A role that asks for more reads than its budget is not failed: the reads
// past the budget are refused as evidence closed, tools are withheld, and the
// role's next reply is its answer. Until 2026-10-30 the role errored with
// "exhausted its configured evidence budget" and the deliberation failed.
func TestAgent_ASpentBudgetClosesEvidenceInsteadOfFailing(t *testing.T) {
	k, read, runs := agentFixture(t, 3, "package x")
	client := &scriptedClient{replies: []*types.LLMToolResponse{{ToolCalls: readCalls(5)}, {Text: `{"recommendation":"c1"}`}}}
	ctx := sessionlog.WithAgent(context.Background(), Challenger, "t1")
	got, err := Agent(ctx, k, client, stubPrompts{}, Challenger, `{"request":"r"}`, read)
	if err != nil {
		t.Fatalf("Agent failed: %v", err)
	}
	if got != `{"recommendation":"c1"}` {
		t.Fatalf("answer = %q", got)
	}
	if *runs != 3 {
		t.Errorf("ran %d reads, want the budget of 3", *runs)
	}
	if len(client.offers) != 2 || client.offers[0] == nil || client.offers[1] != nil {
		t.Fatalf("offers = %v, want tools then none", client.offers)
	}
	if n := strings.Count(client.inputs[1], "evidence closed"); n != 2 {
		t.Errorf("%d reads were told evidence is closed, want 2: %s", n, client.inputs[1])
	}
}

// An oversized read is cut to the room left in the role's context, visibly,
// and the encoded input never exceeds the bound, even when JSON escaping
// grows the text.
func TestAgent_AnOversizedReadFitsTheContextBoundVisibly(t *testing.T) {
	k, read, _ := agentFixture(t, 6, strings.Repeat(`<a href="x">&</a>`+"\n", 1000))
	client := &scriptedClient{replies: []*types.LLMToolResponse{{ToolCalls: readCalls(2)}, {Text: "done"}}}
	ctx := sessionlog.WithAgent(context.Background(), Challenger, "t2")
	if _, err := Agent(ctx, k, client, stubPrompts{}, Challenger, `{"request":"r"}`, read); err != nil {
		t.Fatalf("Agent failed: %v", err)
	}
	for i, in := range client.inputs {
		if len(in) > read.ContextBytes {
			t.Errorf("call %d input is %d bytes, bound %d", i, len(in), read.ContextBytes)
		}
	}
	if !strings.Contains(client.inputs[1], "cut at the deliberation context bound") {
		t.Errorf("the cut is not visible: %.300s", client.inputs[1])
	}
}

// A read that fills the context, followed in the same reply by more reads,
// leaves room for the notes that refuse them: the input stays within the
// bound and the role answers. Seen live: the closing notes pushed the
// proposer over its bound.
func TestAgent_ReadsAfterAFullContextAreRefusedWithinTheBound(t *testing.T) {
	k, read, runs := agentFixture(t, 6, strings.Repeat("x", 20000))
	client := &scriptedClient{replies: []*types.LLMToolResponse{{ToolCalls: readCalls(4)}, {Text: "done"}}}
	ctx := sessionlog.WithAgent(context.Background(), Proposer, "t3")
	if _, err := Agent(ctx, k, client, stubPrompts{}, Proposer, `{"request":"r"}`, read); err != nil {
		t.Fatalf("Agent failed: %v", err)
	}
	if *runs != 1 {
		t.Errorf("ran %d reads, want only the one that fit", *runs)
	}
	if client.offers[1] != nil {
		t.Error("tools were still offered after the context filled")
	}
	for i, in := range client.inputs {
		if len(in) > read.ContextBytes {
			t.Errorf("call %d input is %d bytes, bound %d", i, len(in), read.ContextBytes)
		}
	}
}
