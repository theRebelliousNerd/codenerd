package diff

import (
	"fmt"
	"strings"
	"testing"
)

// buildBenchSource returns a synthetic Go-ish file of n lines. The existing
// benchmarks in diff_test.go and cache_test.go cover repeated and near-uniform
// content; this one is closer to real source, where every line is distinct.
func buildBenchSource(n int, marker string) string {
	var sb strings.Builder
	for i := range n {
		fmt.Fprintf(&sb, "func handler%d(ctx Context) error { return %s(%d) }\n", i, marker, i)
	}
	return sb.String()
}

func BenchmarkComputeDiff_RealisticSource(b *testing.B) {
	oldContent := buildBenchSource(2000, "process")
	newContent := buildBenchSource(2000, "handle")
	e := NewEngineWith(Options{DisableCache: true})

	b.ReportAllocs()
	for b.Loop() {
		_ = e.ComputeDiff("a.go", "a.go", oldContent, newContent)
	}
}

// BenchmarkComputeDiff_CacheHitVerified measures what content verification
// costs on the hot path, so enabling it stays an informed choice.
func BenchmarkComputeDiff_CacheHitVerified(b *testing.B) {
	oldContent := buildBenchSource(2000, "process")
	newContent := buildBenchSource(2000, "handle")
	e := NewEngineWith(Options{VerifyCacheContent: true})
	_ = e.ComputeDiff("a.go", "a.go", oldContent, newContent)

	b.ReportAllocs()
	for b.Loop() {
		_ = e.ComputeDiff("a.go", "a.go", oldContent, newContent)
	}
}

func BenchmarkComputeWordLevelDiff_Line(b *testing.B) {
	oldLine := "\tif err := svc.Process(ctx, request, options...); err != nil {"
	newLine := "\tif err := svc.Handle(ctx, req, opts...); err != nil && !ignorable(err) {"
	e := NewEngine()

	b.ReportAllocs()
	for b.Loop() {
		_ = e.ComputeWordLevelDiff(oldLine, newLine)
	}
}

// TestBenchmarks_WhenRunAsSmoke_ShouldCompleteAndDoWork keeps the package's
// benchmarks honest in ordinary CI, which runs `go test` without -bench: a
// benchmark that stops compiling, panics, or measures nothing would otherwise
// go unnoticed until someone profiled by hand.
func TestBenchmarks_WhenRunAsSmoke_ShouldCompleteAndDoWork(t *testing.T) {
	if testing.Short() {
		t.Skip("benchmark smoke skipped in short mode")
	}

	benches := []struct {
		name string
		fn   func(*testing.B)
	}{
		{"ComputeDiff_Small", BenchmarkComputeDiff_Small},
		{"ComputeDiff_WithCache", BenchmarkComputeDiff_WithCache},
		{"ComputeDiff_CacheHit", BenchmarkComputeDiff_CacheHit},
		{"ComputeDiff_RealisticSource", BenchmarkComputeDiff_RealisticSource},
		{"ComputeDiff_CacheHitVerified", BenchmarkComputeDiff_CacheHitVerified},
		{"ComputeWordLevelDiff_Line", BenchmarkComputeWordLevelDiff_Line},
	}

	for _, bc := range benches {
		res := testing.Benchmark(bc.fn)
		if res.N == 0 {
			t.Errorf("%s: benchmark performed no iterations", bc.name)
		}
	}
}
