package broker

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/types"
)

func TestMeasureAttributesEverySegment(t *testing.T) {
	req := &Request{
		Model:  "m",
		System: strings.Repeat("s", 400),
		User:   strings.Repeat("u", 200),
		Messages: []types.Message{
			{Role: "user", Text: strings.Repeat("h", 300)},
		},
		Tools: []types.ToolDefinition{
			{Name: "t", Description: strings.Repeat("d", 100), InputSchema: map[string]any{"type": "object"}},
		},
	}

	seg := measure(req)
	if seg.System < 400 || seg.User < 200 || seg.History < 300 || seg.Tools < 100 {
		t.Errorf("a segment was undercounted: %+v", seg)
	}
	if seg.Total() != seg.System+seg.History+seg.User+seg.Tools {
		t.Error("Total() does not equal the sum of its parts")
	}
}

func TestMeasureCountsToolCallsAndResults(t *testing.T) {
	// Tool payloads are the bulk of a long agentic turn. Counting only Text
	// would under-report a tool loop by almost all of it.
	bare := &Request{Model: "m", Messages: []types.Message{{Role: "assistant", Text: "x"}}}

	withPayload := &Request{Model: "m", Messages: []types.Message{
		{
			Role: "assistant", Text: "x",
			ToolCalls: []types.ToolCall{{
				ID: "id1", Name: "run", Input: map[string]any{"cmd": strings.Repeat("q", 500)},
			}},
		},
		{
			Role:        "user",
			ToolResults: []types.ToolResult{{ToolUseID: "id1", Content: strings.Repeat("r", 800)}},
		},
	}}

	if measure(withPayload).History <= measure(bare).History+1000 {
		t.Errorf("tool call and result payloads were not counted: bare=%d with=%d",
			measure(bare).History, measure(withPayload).History)
	}
}

func TestMeasureScalesWithMessageCount(t *testing.T) {
	// Framing overhead must scale with request shape so the learned ratio stays
	// stable as a conversation grows.
	one := &Request{Model: "m", Messages: []types.Message{{Role: "user", Text: "hello"}}}
	many := &Request{Model: "m"}
	for i := 0; i < 50; i++ {
		many.Messages = append(many.Messages, types.Message{Role: "user", Text: "hello"})
	}

	// Overhead must be charged per message, so 50 turns cost strictly more than
	// 50 copies of their bare text. It should also be linear in message count:
	// a superlinear framing charge would make the learned ratio drift as a
	// conversation grows, which is the failure this constant exists to avoid.
	textOnly := 50 * len("user"+"hello")
	got := measure(many).History
	if got <= textOnly {
		t.Errorf("per-message framing overhead is not charged: %d for 50 messages of %d chars of text", got, textOnly)
	}
	if want := measure(one).History * 50; got != want {
		t.Errorf("framing overhead is not linear in message count: got %d, want %d", got, want)
	}
}

func TestMeasureChargesHeavilyForUnmarshallableSchema(t *testing.T) {
	// Undercounting is the dangerous direction: a request admitted on a count
	// that omitted a tool schema entirely will be rejected by the provider
	// after transmission. An encoding failure must cost, not vanish.
	bad := &Request{Model: "m", Tools: []types.ToolDefinition{{
		Name:        "broken",
		InputSchema: map[string]any{"fn": func() {}}, // functions do not marshal
	}}}

	if got := measure(bad).Tools; got < unmarshallableSchemaChars {
		t.Errorf("unmarshallable schema charged %d chars, want at least %d", got, unmarshallableSchemaChars)
	}
}

func TestMeasureHandlesNilAndEmpty(t *testing.T) {
	if got := measure(nil); got.Total() != 0 {
		t.Errorf("measure(nil) = %+v, want zero", got)
	}
	if got := measure(&Request{Model: "m"}); got.Total() != 0 {
		t.Errorf("measure(empty) = %+v, want zero", got)
	}
}

func TestSplitProportionalAlwaysSumsToTheTotal(t *testing.T) {
	// A receipt whose parts do not add up invites exactly the doubt this
	// package exists to remove, so integer-division remainder goes somewhere
	// rather than being lost.
	for _, total := range []int{1, 7, 99, 1000, 65537} {
		chars := Segments{System: 333, History: 777, User: 111, Tools: 55}
		got := splitProportional(total, chars)
		if got.Total() != total {
			t.Errorf("split of %d summed to %d: %+v", total, got.Total(), got)
		}
	}
}

func TestSplitProportionalHandlesDegenerateInput(t *testing.T) {
	if got := splitProportional(100, Segments{}); got.Total() != 0 {
		t.Errorf("split with no characters should be zero, got %+v", got)
	}
	if got := splitProportional(0, Segments{System: 10}); got.Total() != 0 {
		t.Errorf("split of zero tokens should be zero, got %+v", got)
	}
	if got := splitProportional(-5, Segments{System: 10}); got.Total() != 0 {
		t.Errorf("split of a negative total should be zero, got %+v", got)
	}
}

func TestEstimatingCounterProducesSegmentedCounts(t *testing.T) {
	counter := NewEstimatingCounter(NewCalibrator())
	req := &Request{
		Model:  "m",
		System: strings.Repeat("s", 4000),
		User:   strings.Repeat("u", 1000),
	}

	count, err := counter.Count(context.Background(), req)
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count.Tokens <= 0 {
		t.Fatal("counter produced no tokens for real content")
	}
	if count.Confidence != ConfidenceSeeded {
		t.Errorf("confidence = %q, want seeded before any observation", count.Confidence)
	}
	if count.Segments.Total() != count.Tokens {
		t.Errorf("segments (%d) do not sum to the total (%d)", count.Segments.Total(), count.Tokens)
	}
	if count.Segments.System <= count.Segments.User {
		t.Error("a 4x larger system prompt should be attributed more tokens than the user prompt")
	}
}

func TestEstimatingCounterRejectsNilRequest(t *testing.T) {
	if _, err := NewEstimatingCounter(nil).Count(context.Background(), nil); err == nil {
		t.Error("counting a nil request must fail rather than silently return zero")
	}
}
