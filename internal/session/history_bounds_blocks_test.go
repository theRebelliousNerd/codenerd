package session

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The transcript bound has to reach what the adapters actually read.
//
// boundToolLoopHistory shrinks old tool results by rewriting them, and the
// obvious way to do that is to assign the message's ToolResults field. On a
// message built from the flat fields that is right, because Content() lifts
// them. On a message built from ORDERED BLOCKS it changes only the projection:
// Content() returns the block list, every adapter goes on sending the full
// payload, and the transcript quietly stops being bounded. Nothing fails, no
// test goes red, no log line appears — the bill arrives instead.
//
// This became reachable the moment the tool loop started appending assistant
// turns through AssistantMessageFrom, because block-built messages stopped
// being a thing only tests construct.
func TestBoundToolLoopHistoryShrinksWhatTheAdaptersRead(t *testing.T) {
	history := blockBuiltToolTranscript(50, 16*1024)

	before := transcriptBytesThroughContent(history)
	if before <= maxToolLoopHistoryBytes {
		t.Fatalf("fixture is only %d bytes, under the %d ceiling: it cannot "+
			"detect an eviction that does not happen", before, maxToolLoopHistoryBytes)
	}

	bounded := boundToolLoopHistory(history)

	after := transcriptBytesThroughContent(bounded)
	if after > maxToolLoopHistoryBytes {
		t.Errorf("through Content(), which is what every adapter reads, the bounded "+
			"transcript is still %d bytes against a %d ceiling: the eviction reached "+
			"the flat projection and not the blocks", after, maxToolLoopHistoryBytes)
	}

	// And the flat view has to agree, or the two disagree about the same turn
	// and whichever a reader picks decides what the provider is billed for.
	if flat := toolLoopHistoryBytes(bounded); flat > maxToolLoopHistoryBytes {
		t.Errorf("flat projection is %d bytes against a %d ceiling", flat, maxToolLoopHistoryBytes)
	}
}

// The newest batch is clamped rather than blanked, and that has to reach the
// blocks too — a clamp that lands only on the projection is the same silent
// failure one notch quieter, because the transcript does shrink, just not by
// as much as the log line claims.
func TestBoundToolLoopHistoryClampsTheNewestBatchInTheBlocksToo(t *testing.T) {
	id := "tool0"
	huge := strings.Repeat("R", maxToolLoopHistoryBytes*2)
	history := []types.Message{
		types.NewUserMessage(types.TextBlock("do the thing")),
		types.NewAssistantMessage(types.ToolUseBlock(id, "read_file", nil)),
		types.NewUserMessage(types.ToolResultBlock(id, huge, false)),
	}

	bounded := boundToolLoopHistory(history)

	newest := bounded[len(bounded)-1]
	blocks := newest.Content()
	if len(blocks) != 1 || blocks[0].Kind != types.BlockToolResult {
		t.Fatalf("expected one tool_result block, got %#v", blocks)
	}
	if len(blocks[0].Text) >= len(huge) {
		t.Errorf("the clamp did not reach the block: %d bytes, was %d",
			len(blocks[0].Text), len(huge))
	}
	if blocks[0].ToolUseID != id {
		t.Errorf("clamping lost the pairing id: %q", blocks[0].ToolUseID)
	}
	// A tool result whose id survives but whose text does not pair to anything
	// is worse than one that was blanked, so both views must carry the same
	// content, not just the same length.
	if newest.ToolResults[0].Content != blocks[0].Text {
		t.Error("the flat projection and the block disagree about the clamped content")
	}
}

func blockBuiltToolTranscript(n, size int) []types.Message {
	h := []types.Message{types.NewUserMessage(types.TextBlock("do the thing"))}
	for i := 0; i < n; i++ {
		id := "tool" + strings.Repeat("x", i%5)
		h = append(h,
			types.NewAssistantMessage(types.ToolUseBlock(id, "read_file", nil)),
			types.NewUserMessage(types.ToolResultBlock(id, strings.Repeat("R", size), false)),
		)
	}
	return h
}

// transcriptBytesThroughContent totals the transcript the way an adapter sees
// it, rather than the way toolLoopHistoryBytes counts it. When those two
// disagree, the second one is the lie.
func transcriptBytesThroughContent(history []types.Message) int {
	total := 0
	for _, m := range history {
		for _, b := range m.Content() {
			switch b.Kind {
			case types.BlockText, types.BlockToolResult:
				total += len(b.Text)
			case types.BlockToolUse:
				total += len(b.Name)
			}
		}
	}
	return total
}
