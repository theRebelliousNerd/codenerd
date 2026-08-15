package autopoiesis

import (
	"encoding/json"
	"testing"
)

// The generated wrapper emits `{"output": <raw json>}`. Reading that back as a
// Go string silently worked only for tools whose return value was NOT valid
// JSON, because the wrapper marshals those into a JSON string. Every other
// tool — anything returning a count, a bool, or a JSON document — registered
// successfully and then failed on its first call.
func TestDecodeToolOutput_WhenWrapperEmitsNonStringJSON_ShouldRenderItAsText(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{"marshalled plain text", `"processed: hi"`, "processed: hi"},
		{"bare number", `3`, "3"},
		{"bare bool", `true`, "true"},
		{"json object passthrough", `{"lines":3}`, `{"lines":3}`},
		{"json array passthrough", `[1,2,3]`, `[1,2,3]`},
		{"null", `null`, ""},
		{"empty", ``, ""},
		{"string containing json", `"{\"a\":1}"`, `{"a":1}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := decodeToolOutput(json.RawMessage(tt.raw)); got != tt.want {
				t.Errorf("decodeToolOutput(%s) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}
