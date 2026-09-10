package broker

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// TestCountedAtOutboundBoundary is the property that makes the receipt worth
// trusting.
//
// Content is routinely appended after a subsystem has finished its own
// arithmetic: cmd/nerd/chat concatenates a persona onto the kernel's
// final_system_prompt, the prompt assembler expands templates after budget
// fitting, tool schemas are serialized last. Counting anywhere but the outbound
// boundary means counting something other than what was sent.
func TestCountedAtOutboundBoundary(t *testing.T) {
	meter := testMeter(1000000, 8000)
	sink := &captureSink{}
	client := meteredClient(t, newFakeClient(), meter, sink)

	base := "you are a code reviewer"
	appended := base + "\n\n" + strings.Repeat("PERSONA TEXT APPENDED AFTER COMPILATION. ", 200)

	if _, err := client.CompleteWithSystem(context.Background(), base, "review this"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	first, _ := sink.last()

	if _, err := client.CompleteWithSystem(context.Background(), appended, "review this"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	second, _ := sink.last()

	if second.Estimated.Tokens <= first.Estimated.Tokens {
		t.Errorf("text appended after compilation was not charged: %d tokens before, %d after",
			first.Estimated.Tokens, second.Estimated.Tokens)
	}
	if second.Estimated.Segments.System <= first.Estimated.Segments.System {
		t.Error("the appended text was not attributed to the system segment")
	}
}

// TestToolSchemasAreCharged covers the segment most likely to be forgotten.
// A large tool contract can be most of a request.
func TestToolSchemasAreCharged(t *testing.T) {
	meter := testMeter(1000000, 8000)
	sink := &captureSink{}
	client := meteredClient(t, newFakeClient(), meter, sink)

	tools := []types.ToolDefinition{{
		Name:        "run_command",
		Description: strings.Repeat("a long tool description. ", 100),
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"cmd": map[string]any{"type": "string", "description": strings.Repeat("x", 2000)},
			},
		},
	}}

	if _, err := client.CompleteWithTools(context.Background(), "sys", "user", nil); err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	without, _ := sink.last()

	if _, err := client.CompleteWithTools(context.Background(), "sys", "user", tools); err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	with, _ := sink.last()

	if with.Estimated.Segments.Tools <= 0 {
		t.Error("tool schemas were not attributed to the tools segment")
	}
	if with.Estimated.Tokens <= without.Estimated.Tokens {
		t.Errorf("tool schemas were not charged: %d without, %d with",
			without.Estimated.Tokens, with.Estimated.Tokens)
	}
}

// TestToolResultPayloadsAreCharged covers the other half of an agentic turn.
func TestToolResultPayloadsAreCharged(t *testing.T) {
	meter := testMeter(1000000, 8000)
	sink := &captureSink{}

	wrapped := meteredClient(t, &fakeToolResults{newFakeClient()}, meter, sink)
	trp, ok := wrapped.(types.ToolResultsProvider)
	if !ok {
		t.Fatal("wrapper lost ToolResultsProvider")
	}

	short := []types.Message{{Role: "user", Text: "go"}}
	long := []types.Message{
		{Role: "user", Text: "go"},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "t1", Name: "run"}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "t1", Content: strings.Repeat("test output line\n", 500)}}},
	}

	if _, err := trp.CompleteWithToolResults(context.Background(), "sys", short, nil); err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	before, _ := sink.last()

	if _, err := trp.CompleteWithToolResults(context.Background(), "sys", long, nil); err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	after, _ := sink.last()

	if after.Estimated.Segments.History <= before.Estimated.Segments.History {
		t.Errorf("tool result payloads were not charged to history: %d -> %d",
			before.Estimated.Segments.History, after.Estimated.Segments.History)
	}
}

// TestSchemaIsChargedOnStructuredCalls: charging only the prompts would
// under-report a structured call by the size of its schema.
func TestSchemaIsChargedOnStructuredCalls(t *testing.T) {
	meter := testMeter(1000000, 8000)
	sink := &captureSink{}

	wrapped := meteredClient(t, &fakeSchema{newFakeClient()}, meter, sink)
	sc, ok := wrapped.(interface {
		CompleteWithSchema(ctx context.Context, systemPrompt, userPrompt, jsonSchema string) (string, error)
	})
	if !ok {
		t.Fatal("wrapper lost CompleteWithSchema")
	}

	if _, err := sc.CompleteWithSchema(context.Background(), "sys", "user", "{}"); err != nil {
		t.Fatalf("CompleteWithSchema: %v", err)
	}
	small, _ := sink.last()

	big := strings.Repeat(`{"type":"object","properties":{"a":{"type":"string"}}}`, 100)
	if _, err := sc.CompleteWithSchema(context.Background(), "sys", "user", big); err != nil {
		t.Fatalf("CompleteWithSchema: %v", err)
	}
	large, _ := sink.last()

	if large.Estimated.Tokens <= small.Estimated.Tokens {
		t.Errorf("the JSON schema was not charged: %d with a tiny schema, %d with a large one",
			small.Estimated.Tokens, large.Estimated.Tokens)
	}
}

