package diff

import (
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestCacheHit_WhenCallerMutatesResult_ShouldNotCorruptCache is the regression
// test for the shallow-copy aliasing bug: the cache used to hand out a struct
// copy that still shared the Hunks array and every Hunk.Lines slice, so one
// caller editing a line rewrote what every later caller saw.
func TestCacheHit_WhenCallerMutatesResult_ShouldNotCorruptCache(t *testing.T) {
	e := NewEngine()
	oldContent := "alpha\nbravo\ncharlie\n"
	newContent := "alpha\nBRAVO\ncharlie\n"

	first := e.ComputeDiff("a.txt", "b.txt", oldContent, newContent)
	if len(first.Hunks) == 0 || len(first.Hunks[0].Lines) == 0 {
		t.Fatalf("expected at least one hunk with lines, got %+v", first.Hunks)
	}

	// Vandalize every line of the returned diff.
	for hi := range first.Hunks {
		for li := range first.Hunks[hi].Lines {
			first.Hunks[hi].Lines[li].Content = "CORRUPTED"
			first.Hunks[hi].Lines[li].LineNum = -999
		}
		first.Hunks[hi].OldCount = -1
	}

	second := e.ComputeDiff("a.txt", "b.txt", oldContent, newContent)
	for hi := range second.Hunks {
		if second.Hunks[hi].OldCount == -1 {
			t.Errorf("hunk %d OldCount leaked caller mutation", hi)
		}
		for li, line := range second.Hunks[hi].Lines {
			if line.Content == "CORRUPTED" || line.LineNum == -999 {
				t.Fatalf("cache returned caller-mutated line at hunk %d line %d: %+v", hi, li, line)
			}
		}
	}
}

// TestCacheHit_ShouldReturnIndependentSlices verifies the two results do not
// share backing arrays, which is the property that makes the test above hold.
func TestCacheHit_ShouldReturnIndependentSlices(t *testing.T) {
	e := NewEngine()
	oldContent := "one\ntwo\nthree\n"
	newContent := "one\nTWO\nthree\n"

	a := e.ComputeDiff("x", "y", oldContent, newContent)
	b := e.ComputeDiff("x", "y", oldContent, newContent)

	if len(a.Hunks) == 0 || len(b.Hunks) == 0 {
		t.Fatal("expected hunks from both computations")
	}
	if &a.Hunks[0] == &b.Hunks[0] {
		t.Fatal("Hunks arrays are shared between cache reads")
	}
	if len(a.Hunks[0].Lines) > 0 && &a.Hunks[0].Lines[0] == &b.Hunks[0].Lines[0] {
		t.Fatal("Hunk.Lines arrays are shared between cache reads")
	}
}

// TestCache_ShouldReportHitsAndMisses covers the Stats() counters.
func TestCache_ShouldReportHitsAndMisses(t *testing.T) {
	e := NewEngine()
	oldContent := "a\nb\n"
	newContent := "a\nc\n"

	e.ComputeDiff("f", "f", oldContent, newContent) // miss + compute
	e.ComputeDiff("f", "f", oldContent, newContent) // hit
	e.ComputeDiff("f", "f", oldContent, newContent) // hit

	s := e.Stats()
	if s.Misses != 1 {
		t.Errorf("Misses = %d, want 1", s.Misses)
	}
	if s.Hits != 2 {
		t.Errorf("Hits = %d, want 2", s.Hits)
	}
	if s.Computes != 1 {
		t.Errorf("Computes = %d, want 1", s.Computes)
	}
	if s.Entries != 1 {
		t.Errorf("Entries = %d, want 1", s.Entries)
	}
	if s.Bytes <= 0 {
		t.Errorf("Bytes = %d, want > 0", s.Bytes)
	}
}

// TestCache_ShouldCountBinaryShortCircuit checks binary inputs never reach the
// diff engine and are counted separately.
func TestCache_ShouldCountBinaryShortCircuit(t *testing.T) {
	e := NewEngine()
	d := e.ComputeDiff("bin", "bin", "abc\x00def", "abc\x00xyz")
	if !d.IsBinary {
		t.Fatal("expected IsBinary=true")
	}
	s := e.Stats()
	if s.Binary != 1 {
		t.Errorf("Binary = %d, want 1", s.Binary)
	}
	if s.Computes != 0 {
		t.Errorf("Computes = %d, want 0 (binary must not be diffed)", s.Computes)
	}
}

// TestCache_ShouldEvictBeyondMaxEntries verifies the entry bound holds, which
// is what stops an unbounded memory climb across a long session.
func TestCache_ShouldEvictBeyondMaxEntries(t *testing.T) {
	e := NewEngineWith(Options{MaxCacheEntries: 4})

	for i := 0; i < 40; i++ {
		e.ComputeDiff("f", "f", fmt.Sprintf("line-%d\ncommon\n", i), fmt.Sprintf("LINE-%d\ncommon\n", i))
	}

	s := e.Stats()
	if s.Entries > 4 {
		t.Errorf("Entries = %d, want <= 4", s.Entries)
	}
	if s.Evicted == 0 {
		t.Error("expected evictions once the entry bound was exceeded")
	}
}

// TestCache_ShouldEvictBeyondMaxBytes verifies the byte bound trips independently.
func TestCache_ShouldEvictBeyondMaxBytes(t *testing.T) {
	e := NewEngineWith(Options{MaxCacheEntries: 100_000, MaxCacheBytes: 8 << 10})

	body := strings.Repeat("padding padding padding\n", 60)
	for i := 0; i < 30; i++ {
		e.ComputeDiff("f", "f", fmt.Sprintf("head-%d\n%s", i, body), fmt.Sprintf("HEAD-%d\n%s", i, body))
	}

	s := e.Stats()
	if s.Bytes > 8<<10 {
		t.Errorf("Bytes = %d, want <= %d", s.Bytes, 8<<10)
	}
	if s.Evicted == 0 {
		t.Error("expected evictions once the byte bound was exceeded")
	}
}

// TestCache_WhenEntryExceedsWholeBudget_ShouldSkipCaching guards against a
// single oversized diff evicting the entire cache and still not fitting.
func TestCache_WhenEntryExceedsWholeBudget_ShouldSkipCaching(t *testing.T) {
	e := NewEngineWith(Options{MaxCacheBytes: 256})

	big := strings.Repeat("a long line of content here\n", 200)
	d := e.ComputeDiff("f", "f", big, strings.Repeat("a different line entirely\n", 200))
	if d == nil {
		t.Fatal("expected a diff even when it is too large to cache")
	}
	if s := e.Stats(); s.Entries != 0 {
		t.Errorf("Entries = %d, want 0 (oversized entry must not be cached)", s.Entries)
	}
}

// TestClearCache_ConcurrentWithComputeDiff_ShouldNotRace is the regression test
// for `e.cache = sync.Map{}`, which reassigned a field other goroutines were
// concurrently reading. Run with -race.
func TestClearCache_ConcurrentWithComputeDiff_ShouldNotRace(t *testing.T) {
	e := NewEngine()
	var wg sync.WaitGroup
	stop := make(chan struct{})

	for w := 0; w < 4; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; ; i++ {
				select {
				case <-stop:
					return
				default:
				}
				e.ComputeDiff("f", "f", fmt.Sprintf("x %d %d\n", w, i%7), fmt.Sprintf("y %d %d\n", w, i%7))
			}
		}(w)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 200; i++ {
			e.ClearCache()
			_ = e.Stats()
		}
	}()

	time.Sleep(150 * time.Millisecond)
	close(stop)
	wg.Wait()
}

