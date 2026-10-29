package core

import "testing"

// Regression test: CodeDOM element edits must resolve through the interactive
// executive gate (Dreamer preflight + post-action validators) like the line
// tools do. An unmapped "edit_element" would silently skip both.
func TestActionTypeForToolName_EditElement(t *testing.T) {
	t.Parallel()

	at, ok := actionTypeForToolName("edit_element")
	if !ok {
		t.Fatal(`actionTypeForToolName("edit_element") not mapped, want ActionEditElement`)
	}
	if at != ActionEditElement {
		t.Fatalf(`actionTypeForToolName("edit_element") = %q, want %q`, at, ActionEditElement)
	}
}

func TestActionTypeForToolName_Unknown(t *testing.T) {
	t.Parallel()

	if at, ok := actionTypeForToolName("definitely_not_a_tool"); ok {
		t.Fatalf("actionTypeForToolName(unknown) = (%q, true), want ok == false", at)
	}
}
