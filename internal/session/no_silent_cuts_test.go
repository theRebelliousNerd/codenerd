package session

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// The invariant, asserted against the real assembly rather than against any
// one cutter: whatever reaches the model is either the source content whole,
// or shorter content carrying the marker that says so.
//
// The per-site tests each pin one cutter. This one is the net underneath them:
// it holds when a cut is added somewhere nobody thought to test, because it
// compares what went in against what came out and asks the question of every
// content-bearing segment. A silent cut added tomorrow fails here even if no
// per-site test knows it exists.
func TestNoSilentCutsInAssembledMessages(t *testing.T) {
	t.Run("a tool-loop turn", func(t *testing.T) {
		assertNoSilentCutsInToolLoopTurn(t)
	})
	t.Run("a chat turn", func(t *testing.T) {
		assertNoSilentCutsInChatTurn(t)
	})
}

// assertNoSilentCutsInToolLoopTurn drives prepareWorkingRequest — the function
// that builds the provider request for one round of the tool loop — with a
// window far too small for the transcript, which is the condition under which
// it archives results out of the messages. Every archived or shortened result
// has to be announced.
func assertNoSilentCutsInToolLoopTurn(t *testing.T) {
	t.Helper()

	e, ctx, history, body := oneWorkingRound(t, 200000)

	// Two more rounds of substantial output, so there is more than one result
	// competing for the window.
	sources := map[string]string{"call-1": body}
	for round := 2; round <= 3; round++ {
		id := fmt.Sprintf("call-%d", round)
		later := fmt.Sprintf("ROUND%d ", round) + strings.Repeat("evidence line that the model would act on\n", 300)
		call := types.ToolCall{ID: id, Name: "read_file", Input: map[string]any{"path": "target.go", "start_line": round}}
		if err := e.recordWorkingResult(ctx, call, later, nil); err != nil {
			t.Fatalf("recordWorkingResult: %v", err)
		}
		history = append(history,
			types.Message{Role: "assistant", ToolCalls: []types.ToolCall{call}},
			types.Message{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: id, Content: later}}})
		sources[id] = later
	}

	// A window that can hold the instructions but not the transcript is the
	// whole point: this is where the assembly has to shed content, and
	// therefore where it has to say that it did. Too small and it refuses
	// outright (which is honest but exercises nothing); too large and nothing
	// is shed.
	e.config.TokenBudget = 9000

	system := "SYSTEM PROMPT " + strings.Repeat("instruction line\n", 40)
	// The system prompt is not the builder's to change: it goes to the provider
	// as it was compiled, and the working section rides on the last user turn.
	gotMessages, err := e.prepareWorkingRequest(ctx, system, history, nil)
	if err != nil {
		t.Fatalf("prepareWorkingRequest at a %d-token window: %v", e.config.TokenBudget, err)
	}
	gotSystem := system

	seen := map[string]bool{}
	shortened := 0
	for _, m := range gotMessages {
		for _, r := range m.ToolResults {
			source, known := sources[r.ToolUseID]
			if !known {
				continue
			}
			seen[r.ToolUseID] = true
			if len(r.Content) >= len(source) {
				continue
			}
			shortened++
			if !types.IsClamped(r.Content) {
				t.Errorf("tool result %s was shortened from %d to %d chars with no marker:\n%s",
					r.ToolUseID, len(source), len(r.Content), truncateForFailure(r.Content))
			}
		}
	}

	// A result that left the transcript entirely is the strongest form of the
	// same cut, and the window has to account for it too.
	for id, source := range sources {
		if seen[id] {
			continue
		}
		shortened++
		if !windowAnnouncesACut(gotSystem, gotMessages) {
			t.Errorf("tool result %s (%d chars) left the request entirely and nothing in the window says so",
				id, len(source))
		}
	}

	if shortened == 0 {
		t.Fatalf("nothing was shed at a %d-token window, so the invariant was not exercised", e.config.TokenBudget)
	}
	t.Logf("tool-loop turn: %d shortened or archived segments, all announced", shortened)
}