func TestConfidenceOrdering(t *testing.T) {
	if !ConfidenceExact.AtLeast(ConfidenceCalibrated) {
		t.Error("exact must outrank calibrated")
	}
	if !ConfidenceCalibrated.AtLeast(ConfidenceSeeded) {
		t.Error("calibrated must outrank seeded")
	}
	if ConfidenceSeeded.AtLeast(ConfidenceExact) {
		t.Error("seeded must not satisfy a demand for exact")
	}
	if Confidence("nonsense").AtLeast(ConfidenceSeeded) {
		t.Error("an unknown confidence must rank below every known one, not above")
	}
}

func TestPromptBudgetDerivesFromTheLedger(t *testing.T) {
	meter := NewMeter(MeterConfig{Window: 200000, OutputReserve: 8000})

	half := meter.PromptBudget(0.5, 65536)
	if want := (200000 - 8000) / 2; half != want {
		t.Errorf("PromptBudget(0.5) = %d, want %d", half, want)
	}

	// A larger configured window must produce a larger prompt allowance; that
	// is the whole point of deriving instead of hard-coding.
	bigger := NewMeter(MeterConfig{Window: 1000000, OutputReserve: 8000})
	if bigger.PromptBudget(0.5, 65536) <= half {
		t.Error("prompt budget does not scale with the configured window")
	}
}

func TestPromptBudgetFallsBackBeforeConfiguration(t *testing.T) {
	// During boot the window is not known yet. Returning zero would produce a
	// prompt with no room for its mandatory atoms.
	meter := NewMeter(MeterConfig{})
	if got := meter.PromptBudget(0.5, 65536); got != 65536 {
		t.Errorf("PromptBudget before configuration = %d, want the fallback 65536", got)
	}
}

func TestPromptBudgetRespectsFloorAndShareBounds(t *testing.T) {
	tiny := NewMeter(MeterConfig{Window: 10000, OutputReserve: 9000})
	if got := tiny.PromptBudget(0.5, 65536); got != minPromptBudget {
		t.Errorf("PromptBudget on a tiny window = %d, want the floor %d", got, minPromptBudget)
	}

	meter := NewMeter(MeterConfig{Window: 200000, OutputReserve: 0})
	full := meter.PromptBudget(1, 0)
	for _, bad := range []float64{0, -1, 2} {
		if got := meter.PromptBudget(bad, 0); got != full {
			t.Errorf("out-of-range share %v gave %d, want it clamped to the full %d", bad, got, full)
		}
	}
}

func TestRingSinkKeepsTheMostRecentReceipts(t *testing.T) {
	ring := NewRingSink(3)
	for i := 0; i < 10; i++ {
		ring.Record(Receipt{Method: string(rune('a' + i))})
	}

	got := ring.Receipts()
	if len(got) != 3 || ring.Len() != 3 {
		t.Fatalf("ring held %d receipts, want 3", len(got))
	}
	// Oldest first, and only the last three survive.
	if got[0].Method != "h" || got[2].Method != "j" {
		t.Errorf("ring order wrong: %q %q %q", got[0].Method, got[1].Method, got[2].Method)
	}
}

func TestRingSinkHandlesPartialFillAndBadSize(t *testing.T) {
	ring := NewRingSink(5)
	ring.Record(Receipt{Method: "a"})
	ring.Record(Receipt{Method: "b"})

	got := ring.Receipts()
	if len(got) != 2 || got[0].Method != "a" || got[1].Method != "b" {
		t.Errorf("partial ring returned %+v", got)
	}

	// A zero size must not silently discard every receipt.
	zero := NewRingSink(0)
	zero.Record(Receipt{Method: "x"})
	if zero.Len() != 1 {
		t.Error("a non-positive ring size discarded receipts instead of using a default")
	}
}

func TestMultiSinkToleratesNilMembers(t *testing.T) {
	capture := &captureSink{}
	sink := MultiSink{nil, capture, nil}
	sink.Record(Receipt{Method: "m"})

	if len(capture.all()) != 1 {
		t.Error("MultiSink dropped a receipt or panicked on a nil member")
	}
}
