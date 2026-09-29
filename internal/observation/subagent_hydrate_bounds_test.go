package observation

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/retain"
)

// Limits cleanup 2026-09-29: hydration bounds lived only as Go constants.
// Callers that resolve the observation.* policy pass their values in through
// ReturnWindow now; the constants stay as the leaf safety net for direct API
// callers, the way DefaultReadLimits backs the file-read codec.
func hydrateBoundsTranscript(lines int) string {
	numbered := make([]string, lines)
	for i := range numbered {
		numbered[i] = fmt.Sprintf("finding line %04d", i)
	}
	return strings.Join(numbered, "\n")
}

func hydrateBoundsHandle(t *testing.T, c *Subagents, lines int) string {
	t.Helper()
	encoded := c.EncodeReturn(Return{Agent: "coder", Output: hydrateBoundsTranscript(lines)}, ReturnLimits{})
	if encoded.Handle == "" {
		t.Fatalf("a %d-line return published no handle, so its transcript is unreachable", lines)
	}
	return encoded.Handle
}

func TestHydrateReturn_WhenCallerSuppliesMaxLines_ShouldHonorThem(t *testing.T) {
	t.Parallel()
	c := NewSubagents(retain.DefaultConfig())
	handle := hydrateBoundsHandle(t, c, 300)

	hydrated, err := c.HydrateReturn(handle, ReturnWindow{Limit: 300, MaxLines: 300})
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	if len(hydrated.Lines) != 300 || hydrated.Total != 300 || hydrated.NextOffset != 0 {
		t.Fatalf("got %d of %d lines next=%d; the caller's resolved cap must win over the leaf %d",
			len(hydrated.Lines), hydrated.Total, hydrated.NextOffset, maxReturnHydrateLines)
	}
}

func TestHydrateReturn_WhenCallerSuppliesDefaultLines_ShouldUseThemForUnbounded(t *testing.T) {
	t.Parallel()
	c := NewSubagents(retain.DefaultConfig())
	handle := hydrateBoundsHandle(t, c, 100)

	hydrated, err := c.HydrateReturn(handle, ReturnWindow{DefaultLines: 100})
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	if len(hydrated.Lines) != 100 || hydrated.NextOffset != 0 {
		t.Fatalf("got %d lines next=%d; an unbounded request must get the caller's default, not the leaf %d",
			len(hydrated.Lines), hydrated.NextOffset, defaultReturnHydrateLines)
	}
}

func TestHydrateReturn_WhenBoundsAreUnset_ShouldKeepLeafFallback(t *testing.T) {
	t.Parallel()
	c := NewSubagents(retain.DefaultConfig())
	handle := hydrateBoundsHandle(t, c, 100)

	hydrated, err := c.HydrateReturn(handle, ReturnWindow{})
	if err != nil {
		t.Fatalf("hydrate: %v", err)
	}
	if len(hydrated.Lines) != defaultReturnHydrateLines || hydrated.NextOffset != defaultReturnHydrateLines {
		t.Fatalf("got %d lines next=%d, want the leaf default of %d: the seam must not move existing callers",
			len(hydrated.Lines), hydrated.NextOffset, defaultReturnHydrateLines)
	}
}
