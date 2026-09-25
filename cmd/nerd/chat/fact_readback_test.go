package chat

import (
	"testing"

	"codenerd/internal/core"
)

// execution_result declares Success /name, and a kernel read-back renders
// /true as the string "/true". parseBool compared against "true", so every
// delegated execution the chat summarised read as a failure.
func TestParseExecutionResults_WhenSuccessReadsBackAsName_ShouldBeTrue(t *testing.T) {
	t.Parallel()
	got := parseExecutionResults([]core.Fact{
		{Predicate: "execution_result", Args: []any{"a1", "/read_file", "main.go", "/true", "ok", int64(1)}},
		{Predicate: "execution_result", Args: []any{"a2", "/exec_cmd", "go test", "/false", "FAIL", int64(2)}},
	})
	if len(got) != 2 || !got[0].Success || got[1].Success {
		t.Fatalf("parsed = %+v, want the first a success and the second a failure", got)
	}
}

// focus_resolution's confidence is an integer percent. Read as a float64 it
// was always zero, and multiplied by 100 besides.
func TestFormatFocusResolution_ShouldRenderTheIntegerPercent(t *testing.T) {
	t.Parallel()
	line, ok := formatFocusResolution(core.Fact{
		Predicate: "focus_resolution",
		Args:      []any{"the parser", "internal/world/parse.go", "Parse", int64(85)},
	})
	if !ok || line != "'the parser' -> internal/world/parse.go (85%)" {
		t.Fatalf("formatFocusResolution = %q, %v", line, ok)
	}
}
