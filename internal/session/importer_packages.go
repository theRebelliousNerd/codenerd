package session

import (
	"bytes"
	"context"
	"encoding/json"
	"slices"
	"strings"

	internalbuild "codenerd/internal/build"
	"codenerd/internal/logging"
)

// The test gate ran the packages the turn wrote and nothing else, so a change
// to a package other packages import was verified by nobody but its own tests
// (external audit N25). Ladder runs R1-13 and R1-14 renamed Go methods in the
// CodeDOM reader: every test of internal/tools/codedom passed, the turn ended
// /done, and internal/observation's codesearch test -- which pins how a hit
// inside a method is named -- failed on the next full suite, outside the
// turn's sight.
//
// The packages that import the turn's are now part of the same gate. One hop,
// not the whole transitive fan-out: a direct importer sees the change's own
// surface, which is what a rename or a contract change breaks, and the cost
// stays proportional to what was touched.
//
// They are run where the verdict is decided -- the test gate itself and the
// closure that re-measures every gate -- and not inside the coverage, vet,
// removed-test and pinning rounds, whose rechecks ask whether the turn's own
// suite is still green while the model edits (gateOwnTests). A repair that
// breaks an importer is caught by the closure, which measures the revision the
// turn actually ends at.
//
// Within those packages the gate runs the tests a change can reach, not every
// test (rts_gate.go): a body-only change is traced through the importers'
// call graph and only the tests that can execute a changed function run. Any
// change the graph cannot vouch for runs them all, as before.

// importerListing is what discovery found: the importers, their directories,
// and the module packages between them and the turn's (what an analysis of
// the importers' tests must read to follow a call from a test into the
// changed code).
type importerListing struct {
	importers []string
	dirs      []string
	// loadSet is the turn's packages, the importers and every module package
	// either imports, as import paths.
	loadSet []string
}

// discoverImporterPackages lists the packages that import any of pkgs --
// through their code or their tests -- and are not among them, with what the
// gate needs to trust it: each importer's directory (what its run reads, for
// the memo key) and whether discovery finished. pkgs are the local patterns
// the gate uses ("./internal/session"); the importers are import paths, which
// go test accepts just as well. A listing that failed found nothing; that
// is not the same as nothing importing the turn's packages, and is not kept.
func discoverImporterPackages(ctx context.Context, workspace string, pkgs []string) (importers, dirs []string, why string) {
	l, why := listImporters(ctx, workspace, pkgs)
	return l.importers, l.dirs, why
}

func listImporters(ctx context.Context, workspace string, pkgs []string) (importerListing, string) {
	if len(pkgs) == 0 {
		return importerListing{}, ""
	}
	own, ok := listImportPaths(ctx, workspace, pkgs)
	if !ok {
		return importerListing{}, "the turn's packages could not be resolved to import paths"
	}
	if len(own) == 0 {
		return importerListing{}, ""
	}
	// go list -json, decoded: typed fields, not a tab-split template.
	out, outcome, reason := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), buildVerifyTimeout,
		"go", []string{"list", "-e", "-json=ImportPath,Dir,Imports,TestImports,XTestImports", "./..."}, verifyBuildRunner)
	if outcome != VerifyPassed {
		logging.Get(logging.CategorySession).Warn("importers not listed (%s%s); the gate runs the turn's own packages only", outcome, suffixed(reason))
		return importerListing{}, "importers not listed (" + string(outcome) + suffixed(reason) + ")"
	}
	type modulePackage struct {
		ImportPath   string
		Dir          string
		Imports      []string
		TestImports  []string
		XTestImports []string
	}
	module := map[string]modulePackage{}
	var order []string
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var mp modulePackage
		if err := dec.Decode(&mp); err != nil {
			return importerListing{}, "importers not listed (go list -json: " + err.Error() + ")"
		}
		module[mp.ImportPath] = mp
		order = append(order, mp.ImportPath)
	}
	var l importerListing
	for _, path := range order {
		mp := module[path]
		if own[path] {
			continue
		}
		for _, imp := range slices.Concat(mp.Imports, mp.TestImports, mp.XTestImports) {
			if own[imp] {
				l.importers = append(l.importers, path)
				l.dirs = append(l.dirs, mp.Dir)
				break
			}
		}
	}
	// What the importers' tests can call into: the importers' code and test
	// imports, and the code imports of everything on the way.
	need := map[string]bool{}
	var visit func(string, bool)
	visit = func(path string, withTests bool) {
		mp, ok := module[path]
		if !ok || need[path] {
			return
		}
		need[path] = true
		for _, imp := range mp.Imports {
			visit(imp, false)
		}
		if withTests {
			for _, imp := range slices.Concat(mp.TestImports, mp.XTestImports) {
				visit(imp, false)
			}
		}
	}
	for path := range own {
		visit(path, false)
	}
	for _, path := range l.importers {
		visit(path, true)
	}
	for _, path := range order {
		if need[path] {
			l.loadSet = append(l.loadSet, path)
		}
	}
	return l, ""
}