// TestClearCache_ShouldPreserveCumulativeCounters documents that Stats is a
// lifetime view: clearing frees memory without rewriting history.
func TestClearCache_ShouldPreserveCumulativeCounters(t *testing.T) {
	e := NewEngine()
	e.ComputeDiff("f", "f", "a\n", "b\n")
	before := e.Stats()

	e.ClearCache()
	after := e.Stats()

	if after.Entries != 0 {
		t.Errorf("Entries = %d after clear, want 0", after.Entries)
	}
	if after.Bytes != 0 {
		t.Errorf("Bytes = %d after clear, want 0", after.Bytes)
	}
	if after.Computes != before.Computes {
		t.Errorf("Computes = %d, want preserved %d", after.Computes, before.Computes)
	}
}

// TestOptions_DisableCache_ShouldRecomputeEveryTime covers the opt-out path.
func TestOptions_DisableCache_ShouldRecomputeEveryTime(t *testing.T) {
	e := NewEngineWith(Options{DisableCache: true})
	for i := 0; i < 3; i++ {
		e.ComputeDiff("f", "f", "a\nb\n", "a\nc\n")
	}
	s := e.Stats()
	if s.Computes != 3 {
		t.Errorf("Computes = %d, want 3 with caching disabled", s.Computes)
	}
	if s.Entries != 0 {
		t.Errorf("Entries = %d, want 0 with caching disabled", s.Entries)
	}
}

// TestOptions_ZeroValue_ShouldMatchNewEngine pins the documented promise that
// the zero Options value changes nothing.
func TestOptions_ZeroValue_ShouldMatchNewEngine(t *testing.T) {
	oldContent := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n"
	newContent := "1\n2\n3\n4\nFIVE\n6\n7\n8\n9\n10\n"

	a := NewEngine().ComputeDiff("f", "f", oldContent, newContent)
	b := NewEngineWith(Options{}).ComputeDiff("f", "f", oldContent, newContent)

	if len(a.Hunks) != len(b.Hunks) {
		t.Fatalf("hunk count differs: %d vs %d", len(a.Hunks), len(b.Hunks))
	}
	for i := range a.Hunks {
		if len(a.Hunks[i].Lines) != len(b.Hunks[i].Lines) {
			t.Errorf("hunk %d line count differs: %d vs %d", i, len(a.Hunks[i].Lines), len(b.Hunks[i].Lines))
		}
	}
}

