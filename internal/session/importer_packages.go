package session

import (
	"context"
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

// importerPackages lists the packages that import any of pkgs -- through
// their code or their tests -- and are not among them. pkgs are the local
// patterns the gate uses ("./internal/session"); the result are import paths,
// which go test accepts just as well.
func importerPackages(ctx context.Context, workspace string, pkgs []string) []string {
	if len(pkgs) == 0 {
		return nil
	}
	own := listImportPaths(ctx, workspace, pkgs)
	if len(own) == 0 {
		return nil
	}
	const format = "{{.ImportPath}}\t{{join .Imports \" \"}} {{join .TestImports \" \"}} {{join .XTestImports \" \"}}"
	out, outcome, reason := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), buildVerifyTimeout,
		"go", []string{"list", "-e", "-f", format, "./..."}, verifyBuildRunner)
	if outcome != VerifyPassed {
		logging.Get(logging.CategorySession).Warn("importers not listed (%s%s); the gate runs the turn's own packages only", outcome, suffixed(reason))
		return nil
	}
	var importers []string
	for _, line := range strings.Split(string(out), "\n") {
		path, imports, found := strings.Cut(strings.TrimSpace(line), "\t")
		if !found || own[path] {
			continue
		}
		for _, imp := range strings.Fields(imports) {
			if own[imp] {
				importers = append(importers, path)
				break
			}
		}
	}
	return importers
}

// listImportPaths resolves local patterns to import paths.
func listImportPaths(ctx context.Context, workspace string, pkgs []string) map[string]bool {
	args := append([]string{"list", "-e", "-f", "{{.ImportPath}}"}, pkgs...)
	out, outcome, reason := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), buildVerifyTimeout,
		"go", args, verifyBuildRunner)
	if outcome != VerifyPassed {
		logging.Get(logging.CategorySession).Warn("the turn's packages could not be resolved to import paths (%s%s)", outcome, suffixed(reason))
		return nil
	}
	own := map[string]bool{}
	for _, line := range strings.Split(string(out), "\n") {
		if p := strings.TrimSpace(line); p != "" {
			own[p] = true
		}
	}
	return own
}

// verifyImporters runs the tests of the packages that import what the turn
// wrote, and charges the turn only with failures that were not already there
// before it (attributeTestFailures, over the same preimages). A skipped or
// unfinished run is no verdict: the turn's own gate stands.
func verifyImporters(ctx context.Context, workspace string, result *ExecutionResult) TestVerification {
	own, _ := splitTagGatedPackages(workspace, packagesForPaths(result.WrittenPaths))
	importers := importerPackages(ctx, workspace, own)
	runnable, _ := splitTagGatedPackages(workspace, importers)
	if len(runnable) == 0 {
		return TestVerification{Outcome: VerifySkipped, Reason: "nothing imports what this turn wrote"}
	}
	logging.Get(logging.CategorySession).Info("test gate: %d package(s) import what this turn wrote; running their tests too", len(runnable))
	v := verifyTests(ctx, workspace, runnable)
	if v.Verdict() != VerifyFailed {
		return v
	}
	v = attributeTestFailures(ctx, workspace, runnable, result.WrittenPaths, result.PreWriteContents, v)
	if v.Verdict() == VerifyFailed {
		v.Reason = "the tests of the packages that import what this turn changed"
	}
	return v
}
