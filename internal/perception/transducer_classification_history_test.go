package perception

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/types"
)

// installClassificationHistory installs a classification history policy for
// the test and puts the previous one back when the test ends. BuildPrompt
// reads the process-wide policy LoadUserConfig installs.
func installClassificationHistory(t *testing.T, h config.ClassificationHistory) {
	t.Helper()
	prev := config.ResolvedClassificationHistory()
	config.SetClassificationHistory(h)
	t.Cleanup(func() { config.SetClassificationHistory(prev) })
}

func classificationPrompt(history []ConversationTurn) string {
	return NewLLMTransducer(nil, nil, "system").BuildPrompt("CURRENT", history, nil, nil, "")
}

// A handful of short turns is the common case: every one of them arrives
// whole, and nothing is announced because nothing was left out.
func TestClassificationHistory_ShortTurnsAreWhole(t *testing.T) {
	installClassificationHistory(t, config.ClassificationHistory{TurnWindow: 5, CharBudget: 1000})
	history := []ConversationTurn{
		{Role: "user", Content: "what does this do?"},
		{Role: "assistant", Content: "it routes requests", ThoughtSummary: "read the handler"},
		{Role: "user", Content: "rename it"},
	}
	got := classificationPrompt(history)
	for _, want := range []string{"what does this do?", "it routes requests", "read the handler", "rename it"} {
		if !strings.Contains(got, want) {
			t.Errorf("prompt lost %q", want)
		}
	}
	if types.IsClamped(got) {
		t.Errorf("a window that fits announced a drop:\n%s", got)
	}
}

// Past the turn window, the newest turns that fit stay whole and the drop
// is stated. The count is of the turns that were real, not of empty ones.
func TestClassificationHistory_WindowKeepsTheNewestWhole(t *testing.T) {
	installClassificationHistory(t, config.ClassificationHistory{TurnWindow: 3, CharBudget: 10000})
	var history []ConversationTurn
	history = append(history, ConversationTurn{}) // empty: does not consume the window
	for i := 1; i <= 6; i++ {
		history = append(history, ConversationTurn{Role: "user", Content: fmt.Sprintf("TURN-%d-BODY", i)})
	}
	got := classificationPrompt(history)
	for _, kept := range []string{"TURN-4-BODY", "TURN-5-BODY", "TURN-6-BODY"} {
		if !strings.Contains(got, kept) {
			t.Errorf("prompt lost %q", kept)
		}
	}
	for _, dropped := range []string{"TURN-1-BODY", "TURN-2-BODY", "TURN-3-BODY"} {
		if strings.Contains(got, dropped) {
			t.Errorf("prompt still contains evicted %q", dropped)
		}
	}
	if !strings.Contains(got, "3 of 6 prior turns") {
		t.Errorf("the drop was not stated:\n%s", got)
	}
	// The kept text is the whole turn, not a clamp around it.
	if strings.Contains(got, "chars from prior turn") {
		t.Error("a kept turn was sliced")
	}
}

// The byte budget evicts oldest whole turns after the count window has.
func TestClassificationHistory_BudgetKeepsTheNewestThatFit(t *testing.T) {
	installClassificationHistory(t, config.ClassificationHistory{TurnWindow: 10, CharBudget: 250})
	var history []ConversationTurn
	for i := 0; i < 6; i++ {
		history = append(history, ConversationTurn{
			Role:    "user",
			Content: fmt.Sprintf("M%02d", i) + strings.Repeat("x", 97), // 100 bytes
		})
	}
	got := classificationPrompt(history)
	// 100-byte turns, budget 250: the newest two fit (200), the third does not.
	for _, kept := range []string{"M04", "M05"} {
		if !strings.Contains(got, kept) {
			t.Errorf("prompt lost %q", kept)
		}
	}
	for _, dropped := range []string{"M00", "M01", "M02", "M03"} {
		if strings.Contains(got, dropped) {
			t.Errorf("prompt still contains evicted %q", dropped)
		}
	}
	if !strings.Contains(got, "4 of 6 prior turns (400 chars)") {
		t.Errorf("the drop was not stated with its size:\n%s", got)
	}
	if !strings.Contains(got, strings.Repeat("x", 97)) {
		t.Error("a kept turn was not whole")
	}
}

// One turn over the budget is still sent whole, with a note, and older
// turns stay out. Classification cannot page that turn back, and a follow-up
// refers to it.
func TestClassificationHistory_OversizedNewestTurnIsWhole(t *testing.T) {
	const budget = 50
	installClassificationHistory(t, config.ClassificationHistory{TurnWindow: 5, CharBudget: budget})
	content := "HEAD" + strings.Repeat("Q", 300) + "TAIL"
	thought := "THOUGHT" + strings.Repeat("Z", 300) + "END"
	history := []ConversationTurn{
		{Role: "user", Content: "OLDER-PLAIN"},
		{Role: "assistant", Content: content, ThoughtSummary: thought},
	}
	got := classificationPrompt(history)
	if !strings.Contains(got, content) {
		t.Error("the oversized content was sliced or dropped")
	}
	if !strings.Contains(got, thought) {
		t.Error("the oversized reasoning summary was sliced or dropped")
	}
	if strings.Contains(got, "OLDER-PLAIN") {
		t.Error("an older turn was included beside a turn that already exceeds the budget")
	}
	newest := len(content) + len(thought)
	want := fmt.Sprintf(
		"the newest prior turn is %d chars, over classification.history_char_budget %d, and is included whole; 1 of 2 prior turns (%d chars) were left out",
		newest, budget, len("OLDER-PLAIN"))
	if !strings.Contains(got, want) {
		t.Errorf("notice missing %q\n%s", want, got)
	}
}

// A window of 0 is the file asking for no prior turn. Nothing is rendered
// and nothing is announced.
func TestClassificationHistory_ZeroWindowSendsNone(t *testing.T) {
	installClassificationHistory(t, config.ClassificationHistory{TurnWindow: 0, CharBudget: 1000})
	got := classificationPrompt([]ConversationTurn{{Role: "user", Content: "SECRET-TURN"}})
	if strings.Contains(got, "SECRET-TURN") || strings.Contains(got, "Recent Conversation") {
		t.Errorf("window 0 still rendered history:\n%s", got)
	}
	if !strings.Contains(got, "CURRENT") {
		t.Error("the current request was dropped with the history")
	}
	if types.IsClamped(got) {
		t.Errorf("window 0 announced a drop the file asked for:\n%s", got)
	}
}

// The default policy is what an unconfigured process classifies with:
// the newest default-window of short turns, and not the one before it.
func TestClassificationHistory_DefaultPolicyDropsTheOldest(t *testing.T) {
	policy := config.DefaultClassificationConfig().Resolve()
	installClassificationHistory(t, policy)
	var history []ConversationTurn
	for i := 0; i < policy.TurnWindow+1; i++ {
		history = append(history, ConversationTurn{Role: "user", Content: fmt.Sprintf("DEF-%d-END", i)})
	}
	got := classificationPrompt(history)
	if strings.Contains(got, "DEF-0-END") {
		t.Error("the default window kept the oldest turn")
	}
	last := fmt.Sprintf("DEF-%d-END", policy.TurnWindow)
	firstKept := fmt.Sprintf("DEF-%d-END", 1)
	if !strings.Contains(got, last) || !strings.Contains(got, firstKept) {
		t.Errorf("the default window lost a turn it should keep:\n%s", got)
	}
	want := fmt.Sprintf("1 of %d prior turns", policy.TurnWindow+1)
	if !strings.Contains(got, want) {
		t.Errorf("default drop notice missing %q\n%s", want, got)
	}
}
