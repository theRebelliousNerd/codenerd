package session

import (
	"strings"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/prompt"
	"codenerd/internal/types"
)

// The turn COUNT was capped at 50; no turn's text was capped at all. Both ends
// of a turn are unbounded input — the user side is whatever was typed or piped
// (`nerd run "$(cat build.log)"`), the assistant side is whatever the model
// returned — and one oversized turn did two separate kinds of damage:
// priorTurnMessages evicts whole messages oldest-first against a 24000-char
// budget, so a single 200 KB turn threw away EVERY prior turn; and perception
// replays the last five turns into every classification call, so the blob was
// re-sent until it aged out of a 50-slot window.
func TestAppendToHistory_BoundsTurnText(t *testing.T) {
	tests := []struct {
		name       string
		turn       perception.ConversationTurn
		wantMarker bool
		mustKeep   []string
	}{
		{
			name:     "an ordinary turn is stored verbatim",
			turn:     perception.ConversationTurn{Role: "user", Content: "fix the router"},
			mustKeep: []string{"fix the router"},
		},
		{
			name: "a piped build log is clamped head and tail",
			turn: perception.ConversationTurn{
				Role:    "user",
				Content: "HEADMARK" + strings.Repeat("L", 5_000_000) + "TAILMARK",
			},
			wantMarker: true,
			// The tail of a build log is the failure; head-only truncation
			// removes exactly the line the turn exists to act on.
			mustKeep: []string{"HEADMARK", "TAILMARK"},
		},
		{
			name: "an oversized reasoning summary is clamped",
			turn: perception.ConversationTurn{
				Role:           "assistant",
				Content:        "done",
				ThoughtSummary: strings.Repeat("T", 500_000),
			},
			wantMarker: true,
			mustKeep:   []string{"done"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Executor{}
			e.appendToHistory(tt.turn)

			got := e.GetHistory()
			if len(got) != 1 {
				t.Fatalf("history has %d turns, want 1", len(got))
			}
			stored := got[0]

			if len(stored.Content) > maxHistoryTurnChars+512 {
				t.Errorf("stored content is %d chars, cap is %d", len(stored.Content), maxHistoryTurnChars)
			}
			if len(stored.ThoughtSummary) > maxHistoryThoughtChars+512 {
				t.Errorf("stored thought summary is %d chars, cap is %d",
					len(stored.ThoughtSummary), maxHistoryThoughtChars)
			}
			marked := prompt.IsClamped(stored.Content) || prompt.IsClamped(stored.ThoughtSummary)
			if marked != tt.wantMarker {
				t.Errorf("truncation marker present = %v, want %v", marked, tt.wantMarker)
			}
			for _, want := range tt.mustKeep {
				if !strings.Contains(stored.Content, want) {
					t.Errorf("stored turn lost %q", want)
				}
			}
		})
	}
}

// The regression this cap exists to prevent: one huge turn used to make the
// char-budget loop evict every message, so the next turn started with no
// conversational memory at all.
func TestPriorTurnMessages_OneHugeTurnDoesNotEvictTheWindow(t *testing.T) {
	e := &Executor{}
	e.SetConfig(ExecutorConfig{
		HistoryTurnWindow: DefaultHistoryTurnWindow,
		HistoryCharBudget: DefaultHistoryCharBudget,
	})

	e.appendToHistory(perception.ConversationTurn{Role: "user", Content: "EARLIEST question"})
	e.appendToHistory(perception.ConversationTurn{Role: "assistant", Content: "an answer"})
	e.appendToHistory(perception.ConversationTurn{
		Role: "user", Content: strings.Repeat("G", 5_000_000)})
	e.appendToHistory(perception.ConversationTurn{Role: "assistant", Content: "LATEST answer"})

	msgs := e.priorTurnMessages()
	if len(msgs) == 0 {
		t.Fatal("a single oversized turn wiped the entire replay window")
	}
	joined := ""
	for _, m := range msgs {
		joined += m.Text
	}
	if !strings.Contains(joined, "LATEST answer") {
		t.Error("the most recent turn must survive")
	}
	if total := len(joined); total > DefaultHistoryCharBudget+2048 {
		t.Errorf("replay window is %d chars, budget is %d", total, DefaultHistoryCharBudget)
	}
}

