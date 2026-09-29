package testfacts

import (
	"strings"
	"testing"
)

const floodSrc = `package fl

import (
	"fmt"
	"testing"
)

func TestFlood(t *testing.T) {
	for i := 0; i < 10000; i++ {
		fmt.Println("flood line standing by")
	}
}
`

// The motivating failure mode: one line printed 10,000 times (produced
// here by a loop at test time, never pasted into a file) collapses to a
// single Repeat with its count, while every line stays in the outputs.
func TestRepeatFlood(t *testing.T) {
	dir := writeModule(t, map[string]string{"fl_test.go": floodSrc})
	res := parseString(t, runGoTestJSON(t, dir, "."))
	if res.Status != StatusPass {
		t.Fatalf("Status = %q, want pass", res.Status)
	}
	var flood *Repeat
	for i := range res.Repeats {
		if res.Repeats[i].Line == "flood line standing by" {
			flood = &res.Repeats[i]
		}
	}
	if flood == nil {
		t.Fatalf("no repeat for the flood line: %+v", res.Repeats)
	}
	if flood.Count != 10000 {
		t.Fatalf("flood count = %d, want 10000", flood.Count)
	}
	// Nothing dropped: all 10,000 occurrences stay across the outputs.
	kept := 0
	for _, p := range res.Packages {
		kept += countLine(p.Output, "flood line standing by")
		for _, ct := range p.Tests {
			kept += countLine(ct.Output, "flood line standing by")
		}
	}
	if kept != 10000 {
		t.Fatalf("kept occurrences = %d, want 10000", kept)
	}
	// The summary names the repeat once with its count and never inlines
	// the body.
	summary := res.Summary()
	if !strings.Contains(summary, "repeated 10000x: flood line standing by") {
		t.Errorf("summary missing the repeat:\n%s", summary)
	}
	if got := strings.Count(summary, "flood line standing by"); got != 1 {
		t.Errorf("flood line appears %d times in summary, want once", got)
	}
}

func countLine(lines []string, want string) int {
	n := 0
	for _, line := range lines {
		if line == want {
			n++
		}
	}
	return n
}
