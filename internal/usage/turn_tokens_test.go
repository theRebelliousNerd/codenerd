package usage

import (
	"context"
	"testing"
)

// Every executor in a campaign shares one session id, so a session-level
// before/after delta absorbed sibling shards' tokens. Per-turn counts are
// keyed by the turn id on the context and kept in memory only.
func TestTurnTokens_IsolatesConcurrentTurnsInOneSession(t *testing.T) {
	tr, err := NewTracker(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	base := WithSessionID(context.Background(), "s1")
	a := WithTurnID(base, "s1#1#a")
	b := WithTurnID(base, "s1#1#b")

	tr.Track(a, "m", "meta", 100, 10, "chat")
	tr.Track(b, "m", "meta", 5000, 500, "chat")
	tr.Track(a, "m", "meta", 50, 5, "chat")

	if got := tr.TurnTokens("s1#1#a"); got.Input != 150 || got.Output != 15 {
		t.Fatalf("turn a = %+v, want input 150 output 15", got)
	}
	if got := tr.TurnTokens("s1#1#b"); got.Input != 5000 || got.Output != 500 {
		t.Fatalf("turn b = %+v, want input 5000 output 500", got)
	}
	if got := tr.SessionTokens("s1"); got.Input != 5150 {
		t.Fatalf("session still aggregates both turns, got input %d", got.Input)
	}
	if got := tr.TurnTokens("never"); got.Input != 0 || got.Output != 0 {
		t.Fatalf("unknown turn must read zero, got %+v", got)
	}
	if TurnIDFromContext(base) != "" || TurnIDFromContext(a) != "s1#1#a" {
		t.Fatal("TurnIDFromContext must round-trip the tag")
	}
}

func TestTurnTokens_BoundedAndNotPersisted(t *testing.T) {
	dir := t.TempDir()
	tr, err := NewTracker(dir)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < maxTurns+50; i++ {
		tr.Track(WithTurnID(context.Background(), "t"+string(rune('a'+i%26))+string(rune('0'+i%10))+string(rune('A'+(i/260)%26))+string(rune('0'+(i/26)%10))), "m", "meta", 1, 1, "chat")
	}
	if n := len(tr.turns); n > maxTurns {
		t.Fatalf("turn map must be bounded to %d, has %d", maxTurns, n)
	}
	if err := tr.Save(); err != nil {
		t.Fatal(err)
	}
	reopened, err := NewTracker(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.turns) != 0 {
		t.Fatal("per-turn counts must not be persisted")
	}
}
