package session

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// Eviction from the generation window is the same event as a clamped tool
// result — the model is given less than there was — and it used to be the one
// such event that said nothing. The char-budget loop dropped whole user and
// assistant turns and left only a debug line the model never sees, so a
// conversation whose first six turns were gone was indistinguishable from a
// conversation that started at turn seven. "As I said earlier" then answers
// from a transcript that no longer contains what was said.
func TestHistoryEviction_MarksAndRetains(t *testing.T) {
	e := &Executor{}
	e.SetConfig(ExecutorConfig{
		HistoryTurnWindow: DefaultHistoryTurnWindow,
		HistoryCharBudget: 4000,
	})

	// Ten turns of 1 KB each: the 4000-char budget can hold roughly four, so
	// the rest must be evicted.
	for i := range 10 {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		e.appendToHistory(perception.ConversationTurn{
			Role:    role,
			Content: fmt.Sprintf("TURN%02d ", i) + strings.Repeat("x", 1000),
		})
	}

	msgs := e.priorTurnMessages()
	if len(msgs) == 0 {
		t.Fatal("the whole window was evicted")
	}

	joined := strings.Join(messageTexts(msgs), "\n")
	if !types.IsClamped(joined) {
		t.Fatalf("history eviction left no marker; the window reads as a whole conversation:\n%s",
			truncateForFailure(joined))
	}
	// The marker must be reachable, not buried: it opens the oldest surviving
	// message, which is the first thing the model reads.
	if !types.IsClamped(msgs[0].Text) {
		t.Errorf("the marker is not on the oldest surviving message; got %q", truncateForFailure(msgs[0].Text))
	}

	// Count and kind, not just "something happened".
	if !strings.Contains(msgs[0].Text, "older conversation messages") {
		t.Errorf("the marker does not name what was evicted: %q", truncateForFailure(msgs[0].Text))
	}

	// Recoverable: the dropped turns are retained, not destroyed.
	evicted := e.recoverHistoryEviction()
	if len(evicted) == 0 {
		t.Fatal("eviction recorded nothing; the dropped turns cannot be brought back")
	}
	if !strings.Contains(evicted[0].Text, "TURN00") {
		t.Errorf("the oldest evicted turn is not the one recovered first: %q", truncateForFailure(evicted[0].Text))
	}
	if len(evicted)+len(msgs) < 10 {
		t.Errorf("evicted %d + surviving %d < 10 turns; turns vanished from both sides",
			len(evicted), len(msgs))
	}
}

// Nothing evicted means nothing announced: a marker on an intact window would
// teach the model to distrust a complete transcript.
func TestHistoryEviction_SaysNothingWhenNothingWasEvicted(t *testing.T) {
	e := &Executor{}
	e.SetConfig(ExecutorConfig{
		HistoryTurnWindow: DefaultHistoryTurnWindow,
		HistoryCharBudget: DefaultHistoryCharBudget,
	})
	e.appendToHistory(perception.ConversationTurn{Role: "user", Content: "fix the router"})
	e.appendToHistory(perception.ConversationTurn{Role: "assistant", Content: "done"})

	msgs := e.priorTurnMessages()
	joined := strings.Join(messageTexts(msgs), "\n")
	if types.IsClamped(joined) {
		t.Errorf("an intact window carries a truncation marker: %q", joined)
	}
	if got := e.recoverHistoryEviction(); len(got) != 0 {
		t.Errorf("recorded %d evicted messages for an intact window", len(got))
	}
}

func messageTexts(msgs []types.Message) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Text)
	}
	return out
}

// truncateForFailure keeps a failure message readable. It is a test-only
// display cut on text nothing will act on, not a cut in the model's window.
func truncateForFailure(s string) string {
	if len(s) <= 400 {
		return s
	}
	return s[:400] + "<test display cut>"
}
