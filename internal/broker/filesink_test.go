package broker

import (
	"path/filepath"
	"testing"
	"time"
)

func TestReceiptSurvivesTheRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "meter", "receipts.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}

	want := Receipt{
		Purpose:  PurposeCompression,
		Provider: "anthropic",
		Model:    "claude-opus-5",
		Method:   "CompleteWithTools",
		Started:  time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC),
		Duration: 1234 * time.Millisecond,
		Scope:    "sess-1",
		Prefix:   "deadbeef",
		Estimated: Count{
			Tokens:     4096,
			Segments:   Segments{System: 2048, History: 1024, User: 512, Tools: 512},
			Confidence: ConfidenceExact,
			Source:     "anthropic.count_tokens",
			Model:      "claude-opus-5",
		},
		Actual:           Spend{InputTokens: 4100, OutputTokens: 900, CachedTokens: 3000, ThinkingTokens: 200, Calls: 1},
		EstimateErrorPct: -0.0976,
		Decision:         Decision{Allowed: true, Code: DecisionAdmitted, Window: 200000, Headroom: 195904},
		Err:              "",
	}
	sink.Record(want)
	if err := sink.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	got, truncated, err := ReadReceiptLog(path)
	if err != nil {
		t.Fatalf("ReadReceiptLog: %v", err)
	}
	if truncated != 0 || len(got) != 1 {
		t.Fatalf("got %d receipts (%d truncated), want 1", len(got), truncated)
	}

	// Field-by-field rather than DeepEqual on purpose: a field silently lost to
	// a missing json tag is exactly the failure that makes a readout wrong in a
	// way nobody notices, and the segments and the new scope/prefix pair are the
	// ones the epoch analysis depends on.
	g := got[0]
	if g.Purpose != want.Purpose || g.Provider != want.Provider || g.Model != want.Model || g.Method != want.Method {
		t.Errorf("identity fields differ: %+v", g)
	}
	if !g.Started.Equal(want.Started) {
		t.Errorf("started = %v, want %v", g.Started, want.Started)
	}
	if g.Duration != want.Duration {
		t.Errorf("duration = %v, want %v", g.Duration, want.Duration)
	}
	if g.Scope != want.Scope || g.Prefix != want.Prefix {
		t.Errorf("scope/prefix = %q/%q, want %q/%q — epoch segmentation reads both",
			g.Scope, g.Prefix, want.Scope, want.Prefix)
	}
	if g.Estimated != want.Estimated {
		t.Errorf("estimated = %+v, want %+v", g.Estimated, want.Estimated)
	}
	if g.Actual != want.Actual {
		t.Errorf("actual = %+v, want %+v", g.Actual, want.Actual)
	}
	if g.Decision.Allowed != want.Decision.Allowed || g.Decision.Code != want.Decision.Code ||
		g.Decision.Window != want.Decision.Window || g.Decision.Headroom != want.Decision.Headroom {
		t.Errorf("decision = %+v, want %+v", g.Decision, want.Decision)
	}
}

func TestFileSinkRecordsRefusalsToo(t *testing.T) {
	path := filepath.Join(t.TempDir(), "receipts.jsonl")
	sink, err := NewFileSink(path)
	if err != nil {
		t.Fatalf("NewFileSink: %v", err)
	}

	// A refusal is the receipt an operator must not have to go looking for: it
	// means a turn did not happen. If the disk log dropped them, the one thing
	// worth reading back would be the one thing missing.
	sink.Record(Receipt{
		Purpose:  PurposeSession,
		Provider: "anthropic",
		Decision: Decision{Allowed: false, Code: DecisionWindowExceeded, Reason: "too big"},
	})
	_ = sink.Close()

	got, _, err := ReadReceiptLog(path)
	if err != nil {
		t.Fatalf("ReadReceiptLog: %v", err)
	}
	if len(got) != 1 || got[0].Decision.Allowed {
		t.Fatalf("refusal not recorded: %+v", got)
	}
	if got[0].Decision.Reason != "too big" {
		t.Fatalf("refusal reason lost: %+v", got[0].Decision)
	}
}

func TestNilFileSinkIsInert(t *testing.T) {
	// A workspace that cannot be written to still runs the agent, so the boot
	// path may hold a nil sink. Recording into one must not panic.
	var s *FileSink
	s.Record(Receipt{})
	s.SetMaxBytes(10)
	if n, err := s.Failures(); err != nil || n != 0 {
		t.Fatalf("nil sink reported (%d, %v)", n, err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("nil sink Close: %v", err)
	}
}
