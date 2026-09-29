package session

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// A stored turn is the text the next window and recall_context hand back.
// Slicing it on the way in (and dropping turns past a silent count) made both
// of those lie about what was said.
func TestHistory_AppendStoresTurnsWhole(t *testing.T) {
	e := &Executor{}
	huge := "HEADMARK" + strings.Repeat("L", 200_000) + "TAILMARK"
	thought := strings.Repeat("T", 50_000)
	e.appendToHistory(perception.ConversationTurn{
		Role: "user", Content: huge, ThoughtSummary: thought,
	})

	got := e.GetHistory()
	if len(got) != 1 {
		t.Fatalf("history has %d turns, want 1", len(got))
	}
	if got[0].Content != huge {
		t.Fatalf("stored content is %d chars, want the %d that were appended", len(got[0].Content), len(huge))
	}
	if got[0].ThoughtSummary != thought {
		t.Fatalf("stored thought summary is %d chars, want the %d that were appended", len(got[0].ThoughtSummary), len(thought))
	}
	if types.IsClamped(got[0].Content) || types.IsClamped(got[0].ThoughtSummary) {
		t.Fatal("a stored turn carries a truncation marker; the window is what bounds the prompt")
	}

	for i := 0; i < 60; i++ {
		e.appendToHistory(perception.ConversationTurn{Role: "user", Content: fmt.Sprintf("TURN%02d", i)})
	}
	got = e.GetHistory()
	if len(got) != 61 {
		t.Fatalf("stored %d turns, want 61; a silent cap dropped some", len(got))
	}
	if got[1].Content != "TURN00" {
		t.Fatalf("oldest counted turn = %q, want TURN00", got[1].Content)
	}
}

// One message longer than the char budget used to be sliced so it would fit,
// which threw away the tail of a pasted log, or it emptied the window and
// said nothing. The exchange leaves whole, earlier exchanges that fit stay,
// and recall_context returns the oversized text.
func TestHistory_OversizedTurnIsEvictedWholeAndRecallable(t *testing.T) {
	e := &Executor{}
	e.SetConfig(ExecutorConfig{
		HistoryTurnWindow: defaultSessionPolicy.HistoryTurnWindow,
		HistoryCharBudget: defaultSessionPolicy.HistoryCharBudget,
	})
	huge := "HEADMARK" + strings.Repeat("G", 80_000) + "TAILMARK"
	e.appendToHistory(perception.ConversationTurn{Role: "user", Content: "EARLIEST question"})
	e.appendToHistory(perception.ConversationTurn{Role: "assistant", Content: "an answer"})
	e.appendToHistory(perception.ConversationTurn{Role: "user", Content: huge})
	e.appendToHistory(perception.ConversationTurn{Role: "assistant", Content: "LATEST answer"})

	msgs, evicted := e.priorTurnWindow(true)
	if len(msgs) == 0 {
		t.Fatal("the oversized exchange wiped the window and left no notice")
	}
	joined := strings.Join(messageTexts(msgs), "\n")
	if !strings.Contains(joined, "EARLIEST question") || !strings.Contains(joined, "an answer") {
		t.Fatalf("earlier turns that fit were dropped:\n%s", truncateForFailure(joined))
	}
	if strings.Contains(joined, "HEADMARK") || strings.Contains(joined, "TAILMARK") {
		t.Fatal("the oversized turn was placed in the window, sliced or whole")
	}
	if !types.IsClamped(joined) || !strings.Contains(joined, "recall_context id=") {
		t.Fatalf("the eviction was not announced with a recall handle:\n%s", truncateForFailure(joined))
	}

	var recovered string
	for _, m := range evicted {
		recovered += m.Text
	}
	if !strings.Contains(recovered, huge) || !strings.Contains(recovered, "LATEST answer") {
		t.Fatal("the evicted exchange is not the oversized turn and its reply, whole")
	}

	handle := historyEvictionHandle(evicted)
	body, err := (historyRecall{handle: handle, evicted: evicted}).Recall(context.Background(), handle, 0, 0)
	if err != nil {
		t.Fatalf("recall: %v", err)
	}
	if !strings.Contains(body, "HEADMARK") || !strings.Contains(body, "TAILMARK") || !strings.Contains(body, "LATEST answer") {
		t.Fatalf("recall did not return the evicted exchange whole:\n%s", truncateForFailure(body))
	}
}

// A window no exchange fits is still told that the turns exist. Returning
// nothing there used to look like a conversation that had not started.
func TestHistory_WindowThatFitsNothingStillAnnounces(t *testing.T) {
	e := &Executor{}
	e.SetConfig(ExecutorConfig{
		HistoryTurnWindow: defaultSessionPolicy.HistoryTurnWindow,
		HistoryCharBudget: 20,
	})
	question := strings.Repeat("Q", 100)
	answer := strings.Repeat("A", 100)
	e.appendToHistory(perception.ConversationTurn{Role: "user", Content: question})
	e.appendToHistory(perception.ConversationTurn{Role: "assistant", Content: answer})

	msgs, evicted := e.priorTurnWindow(true)
	if len(msgs) != 1 || msgs[0].Role != "assistant" {
		t.Fatalf("window = %+v, want one assistant notice", msgs)
	}
	if strings.Contains(msgs[0].Text, question) || strings.Contains(msgs[0].Text, answer) {
		t.Fatal("the oversized exchange was placed in the window")
	}
	if !types.IsClamped(msgs[0].Text) || !strings.Contains(msgs[0].Text, "recall_context id=") {
		t.Fatalf("the drop was not announced with a recall handle: %q", msgs[0].Text)
	}
	if len(evicted) != 2 || evicted[0].Text != question || evicted[1].Text != answer {
		t.Fatalf("evicted = %+v, want both turns whole", evicted)
	}

	plain, _ := e.priorTurnWindow(false)
	if len(plain) != 1 || strings.Contains(plain[0].Text, "recall_context") {
		t.Fatalf("outside a working loop the notice named a handle nothing redeems: %+v", plain)
	}
}

// Nothing that fits is announced, and a window with no history is empty
// rather than a notice about nothing.
func TestHistory_IntactWindowIsNotAnnounced(t *testing.T) {
	e := &Executor{}
	e.SetConfig(ExecutorConfig{
		HistoryTurnWindow: defaultSessionPolicy.HistoryTurnWindow,
		HistoryCharBudget: defaultSessionPolicy.HistoryCharBudget,
	})
	e.appendToHistory(perception.ConversationTurn{Role: "user", Content: "fix the router"})
	e.appendToHistory(perception.ConversationTurn{Role: "assistant", Content: "done"})

	msgs, evicted := e.priorTurnWindow(true)
	joined := strings.Join(messageTexts(msgs), "\n")
	if types.IsClamped(joined) || strings.Contains(joined, "recall_context") {
		t.Errorf("an intact window carries an eviction notice: %q", joined)
	}
	if len(evicted) != 0 {
		t.Errorf("recorded %d evicted messages for an intact window", len(evicted))
	}
}
