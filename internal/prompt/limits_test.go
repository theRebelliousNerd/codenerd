package prompt

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// Truncation that the model cannot see is the failure mode these bounds exist
// to prevent: a model handed the first 4 KB of a 40 MB log with no marker will
// report that the build passed because the failure was in the part we dropped.
// Every case here asserts the marker as hard as it asserts the size.
func TestClampText(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		maxChars   int
		wantMarker bool
		wantHead   string
		wantTail   string
	}{
		{
			name:     "under the cap is returned verbatim",
			text:     "short",
			maxChars: 100,
			wantHead: "short",
			wantTail: "short",
		},
		{
			name:     "exactly at the cap is returned verbatim",
			text:     strings.Repeat("a", 100),
			maxChars: 100,
			wantHead: "aaa",
		},
		{
			name:       "over the cap keeps head and tail",
			text:       "HEAD" + strings.Repeat("x", 4000) + "TAIL",
			maxChars:   1000,
			wantMarker: true,
			wantHead:   "HEAD",
			wantTail:   "TAIL",
		},
		{
			name:       "below the head+tail floor degrades to head only",
			text:       "HEAD" + strings.Repeat("x", 4000) + "TAIL",
			maxChars:   50,
			wantMarker: true,
			wantHead:   "HEAD",
		},
		{
			name:     "zero budget yields nothing",
			text:     "anything",
			maxChars: 0,
		},
		{
			name:     "negative budget yields nothing",
			text:     "anything",
			maxChars: -5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampText(tt.text, tt.maxChars, "unit test")

			if IsClamped(got) != tt.wantMarker {
				t.Fatalf("IsClamped = %v, want %v; got %q", IsClamped(got), tt.wantMarker, got)
			}
			if tt.wantHead != "" && !strings.HasPrefix(got, tt.wantHead) {
				t.Errorf("result does not start with %q: %q", tt.wantHead, got[:min(len(got), 60)])
			}
			if tt.wantTail != "" && !strings.HasSuffix(got, tt.wantTail) {
				t.Errorf("result does not end with %q: %q", tt.wantTail, got[max(0, len(got)-60):])
			}
			if !utf8.ValidString(got) {
				t.Error("clamped result is not valid UTF-8")
			}
			// The marker is allowed to overshoot, but only by the marker.
			if tt.maxChars > 0 && len(got) > tt.maxChars+200 {
				t.Errorf("clamped length %d exceeds cap %d by more than the marker", len(got), tt.maxChars)
			}
		})
	}
}

// A byte-slice cut through a multi-byte rune produces U+FFFD, which reads to
// the model as corrupted content rather than truncated content.
func TestClampText_UTF8Boundaries(t *testing.T) {
	text := strings.Repeat("日本語テキスト", 400)
	for cap := 200; cap < 260; cap++ {
		got := ClampText(text, cap, "utf8")
		if !utf8.ValidString(got) {
			t.Fatalf("cap=%d produced invalid UTF-8", cap)
		}
		if strings.ContainsRune(got, '�') {
			t.Fatalf("cap=%d produced a replacement rune", cap)
		}
	}
}

func TestClampHead(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		maxChars   int
		wantMarker bool
	}{
		{name: "under cap", text: "short", maxChars: 50},
		{name: "over cap", text: strings.Repeat("z", 500), maxChars: 50, wantMarker: true},
		{name: "zero cap", text: "anything", maxChars: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClampHead(tt.text, tt.maxChars, "unit test")
			if IsClamped(got) != tt.wantMarker {
				t.Fatalf("IsClamped = %v, want %v", IsClamped(got), tt.wantMarker)
			}
			if !tt.wantMarker && tt.maxChars > 0 && got != tt.text {
				t.Errorf("unclamped text was modified: %q", got)
			}
		})
	}
}

// Line-oriented output must not be cut mid-line: half a compiler diagnostic
// reads as a complete diagnostic about a different thing.
func TestClampLines(t *testing.T) {
	lines := make([]string, 500)
	for i := range lines {
		lines[i] = "line"
	}
	lines[0] = "FIRST"
	lines[len(lines)-1] = "LAST"
	text := strings.Join(lines, "\n")

	got := ClampLines(text, 50, "unit test")
	if !IsClamped(got) {
		t.Fatal("expected a truncation marker")
	}
	if !strings.HasPrefix(got, "FIRST") {
		t.Error("head line was dropped")
	}
	if !strings.HasSuffix(got, "LAST") {
		t.Error("tail line was dropped — the tail is where the verdict lives")
	}
	if n := strings.Count(got, "\n") + 1; n > 60 {
		t.Errorf("kept %d lines, want <= ~50 plus marker", n)
	}
	if unchanged := ClampLines("a\nb", 50, "unit test"); unchanged != "a\nb" {
		t.Errorf("under-cap text was modified: %q", unchanged)
	}
}

