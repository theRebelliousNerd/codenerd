package articulation

import (
	"strings"
	"testing"
)

func TestStreamParser(t *testing.T) {
	parser := NewStreamParser()

	// Test extracting the surface_response correctly out of streaming chunks
	chunks := []string{
		`{`,
		`"control_packet": `,
		`{"intent_classification": {"category": "mutation"}`,
		`},`,
		`"surface_response": "`,
		`Hello `,
		`World\n`,
		`Here is a quote: \"test\"`,
		`"}`,
	}

	var output string
	for _, chunk := range chunks {
		output += parser.ProcessChunk(chunk)
	}

	expected := "Hello World\nHere is a quote: \"test\""
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

// TODO: TEST_GAP: [Null/Empty] Verify ProcessChunk behavior when given empty strings, nil slices (if applicable), or chunks of purely whitespace.
// TODO: TEST_GAP: [Null/Empty] Verify parser handles streams that abruptly terminate without completing the "surface_response" key or missing the closing quote.
// TODO: TEST_GAP: [Null/Empty] Verify behavior when "surface_response" contains an empty string (e.g. "surface_response": "").
// TODO: TEST_GAP: [Type Coercion] Verify handling of single quotes vs double quotes, or unquoted keys (e.g. 'surface_response', surface_response).
// TODO: TEST_GAP: [Type Coercion] Verify handling of varying whitespaces between the key, colon, and value (e.g. "surface_response"   :   ").
// TODO: TEST_GAP: [Type Coercion] Verify behavior when invalid escape sequences are encountered within the surface response string.
// TODO: TEST_GAP: [User Request Extremes] Verify performance and memory allocation when processing extremely large chunks (e.g., 50MB+ in a single ProcessChunk call).
// TODO: TEST_GAP: [User Request Extremes] Verify parser handles thousands of 1-byte chunks correctly without degradation.
// TODO: TEST_GAP: [User Request Extremes] Verify parser handles "surface_response": " string embedded inside another JSON string value (decoy marker).
// TODO: TEST_GAP: [User Request Extremes] Verify parser does not panic on deeply nested or highly malformed JSON before "surface_response".
// TODO: TEST_GAP: [State Conflicts] Verify data race safety if ProcessChunk is called concurrently from multiple goroutines on the same StreamParser instance.
// TODO: TEST_GAP: [State Conflicts] Verify state consistency if ProcessChunk is called after the surface response string has already been successfully parsed and closed.
// TODO: TEST_GAP: [State Conflicts] Verify the parser's internal buffer behavior when encountering multiple occurrences of "surface_response" in the stream.

// TestStreamParser_NoReEmitAfterClose fills the "[State Conflicts] called after
// the surface response has already been parsed and closed" gap. Once the closing
// quote is consumed, trailing chunks (a '}' arriving split from the quote, a
// newline, whitespace padding) must NOT re-detect the key and re-stream the whole
// surface. Before the terminal-state fix these produced duplicated output
// (e.g. "HiHi").
func TestStreamParser_NoReEmitAfterClose(t *testing.T) {
	p := NewStreamParser()

	var out string
	out += p.ProcessChunk(`{"control_packet":{},"surface_response":"Hi`)
	out += p.ProcessChunk(`"`) // closing quote arrives split from the brace
	out += p.ProcessChunk(`}`) // trailing chunk must NOT re-emit
	out += p.ProcessChunk("\n")
	out += p.ProcessChunk(`   `)

	if out != "Hi" {
		t.Fatalf("surface must stream exactly once; expected %q, got %q", "Hi", out)
	}
	// The full buffer must still retain the trailing padding for GetFullBuffer.
	if got := p.GetFullBuffer(); !strings.Contains(got, "}") {
		t.Errorf("GetFullBuffer should retain trailing chunks, got %q", got)
	}
}

// TestStreamParser_MultilineNoReEmitAfterClose exercises the same terminal-state
// guarantee with a multi-line surface and an escaped quote before the close,
// plus trailing JSON that itself contains quoted strings.
func TestStreamParser_MultilineNoReEmitAfterClose(t *testing.T) {
	p := NewStreamParser()

	var out string
	out += p.ProcessChunk(`{"surface_response":"Line1\nsay \"hi\"`)
	out += p.ProcessChunk(`"`)
	out += p.ProcessChunk(`,"trailing":"junk"}`)

	expected := "Line1\nsay \"hi\""
	if out != expected {
		t.Fatalf("expected %q emitted once, got %q", expected, out)
	}
}
