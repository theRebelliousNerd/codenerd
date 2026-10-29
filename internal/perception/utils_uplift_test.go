package perception

import (
	"errors"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// TestOrDefaultTokens pins ceiling resolution: configured positive values
// win, anything else falls back to the client's documented default.
func TestOrDefaultTokens(t *testing.T) {
	if got := orDefaultTokens(500, 100); got != 500 {
		t.Errorf("orDefaultTokens(500, 100)=%d, want 500", got)
	}
	for _, n := range []int{0, -1} {
		if got := orDefaultTokens(n, 100); got != 100 {
			t.Errorf("orDefaultTokens(%d, 100)=%d, want 100", n, got)
		}
	}
}

// TestOutputTruncated pins the typed truncation error: machine-readable
// fields for the broker's restate loop, distinguishable via errors.As.
func TestOutputTruncated(t *testing.T) {
	err := outputTruncated(ProviderZAI, "glm-4", "Complete", "length", "partial…", 100, 100)
	var te *types.OutputTruncated
	if !errors.As(err, &te) {
		t.Fatalf("error %T does not unwrap to *types.OutputTruncated", err)
	}
	if te.Provider != string(ProviderZAI) || te.Model != "glm-4" || te.Method != "Complete" {
		t.Errorf("identity fields=%+v, want zai/glm-4/Complete", te)
	}
	if te.Reason != "length" || te.LimitTokens != 100 || te.OutputTokens != 100 {
		t.Errorf("limit fields=%+v, want length/100/100", te)
	}
	if te.Partial != "partial…" {
		t.Errorf("Partial=%q, want the cut text for diagnostics", te.Partial)
	}
}

// TestNewPooledScanner_ReusesBuffers pins the pool's whole point: a
// returned 64KiB buffer is handed out again instead of reallocated.
func TestNewPooledScanner_ReusesBuffers(t *testing.T) {
	s1, cleanup1 := newPooledScanner(strings.NewReader("a\n"), 1024*1024)
	for s1.Scan() {
	}
	cleanup1()
	s2, cleanup2 := newPooledScanner(strings.NewReader("b\n"), 1024*1024)
	defer cleanup2()
	for s2.Scan() {
		if got := s2.Text(); got != "b" {
			t.Errorf("second scanner read %q, want b", got)
		}
	}
	if err := s2.Err(); err != nil {
		t.Errorf("second scanner error: %v", err)
	}
}

// TestNewPooledScanner_GrowsPastPoolBuffer pins the overflow path: a line
// larger than the pooled 64KiB still scans via bufio's internal growth.
func TestNewPooledScanner_GrowsPastPoolBuffer(t *testing.T) {
	big := strings.Repeat("x", 200*1024)
	sc, cleanup := newPooledScanner(strings.NewReader(big+"\n"), 1024*1024)
	defer cleanup()
	if !sc.Scan() {
		t.Fatalf("Scan failed: %v", sc.Err())
	}
	if got := sc.Text(); got != big {
		t.Errorf("scanned %d chars, want %d", len(got), len(big))
	}
}
