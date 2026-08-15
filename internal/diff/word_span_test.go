package diff

import (
	"strings"
	"testing"
)

func TestComputeWordLevelDiff_WhenLinesDiffer_ShouldReturnCodeNERDSpans(t *testing.T) {
	e := NewEngine()
	spans := e.ComputeWordLevelDiff("the quick brown fox", "the quick red fox")

	if len(spans) == 0 {
		t.Fatal("expected spans for a changed line pair")
	}

	// The spans must reconstruct both sides exactly; a renderer that trusts them
	// would otherwise print text neither file contains.
	var oldSide, newSide strings.Builder
	var sawDelete, sawInsert bool
	for _, s := range spans {
		if s.Text == "" {
			t.Fatal("empty span: renderers would emit a zero-width styled run")
		}
		switch s.Type {
		case SpanEqual:
			oldSide.WriteString(s.Text)
			newSide.WriteString(s.Text)
		case SpanDelete:
			oldSide.WriteString(s.Text)
			sawDelete = true
		case SpanInsert:
			newSide.WriteString(s.Text)
			sawInsert = true
		}
	}

	if oldSide.String() != "the quick brown fox" {
		t.Errorf("delete+equal spans rebuild %q, want the old line", oldSide.String())
	}
	if newSide.String() != "the quick red fox" {
		t.Errorf("insert+equal spans rebuild %q, want the new line", newSide.String())
	}
	if !sawDelete || !sawInsert {
		t.Errorf("expected both a delete and an insert span, got %+v", spans)
	}
}

func TestComputeWordLevelDiff_WhenLinesIdentical_ShouldReturnOnlyEqualSpans(t *testing.T) {
	spans := ComputeWordLevelDiff("same line", "same line")
	for _, s := range spans {
		if s.Type != SpanEqual {
			t.Fatalf("identical lines produced a %v span (%q)", s.Type, s.Text)
		}
	}
}

func TestComputeWordLevelDiff_WhenOneSideEmpty_ShouldReturnWholeSideAsOneSpan(t *testing.T) {
	spans := ComputeWordLevelDiff("", "brand new")
	if len(spans) != 1 || spans[0].Type != SpanInsert || spans[0].Text != "brand new" {
		t.Fatalf("spans=%+v, want a single insert span", spans)
	}
}

func TestComputeDiff_WhenAnyInput_ShouldNeverEmitLineHeader(t *testing.T) {
	// LineHeader is a UI-owned member of the enum: hunk framing lives in the
	// Hunk counters, and a renderer synthesizes its own "@@" row. If the engine
	// ever starts emitting one, every consumer's line arithmetic silently
	// shifts by a line per hunk.
	cases := []struct{ name, oldContent, newContent string }{
		{"insert", "a\nb\n", "a\nb\nc\n"},
		{"delete", "a\nb\nc\n", "a\n"},
		{"replace", "a\nb\nc\n", "a\nX\nc\n"},
		{"multi-hunk", strings.Repeat("ctx\n", 40) + "old\n" + strings.Repeat("ctx\n", 40) + "old2\n",
			strings.Repeat("ctx\n", 40) + "new\n" + strings.Repeat("ctx\n", 40) + "new2\n"},
		{"empty-old", "", "only\n"},
		{"empty-new", "only\n", ""},
		{"trailing-newline", "a", "a\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fd := NewEngine().ComputeDiff("a", "b", tc.oldContent, tc.newContent)
			for _, h := range fd.Hunks {
				for _, l := range h.Lines {
					if l.Type == LineHeader {
						t.Fatalf("engine emitted LineHeader for %q -> %q", tc.oldContent, tc.newContent)
					}
				}
			}
		})
	}
}

func TestCache_WhenVerifyEnabledAndKeyCollides_ShouldRecomputeRatherThanServeWrongDiff(t *testing.T) {
	e := NewEngineWith(Options{VerifyCacheContent: true})

	oldContent := "alpha\nbeta\n"
	newContent := "alpha\ngamma\n"
	want := e.ComputeDiff("f", "f", oldContent, newContent)
	if want == nil {
		t.Fatal("nil diff")
	}

	// Forge a collision: park a diff of unrelated content under the key the next
	// lookup will compute. Without verification the cache would hand it back.
	key := keyFor(e, oldContent, newContent)
	poison := e.ComputeDiff("x", "x", "totally\nother\n", "content\nhere\n")
	e.cache.put(key, poison, "totally\nother\n", "content\nhere\n", true)

	got := e.ComputeDiff("f", "f", oldContent, newContent)
	if len(got.Hunks) != len(want.Hunks) {
		t.Fatalf("served a poisoned entry: hunks=%d want %d", len(got.Hunks), len(want.Hunks))
	}
	for i, h := range got.Hunks {
		for j, l := range h.Lines {
			if l.Content != want.Hunks[i].Lines[j].Content {
				t.Fatalf("hunk %d line %d = %q, want %q", i, j, l.Content, want.Hunks[i].Lines[j].Content)
			}
		}
	}
	if stats := e.Stats(); stats.Collisions == 0 {
		t.Error("a rejected hit must be counted in Stats.Collisions")
	}
}

func TestCache_WhenVerifyDisabled_ShouldNotRetainContentOrCountCollisions(t *testing.T) {
	e := NewEngine()
	e.ComputeDiff("f", "f", "a\nb\n", "a\nc\n")
	e.ComputeDiff("f", "f", "a\nb\n", "a\nc\n")

	stats := e.Stats()
	if stats.Hits != 1 {
		t.Fatalf("hits=%d, want 1", stats.Hits)
	}
	if stats.Collisions != 0 {
		t.Errorf("collisions=%d with verification off, want 0", stats.Collisions)
	}
}

func TestFingerprint_WhenContentDiffers_ShouldDifferInLengthOrBothHashes(t *testing.T) {
	a := fingerprint("the quick brown fox")
	b := fingerprint("the quick brown fix")
	if a == b {
		t.Fatal("distinct content produced identical fingerprints")
	}
	if got := fingerprint("abc").length; got != 3 {
		t.Errorf("length=%d, want 3", got)
	}
	// The primary half must stay FNV-1a so hash() and the cache key agree.
	if fingerprint("abc").primary != hash("abc") {
		t.Error("fingerprint primary drifted from hash(); the two must stay the same function")
	}
}

// keyFor rebuilds the cache key an engine would compute for a content pair.
func keyFor(e *Engine, oldContent, newContent string) cacheKey {
	oldFP := fingerprint(oldContent)
	newFP := fingerprint(newContent)
	return cacheKey{
		oldHash: oldFP.primary, oldHash2: oldFP.secondary, oldLen: oldFP.length,
		newHash: newFP.primary, newHash2: newFP.secondary, newLen: newFP.length,
		contextLines: e.opts.contextLines(),
	}
}
