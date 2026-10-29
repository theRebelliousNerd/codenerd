package session

import (
	"strings"
	"testing"
	"unicode/utf8"
)

// The give-up record carries each older attempt's failing output, bounded.
// A byte slice at the bound split a multi-byte character into invalid UTF-8
// and dropped the tail, where a test run says what failed.
func TestExcerpt_KeepsTheTailAndWholeCharacters(t *testing.T) {
	var b strings.Builder
	for b.Len() < 3*repairRetainedOutputCap {
		b.WriteString("--- log line with a multi-byte rune: é — ✓\n")
	}
	b.WriteString("FAIL\tcodenerd/internal/example\t0.01s\n")
	out := excerpt(b.String(), repairRetainedOutputCap)

	if !utf8.ValidString(out) {
		t.Fatal("excerpt produced invalid UTF-8")
	}
	if !strings.Contains(out, "FAIL\tcodenerd/internal/example") {
		t.Errorf("excerpt dropped the run's summary line:\n%s", out)
	}
	if len(out) > repairRetainedOutputCap+200 {
		t.Errorf("excerpt is %d bytes, want it bounded near %d", len(out), repairRetainedOutputCap)
	}
	if short := "all good"; excerpt(short, repairRetainedOutputCap) != short {
		t.Error("output under the bound must come back unchanged")
	}
}
