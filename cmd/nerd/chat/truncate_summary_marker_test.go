package chat

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// truncateSummary's output is not private. It becomes the fifth argument of
// shard_result/5 and the body of pending_test and pending_fix, and kernel
// facts are what a later turn's injectable_context renders into the window.
// Cut at 200 characters with a bare "...", a reviewer's return reaches the
// model as a complete short answer whose author happened to trail off.
func TestTruncateSummary_MarksTheCut(t *testing.T) {
	long := "REVIEWER FOUND: " + strings.Repeat("a finding the next shard has to act on. ", 40)

	got := truncateSummary(long, 200)

	if !types.IsClamped(got) {
		t.Fatalf("cut %d chars with no marker: %q", len(long)-len(got), got)
	}
	if !strings.HasPrefix(got, "REVIEWER FOUND:") {
		t.Errorf("the head was lost: %q", got)
	}
	if strings.Contains(got, "\n") {
		t.Errorf("a fact argument must stay on one line: %q", got)
	}

	// Short input is untouched: a marker on a complete summary is as costly a
	// lie as a missing one on an incomplete one.
	if got := truncateSummary("done", 200); got != "done" {
		t.Errorf("a short summary was altered: %q", got)
	}
	// Newlines are still flattened, which is what the fact store needs.
	if got := truncateSummary("two\nlines", 200); got != "two lines" {
		t.Errorf("flattening changed: %q", got)
	}
}
