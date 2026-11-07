package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
	codedom "codenerd/internal/tools/codedom"
	coretools "codenerd/internal/tools/core"
	"codenerd/internal/types"
)

// mixedGuide has the shape of the file the defect was observed on: its first
// six lines end in LF and every other line in CRLF, so CRLF is the dominant
// ending and a whole-file renormalization would rewrite the six LF lines.
const mixedGuide = "# Guide\n" +
	"\n" +
	"Intro text.\n" +
	"\n" +
	"## Install\n" +
	"\n" +
	"Install steps.\r\n" +
	"\r\n" +
	"## Usage\r\n" +
	"\r\n" +
	"Run the tool.\r\n" +
	"\r\n" +
	"## Notes\r\n" +
	"\r\n" +
	"Final notes.\r\n"

// mixedFixture writes mixedGuide into a fresh workspace and returns a tool
// registry rooted there with the CodeDOM and core file tools registered
// through their production constructors.
func mixedFixture(t *testing.T) (*tools.Registry, string, string) {
	t.Helper()
	dir := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	abs := filepath.Join(dir, "GUIDE.md")
	if err := os.WriteFile(abs, []byte(mixedGuide), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := tools.NewRegistry()
	if err := codedom.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	if err := coretools.RegisterAll(reg); err != nil {
		t.Fatal(err)
	}
	reg.SetWorkspaceRoot(dir)
	return reg, dir, abs
}

// assertMixedBytes compares the file with the exact bytes expected and names
// the first line that differs, so a churned ending outside the edit is easy to
// see.
func assertMixedBytes(t *testing.T, abs, want string) {
	t.Helper()
	got, err := os.ReadFile(abs)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) == want {
		return
	}
	gl, wl := strings.SplitAfter(string(got), "\n"), strings.SplitAfter(want, "\n")
	for i := 0; i < len(gl) && i < len(wl); i++ {
		if gl[i] != wl[i] {
			t.Fatalf("line %d differs\n got  %q\n want %q\nfull file:\n%q", i+1, gl[i], wl[i], got)
		}
	}
	t.Fatalf("file length differs: got %d lines, want %d\n%q", len(gl), len(wl), got)
}

// assertValidated runs the production validator pipeline over a finished
// action and requires every validator to accept it.
func assertValidated(t *testing.T, typ ActionType, abs string, payload map[string]any) {
	t.Helper()
	registry := NewValidatorRegistry()
	RegisterAllValidators(registry)
	req := ActionRequest{Type: typ, Target: abs, Payload: payload}
	got := registry.Validate(t.Context(), req, ActionResult{Success: true})
	if state := SummarizeValidation(abs, got).State; state != types.ToolValidationVerified {
		t.Fatalf("%s on a mixed-ending file was not verified (%s): %+v", typ, state, got)
	}
}

func execMixed(t *testing.T, reg *tools.Registry, verb string, args map[string]any) {
	t.Helper()
	if _, err := reg.Execute(t.Context(), verb, args); err != nil {
		t.Fatalf("%s: %v", verb, err)
	}
}

// edit_element in a CRLF section and in an LF section: no byte outside the
// edited line moves.
func TestMixedLineEndings_EditElementTouchesOnlyItsLine(t *testing.T) {
	reg, _, abs := mixedFixture(t)

	execMixed(t, reg, "edit_element", map[string]any{
		"path": "GUIDE.md", "ref": "Guide > Usage", "old": "Run the tool.", "new": "Run the tool with care.",
	})
	want := strings.Replace(mixedGuide, "Run the tool.", "Run the tool with care.", 1)
	assertMixedBytes(t, abs, want)
	assertValidated(t, ActionEditElement, abs, map[string]any{"ref": "GUIDE.md:Guide > Usage"})

	execMixed(t, reg, "edit_element", map[string]any{
		"path": "GUIDE.md", "ref": "Guide", "old": "Intro text.", "new": "Intro changed.",
	})
	want = strings.Replace(want, "Intro text.", "Intro changed.", 1)
	assertMixedBytes(t, abs, want)
	assertValidated(t, ActionEditElement, abs, map[string]any{"ref": "GUIDE.md:Guide"})
}

// replace_element swaps a whole section: the lines before and after it keep
// their bytes, the replaced lines keep the endings of the lines they replace,
// and an added line takes the file's dominant ending.
func TestMixedLineEndings_ReplaceElementKeepsLinesAround(t *testing.T) {
	reg, _, abs := mixedFixture(t)

	execMixed(t, reg, "replace_element", map[string]any{
		"path": "GUIDE.md", "ref": "Guide > Notes", "source": "## Notes\n\nNew notes.\nMore notes.\n",
	})
	want := strings.Replace(mixedGuide, "## Notes\r\n\r\nFinal notes.\r\n", "## Notes\r\n\r\nNew notes.\r\nMore notes.\r\n", 1)
	assertMixedBytes(t, abs, want)
	assertValidated(t, ActionEditElement, abs, map[string]any{"ref": "GUIDE.md:Guide > Notes"})
}

