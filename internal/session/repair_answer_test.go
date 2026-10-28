package session

import (
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// A repair attempt ends at its write, and that response is usually tool calls
// with no text. It used to replace the turn's answer: the model's own
// summary was dropped and articulation parsed an empty response ("Fallback
// parse: empty response with no Piggyback JSON", problems log 2026-10-28,
// once after the pinning round and once after a coverage round). A round's
// response replaces the answer only when it says something.
func TestRepairLoop_AToolOnlyRepairKeepsTheTurnsAnswer(t *testing.T) {
	h := newRepairHarness(t, nil)
	h.initial = func() *types.LLMToolResponse {
		return &types.LLMToolResponse{Text: "writing", ToolCalls: []types.ToolCall{
			h.writeCall("c1", "write_file", filepath.Join(h.ws, "main.go"), repairMainGo),
			h.writeCall("c2", "write_file", filepath.Join(h.ws, "main_test.go"), repairTestBroken),
		}}
	}
	h.onRepair = func(int, []types.Message) *types.LLMToolResponse {
		return &types.LLMToolResponse{ToolCalls: []types.ToolCall{
			h.writeCall("r1", "write_file", filepath.Join(h.ws, "main_test.go"), repairTestFixed),
		}}
	}
	result, err := h.drive(t, "fix make the failing test pass")
	if err != nil {
		t.Fatalf("ProcessWithIntent: %v", err)
	}
	if result.TestCheck.Repair == nil || !result.TestCheck.Repair.Passed {
		t.Fatalf("the repair did not run and converge: %+v", result.TestCheck)
	}
	if !strings.Contains(result.Response, "done") {
		t.Fatalf("the turn's answer was replaced by the repair's empty response: %q", result.Response)
	}
}
