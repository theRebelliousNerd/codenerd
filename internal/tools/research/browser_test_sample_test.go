package research

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/browser"
	"codenerd/internal/types"
)

// Limits cleanup 2026-09-29: in view "full" the matched verification samples
// were silently cut to 3 with no marker. Full means full now; the row's
// "matched" count tells the model how many there are, and the other views
// carry no sample at all.
func TestBrowserTestAssertionSample_WhenFull_ShouldReturnEveryFact(t *testing.T) {
	manager := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	const total = 10 // past the deleted 3-sample cut
	facts := make([]types.Fact, total)
	for i := range facts {
		facts[i] = types.Fact{
			Predicate: "console_event",
			Args:      []any{"session-a", "error", fmt.Sprintf("m-%02d", i), int64(i + 1)},
		}
	}
	sample := browserTestAssertionSample(manager, facts, "full")
	if len(sample) != total {
		t.Fatalf("sample = %d facts, want all %d matched", len(sample), total)
	}
	encoded, err := json.Marshal(sample)
	if err != nil {
		t.Fatalf("marshal sample: %v", err)
	}
	for i := 0; i < total; i++ {
		if marker := fmt.Sprintf("m-%02d", i); !strings.Contains(string(encoded), marker) {
			t.Fatalf("sample lost %s; the cut is back: %s", marker, encoded)
		}
	}
}

func TestBrowserTestAssertionSample_WhenNotFull_ShouldCarryNoSample(t *testing.T) {
	manager := browser.NewSessionManagerWithSink(browser.DefaultConfig(), nil)
	facts := []types.Fact{{Predicate: "console_event", Args: []any{"session-a", "error", "m", int64(1)}}}
	for _, view := range []string{"summary", "compact", ""} {
		if got := browserTestAssertionSample(manager, facts, view); got != nil {
			t.Fatalf("view %q sample = %+v, want nil: only full carries samples", view, got)
		}
	}
	if got := browserTestAssertionSample(manager, nil, "full"); got != nil {
		t.Fatalf("empty facts sample = %+v, want nil: no row to attach it to", got)
	}
}