// assertNoSilentCutsInChatTurn drives the window a plain interactive turn
// sends: priorTurnMessages plus the transcript rendering used for clients with
// no native history channel.
func assertNoSilentCutsInChatTurn(t *testing.T) {
	t.Helper()

	e := &Executor{}
	e.SetConfig(ExecutorConfig{
		HistoryTurnWindow: defaultSessionPolicy.HistoryTurnWindow,
		HistoryCharBudget: 5000,
	})

	sources := make([]string, 0, 14)
	for i := range 14 {
		role := "user"
		if i%2 == 1 {
			role = "assistant"
		}
		content := fmt.Sprintf("NEEDLE%02d ", i) + strings.Repeat("conversation text that the model may be asked about later. ", 12)
		e.appendToHistory(perception.ConversationTurn{Role: role, Content: content})
		sources = append(sources, content)
	}

	msgs := e.priorTurnMessages()
	if len(msgs) == 0 {
		t.Fatal("the window is empty")
	}
	window := strings.Join(messageTexts(msgs), "\n")
	rendered := renderHistoryTranscript(msgs, "and now the current question")

	dropped := 0
	for i := range sources {
		if strings.Contains(window, fmt.Sprintf("NEEDLE%02d", i)) {
			continue
		}
		dropped++
	}
	if dropped == 0 {
		t.Fatalf("no turn was evicted at a 5000-char budget over %d turns; the invariant was not exercised", len(sources))
	}
	if !types.IsClamped(window) {
		t.Errorf("%d of %d conversation turns left the window and nothing in it says so:\n%s",
			dropped, len(sources), truncateForFailure(window))
	}
	// The degraded rendering is a separate string handed to clients without a
	// history channel, and it must carry the same announcement.
	if !types.IsClamped(rendered) {
		t.Errorf("the rendered transcript lost %d turns with no marker:\n%s", dropped, truncateForFailure(rendered))
	}
	if len(e.recoverHistoryEviction()) != dropped {
		t.Errorf("recovered %d evicted turns, %d were dropped; the record and the window disagree",
			len(e.recoverHistoryEviction()), dropped)
	}
	t.Logf("chat turn: %d evicted turns, announced and recoverable", dropped)
}

// windowAnnouncesACut reports whether anything in the assembled request tells
// the model that content was removed.
func windowAnnouncesACut(system string, msgs []types.Message) bool {
	if types.IsClamped(system) {
		return true
	}
	for _, m := range msgs {
		if types.IsClamped(m.Text) {
			return true
		}
		for _, r := range m.ToolResults {
			if types.IsClamped(r.Content) {
				return true
			}
		}
	}
	return false
}

// A turn that fits is delivered whole, with nothing added. The invariant cuts
// both ways: a marker on intact content teaches the model to distrust a
// complete record, which costs exactly as much as a missing one.
func TestNoSilentCutsInAssembledMessages_LeavesAnIntactTurnAlone(t *testing.T) {
	e := newWorkingLoopExecutor(t, &MockLLMClient{})
	e.config.TokenBudget = 200000

	ctx, closeLoop, err := e.beginWorkingLoop(context.Background(), "fix target.go",
		&prompt.CompilationContext{ShardID: "probe", IntentTarget: "target.go"})
	if err != nil {
		t.Fatalf("beginWorkingLoop: %v", err)
	}
	t.Cleanup(closeLoop)

	body := "short and complete tool output\n"
	call := types.ToolCall{ID: "call-1", Name: "read_file", Input: map[string]any{"path": "target.go"}}
	if err := e.recordWorkingResult(ctx, call, body, nil); err != nil {
		t.Fatalf("recordWorkingResult: %v", err)
	}
	history := []types.Message{
		{Role: "user", Text: "fix target.go"},
		{Role: "assistant", ToolCalls: []types.ToolCall{call}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: call.ID, Content: body}}},
	}

	gotMessages, err := e.prepareWorkingRequest(ctx, "SYSTEM", history, nil)
	if err != nil {
		t.Fatalf("prepareWorkingRequest: %v", err)
	}
	if got := lastToolResult(t, gotMessages); got != body {
		t.Errorf("an intact tool result was altered: %q", got)
	}
	// The working section rides on the last user turn; it is where a marker on
	// intact content would now appear.
	for _, m := range gotMessages {
		if types.IsClamped(m.Text) {
			t.Errorf("an uncut turn carries a truncation marker:\n%s", truncateForFailure(m.Text))
		}
	}
}
