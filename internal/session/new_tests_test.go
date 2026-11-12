package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// A failing test the turn wrote is named as the turn's own, in the repair
// round and in the give-up -- not "edits broke the tests", which sent three
// live rounds to edit production code under a new test whose own program did
// not analyze (2026-11-12). A test that existed before the turn keeps the old
// wording.
func TestRepairLoop_AFailingNewTestIsNamedAsTheTurnsOwn(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		existing   string // main_test.go before the turn; "" when the turn creates it
		turnWrites map[string]string
		repair     string // what each repair round writes
		wantPhrase string
		wantNote   bool
	}{
		"the turn's new test fails": {
			turnWrites: map[string]string{"main.go": repairMainGo, "main_test.go": repairTestBroken},
			repair:     strings.Replace(repairTestBroken, "broken", "still broken", 1),
			wantPhrase: "the tests this turn wrote fail (TestProbe)",
			wantNote:   true,
		},
		"an existing test fails": {
			existing:   "package main\n\nimport \"testing\"\n\nfunc TestProbe(t *testing.T) {\n\tif Val() != 1 {\n\t\tt.Fatal(\"broken\")\n\t}\n}\n",
			turnWrites: map[string]string{"main.go": "package main\n\nfunc Val() int { return 2 }\n\nfunc main() {}\n"},
			repair:     "package main\n\nfunc Val() int { return 3 }\n\nfunc main() {}\n",
			wantPhrase: "edits broke the tests",
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			h := newRepairHarness(t, func(e *Executor) { e.config.RepairMaxAttempts = 1 })
			repairTarget := "main_test.go"
			if tc.existing != "" {
				repairTarget = "main.go"
				if err := os.WriteFile(filepath.Join(h.ws, "main.go"), []byte("package main\n\nfunc Val() int { return 1 }\n\nfunc main() {}\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(h.ws, "main_test.go"), []byte(tc.existing), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			h.initial = func() *types.LLMToolResponse {
				var calls []types.ToolCall
				for _, f := range []string{"main.go", "main_test.go"} {
					if content, ok := tc.turnWrites[f]; ok {
						calls = append(calls, h.writeCall("c-"+f, "write_file", filepath.Join(h.ws, f), content))
					}
				}
				return &types.LLMToolResponse{Text: "writing", ToolCalls: calls}
			}
			var prompts []string
			h.onRepair = func(_ int, history []types.Message) *types.LLMToolResponse {
				prompts = append(prompts, history[len(history)-1].Text)
				return &types.LLMToolResponse{Text: "trying", ToolCalls: []types.ToolCall{
					h.writeCall("r1", "write_file", filepath.Join(h.ws, repairTarget), tc.repair),
				}}
			}
			_, err := h.drive(t, "fix the probe test")
			if err == nil || !strings.Contains(err.Error(), tc.wantPhrase) {
				t.Fatalf("error = %v, want %q", err, tc.wantPhrase)
			}
			joined := strings.Join(prompts, "\n")
			if got := strings.Contains(joined, "These failing tests are new: this turn wrote them (TestProbe)"); got != tc.wantNote {
				t.Fatalf("new-test note in the repair prompt = %v, want %v:\n%s", got, tc.wantNote, joined)
			}
		})
	}
}