// The in-turn tool transcript is append-only and re-sent WHOLE on every
// provider round-trip. Per-result truncation caps one result at 16 KiB; nothing
// capped their sum, and the arithmetic is not hypothetical — 50 tool calls x
// 16 KiB, replayed across ~24 iterations.
func TestBoundToolLoopHistory(t *testing.T) {
	result := func(id string, size int) types.Message {
		return types.Message{Role: "user", ToolResults: []types.ToolResult{
			{ToolUseID: id, Content: strings.Repeat("R", size)},
		}}
	}

	tests := []struct {
		name        string
		history     []types.Message
		wantEvicted bool
	}{
		{
			name: "a normal turn is untouched",
			history: []types.Message{
				{Role: "user", Text: "fix the router"},
				{Role: "assistant", Text: "reading files", ToolCalls: []types.ToolCall{{ID: "t1", Name: "read_file"}}},
				result("t1", 4000),
			},
		},
		{
			name:        "fifty full-size results are capped",
			history:     bigToolTranscript(50, 16*1024),
			wantEvicted: true,
		},
		{
			name:        "one pathological result is capped",
			history:     []types.Message{result("t1", 4_000_000)},
			wantEvicted: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := boundToolLoopHistory(tt.history)

			if n := toolLoopHistoryBytes(got); n > maxToolLoopHistoryBytes {
				t.Errorf("transcript is %d bytes, cap is %d", n, maxToolLoopHistoryBytes)
			}
			if len(got) != len(tt.history) {
				t.Fatalf("message count changed %d -> %d; an unpaired tool_use is a hard provider error",
					len(tt.history), len(got))
			}
			// Every ToolUseID must survive: Anthropic rejects a request whose
			// tool_use block has no matching tool_result.
			for i := range tt.history {
				if len(got[i].ToolResults) != len(tt.history[i].ToolResults) {
					t.Fatalf("message %d lost tool results", i)
				}
				for j := range tt.history[i].ToolResults {
					if got[i].ToolResults[j].ToolUseID != tt.history[i].ToolResults[j].ToolUseID {
						t.Fatalf("message %d result %d lost its ToolUseID", i, j)
					}
				}
			}

			rendered := renderTranscript(got)
			bounded := strings.Contains(rendered, "evicted from the transcript") || prompt.IsClamped(rendered)
			if bounded != tt.wantEvicted {
				t.Errorf("bounding marker present = %v, want %v", bounded, tt.wantEvicted)
			}
			if !tt.wantEvicted {
				return
			}
			// The most recent result is what the next decision depends on.
			last := got[len(got)-1]
			if len(last.ToolResults) > 0 && last.ToolResults[0].Content == evictedToolResultNotice {
				t.Error("the newest tool result was evicted; eviction must be oldest-first")
			}
			// Eviction must not mutate the caller's slice.
			original := renderTranscript(tt.history)
			if strings.Contains(original, "evicted from the transcript") || prompt.IsClamped(original) {
				t.Error("boundToolLoopHistory mutated its input")
			}
		})
	}
}

// A transcript already at the ceiling must not be re-evicted on the next round:
// a marker that keeps replacing itself would loop and drop everything.
func TestBoundToolLoopHistory_IsIdempotent(t *testing.T) {
	once := boundToolLoopHistory(bigToolTranscript(50, 16*1024))
	twice := boundToolLoopHistory(once)
	if renderTranscript(once) != renderTranscript(twice) {
		t.Error("a second pass changed an already-bounded transcript")
	}
}

func bigToolTranscript(n, size int) []types.Message {
	var h []types.Message
	h = append(h, types.Message{Role: "user", Text: "do the thing"})
	for i := 0; i < n; i++ {
		id := "tool" + strings.Repeat("x", i%5)
		h = append(h,
			types.Message{Role: "assistant", ToolCalls: []types.ToolCall{{ID: id, Name: "read_file"}}},
			types.Message{Role: "user", ToolResults: []types.ToolResult{
				{ToolUseID: id, Content: strings.Repeat("R", size)},
			}},
		)
	}
	return h
}

func renderTranscript(h []types.Message) string {
	var b strings.Builder
	for _, m := range h {
		b.WriteString(m.Text)
		for _, tr := range m.ToolResults {
			b.WriteString(tr.Content)
		}
	}
	return b.String()
}
