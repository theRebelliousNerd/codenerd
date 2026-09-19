package world

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A turn or planned step aimed at a file is served that file's outline -- every
// declaration with its current line range -- so the model can read and edit by
// range instead of spending its first calls finding its way (observed
// 2026-09-18: 40 read_file and 13 grep calls around a two-file change's 10
// edits). The outline is read fresh, so after an edit the ranges are current.
func TestPromptSection_ServesTheTargetsOutlineWithCurrentLineRanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.go")
	src := "package p\n\nfunc First() {}\n\nfunc Second() int {\n\treturn 2\n}\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	h := NewHolographicProvider(nil, dir)

	section := h.PromptSection(context.Background(), path)
	if !strings.Contains(section, "### Outline of target.go") {
		t.Fatalf("no outline in the section:\n%s", section)
	}
	for _, want := range []string{"3-3 function", "5-7 function"} {
		if !strings.Contains(section, want) {
			t.Errorf("outline lacks %q:\n%s", want, section)
		}
	}

	// An edit that moves Second down by two lines moves its range in the next request.
	moved := "package p\n\n// added\n// lines\nfunc First() {}\n\nfunc Second() int {\n\treturn 2\n}\n"
	if err := os.WriteFile(path, []byte(moved), 0o644); err != nil {
		t.Fatal(err)
	}
	after := h.PromptSection(context.Background(), path)
	if !strings.Contains(after, "7-9 function") {
		t.Errorf("the outline did not follow the edit (want Second at 7-9):\n%s", after)
	}
}

// A file with more declarations than the outline carries says how many it left
// out and where to get them; nothing is dropped silently.
func TestPromptSection_OutlineNamesWhatItLeavesOut(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.go")
	var b strings.Builder
	b.WriteString("package p\n\n")
	for i := 0; i < maxOutlineElements+7; i++ {
		fmt.Fprintf(&b, "func F%d() {}\n", i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	section := NewHolographicProvider(nil, dir).PromptSection(context.Background(), path)
	if !strings.Contains(section, "7 more declarations not listed") || !strings.Contains(section, "get_elements path=") {
		t.Errorf("the capped outline does not say what it left out:\n%s", section[max(0, len(section)-400):])
	}
}
