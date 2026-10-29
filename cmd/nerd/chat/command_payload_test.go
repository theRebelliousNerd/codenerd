package chat

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// submit sends one textarea submission through the real handleSubmit route.
func submitText(t *testing.T, m Model, value string) Model {
	t.Helper()
	m.textarea.SetValue(value)
	next, _ := m.handleSubmit()
	return next.(Model)
}

func readWorkspaceFile(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(b)
}

func TestSlashWrite_SubmissionReachesTheFileByteForByte(t *testing.T) {
	cases := map[string]struct{ submission, want string }{
		"repeated spaces":     {"/write a.txt alpha  beta", "alpha  beta"},
		"json string spacing": {`/write a.json {"k": "a  b"}`, `{"k": "a  b"}`},
		"trailing whitespace": {"/write a.txt keep me  ", "keep me  "},
		"multiline after newline": {
			"/write a.py\ndef f():\n    return 1\n\n\nprint(f())\n",
			"def f():\n    return 1\n\n\nprint(f())\n",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			m := submitText(t, NewTestModel(WithWorkspace(dir)), tc.submission)
			file := strings.Fields(tc.submission)[1]
			if got := readWorkspaceFile(t, dir, file); got != tc.want {
				t.Fatalf("file = %q, want %q", got, tc.want)
			}
			if reply := lastAssistant(m); !strings.Contains(reply, "Wrote ") || !strings.Contains(reply, file) {
				t.Fatalf("reply = %q", reply)
			}
		})
	}
}

// Tabs cannot pass the bubbles textarea (it expands them), so the
// dispatcher boundary is exercised directly.
func TestSlashWrite_DispatcherKeepsTabsAndCarriageReturns(t *testing.T) {
	dir := t.TempDir()
	m := NewTestModel(WithWorkspace(dir))
	next, _ := m.handleCommand("/write main.go\r\npackage main\r\n\tfunc f() {}\t\n")
	if got := readWorkspaceFile(t, dir, "main.go"); got != "package main\r\n\tfunc f() {}\t\n" {
		t.Fatalf("file = %q", got)
	}
	if reply := lastAssistant(next.(Model)); !strings.Contains(reply, "Wrote 28 bytes") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestSlashWrite_RefusesBeforeMutation(t *testing.T) {
	for name, raw := range map[string]string{
		"control byte":  "/write keep.txt bad\x1b[31m",
		"empty payload": "/write keep.txt ",
		"no payload":    "/write keep.txt",
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "keep.txt"), []byte("original"), 0o644); err != nil {
				t.Fatal(err)
			}
			next, _ := NewTestModel(WithWorkspace(dir)).handleCommand(raw)
			if got := readWorkspaceFile(t, dir, "keep.txt"); got != "original" {
				t.Fatalf("file changed to %q", got)
			}
			if reply := lastAssistant(next.(Model)); strings.Contains(reply, "Wrote") {
				t.Fatalf("refusal reported as a write: %q", reply)
			}
		})
	}
}

func TestSlashAppend_KeepsPayloadAndAddsOneDeclaredNewline(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "log.txt"), []byte("head\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := submitText(t, NewTestModel(WithWorkspace(dir)), "/append log.txt a  b")
	m = submitText(t, m, "/append log.txt two\nlines\n")
	if got := readWorkspaceFile(t, dir, "log.txt"); got != "head\na  b\ntwo\nlines\n" {
		t.Fatalf("file = %q", got)
	}
	if reply := lastAssistant(m); !strings.Contains(reply, "Appended 10 bytes") {
		t.Fatalf("reply = %q", reply)
	}
}

func TestPatchMode_LineByLineKeepsContextMarkersAndApplies(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("a\n\nb  \nc\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewTestModel(WithWorkspace(dir), WithInputMode(InputModePatch))
	// The working directory is not the workspace: the patch must apply
	// to the workspace's file all the same.
	for _, line := range []string{
		"--- a/f.txt",
		"+++ b/f.txt",
		"@@ -1,4 +1,4 @@",
		" a",
		" ",
		"-b  ",
		"+B  ",
		" c",
	} {
		m = submitText(t, m, line)
	}
	if m.pendingPatchLines[3] != " a" || m.pendingPatchLines[4] != " " || m.pendingPatchLines[5] != "-b  " {
		t.Fatalf("ingestion changed the patch: %q", m.pendingPatchLines)
	}
	m = submitText(t, m, "--END--")
	if reply := lastAssistant(m); reply != "Patch applied." {
		t.Fatalf("reply = %q", reply)
	}
	if got := readWorkspaceFile(t, dir, "f.txt"); got != "a\n\nB  \nc\n" {
		t.Fatalf("file = %q", got)
	}
}

func TestPatchMode_ChunkSizeDoesNotChangeTheAdmittedPatch(t *testing.T) {
	lines := []string{"@@ -1,2 +1,2 @@", " keep ", "", "-x", "+y"}
	one := NewTestModel(WithInputMode(InputModePatch))
	for _, l := range lines {
		one = submitText(t, one, l)
	}
	chunked := NewTestModel(WithInputMode(InputModePatch))
	chunked = submitText(t, chunked, strings.Join(lines[:3], "\n")+"\n")
	chunked = submitText(t, chunked, strings.Join(lines[3:], "\n"))
	a, b := strings.Join(one.pendingPatchLines, "\n"), strings.Join(chunked.pendingPatchLines, "\n")
	if a != b {
		t.Fatalf("line by line %q != chunked %q", a, b)
	}
	// A chunk holding the terminator text among other lines is data.
	chunked = submitText(t, chunked, "--END--\n more")
	if chunked.inputMode != InputModePatch {
		t.Fatal("a multi-line chunk ended patch mode")
	}
}

func TestPatchMode_TerminatorWithNoLinesAppliesNothing(t *testing.T) {
	m := submitText(t, NewTestModel(WithInputMode(InputModePatch)), "--END--")
	if m.inputMode == InputModePatch || !strings.Contains(lastAssistant(m), "nothing was applied") {
		t.Fatalf("mode=%v reply=%q", m.inputMode, lastAssistant(m))
	}
}

func TestPaste_BeyondTheInputLimitIsReportedNotSilentlyCut(t *testing.T) {
	m := NewTestModel()
	m.textarea.CharLimit = 10
	m.textarea.Focus()
	next, _, _ := m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("0123456789abcdef"), Paste: true})
	if got := next.textarea.Value(); got != "0123456789" {
		t.Fatalf("box = %q", got)
	}
	if note := lastAssistant(next); !strings.Contains(note, "6 of the 16 characters") {
		t.Fatalf("note = %q", note)
	}

	m = NewTestModel()
	m.textarea.Focus()
	next, _, _ = m.handleKeyMsg(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a\tb"), Paste: true})
	if note := lastAssistant(next); note != "" {
		t.Fatalf("a whole paste produced a note: %q", note)
	}
}
