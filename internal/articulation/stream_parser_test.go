package articulation

import (
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
