package perception

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// The envelope contract used to be inferred by searching the prompt for
// "control_packet". That cannot distinguish a prompt that TEACHES the envelope
// from one that FORBIDS it, and the failure was not cosmetic: attaching the
// envelope response_format is a strict JSON schema, so the model is structurally
// required to produce an envelope no matter what the prose says.
//
// Live consequence: the campaign planner prompt was rewritten to say "Do NOT
// wrap it in a control_packet envelope". That sentence made isPiggybackPrompt
// return true, the client attached the schema, and the model returned a
// fully-formed envelope with every field at its zero value —
// overall_usefulness: 0, missing_context: "", noise_facts: [] — because the
// schema demanded them. RawPlan unmarshalled it happily with zero phases and
// the campaign silently ran a generic three-task placeholder. Five runs.
func TestIsPiggybackPrompt_ExplicitContractBeatsPromptSniffing(t *testing.T) {
	// The exact shape of the planner prompt that broke it.
	forbidding := `VII. OUTPUT PROTOCOL

Your entire reply is ONE JSON object matching the RawPlan schema.

Do NOT wrap it in a "control_packet" envelope. Do NOT emit "tool_requests",
"knowledge_requests", "surface_response" or "reasoning_trace".`

	if !isPiggybackPrompt(context.Background(), forbidding, "") {
		t.Fatal("precondition failed: the substring heuristic no longer matches a prompt that " +
			"forbids the envelope — if that is intentional, this test needs rewriting, but the " +
			"heuristic is exactly why the explicit flag exists")
	}

	ctx := types.WithStructuredOutputOnly(context.Background())
	if isPiggybackPrompt(ctx, forbidding, "") {
		t.Error("isPiggybackPrompt honoured the substring over the caller's explicit declaration; " +
			"the envelope schema would be attached to a call whose reply is parsed directly as JSON, " +
			"forcing the model to emit an envelope the parser then reads as empty")
	}
}

// Conversational shards never set the flag and must keep the envelope.
// Over-applying this would strip the tool protocol from the coding path, which
// is how those shards request tools at all.
func TestIsPiggybackPrompt_UnmarkedContextKeepsHeuristic(t *testing.T) {
	cases := []struct {
		name   string
		system string
		user   string
		want   bool
	}{
		{"teaches envelope", `Emit {"control_packet": {...}}`, "", true},
		{"surface_response", `Your "surface_response" is the user-visible text.`, "", true},
		{"user-side envelope", "", `{"control_packet": {}}`, true},
		{"user-side type name", "", "PiggybackEnvelope", true},
		{"plain prompt", "You are a helpful assistant.", "hello", false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isPiggybackPrompt(context.Background(), tc.system, tc.user); got != tc.want {
				t.Errorf("isPiggybackPrompt = %v, want %v", got, tc.want)
			}
		})
	}
}

// A nil context must not panic — Complete is called with one in older paths.
func TestIsStructuredOutputOnlyCtx_NilSafe(t *testing.T) {
	if types.IsStructuredOutputOnlyCtx(nil) {
		t.Error("nil context reported as structured-output-only")
	}
	if isPiggybackPrompt(nil, "control_packet", "") != true { //nolint:staticcheck // explicitly testing nil ctx
		t.Error("nil context should fall through to the heuristic, not suppress it")
	}
}