func TestMixedLineEndings_EditLinesTouchesOnlyItsLines(t *testing.T) {
	reg, _, abs := mixedFixture(t)

	execMixed(t, reg, "edit_lines", map[string]any{
		"path": "GUIDE.md", "start_line": 11, "end_line": 11, "new_content": "Run it.",
	})
	want := strings.Replace(mixedGuide, "Run the tool.", "Run it.", 1)
	assertMixedBytes(t, abs, want)
	assertValidated(t, ActionEditLines, abs, map[string]any{"path": "GUIDE.md", "start_line": float64(11), "end_line": float64(11)})

	// An edit in the LF region of the same file.
	execMixed(t, reg, "edit_lines", map[string]any{
		"path": "GUIDE.md", "start_line": 3, "end_line": 3, "new_content": "Intro changed.",
	})
	want = strings.Replace(want, "Intro text.", "Intro changed.", 1)
	assertMixedBytes(t, abs, want)
	assertValidated(t, ActionEditLines, abs, map[string]any{"path": "GUIDE.md", "start_line": float64(3), "end_line": float64(3)})
}

func TestMixedLineEndings_EditFileTouchesOnlyItsLines(t *testing.T) {
	reg, _, abs := mixedFixture(t)

	args := map[string]any{"path": "GUIDE.md", "old_text": "Run the tool.\n\n## Notes", "new_text": "Run the tool twice.\n\n## Notes"}
	execMixed(t, reg, "edit_file", args)
	want := strings.Replace(mixedGuide, "Run the tool.", "Run the tool twice.", 1)
	assertMixedBytes(t, abs, want)
	assertValidated(t, ActionEditFile, abs, args)

	args = map[string]any{"path": "GUIDE.md", "old_text": "Intro text.", "new_text": "Intro changed."}
	execMixed(t, reg, "edit_file", args)
	want = strings.Replace(want, "Intro text.", "Intro changed.", 1)
	assertMixedBytes(t, abs, want)
	assertValidated(t, ActionEditFile, abs, args)
}

// write_file over an existing file: the model re-sends the whole file in LF;
// every line it did not change keeps its original bytes, and the file
// validators accept the result.
func TestMixedLineEndings_WriteFileOverExistingKeepsUnchangedLines(t *testing.T) {
	reg, _, abs := mixedFixture(t)

	content := strings.ReplaceAll(mixedGuide, "\r\n", "\n")
	content = strings.Replace(content, "Run the tool.", "Run the tool again.", 1)
	args := map[string]any{"path": "GUIDE.md", "content": content}
	execMixed(t, reg, "write_file", args)
	want := strings.Replace(mixedGuide, "Run the tool.", "Run the tool again.", 1)
	assertMixedBytes(t, abs, want)
	assertValidated(t, ActionWriteFile, abs, args)

	if got := NewFileWriteValidator().Validate(t.Context(), ActionRequest{Type: ActionWriteFile, Target: abs, Payload: args}, ActionResult{Success: true}); !got.Verified {
		t.Fatalf("file write validator rejected a spliced mixed file: %+v", got)
	}
	if got := NewParanoidFileValidator().Validate(t.Context(), ActionRequest{Type: ActionWriteFile, Target: abs, Payload: args}, ActionResult{Success: true}); !got.Verified {
		t.Fatalf("paranoid validator rejected a spliced mixed file: %+v", got)
	}

	// Content that really differs is still refused.
	if err := os.WriteFile(abs, []byte(mixedGuide), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := NewFileWriteValidator().Validate(t.Context(), ActionRequest{Type: ActionWriteFile, Target: abs, Payload: args}, ActionResult{Success: true}); got.Verified {
		t.Fatal("different content accepted by the file write validator")
	}
	if got := NewParanoidFileValidator().Validate(t.Context(), ActionRequest{Type: ActionWriteFile, Target: abs, Payload: args}, ActionResult{Success: true}); got.Verified {
		t.Fatal("different content accepted by the paranoid validator")
	}
}

// The VirtualStore write and edit handlers are the kernel's own path to the
// same files.
func TestMixedLineEndings_VirtualStoreHandlersKeepUnchangedLines(t *testing.T) {
	vs, dir := createActionsTestVS(t)
	abs := filepath.Join(dir, "GUIDE.md")
	if err := os.WriteFile(abs, []byte(mixedGuide), 0o644); err != nil {
		t.Fatal(err)
	}

	edit, err := vs.handleEditFile(t.Context(), ActionRequest{
		ActionID: "mixed-edit", Target: "GUIDE.md",
		Payload: map[string]any{"old": "Run the tool.", "new": "Run it."},
	})
	if err != nil || !edit.Success {
		t.Fatalf("edit handler: %v %+v", err, edit)
	}
	want := strings.Replace(mixedGuide, "Run the tool.", "Run it.", 1)
	assertMixedBytes(t, abs, want)

	content := strings.ReplaceAll(want, "\r\n", "\n")
	content = strings.Replace(content, "Intro text.", "Intro changed.", 1)
	write, err := vs.handleWriteFile(t.Context(), ActionRequest{
		ActionID: "mixed-write", Target: "GUIDE.md",
		Payload: map[string]any{"content": content},
	})
	if err != nil || !write.Success {
		t.Fatalf("write handler: %v %+v", err, write)
	}
	assertMixedBytes(t, abs, strings.Replace(want, "Intro text.", "Intro changed.", 1))
}
