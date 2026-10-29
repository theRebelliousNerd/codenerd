package retrieval

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// testRetriever builds a SparseRetriever rooted at dir with deterministic,
// fast settings and no exclude patterns so test fixtures are always scanned.
func testRetriever(dir string) *SparseRetriever {
	return NewSparseRetriever(&SparseRetrieverConfig{
		WorkDir:         dir,
		MaxResults:      50,
		SearchTimeout:   5 * time.Second,
		Parallelism:     2,
		ExcludePatterns: nil,
		CacheSize:       16,
		CacheTTL:        time.Minute,
	})
}

func TestScanBuffer(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		buf     string
		keyword string
		want    []int
	}{
		{"empty keyword", "hello", "", nil},
		{"single match", "abcXYZdef", "XYZ", []int{3}},
		{"match at start", "needle in haystack", "needle", []int{0}},
		{"match at end", "find the cat", "cat", []int{9}},
		{"multiple non-overlapping", "ababab", "ab", []int{0, 2, 4}},
		{"overlap consumed", "aaaa", "aa", []int{0, 2}},
		{"not found", "abcdef", "xyz", nil},
		{"keyword longer than buf", "ab", "abcdef", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ScanBuffer([]byte(tc.buf), []byte(tc.keyword))
			if len(got) != len(tc.want) {
				t.Fatalf("ScanBuffer(%q,%q)=%v, want %v", tc.buf, tc.keyword, got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("ScanBuffer(%q,%q)=%v, want %v", tc.buf, tc.keyword, got, tc.want)
				}
			}
		})
	}
}

func TestIsAlphanumeric(t *testing.T) {
	t.Parallel()
	yes := []byte{'a', 'z', 'A', 'Z', '0', '9', '_'}
	for _, b := range yes {
		if !isAlphanumeric(b) {
			t.Errorf("isAlphanumeric(%q)=false, want true", b)
		}
	}
	no := []byte{' ', '-', '.', '(', ')', '\n', '/', ':'}
	for _, b := range no {
		if isAlphanumeric(b) {
			t.Errorf("isAlphanumeric(%q)=true, want false", b)
		}
	}
}

func TestIsWordBoundary(t *testing.T) {
	t.Parallel()
	// "the cat sat" — "cat" at offset 4, len 3. Surrounded by spaces -> boundary.
	data := []byte("the cat sat")
	if !isWordBoundary(data, 4, 3) {
		t.Error("expected word boundary for standalone 'cat'")
	}
	// "scattered" — "cat" at offset 1 is inside a word -> not a boundary.
	data = []byte("scattered")
	if isWordBoundary(data, 1, 3) {
		t.Error("expected no word boundary for 'cat' inside 'scattered'")
	}
	// Match at very start/end of buffer is a boundary on the missing side.
	data = []byte("cat")
	if !isWordBoundary(data, 0, 3) {
		t.Error("expected boundary for whole-buffer match")
	}
}

func TestAllKeywords(t *testing.T) {
	t.Parallel()
	kw := &IssueKeywords{
		Primary:   []string{"p1", "p2"},
		Secondary: []string{"s1"},
		Tertiary:  []string{"t1"},
	}
	got := kw.AllKeywords()
	want := []string{"p1", "p2", "s1", "t1"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("AllKeywords()=%v, want %v (primary then secondary then tertiary)", got, want)
	}
}

func TestSearchKeywords_NilAndEmpty(t *testing.T) {
	t.Parallel()
	r := testRetriever(t.TempDir())
	ctx := context.Background()

	hits, outcome := r.SearchKeywords(ctx, nil)
	if outcome.Status != StatusComplete || hits != nil {
		t.Fatalf("nil keywords: got hits=%v outcome=%s, want nil, complete", hits, outcome)
	}

	hits, outcome = r.SearchKeywords(ctx, &IssueKeywords{})
	if outcome.Status != StatusComplete || hits != nil {
		t.Fatalf("empty keywords: got hits=%v outcome=%s, want nil, complete", hits, outcome)
	}
}

func TestSearchKeywords_MentionedFilesOnly(t *testing.T) {
	t.Parallel()
	r := testRetriever(t.TempDir())
	kw := &IssueKeywords{MentionedFiles: []string{"a.go", "b.go"}}

	hits, outcome := r.SearchKeywords(context.Background(), kw)
	if outcome.Status != StatusComplete {
		t.Fatalf("search not complete: %s", outcome)
	}
	if len(hits) != 2 {
		t.Fatalf("expected 2 mentioned-file hits, got %d", len(hits))
	}
	for _, h := range hits {
		if h.Keyword != "[mentioned]" {
			t.Errorf("expected [mentioned] keyword, got %q", h.Keyword)
		}
	}
}

func TestSearchKeywords_RealFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// Whole-word, case-insensitive match across a real file on disk.
	src := "package main\n\nfunc handleError() error { return nil }\n"
	if err := os.WriteFile(filepath.Join(dir, "alpha.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	// A decoy file with no match ensures ranking only surfaces the real hit.
	if err := os.WriteFile(filepath.Join(dir, "beta.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	r := testRetriever(dir)
	kw := &IssueKeywords{Primary: []string{"handleError"}}
	hits, outcome := r.SearchKeywords(context.Background(), kw)
	if outcome.Status != StatusComplete {
		t.Fatalf("search not complete: %s", outcome)
	}
	if len(hits) == 0 {
		t.Fatal("expected at least one hit for handleError")
	}
	found := false
	for _, h := range hits {
		if strings.HasSuffix(h.FilePath, "alpha.go") {
			found = true
			if h.Keyword != "handleError" {
				t.Errorf("hit keyword=%q, want handleError", h.Keyword)
			}
			if h.Line != 3 {
				t.Errorf("hit line=%d, want 3", h.Line)
			}
		}
	}
	if !found {
		t.Errorf("expected a hit in alpha.go, got %+v", hits)
	}
}

func TestFindRelevant_EndToEnd(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	src := "package main\n\nfunc handle_error() error { return nil }\n"
	if err := os.WriteFile(filepath.Join(dir, "alpha.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	r := testRetriever(dir)
	// Crosses the full pipeline: ExtractKeywords -> SearchKeywords -> RankFiles.
	issue := "The handle_error() function returns the wrong value"
	candidates, err := findRelevant(context.Background(), r, issue, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(candidates) == 0 {
		t.Fatal("expected at least one candidate file")
	}
	if !strings.HasSuffix(candidates[0].FilePath, "alpha.go") {
		t.Errorf("expected alpha.go as top candidate, got %q", candidates[0].FilePath)
	}
	if candidates[0].TotalHits < 1 || candidates[0].RelevanceScore <= 0 {
		t.Errorf("expected positive hits/score, got %+v", candidates[0])
	}
}

// findRelevant is extraction, search and ranking end to end, the way the
// tiered builder's keyword tier runs them.
func findRelevant(ctx context.Context, r *SparseRetriever, issue string, limit int) ([]CandidateFile, error) {
	keywords := ExtractKeywords(issue)
	hits, outcome := r.SearchKeywords(ctx, keywords)
	if outcome.Status == StatusUnavailable {
		return nil, fmt.Errorf("keyword search unavailable: %s", outcome)
	}
	return r.RankFiles(hits, keywords, limit), nil
}
