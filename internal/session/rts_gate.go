package session

import (
	"context"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"codenerd/internal/build"
)

// Static regression test selection, part 3: the gate's use of it.
//
// selectImporterRuns turns "which importer tests can reach what the turn
// changed" into the `go test` invocations the importer gate makes. It never
// weakens a verdict: whenever the change or the analysis cannot be trusted
// (rts_changes.go, rts_analysis.go) the plan is today's, every test of every
// importer, and the selection records why.

// TestSelection is what the importer gate chose to run, kept on the
// verification so a reader of the result can tell a pass over a few tests
// from a pass over all of them.
type TestSelection struct {
	// Packages are the importer packages whose tests ran, in the order
	// they were listed.
	Packages []string
	// Skipped are importer packages none of whose tests can reach a change.
	Skipped []string
	// Selected and Total count the tests across the analysed importers:
	// those run, and all of them. Zero on a fallback, which does not count.
	Selected, Total int
	// Analysis is how long the analysis took, and Phases where it went.
	Analysis time.Duration
	Phases   string
	// Fallback is why every importer test ran instead; empty when the
	// selection stood.
	Fallback string
}

// Summary is the one-line account the log and the live progress carry.
func (s TestSelection) Summary() string {
	if s.Fallback != "" {
		return fmt.Sprintf("running all importer tests: %s (analysis %s)", s.Fallback, s.Analysis.Round(time.Millisecond))
	}
	return fmt.Sprintf("selected %d of %d importer tests in %d package(s); %d package(s) have no test that reaches the change (analysis %s)",
		s.Selected, s.Total, len(s.Packages), len(s.Skipped), s.Analysis.Round(time.Millisecond))
}

// importerPlan is the invocations to make and the selection behind them.
type importerPlan struct {
	runs      []testRun
	selection TestSelection
}

// selectImporterRuns plans the importer gate: the selected tests with an
// anchored -run per package, or every importer test when the selection cannot
// be trusted.
func selectImporterRuns(ctx context.Context, workspace string, result *ExecutionResult, scope importerScope) importerPlan {
	all := func(why string, took time.Duration) importerPlan {
		return importerPlan{
			runs:      []testRun{{Packages: scope.runnable}},
			selection: TestSelection{Packages: scope.runnable, Fallback: why, Analysis: took},
		}
	}
	start := time.Now()
	changes := changedBodies(workspace, result.WrittenPaths, result.PreWriteContents)
	if changes.Fallback != "" {
		return all(changes.Fallback, time.Since(start))
	}
	done := liveVerification(ctx, []string{"select", "importer", "tests", fmt.Sprintf("(%d package(s))", len(scope.runnable))}, "")
	analysis, why := analyzeImporterTests(ctx, workspace, build.GetBuildEnv(nil, workspace), scope.loadSet, scope.runnable, changes)
	if why != "" {
		done(VerifyIndeterminate)
		return all(why, time.Since(start))
	}
	done(VerifyPassed)

	sel := TestSelection{Analysis: time.Since(start), Phases: analysis.Phases}
	byPattern := map[string][]string{}
	for _, p := range analysis.Packages {
		sel.Total += len(p.All)
		switch {
		case p.Whole:
			sel.Selected += len(p.All)
			sel.Packages = append(sel.Packages, p.Path)
			byPattern[""] = append(byPattern[""], p.Path)
		case len(p.Selected) == 0:
			sel.Skipped = append(sel.Skipped, p.Path)
		case len(p.Selected) == len(p.All):
			sel.Selected += len(p.All)
			sel.Packages = append(sel.Packages, p.Path)
			byPattern[""] = append(byPattern[""], p.Path)
		default:
			sel.Selected += len(p.Selected)
			sel.Packages = append(sel.Packages, p.Path)
			quoted := make([]string, len(p.Selected))
			for i, name := range p.Selected {
				quoted[i] = regexp.QuoteMeta(name)
			}
			pattern := "^(" + strings.Join(quoted, "|") + ")$"
			byPattern[pattern] = append(byPattern[pattern], p.Path)
		}
	}
	plan := importerPlan{selection: sel}
	patterns := make([]string, 0, len(byPattern))
	for pattern := range byPattern {
		patterns = append(patterns, pattern)
	}
	slices.Sort(patterns)
	for _, pattern := range patterns {
		run := testRun{Packages: byPattern[pattern]}
		if pattern != "" {
			run.Args = []string{"-run", pattern}
		}
		plan.runs = append(plan.runs, run)
	}
	return plan
}