func TestTruncationNotice(t *testing.T) {
	tests := []struct {
		name  string
		shown int
		total int
		want  bool
	}{
		{name: "nothing dropped", shown: 5, total: 5},
		{name: "all shown of fewer", shown: 5, total: 3},
		{name: "some dropped", shown: 5, total: 40, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TruncationNotice(tt.shown, tt.total, "rows")
			if (got != "") != tt.want {
				t.Fatalf("TruncationNotice(%d,%d) = %q, want empty=%v", tt.shown, tt.total, got, !tt.want)
			}
			if tt.want && !IsClamped(got) {
				t.Errorf("notice %q is not recognised as a truncation marker", got)
			}
		})
	}
}

// The tests below pin the line-snapping behaviour of ClampText against the
// output shapes it actually sees in production: `go test` results and compiler
// diagnostics arriving as tool results.

func TestClampText_ShouldNotEmitAPartialLine(t *testing.T) {
	// A byte-position cut lands mid-line most of the time, and a half-line
	// reads to the model as a whole record. "FAIL github.com/x/parser" cut
	// after "...x/pars" names a package that does not exist.
	var b strings.Builder
	for i := 0; i < 400; i++ {
		fmt.Fprintf(&b, "ok  \tgithub.com/example/project/pkg%03d\t0.0%02ds\n", i, i%100)
	}
	b.WriteString("--- FAIL: TestCancelDuringParse (2.01s)\n")
	b.WriteString("    parser_test.go:88: timed out waiting for Parse to return\n")
	b.WriteString("FAIL\tgithub.com/example/project/parser\t2.014s\n")
	full := b.String()

	got := ClampText(full, 2000, "tool result")

	if !IsClamped(got) {
		t.Fatal("expected a truncation marker")
	}

	head, tail, found := strings.Cut(got, clampMarkerPrefix)
	if !found {
		t.Fatal("marker not found")
	}
	// Whatever survives on each side must consist of whole lines.
	if head != "" && !strings.HasSuffix(head, "\n") {
		last := head[strings.LastIndexByte(head[:len(head)-1], '\n')+1:]
		t.Errorf("head ends mid-line with %q", last)
	}
	if idx := strings.IndexByte(tail, '\n'); idx >= 0 {
		afterMarker := tail[idx+1:]
		if afterMarker != "" {
			firstLine := afterMarker
			if i := strings.IndexByte(firstLine, '\n'); i >= 0 {
				firstLine = firstLine[:i]
			}
			if !strings.Contains(full, "\n"+firstLine) {
				t.Errorf("tail starts mid-line with %q", firstLine)
			}
		}
	}
}

func TestClampText_ShouldKeepTheVerdictLine(t *testing.T) {
	// The whole reason truncation is head+tail: the line the turn exists to act
	// on is the last one.
	var b strings.Builder
	for i := 0; i < 500; i++ {
		fmt.Fprintf(&b, "ok  \tgithub.com/example/project/pkg%03d\t0.010s\n", i)
	}
	b.WriteString("FAIL\tgithub.com/example/project/parser\t2.014s\n")
	b.WriteString("FAIL\n")

	got := ClampText(b.String(), 1500, "tool result")

	if !strings.Contains(got, "FAIL\tgithub.com/example/project/parser") {
		t.Error("the failing package line was truncated away; head-only truncation on test " +
			"output removes exactly the line the turn exists to act on")
	}
}

func TestClampText_WhenOneEnormousLine_ShouldStillTruncate(t *testing.T) {
	// A minified bundle or a base64 blob is one line of megabytes. Snapping
	// unconditionally would discard the whole excerpt chasing a newline that
	// never arrives, so past the snap budget the raw cut is correct.
	oneLine := strings.Repeat("x", 100000)

	got := ClampText(oneLine, 1000, "tool result")

	if !IsClamped(got) {
		t.Fatal("expected a truncation marker")
	}
	body := strings.ReplaceAll(got, "\n", "")
	if len(body) < 500 {
		t.Errorf("snapping ate the excerpt: kept %d chars of a %d-char single line",
			len(body), len(oneLine))
	}
}

func TestClampText_ShouldNotSnapAcrossAWholeBudget(t *testing.T) {
	// Lines longer than the snap budget must not cause a cut to travel an
	// unbounded distance backwards.
	line := strings.Repeat("y", 900) + "\n"
	text := strings.Repeat(line, 20)

	got := ClampText(text, 1200, "tool result")

	if !IsClamped(got) {
		t.Fatal("expected a truncation marker")
	}
	head, _, _ := strings.Cut(got, clampMarkerPrefix)
	if len(head) < 300 {
		t.Errorf("head collapsed to %d chars snapping to a line boundary 900 chars away", len(head))
	}
}

func TestClampText_ShouldPreserveShortTextExactly(t *testing.T) {
	const s = "one\ntwo\nthree\n"
	if got := ClampText(s, 1000, "unit test"); got != s {
		t.Errorf("text under budget was modified: %q", got)
	}
}
