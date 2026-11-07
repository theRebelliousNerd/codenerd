package session

import (
	"cmp"
	"codenerd/internal/build"
	"codenerd/internal/testfacts"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"testing"
	"time"
)

// TestMeasureImporterSelection times the importer gate on a real checkout with
// and without test selection, for one body-only edit. It is a measurement, not
// a regression test, and only runs when asked:
//
//	CODENERD_RTS_WORKSPACE=<a copy of the repository>  (the edit is made there)
//	CODENERD_RTS_FILE=internal/core/foo.go             (workspace-relative)
//	CODENERD_RTS_OLD / CODENERD_RTS_NEW                (text replaced once in it)
//	CODENERD_RTS_ALL=1                                 (also run every importer test)
//
// It reports the selection, the wall time of the selected gate, and, with
// CODENERD_RTS_ALL, the wall time and test count of the unselected one.
func TestMeasureImporterSelection(t *testing.T) {
	ws := os.Getenv("CODENERD_RTS_WORKSPACE")
	file := os.Getenv("CODENERD_RTS_FILE")
	oldText, newText := os.Getenv("CODENERD_RTS_OLD"), os.Getenv("CODENERD_RTS_NEW")
	if ws == "" || file == "" || oldText == "" {
		t.Skip("set CODENERD_RTS_WORKSPACE, CODENERD_RTS_FILE and CODENERD_RTS_OLD/NEW to measure")
	}
	path := filepath.Join(ws, filepath.FromSlash(file))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(before), oldText) != 1 {
		t.Fatalf("%s must contain CODENERD_RTS_OLD exactly once", file)
	}
	if err := os.WriteFile(path, []byte(strings.Replace(string(before), oldText, newText, 1)), 0o644); err != nil {
		t.Fatal(err)
	}
	result := mutationResult()
	result.WrittenPaths = []string{file}
	result.PreWriteContents = map[string]PreImage{file: existed(string(before))}
	ctx := context.Background()

	scope := discoverImporters(ctx, ws, result)
	if scope.why != "" {
		t.Fatalf("importers: %s", scope.why)
	}
	t.Logf("%d importer package(s), %d module package(s) to load", len(scope.runnable), len(scope.loadSet))

	start := time.Now()
	plan := selectImporterRuns(ctx, ws, result, scope)
	t.Logf("selection (%s): %s", time.Since(start).Round(time.Millisecond), plan.selection.Summary())
	t.Logf("skipped packages: %v", plan.selection.Skipped)
	t.Logf("analysis phases: %s", plan.selection.Phases)
	if os.Getenv("CODENERD_RTS_EXPLAIN") != "" {
		changes := changedBodies(ws, result.WrittenPaths, result.PreWriteContents)
		analysis, why := analyzeImporterTests(ctx, ws, build.GetBuildEnv(nil, ws), scope.loadSet, scope.runnable, changes)
		t.Logf("explain: fallback %q", why)
		for _, p := range analysis.Packages {
			t.Logf("explain: %s selected %d of %d whole=%v why=%q", p.Path, len(p.Selected), len(p.All), p.Whole, p.Why)
		}
		if os.Getenv("CODENERD_RTS_EXPLAIN") == "only" {
			return
		}
	}

	// Compile and link every importer's tests once, running none, so both
	// timed gates start from the same build cache; set GOFLAGS=-count=1 as well
	// so neither is answered from cached test results.
	start = time.Now()
	verifyTests(ctx, ws, scope.runnable, "-run", "^$")
	t.Logf("warm build of every importer test binary: %s", time.Since(start).Round(time.Second))

	start, cpu := time.Now(), childCPU()
	selected := verifyImporters(ctx, ws, result, scope)
	t.Logf("gate with selection: %s in %s wall, %s cpu (%s)", selected.Verdict(), time.Since(start).Round(time.Second), (childCPU() - cpu).Round(time.Second), tally(selected))

	if os.Getenv("CODENERD_RTS_ALL") != "" {
		start, cpu = time.Now(), childCPU()
		all := verifyTests(ctx, ws, scope.runnable)
		t.Logf("gate without selection: %s in %s wall, %s cpu (%s)", all.Verdict(), time.Since(start).Round(time.Second), (childCPU() - cpu).Round(time.Second), tally(all))
	}
}

// childCPU is the user+system CPU time of every child process this test
// process has waited for so far: unlike wall time it does not grow when other
// work shares the machine.
func childCPU() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_CHILDREN, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}

func tally(v TestVerification) string {
	if v.Result == nil {
		return "no result"
	}
	lines := strings.Split(strings.TrimSpace(v.Result.Summary()), "\n")
	pkgs := slices.Clone(v.Result.Packages)
	slices.SortFunc(pkgs, func(a, b *testfacts.Package) int { return cmp.Compare(b.Elapsed, a.Elapsed) })
	var slow []string
	for _, p := range pkgs[:min(5, len(pkgs))] {
		slow = append(slow, fmt.Sprintf("%s %.0fs", strings.TrimPrefix(p.Name, "codenerd/"), p.Elapsed))
	}
	return lines[len(lines)-1] + "; slowest: " + strings.Join(slow, ", ")
}
