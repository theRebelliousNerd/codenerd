package prompt

import (
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
