package context

import (
	"strings"
	"testing"

	"codenerd/internal/config"
)

// newTestCompressor builds a Compressor with no kernel/store/LLM. The internal
// constructor still wires up a real token budget and counter, so the read-only
// accessors below are safe to exercise without external dependencies.
func newTestCompressor(maxTokens int) *Compressor {
	return NewCompressorWithParams(nil, nil, nil,
		maxTokens, 5, 30, 15, 50, // budget + reserve percentages
		7, 0.6, 100.0, 30.0) // recentWindow, threshold, targetRatio, activationThreshold
}

func TestCompressorParamsConstructorAndAccessors(t *testing.T) {
	c := newTestCompressor(10000)

	if got := c.GetRecentTurnWindow(); got != 7 {
		t.Errorf("GetRecentTurnWindow=%d, want 7", got)
	}
	// Fresh compressor has compressed nothing yet -> ratio is the 1.0 identity.
	if got := c.GetCompressionRatio(); got != 1.0 {
		t.Errorf("GetCompressionRatio=%v, want 1.0", got)
	}

	used, total := c.GetBudgetUsage()
	if total != 10000 {
		t.Errorf("GetBudgetUsage total=%d, want 10000", total)
	}
	if used < 0 {
		t.Errorf("GetBudgetUsage used=%d, want >= 0", used)
	}
	if u := c.GetBudgetUtilization(); u < 0 || u > 1 {
		t.Errorf("GetBudgetUtilization=%v, want in [0,1]", u)
	}
	// Nothing processed and no segments -> not yet compressing.
	if c.IsCompressionActive() {
		t.Error("IsCompressionActive should be false on a fresh compressor")
	}
}

func TestCompressorGetMetrics(t *testing.T) {
	c := newTestCompressor(8000)
	m := c.GetMetrics()
	for _, key := range []string{
		"turn_number", "recent_turns", "compressed_segments",
		"total_original_tokens", "total_compressed_tokens",
		"compression_ratio", "target_ratio",
	} {
		if _, ok := m[key]; !ok {
			t.Errorf("GetMetrics missing key %q", key)
		}
	}
	if m["compression_ratio"].(float64) != 1.0 {
		t.Errorf("initial compression_ratio=%v, want 1.0", m["compression_ratio"])
	}
	if m["target_ratio"].(float64) != 100.0 {
		t.Errorf("target_ratio=%v, want 100.0", m["target_ratio"])
	}
	if m["turn_number"].(int) != 0 {
		t.Errorf("initial turn_number=%v, want 0", m["turn_number"])
	}
}

func TestCompressorWithConfigConstructor(t *testing.T) {
	cfg := config.DefaultContextWindowConfig()
	cfg.RecentTurnWindow = 9
	c := NewCompressorWithConfig(nil, nil, nil, cfg)
	if got := c.GetRecentTurnWindow(); got != 9 {
		t.Errorf("GetRecentTurnWindow=%d, want 9 (from config)", got)
	}
	_, total := c.GetBudgetUsage()
	if total != cfg.MaxTokens {
		t.Errorf("budget total=%d, want %d (config.MaxTokens)", total, cfg.MaxTokens)
	}
}

func TestCompressorSessionIDSetter(t *testing.T) {
	c := newTestCompressor(4000)
	// Empty id is ignored (keeps the constructor-generated default).
	c.SetSessionID("")
	if c.sessionID == "" {
		t.Error("empty SetSessionID should not clear the existing session id")
	}
	c.SetSessionID("sess_custom")
	if c.sessionID != "sess_custom" {
		t.Errorf("sessionID=%q, want sess_custom", c.sessionID)
	}
}

func TestCompressorTrimToTokens(t *testing.T) {
	c := newTestCompressor(10000)

	// maxTokens <= 0 short-circuits to a trimmed copy.
	if got := c.trimToTokens("  spaced  ", 0); got != "spaced" {
		t.Errorf("trimToTokens(_,0)=%q, want \"spaced\"", got)
	}
	// Under budget -> returned trimmed but otherwise intact.
	if got := c.trimToTokens("  hi there  ", 1000); got != "hi there" {
		t.Errorf("trimToTokens under budget=%q, want \"hi there\"", got)
	}
	// Over budget -> result is strictly shorter and fits the token budget.
	long := strings.Repeat("alpha beta gamma delta ", 200)
	trimmed := c.trimToTokens(long, 5)
	if len(trimmed) >= len(long) {
		t.Errorf("trimToTokens should shorten oversized input (len %d -> %d)", len(long), len(trimmed))
	}
	if got := c.counter.CountString(trimmed); got > 5 {
		t.Errorf("trimmed token count=%d, want <= 5", got)
	}
}
