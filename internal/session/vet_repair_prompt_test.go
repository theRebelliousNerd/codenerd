package session

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The vet obligation's repair round points the model at run_lint when the turn
// offers it, so the model re-runs vet itself instead of asking for a vet tool
// (2026-10-29: missing_tool_for(_, /go_vet) sent autopoiesis to build one).
func TestVetRepairPrompt_NamesRunLintWhenTheTurnOffersIt(t *testing.T) {
	t.Parallel()
	defs := []types.ToolDefinition{{Name: "edit_file"}, {Name: "run_lint"}}
	with := vetRepairPrompt("a.go:3:2: unreachable code", offersTool(defs, "run_lint"))
	if !strings.Contains(with, `run_lint with gate "vet"`) || !strings.Contains(with, "a.go:3:2: unreachable code") {
		t.Fatalf("prompt with run_lint offered does not name it:\n%s", with)
	}
	without := vetRepairPrompt("a.go:3:2: unreachable code", offersTool(defs[:1], "run_lint"))
	if strings.Contains(without, "run_lint") {
		t.Fatalf("prompt names a tool the turn does not offer:\n%s", without)
	}
}
