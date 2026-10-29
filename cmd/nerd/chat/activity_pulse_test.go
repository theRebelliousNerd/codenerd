package chat

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"codenerd/internal/transparency"
)

func TestLivePulseBar_MovesWithElapsed(t *testing.T) {
	a := livePulseBar(0, 12)
	b := livePulseBar(200*time.Millisecond, 12)
	if utf8.RuneCountInString(a) != 12 || utf8.RuneCountInString(b) != 12 {
		t.Fatalf("expected width 12, got %d and %d", utf8.RuneCountInString(a), utf8.RuneCountInString(b))
	}
	if a == b {
		// 200ms / 80ms = 2 steps — should shift
		t.Fatalf("pulse bar should move over time: %q == %q", a, b)
	}
	// Must contain the bright peak character.
	if !strings.Contains(a, "█") {
		t.Fatalf("pulse bar missing peak: %q", a)
	}
}

func TestFormatElapsedShort(t *testing.T) {
	if got := formatElapsedShort(1500 * time.Millisecond); got != "1.5s" {
		t.Fatalf("got %q want 1.5s", got)
	}
	if got := formatElapsedShort(65 * time.Second); got != "1m05s" {
		t.Fatalf("got %q want 1m05s", got)
	}
}

func TestPushActivityPulse_TrailAndDedup(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	m.glassBoxEnabled = true

	m.pushActivityPulse(activityPulse{
		Summary:  "Spawning coder",
		Category: transparency.CategoryShard,
		At:       time.Now(),
	})
	m.pushActivityPulse(activityPulse{
		Summary:  "Spawning coder", // dedup
		Category: transparency.CategoryShard,
		At:       time.Now(),
	})
	m.pushActivityPulse(activityPulse{
		Summary:  "Intent: /fix",
		Category: transparency.CategoryPerception,
		At:       time.Now(),
	})

	if len(m.activityTrail) != 2 {
		t.Fatalf("expected 2 trail entries after dedup, got %d", len(m.activityTrail))
	}
	if m.activityTrail[0].Summary != "Intent: /fix" {
		t.Fatalf("newest should be first, got %q", m.activityTrail[0].Summary)
	}
}

func TestRenderActivityLine_LivePanel(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	m.glassBoxEnabled = true
	m.isLoading = true
	m.beginLiveTurn("Thinking...")
	m.pushActivityPulse(activityPulse{
		Summary:  "Spawning coder",
		Category: transparency.CategoryShard,
		At:       time.Now(),
	})

	out := m.renderActivityLine()
	if out == "" {
		t.Fatal("expected live activity panel output")
	}
	if !strings.Contains(out, "LIVE") {
		t.Fatalf("expected LIVE header, got:\n%s", out)
	}
	if !strings.Contains(out, "Spawning coder") {
		t.Fatalf("expected trail summary, got:\n%s", out)
	}
}

func TestRenderGlassBoxMessage_Timeline(t *testing.T) {
	m := NewTestModel(WithSize(100, 50))
	m.glassBoxEnabled = true
	out := m.renderGlassBoxMessage(Message{
		Role:             "system",
		Content:          "Spawning coder\ntask: fix flaky test",
		Time:             time.Now(),
		GlassBoxCategory: transparency.CategoryShard,
		IsCollapsed:      false,
	})
	if !strings.Contains(out, "SHARD") {
		t.Fatalf("expected SHARD pill, got %q", out)
	}
	if !strings.Contains(out, "│") {
		t.Fatalf("expected timeline rail, got %q", out)
	}
	if !strings.Contains(out, "task: fix flaky test") {
		t.Fatalf("expected expanded details, got %q", out)
	}
}