// TestOptions_ContextLines_ShouldWidenHunks verifies ContextLines is honored and
// that a wider setting keeps at least as many lines as a narrow one.
func TestOptions_ContextLines_ShouldWidenHunks(t *testing.T) {
	oldContent := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n"
	newContent := "1\n2\n3\n4\n5\n6\nSEVEN\n8\n9\n10\n11\n12\n"

	narrow := NewEngineWith(Options{ContextLines: 1}).ComputeDiff("f", "f", oldContent, newContent)
	wide := NewEngineWith(Options{ContextLines: 5}).ComputeDiff("f", "f", oldContent, newContent)

	countLines := func(d *FileDiff) int {
		n := 0
		for _, h := range d.Hunks {
			n += len(h.Lines)
		}
		return n
	}

	if countLines(wide) <= countLines(narrow) {
		t.Errorf("wide context produced %d lines, narrow %d; want wide > narrow",
			countLines(wide), countLines(narrow))
	}
}

// TestCacheKey_ShouldSeparateContextWidths ensures engines with different
// context widths never read each other's entries for the same content.
func TestCacheKey_ShouldSeparateContextWidths(t *testing.T) {
	oldContent := "1\n2\n3\n4\n5\n6\n7\n8\n9\n10\n11\n12\n"
	newContent := "1\n2\n3\n4\n5\n6\nSEVEN\n8\n9\n10\n11\n12\n"

	shared := newDiffCache(0, 0)
	narrow := NewEngineWith(Options{ContextLines: 1})
	wide := NewEngineWith(Options{ContextLines: 5})
	narrow.cache = shared
	wide.cache = shared

	n := narrow.ComputeDiff("f", "f", oldContent, newContent)
	w := wide.ComputeDiff("f", "f", oldContent, newContent)

	nLines, wLines := 0, 0
	for _, h := range n.Hunks {
		nLines += len(h.Lines)
	}
	for _, h := range w.Hunks {
		wLines += len(h.Lines)
	}
	if nLines == wLines {
		t.Errorf("context widths collided in the cache: both produced %d lines", nLines)
	}
}

// TestDiffTimeout_ShouldCompleteOnPathologicalInput asserts the timeout keeps
// a hostile input bounded rather than hanging the caller.
func TestDiffTimeout_ShouldCompleteOnPathologicalInput(t *testing.T) {
	e := NewEngineWith(Options{Timeout: 100 * time.Millisecond})

	var a, b strings.Builder
	for i := 0; i < 4000; i++ {
		fmt.Fprintf(&a, "%d-%s\n", i%97, strings.Repeat("x", i%53))
		fmt.Fprintf(&b, "%d-%s\n", (i*7)%89, strings.Repeat("y", i%47))
	}

	done := make(chan *FileDiff, 1)
	go func() { done <- e.ComputeDiff("f", "f", a.String(), b.String()) }()

	select {
	case d := <-done:
		if d == nil {
			t.Fatal("expected a diff result")
		}
	case <-time.After(30 * time.Second):
		t.Fatal("ComputeDiff did not respect its timeout on pathological input")
	}
}

// TestComputeDiff_TrailingNewlineOnlyChange pins how a pure trailing-newline
// change is represented, so future engine changes cannot silently drop it.
func TestComputeDiff_TrailingNewlineOnlyChange(t *testing.T) {
	withNewline := "alpha\nbravo\n"
	withoutNewline := "alpha\nbravo"

	d := NewEngine().ComputeDiff("f", "f", withNewline, withoutNewline)

	changed := 0
	for _, h := range d.Hunks {
		for _, l := range h.Lines {
			if l.Type == LineAdded || l.Type == LineRemoved {
				changed++
			}
		}
	}
	if changed == 0 {
		t.Error("trailing-newline-only change produced no added/removed lines")
	}
}

// TestFileDiff_Clone_ShouldHandleNilAndEmpty guards the Clone edge cases the
// cache relies on.
func TestFileDiff_Clone_ShouldHandleNilAndEmpty(t *testing.T) {
	if (*FileDiff)(nil).Clone() != nil {
		t.Error("Clone of nil should be nil")
	}

	empty := &FileDiff{OldPath: "a", NewPath: "b"}
	c := empty.Clone()
	if c.OldPath != "a" || c.NewPath != "b" || c.Hunks != nil {
		t.Errorf("Clone of empty diff mismatched: %+v", c)
	}

	withNilLines := &FileDiff{Hunks: []Hunk{{OldStart: 1}}}
	c2 := withNilLines.Clone()
	if len(c2.Hunks) != 1 || c2.Hunks[0].OldStart != 1 {
		t.Errorf("Clone dropped hunk metadata: %+v", c2.Hunks)
	}
}

func BenchmarkComputeDiff_CacheHit(b *testing.B) {
	e := NewEngine()
	oldContent := strings.Repeat("some line of code\n", 200)
	newContent := strings.Repeat("some line of code\n", 199) + "changed\n"
	e.ComputeDiff("f", "f", oldContent, newContent)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ComputeDiff("f", "f", oldContent, newContent)
	}
}
