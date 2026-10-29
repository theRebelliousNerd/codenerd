package world

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
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

// A file with more declarations than the outline's share holds says how many
// it left out and where to get them; nothing is dropped silently.
//
// Thirty funcs start at line 10, so every uncut entry is the same 30 bytes:
// "- 10-10 function `func F00()`\n". 10% of 1000 is 100 bytes: 100/30 = 3
// render, and 27 remain. get_elements reads the rest.
func TestPromptSection_OutlineNamesWhatItLeavesOut(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.go")
	var b strings.Builder
	b.WriteString("package p\n")
	for line := 2; line < 10; line++ {
		b.WriteString("\n")
	}
	const n = 30
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "func F%02d() {}\n", i)
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	const wantLine = "- 10-10 function `func F00()`\n"
	if len(wantLine) != 30 {
		t.Fatalf("hand-computed outline line is %d bytes, want 30", len(wantLine))
	}
	section, seen := budgetedRender(t, NewHolographicProvider(nil, dir), path, func(c *config.WorkingConfig) {
		c.HolographicOutlineSharePercent = 10
	}, 1000)
	m := dimensionMeasurement(t, seen, holoOutline)
	if m.total != n || m.avg != len(wantLine) {
		t.Fatalf("measured outline pool = (%d entries, %d bytes), want (%d, %d)", m.total, m.avg, n, len(wantLine))
	}
	if strings.Count(section, "function `func F") != 3 {
		t.Fatalf("rendered outline entries, want the derived 3:\n%s", section)
	}
	if !strings.Contains(section, "function `func F00()`") || !strings.Contains(section, "function `func F02()`") {
		t.Fatalf("the first three entries must render whole:\n%s", section)
	}
	if strings.Contains(section, "function `func F03()`") {
		t.Fatalf("F03 renders past the derived count:\n%s", section)
	}
	if !strings.Contains(section, "27 more declarations not listed") || !strings.Contains(section, "get_elements path=") {
		t.Errorf("the capped outline does not say what it left out:\n%s", section[max(0, len(section)-400):])
	}
}
