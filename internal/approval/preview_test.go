package approval

import (
	"fmt"
	"strings"
	"testing"
	"unicode/utf8"
)

// A decision preview is recomputed from its exact inputs every time: a cache
// hit that served another input's hunks would show a person one change and
// authorize another (internal/diff TestDiffImporters_AreDisplayOnlyOrDecisionSafe).
func TestPreviewEngine_NeverServesFromCache(t *testing.T) {
	before := previewEngine.Stats()
	for i := 0; i < 3; i++ {
		if _, err := BuildPreview(PreviewInput{Summary: "s", MaxBytes: 1024,
			File: &FileChange{Path: "same.txt", Before: []byte("identical\n"), Exists: true}}); err != nil {
			t.Fatal(err)
		}
	}
	after := previewEngine.Stats()
	if after.Hits != before.Hits {
		t.Fatalf("the decision preview engine served %d diffs from cache", after.Hits-before.Hits)
	}
	if after.Entries != 0 {
		t.Fatalf("the decision preview engine retained %d cache entries", after.Entries)
	}
}

func TestBuildPreview_TruncationPreservesUTF8Boundaries(t *testing.T) {
	cases := []struct {
		name         string
		arguments    string
		limit        int
		wantText     string
		wantComplete bool
	}{
		{name: "empty", arguments: "", limit: 1, wantText: "", wantComplete: true},
		{name: "ascii at limit", arguments: "abc", limit: 3, wantText: "abc", wantComplete: true},
		{name: "ascii over limit", arguments: "abcd", limit: 3, wantText: "abc"},
		{name: "multibyte at limit", arguments: "é", limit: 2, wantText: "é", wantComplete: true},
		{name: "inside first rune", arguments: "éx", limit: 1, wantText: ""},
		{name: "inside later rune", arguments: "a界z", limit: 3, wantText: "a"},
		{name: "after complete rune", arguments: "a界z", limit: 4, wantText: "a界"},
		{name: "inside four byte rune", arguments: "a🙂z", limit: 4, wantText: "a"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := BuildPreview(PreviewInput{Summary: "bounded arguments", Arguments: tt.arguments, MaxBytes: tt.limit})
			if err != nil {
				t.Fatalf("BuildPreview() error = %v", err)
			}
			if p.Text != tt.wantText {
				t.Errorf("Text = %q, want %q", p.Text, tt.wantText)
			}
			if !utf8.ValidString(p.Text) || len(p.Text) > tt.limit {
				t.Errorf("Text = %q is not valid UTF-8 within %d bytes", p.Text, tt.limit)
			}
			if p.Complete != tt.wantComplete {
				t.Errorf("Complete = %v, want %v", p.Complete, tt.wantComplete)
			}
			if tt.wantComplete {
				if len(p.Incomplete) != 0 {
					t.Errorf("complete preview has withholding reasons: %v", p.Incomplete)
				}
			} else {
				wantReason := fmt.Sprintf("arguments truncated: %d of %d bytes shown", len(tt.wantText), len(tt.arguments))
				if len(p.Incomplete) != 1 || p.Incomplete[0] != wantReason {
					t.Errorf("Incomplete = %v, want [%s]", p.Incomplete, wantReason)
				}
			}
			if !p.Verify() {
				t.Fatal("bounded preview does not verify")
			}
		})
	}
}

func TestBuildPreview_WithholdsInvalidUTF8FileContents(t *testing.T) {
	cases := []struct {
		name   string
		before []byte
		after  []byte
	}{
		{name: "invalid before", before: []byte{0xff}, after: []byte("valid\n")},
		{name: "invalid after", before: []byte("valid\n"), after: []byte{0xff}},
		{name: "incomplete rune", before: []byte("valid\n"), after: []byte{0xe7, 0x95}},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := BuildPreview(PreviewInput{Summary: "change file", MaxBytes: 64,
				File: &FileChange{Path: "file.txt", Before: tt.before, After: tt.after, Exists: true}})
			if err != nil {
				t.Fatalf("BuildPreview() error = %v", err)
			}
			if p.Complete || p.Diff != nil {
				t.Fatalf("invalid UTF-8 content was presented as a complete diff: %+v", p)
			}
			const wantReason = "file.txt is not valid UTF-8; its bytes are not shown"
			if len(p.Incomplete) != 1 || p.Incomplete[0] != wantReason {
				t.Errorf("Incomplete = %v, want [%s]", p.Incomplete, wantReason)
			}
			if !strings.Contains(p.String(), "not shown: "+wantReason) {
				t.Errorf("plain-text preview omits the withholding reason: %q", p.String())
			}
			if !p.Verify() {
				t.Fatal("withheld preview does not verify")
			}
		})
	}
}

func TestBuildPreview_RejectsNonPositiveLimits(t *testing.T) {
	cases := []struct {
		name  string
		limit int
	}{
		{name: "zero", limit: 0},
		{name: "negative", limit: -1},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			p, err := BuildPreview(PreviewInput{Summary: "invalid limit", Arguments: "payload", MaxBytes: tt.limit})
			if err == nil {
				t.Fatal("BuildPreview() accepted a non-positive limit")
			}
			if p.Digest != "" || p.Verify() || p.Complete || p.Text != "" || p.Diff != nil {
				t.Fatalf("invalid limit returned a usable preview: %+v", p)
			}
		})
	}
}
