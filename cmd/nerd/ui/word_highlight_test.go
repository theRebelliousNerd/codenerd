package ui

import (
	"strings"
	"testing"

	"codenerd/internal/diff"
)

// stripANSI removes escape sequences so a rendered line can be compared as text.
func stripANSI(s string) string {
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == 0x1b {
			for i < len(s) && s[i] != 'm' {
				i++
			}
			continue
		}
		sb.WriteByte(s[i])
	}
	return sb.String()
}

func TestRenderLineWithWordHighlights_WhenSpansGiven_ShouldPreserveExactLineText(t *testing.T) {
	view := NewDiffApprovalView(DefaultStyles(), 120, 40)

	oldLine := "the quick brown fox"
	newLine := "the quick red fox"
	spans := diff.ComputeWordLevelDiff(oldLine, newLine)

	removed := view.renderLineWithWordHighlights(DiffLine{Content: oldLine, Type: DiffLineRemoved}, spans, true)
	added := view.renderLineWithWordHighlights(DiffLine{Content: newLine, Type: DiffLineAdded}, spans, false)

	if got := stripANSI(removed); got != "- "+oldLine {
		t.Errorf("removed line text = %q, want %q", got, "- "+oldLine)
	}
	if got := stripANSI(added); got != "+ "+newLine {
		t.Errorf("added line text = %q, want %q", got, "+ "+newLine)
	}
}

func TestRenderLineWithWordHighlights_WhenWordChanged_ShouldHighlightOnlyTheChangedRun(t *testing.T) {
	// Asserted on the segment plan rather than on ANSI output: lipgloss renders
	// unstyled when tests run without a TTY, so the escape codes are not a
	// reliable signal. The segment plan is what decides the styling.
	spans := diff.ComputeWordLevelDiff("value := compute(a)", "value := derive(a)")

	added := wordDiffSegments("+ ", "value := derive(a)", spans, false)
	var highlighted, plain strings.Builder
	for _, seg := range added {
		if seg.highlight {
			highlighted.WriteString(seg.text)
			continue
		}
		plain.WriteString(seg.text)
	}

	// Before word spans existed this path painted the whole line in one style
	// and dropped the comparison entirely.
	if highlighted.Len() == 0 {
		t.Fatalf("no highlighted run in %+v", added)
	}
	// dmp's semantic cleanup keeps a shared suffix ("e") equal, so the
	// highlighted run is a substring of the changed identifier rather than the
	// whole word — what matters is that it sits inside it and nowhere else.
	if !strings.Contains("derive", highlighted.String()) {
		t.Errorf("highlighted %q, want a run inside the changed identifier", highlighted.String())
	}
	if strings.Contains(highlighted.String(), "value") {
		t.Errorf("highlighted %q, want unchanged text left plain", highlighted.String())
	}

	removed := wordDiffSegments("- ", "value := compute(a)", spans, true)
	var removedHighlight strings.Builder
	for _, seg := range removed {
		if seg.highlight {
			removedHighlight.WriteString(seg.text)
		}
	}
	if removedHighlight.Len() == 0 || !strings.Contains("compute", removedHighlight.String()) {
		t.Errorf("removed-side highlight %q, want a run inside the old identifier", removedHighlight.String())
	}
}

func TestWordDiffSegments_WhenSpansDoNotMatchLine_ShouldFallBackToWholeLine(t *testing.T) {
	// Stale spans must never be allowed to print text the file does not contain.
	spans := []diff.WordSpan{{Type: diff.SpanEqual, Text: "something else"}}
	segs := wordDiffSegments("- ", "actual content", spans, true)

	if len(segs) != 1 || segs[0].text != "- actual content" || segs[0].highlight {
		t.Fatalf("segments=%+v, want a single unhighlighted whole-line segment", segs)
	}
}

func TestWordDiffSegments_WhenNoSpans_ShouldStillRenderTheLine(t *testing.T) {
	segs := wordDiffSegments("+ ", "added line", nil, false)
	if len(segs) != 1 || segs[0].text != "+ added line" {
		t.Fatalf("segments=%+v, want the whole line", segs)
	}
}

func TestSliceSegments_WhenScrolledRight_ShouldWindowAcrossSegmentBoundaries(t *testing.T) {
	segs := []styledSegment{
		{text: "- "},
		{text: "abc"},
		{text: "DEF", highlight: true},
		{text: "ghi"},
	}

	got := sliceSegments(segs, 3, 4)
	var sb strings.Builder
	for _, s := range got {
		sb.WriteString(s.text)
	}
	// Columns 3..6 of "- abcDEFghi" are "bcDE".
	if sb.String() != "bcDE" {
		t.Fatalf("window=%q, want %q", sb.String(), "bcDE")
	}

	// The window must not lose which part was highlighted.
	var sawHighlight bool
	for _, s := range got {
		if s.highlight && s.text == "DE" {
			sawHighlight = true
		}
	}
	if !sawHighlight {
		t.Errorf("scrolling dropped the highlight: %+v", got)
	}
}

func TestSliceSegments_WhenWidthNonPositive_ShouldReturnNothing(t *testing.T) {
	if got := sliceSegments([]styledSegment{{text: "abc"}}, 0, 0); len(got) != 0 {
		t.Fatalf("got %+v, want no segments", got)
	}
}

func TestCreateDiffFromStrings_ShouldUseTheSameEngineAsTheView(t *testing.T) {
	// The bridge used to run on diff.DefaultEngine while each view held its own
	// engine, so identical content was diffed and cached twice in caches with
	// different lifetimes. One engine per package removes that surprise.
	before := DiffEngineStats()
	_ = CreateDiffFromStrings("a.go", "a.go", "one\ntwo\n", "one\nthree\n")
	after := DiffEngineStats()

	if after.Computes == before.Computes {
		t.Fatal("CreateDiffFromStrings did not run on the UI engine")
	}

	view := NewDiffApprovalView(DefaultStyles(), 100, 40)
	if view.diffEngine != uiDiffEngine {
		t.Error("DiffApprovalView holds a private engine again; word diffs and file diffs would use separate caches")
	}
}