// listImportPaths resolves local patterns to import paths; ok is false when
// the listing did not finish.
func listImportPaths(ctx context.Context, workspace string, pkgs []string) (map[string]bool, bool) {
	args := append([]string{"list", "-e", "-f", "{{.ImportPath}}"}, pkgs...)
	out, outcome, reason := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), buildVerifyTimeout,
		"go", args, verifyBuildRunner)
	if outcome != VerifyPassed {
		logging.Get(logging.CategorySession).Warn("the turn's packages could not be resolved to import paths (%s%s)", outcome, suffixed(reason))
		return nil, false
	}
	own := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if p := strings.TrimSpace(line); p != "" {
			own[p] = true
		}
	}
	return own, true
}

// importerScope is the discovered importers the gate will run, their
// directories, the module packages an analysis of their tests reads, and why
// discovery did not finish ("" when it did).
type importerScope struct {
	runnable []string
	dirs     []string
	loadSet  []string
	why      string
}

func discoverImporters(ctx context.Context, workspace string, result *ExecutionResult) importerScope {
	workspace = goWorkspace(workspace)
	own, _ := splitTagGatedPackages(workspace, packagesForPaths(nonTestPaths(result.WrittenPaths)))
	listing, why := listImporters(ctx, workspace, own)
	if why != "" {
		return importerScope{why: why}
	}
	runnable, _ := splitTagGatedPackages(workspace, listing.importers)
	keep := map[string]bool{}
	for _, p := range runnable {
		keep[p] = true
	}
	scope := importerScope{runnable: runnable, loadSet: listing.loadSet}
	for i, p := range listing.importers {
		if keep[p] {
			scope.dirs = append(scope.dirs, listing.dirs[i])
		}
	}
	return scope
}

// verifyImporters runs the tests of the packages that import what the turn
// wrote (scope.runnable, from discoverImporters) that can reach what it
// changed (selectImporterRuns). attributeTestFailures records which of those
// failures also failed before the turn; it does not decide the gate. The /test
// rule does, from the rows syncImporterGateFacts asserts for this run.
//
// A list that fails, or a turn nothing imports, is no measurement: the own
// gate stands. A test run that does not finish is returned as itself
// (canceled or indeterminate). That is a measurement. The /test rule
// withholds a pass for it (R1-18: an importer run that hit its budget was
// reported as tests ok).
//
// A turn whose change no importer test can reach runs no importer test and
// comes back skipped, with the selection on it saying how many tests were
// left out and why.
func verifyImporters(ctx context.Context, workspace string, result *ExecutionResult, scope importerScope) TestVerification {
	workspace = goWorkspace(workspace)
	if len(scope.runnable) == 0 {
		return TestVerification{Outcome: VerifySkipped, Reason: "nothing imports what this turn wrote"}
	}
	plan := selectImporterRuns(ctx, workspace, result, scope)
	sel := plan.selection
	logging.Get(logging.CategorySession).Info("test gate: %d package(s) import what this turn wrote; %s%s", len(scope.runnable), sel.Summary(), suffixed(sel.Phases))
	if len(plan.runs) == 0 {
		return TestVerification{Outcome: VerifySkipped, Reason: sel.Summary(), Selection: &sel}
	}
	v := verifyTestRuns(ctx, workspace, sel.Summary(), plan.runs...)
	v.Selection = &sel
	if v.Verdict() != VerifyFailed {
		return v
	}
	v = attributeTestFailures(ctx, workspace, sel.Packages, result.WrittenPaths, result.PreWriteContents, v)
	if v.Verdict() == VerifyFailed {
		v.Reason = "the tests of the packages that import what this turn changed"
	}
	return v
}
