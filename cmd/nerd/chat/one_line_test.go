package chat

import (
	"strings"
	"testing"
)

// A spawn event, a log line or a fact argument keeps the whole text: oneLine
// only flattens it.
func TestOneLine_FlattensWithoutCutting(t *testing.T) {
	long := strings.Repeat("research the retry policy in internal/session ", 10)
	if got := oneLine(long); got != long {
		t.Fatalf("oneLine cut a %d-char task to %d chars", len(long), len(got))
	}
	if got := oneLine("two\r\nlines"); got != "two lines" {
		t.Fatalf("oneLine(%q) = %q", "two\r\nlines", got)
	}
}
