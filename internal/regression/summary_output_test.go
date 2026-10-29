package regression

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// A failed task's output reaches the operator under the table, line-clamped
// head and tail: the table said only "expected exit 0, got 1", and the output
// that said why was in the run record the operator then had to open.
func TestFormatSummary_WhenATaskFails_ShouldShowItsOutputClampedByLine(t *testing.T) {
	t.Parallel()
	var out strings.Builder
	for i := 1; i <= 200; i++ {
		fmt.Fprintf(&out, "=== RUN   TestCase%03d\n", i)
	}
	out.WriteString("--- FAIL: TestCase200 (0.00s)\nFAIL\tcodenerd/internal/example\t0.012s\n")

	got := FormatSummary(Summary{
		Total:  2,
		Passed: 1,
		Failed: 1,
		Results: []Result{
			{TaskID: "build", Success: true, Output: "ok\n"},
			{TaskID: "tests", Success: false, Error: "expected exit 0, got 1", Output: out.String()},
		},
	})

	if !strings.Contains(got, "--- tests output ---") {
		t.Fatalf("no output block for the failed task:\n%s", got)
	}
	if strings.Contains(got, "--- build output ---") {
		t.Errorf("a passing task's output was printed:\n%s", got)
	}
	if !strings.Contains(got, "FAIL\tcodenerd/internal/example") || !strings.Contains(got, "=== RUN   TestCase001") {
		t.Errorf("the excerpt lost its head or its verdict:\n%s", got)
	}
	if !types.IsClamped(got) || strings.Contains(got, "TestCase100\n") {
		t.Errorf("the 200-line output was not clamped with a marker:\n%s", got)
	}
}
