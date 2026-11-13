package broker

import (
	"context"
	"fmt"
)

// Composition is one request's input by part, as the caller that assembled
// it measured it: UTF-8 bytes of what each part puts in the request, not
// tokens. The receipt's Estimated and Actual are the token totals; this says
// which part they went to. Live 2026-11-12 one self-repair fix turn sent
// 1,631,694 input tokens over 17 model calls and nothing could say what they
// were.
type Composition struct {
	// System is the compiled system prompt (the JIT prompt).
	System int `json:"system"`
	// Catalog is the tool definitions.
	Catalog int `json:"catalog"`
	// Prior is the conversation's earlier turns, replayed.
	Prior int `json:"prior"`
	// Task is the anchor: the request as given, with its retrieval brief.
	Task int `json:"task"`
	// Assistant is the loop's assistant turns: their text and tool calls.
	Assistant int `json:"assistant"`
	// Reasoning is the replayed reasoning of those turns.
	Reasoning int `json:"reasoning"`
	// ToolResults is what the tools answered, as carried (an archived result
	// counts its pointer).
	ToolResults int `json:"tool_results"`
	// Views are the focus-view snapshots the working ledger appended.
	Views int `json:"views"`
	// Notes are the harness's other text: nudges, restatements, pointers.
	Notes int `json:"notes"`
	// RepairPayload is the repair rounds' prompts: failing output, the tests
	// and the diff they were shown (a retired one counts its pointer).
	RepairPayload int `json:"repair_payload"`
	// RepairPayloads and RetiredPayloads count the repair prompts carried
	// whole and those carried as a recall pointer.
	RepairPayloads  int `json:"repair_payloads,omitempty"`
	RetiredPayloads int `json:"retired_payloads,omitempty"`
}

// Total is the request's bytes over every part.
func (c Composition) Total() int {
	return c.System + c.Catalog + c.Prior + c.Task + c.Assistant + c.Reasoning + c.ToolResults + c.Views + c.Notes + c.RepairPayload
}

// Plus is c and o summed part by part.
func (c Composition) Plus(o Composition) Composition {
	return Composition{
		System: c.System + o.System, Catalog: c.Catalog + o.Catalog, Prior: c.Prior + o.Prior, Task: c.Task + o.Task,
		Assistant: c.Assistant + o.Assistant, Reasoning: c.Reasoning + o.Reasoning, ToolResults: c.ToolResults + o.ToolResults,
		Views: c.Views + o.Views, Notes: c.Notes + o.Notes, RepairPayload: c.RepairPayload + o.RepairPayload,
		RepairPayloads: c.RepairPayloads + o.RepairPayloads, RetiredPayloads: c.RetiredPayloads + o.RetiredPayloads,
	}
}

// String names every part, in bytes.
func (c Composition) String() string {
	return fmt.Sprintf("bytes total=%d system=%d catalog=%d prior=%d task=%d assistant=%d reasoning=%d tool_results=%d views=%d notes=%d repair_payload=%d (%d carried, %d retired)",
		c.Total(), c.System, c.Catalog, c.Prior, c.Task, c.Assistant, c.Reasoning, c.ToolResults, c.Views, c.Notes, c.RepairPayload, c.RepairPayloads, c.RetiredPayloads)
}

type compositionKey struct{}

// WithComposition tags the one request made under ctx with what it is made
// of; the receipt records it (Receipt.Composition).
func WithComposition(ctx context.Context, c Composition) context.Context {
	if ctx == nil {
		return ctx
	}
	return context.WithValue(ctx, compositionKey{}, c)
}

// CompositionFromContext returns the composition tagged on ctx, if any.
func CompositionFromContext(ctx context.Context) (Composition, bool) {
	if ctx == nil {
		return Composition{}, false
	}
	c, ok := ctx.Value(compositionKey{}).(Composition)
	return c, ok
}
