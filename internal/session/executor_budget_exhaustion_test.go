package session

import (
	"testing"

	"codenerd/internal/types"
)

// When the tool loop runs out of iterations it makes one more call with the
// exploration tools removed. Which tools survive that cut decides whether a
// large task is truncated or made impossible.
//
// Stripping every tool is right for a query verb and wrong for a write verb.
// Live: `nerd create <architecture doc>` spent 35 tool calls researching, hit
// the ceiling, and the tool-free final call could only describe the file it had
// been asked to write. The hollow-success guard then correctly failed the turn,
// so the work was lost rather than merely incomplete.
func TestWriteOnlyToolDefinitions_KeepsWritesDropsExploration(t *testing.T) {
	defs := []types.ToolDefinition{
		{Name: "read_file"}, {Name: "write_file"}, {Name: "grep"},
		{Name: "edit_file"}, {Name: "glob"}, {Name: "list_files"},
		{Name: "delete_file"}, {Name: "search_code"}, {Name: "run_tests"},
	}

	got := writeOnlyToolDefinitions(defs)

	kept := map[string]bool{}
	for _, d := range got {
		kept[d.Name] = true
	}

	for _, want := range []string{"write_file", "edit_file", "delete_file"} {
		if !kept[want] {
			t.Errorf("%s was dropped; a write verb could then never land its deliverable", want)
		}
	}
	// The budget was exhausted precisely because reading is the cheap, endless
	// option. Handing read_file back guarantees the model spends the last call
	// on more exploration.
	for _, unwanted := range []string{"read_file", "grep", "glob", "list_files", "search_code", "run_tests"} {
		if kept[unwanted] {
			t.Errorf("%s survived the cut; the model will use it and the final call is wasted", unwanted)
		}
	}
}

func TestWriteOnlyToolDefinitions_EmptyInputIsEmptyOutput(t *testing.T) {
	if got := writeOnlyToolDefinitions(nil); len(got) != 0 {
		t.Errorf("writeOnlyToolDefinitions(nil) = %v, want empty", got)
	}
	readOnly := []types.ToolDefinition{{Name: "read_file"}, {Name: "grep"}}
	if got := writeOnlyToolDefinitions(readOnly); len(got) != 0 {
		t.Errorf("a read-only tool set must yield no write tools, got %v", got)
	}
}

// The two nudges must differ in the one way that matters: the write nudge has to
// forbid the specific failure it exists to prevent, which is answering with a
// description of the artifact instead of writing it.
func TestBudgetExhaustedNudges_TellTheModelDifferentThings(t *testing.T) {
	if readOnlyBudgetExhaustedNudge == writeBudgetExhaustedNudge {
		t.Fatal("a write turn and a query turn need different final instructions")
	}
	if !containsAll(writeBudgetExhaustedNudge, "write tool", "NOW") {
		t.Error("the write nudge must demand the artifact now, not a plan to produce it")
	}
	if !containsAll(readOnlyBudgetExhaustedNudge, "could not verify") {
		t.Error("the read-only nudge must ask for an honest account of gaps rather than more tools")
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		if !contains(s, sub) {
			return false
		}
	}
	return true
}

func contains(s, sub string) bool {
	return len(sub) == 0 || (len(s) >= len(sub) && indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
