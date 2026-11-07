package codedom

import (
	"strings"
	"testing"
)

// mixedGuide: lines 1-6 end in LF, the rest in CRLF (the shape of a real
// documentation file edited by hand on two platforms).
const mixedGuide = "# Guide\n\nIntro text.\n\n## Install\n\nInstall steps.\r\n\r\n## Usage\r\n\r\nRun the tool.\r\n\r\n## Notes\r\n\r\nFinal notes.\r\n"

func TestMixedEndings_ElementEditsLeaveOtherLinesAlone(t *testing.T) {
	reg, abs, rel := editFixture(t, "GUIDE.md", mixedGuide)

	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "Guide > Usage", "old": "Run the tool.", "new": "Run it."}); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(mixedGuide, "Run the tool.", "Run it.", 1)
	if got := fileText(t, abs); got != want {
		t.Fatalf("edit_element in the CRLF region moved other bytes:\n got  %q\n want %q", got, want)
	}

	if _, _, err := execTool(reg, "edit_element", map[string]any{"path": rel, "ref": "Guide", "old": "Intro text.", "new": "Intro changed."}); err != nil {
		t.Fatal(err)
	}
	want = strings.Replace(want, "Intro text.", "Intro changed.", 1)
	if got := fileText(t, abs); got != want {
		t.Fatalf("edit_element in the LF region moved other bytes:\n got  %q\n want %q", got, want)
	}

	if _, _, err := execTool(reg, "replace_element", map[string]any{"path": rel, "ref": "Guide > Notes", "source": "## Notes\n\nNew notes.\n"}); err != nil {
		t.Fatal(err)
	}
	want = strings.Replace(want, "Final notes.", "New notes.", 1)
	if got := fileText(t, abs); got != want {
		t.Fatalf("replace_element moved bytes outside the section:\n got  %q\n want %q", got, want)
	}
}

func TestMixedEndings_LineEditsLeaveOtherLinesAlone(t *testing.T) {
	reg, abs, rel := editFixture(t, "GUIDE.md", mixedGuide)

	steps := []struct {
		verb string
		args map[string]any
		want func(string) string
	}{
		{"edit_lines", map[string]any{"path": rel, "start_line": 11, "end_line": 11, "new_content": "Run it."},
			func(s string) string { return strings.Replace(s, "Run the tool.\r\n", "Run it.\r\n", 1) }},
		{"insert_lines", map[string]any{"path": rel, "after_line": 3, "content": "Added."},
			func(s string) string { return strings.Replace(s, "Intro text.\n", "Intro text.\nAdded.\r\n", 1) }},
		{"delete_lines", map[string]any{"path": rel, "start_line": 2, "end_line": 2},
			func(s string) string { return strings.Replace(s, "# Guide\n\n", "# Guide\n", 1) }},
	}
	want := mixedGuide
	for _, st := range steps {
		if _, _, err := execTool(reg, st.verb, st.args); err != nil {
			t.Fatalf("%s: %v", st.verb, err)
		}
		want = st.want(want)
		if got := fileText(t, abs); got != want {
			t.Fatalf("%s moved bytes outside its lines:\n got  %q\n want %q", st.verb, got, want)
		}
	}
}

func TestMixedEndings_ApplyEditsBatchLeavesOtherLinesAlone(t *testing.T) {
	dir := t.TempDir()
	abs := writeFixture(t, dir, "GUIDE.md", mixedGuide)
	if _, err := executeApplyEdits(ctxForDir(dir), map[string]any{
		"edits": []any{
			map[string]any{"operation": "edit_lines", "path": "GUIDE.md", "start_line": 3, "end_line": 3, "new_content": "Intro changed."},
			map[string]any{"operation": "edit_lines", "path": "GUIDE.md", "start_line": 11, "end_line": 11, "new_content": "Run it."},
		},
	}); err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(strings.Replace(mixedGuide, "Intro text.", "Intro changed.", 1), "Run the tool.", "Run it.", 1)
	if got := fileText(t, abs); got != want {
		t.Fatalf("apply_edits moved bytes outside its lines:\n got  %q\n want %q", got, want)
	}
}
