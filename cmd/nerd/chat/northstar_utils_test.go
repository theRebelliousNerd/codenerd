package chat

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestParseFilePaths(t *testing.T) {
	input := "one.md, two.md\nthree.md"
	paths := parseFilePaths(input)
	if len(paths) != 3 {
		t.Fatalf("expected 3 paths, got %d", len(paths))
	}
	if paths[1] != "two.md" {
		t.Fatalf("expected trimmed path, got %q", paths[1])
	}
}

func TestExpandPath(t *testing.T) {
	workspace := t.TempDir()
	home := filepath.Join(workspace, "home")
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	got := expandPath(workspace, "~/docs/spec.md")
	if got != filepath.Join(home, "docs", "spec.md") {
		t.Fatalf("unexpected home expansion: %s", got)
	}

	got = expandPath(workspace, "notes.md")
	if got != filepath.Join(workspace, "notes.md") {
		t.Fatalf("unexpected relative expansion: %s", got)
	}

	abs := filepath.Join(workspace, "abs.md")
	got = expandPath(workspace, abs)
	if got != abs {
		t.Fatalf("expected absolute path to remain unchanged")
	}
}

func TestSplitAndTrim(t *testing.T) {
	input := " a, b\nc ,  d"
	items := splitAndTrim(input)
	if strings.Join(items, "|") != "a|b|c|d" {
		t.Fatalf("unexpected split results: %v", items)
	}
}

func TestParseTimeline(t *testing.T) {
	if parseTimeline("1") != "now" {
		t.Fatalf("expected now")
	}
	if parseTimeline("6 months") != "6mo" {
		t.Fatalf("expected 6mo")
	}
	if parseTimeline("moonshot") != "moonshot" {
		t.Fatalf("expected moonshot")
	}
}

func TestParsePriority(t *testing.T) {
	if parsePriority("1") != "critical" {
		t.Fatalf("expected critical")
	}
	if parsePriority("low") != "low" {
		t.Fatalf("expected low")
	}
	if parsePriority("unknown") != "medium" {
		t.Fatalf("expected medium default")
	}
}

func TestParseLikelihood(t *testing.T) {
	if parseLikelihood("h") != "high" {
		t.Fatalf("expected high")
	}
	if parseLikelihood("unlikely") != "low" {
		t.Fatalf("expected low")
	}
}

func TestParseReqType(t *testing.T) {
	if parseReqType("nf") != "non-functional" {
		t.Fatalf("expected non-functional")
	}
	if parseReqType("constraint") != "constraint" {
		t.Fatalf("expected constraint")
	}
}

func TestParseReqPriority(t *testing.T) {
	if parseReqPriority("must") != "must-have" {
		t.Fatalf("expected must-have")
	}
	if parseReqPriority("nice") != "nice-to-have" {
		t.Fatalf("expected nice-to-have")
	}
	if parseReqPriority("other") != "should-have" {
		t.Fatalf("expected should-have default")
	}
}

func TestTruncateWithEllipsis(t *testing.T) {
	if truncateWithEllipsis("short", 10) != "short" {
		t.Fatalf("expected short string unchanged")
	}
	if got := truncateWithEllipsis("longer string", 8); got != "longe..." {
		t.Fatalf("unexpected truncation: %s", got)
	}
}
