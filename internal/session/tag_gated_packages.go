package session

import (
	"bufio"
	internalbuild "codenerd/internal/build"
	"codenerd/internal/logging"
	"context"
	"go/build"
	"go/build/constraint"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// Post-edit test gate support for build-tag-gated packages.
//
// packagesForPaths maps every written .go file to its directory with no
// regard to build constraints. A package whose every file needs extra tags
// (e.g. //go:build integration) cannot be tested with the default tags:
// `go test` fails with "build constraints exclude all Go files", which says
// nothing about the turn's edits, and the edited file is never compiled.
//
// splitTagGatedPackages separates the packages the test gate can run with the
// default build tags from the ones whose Go files all need extra tags. For
// each gated package it returns the tags its files ask for, so the gate can
// still compile-check them.
func splitTagGatedPackages(workspace string, packages []string) (runnable []string, gated map[string][]string) {
	gated = make(map[string][]string)
	for _, pkg := range packages {
		trimmed := strings.TrimSpace(pkg)
		if trimmed == "" {
			continue
		}
		dir := packageDirForTags(workspace, trimmed)
		imp, err := build.Default.ImportDir(dir, 0)
		if err != nil {
			if _, ok := err.(*build.NoGoError); ok && imp != nil && len(imp.IgnoredGoFiles) > 0 {
				gated[trimmed] = tagsForIgnoredFiles(dir, imp.IgnoredGoFiles)
				continue
			}
			// Any other outcome — including other errors — stays runnable so
			// the gate's existing handling is unchanged.
			runnable = append(runnable, trimmed)
			continue
		}
		// No error means at least one file builds with the default tags.
		// A package with no Go files at all reports NoGoError above; a nil
		// error with no files is still runnable — the gate already knows how
		// to report it.
		runnable = append(runnable, trimmed)
	}
	return runnable, gated
}

func packageDirForTags(workspace, pkg string) string {
	p := filepath.ToSlash(strings.TrimSpace(pkg))
	if p == "" || p == "." {
		return workspace
	}
	p = strings.TrimPrefix(p, "./")
	if p == "" || p == "." {
		return workspace
	}
	return filepath.Join(workspace, filepath.FromSlash(p))
}

// tagsForIgnoredFiles reads each ignored file's //go:build line and collects
// the tag identifiers the expression needs set, sorted and de-duplicated.
// Constraint-only names (GOOS/GOARCH, unix, cgo, gc, gccgo, go1.x, ignore)
// are dropped: they describe the toolchain, not the turn's feature tags.
// Negated tags (e.g. !race in "integration && !race") are not collected:
// enabling them would unsatisfy the expression. A package whose only
// constraint tag is "ignore" gets an empty tag list.
func tagsForIgnoredFiles(dir string, ignored []string) []string {
	set := make(map[string]struct{})
	for _, name := range ignored {
		tags := tagsForFile(filepath.Join(dir, name))
		for _, t := range tags {
			set[t] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for t := range set {
		if isTagGatedToolchainTag(t) {
			continue
		}
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

func tagsForFile(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var out []string
	scanner := bufio.NewScanner(f)
	// A //go:build line is short; the default 64k buffer is ample, but a
	// long file must not abort the scan.
	const maxLine = 1024 * 1024
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, maxLine)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !constraint.IsGoBuild(line) {
			// Stop at the package clause: build lines always precede it.
			if strings.HasPrefix(line, "package ") {
				break
			}
			continue
		}
		expr, err := constraint.Parse(line)
		if err != nil {
			continue
		}
		collectPositiveTags(expr, false, &out)
	}
	return out
}

func collectPositiveTags(expr constraint.Expr, negated bool, out *[]string) {
	switch x := expr.(type) {
	case *constraint.TagExpr:
		if !negated {
			*out = append(*out, x.Tag)
		}
	case *constraint.NotExpr:
		collectPositiveTags(x.X, !negated, out)
	case *constraint.AndExpr:
		collectPositiveTags(x.X, negated, out)
		collectPositiveTags(x.Y, negated, out)
	case *constraint.OrExpr:
		collectPositiveTags(x.X, negated, out)
		collectPositiveTags(x.Y, negated, out)
	}
}

var tagGatedIgnoredTagSet = map[string]struct{}{
	"ignore": {},
	"unix":   {},
	"cgo":    {},
	"gc":     {},
	"gccgo":  {},
	// GOOS values.
	"aix": {}, "android": {}, "darwin": {}, "dragonfly": {}, "freebsd": {},
	"hurd": {}, "illumos": {}, "ios": {}, "js": {}, "linux": {},
	"netbsd": {}, "openbsd": {}, "plan9": {}, "solaris": {}, "wasip1": {},
	"windows": {}, "zos": {},
	// GOARCH values.
	"386": {}, "amd64": {}, "arm": {}, "arm64": {}, "loong64": {},
	"mips": {}, "mipsle": {}, "mips64": {}, "mips64le": {},
	"ppc64": {}, "ppc64le": {}, "riscv64": {}, "s390x": {}, "wasm": {},
}

func isTagGatedToolchainTag(tag string) bool {
	if _, ok := tagGatedIgnoredTagSet[tag]; ok {
		return true
	}
	if strings.HasPrefix(tag, "go1.") {
		return true
	}
	return false
}

// vetTagGatedPackages compile-checks each tag-gated package with
// `go vet -tags <tags>`: its tests need tags (and often services) the gate
// does not have, but the turn's edit must still compile. Packages whose
// only constraint is "ignore" (no tags) are skipped. It returns the first
// failing vet as a failed TestVerification, or ok=true when every vet passed.
func vetTagGatedPackages(ctx context.Context, workspace string, gated map[string][]string) (failed TestVerification, ok bool) {
	pkgs := make([]string, 0, len(gated))
	for pkg := range gated {
		pkgs = append(pkgs, pkg)
	}
	sort.Strings(pkgs)
	for _, pkg := range pkgs {
		tags := gated[pkg]
		if len(tags) == 0 {
			logging.SessionDebug("test gate: %s is tag-gated with no tags; skipping compile check", pkg)
			continue
		}
		tagList := strings.Join(tags, ",")
		command := []string{"go", "vet", "-tags", tagList, pkg}
		out, outcome, reason := runVerificationCommand(ctx, workspace, internalbuild.GetBuildEnv(nil, workspace), testVerifyTimeout, command[0], command[1:], verifyTestRunner)
		switch outcome {
		case VerifyPassed:
			logging.Get(logging.CategorySession).Info("test gate: %s builds only with -tags %s; compile-checked with go vet, its tests were not run", pkg, tagList)
		case VerifyFailed:
			return TestVerification{Ran: true, OK: false, Outcome: VerifyFailed, Output: strings.TrimSpace(string(out)), Command: command, Reason: reason}, false
		default: // VerifyCanceled, VerifyIndeterminate
			return TestVerification{Ran: true, Outcome: outcome, Output: strings.TrimSpace(string(out)), Command: command, Reason: reason}, false
		}
	}
	return TestVerification{}, true
}

// testFuncDecl matches a top-level Go test function's name.
var testFuncDecl = regexp.MustCompile(`(?m)^func (Test[A-Za-z0-9_]*)\(`)

// withWrittenTagGatedTests closes the hole a build tag opens in a green gate:
// a test file THIS turn wrote that the default tags exclude is not compiled by
// the package's `go test`, so the gate reported green over a test that never
// ran. Observed 2026-09-18: a turn asked for one test wrote it under
// //go:build integration (copied from the sibling it modelled), the gate ran
// the package without the tag, the turn was recorded /done, and the test failed
// the first time anyone ran it. A green verdict now also requires the written
// tag-gated tests to pass under their own tags; any other verdict stands as is.
func withWrittenTagGatedTests(ctx context.Context, workspace string, written []string, v TestVerification) TestVerification {
	if v.Verdict() != VerifyPassed {
		return v
	}
	for _, rel := range written {
		if !strings.HasSuffix(rel, "_test.go") {
			continue
		}
		abs := rel
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(workspace, rel)
		}
		tags := tagsForFile(abs)
		if len(tags) == 0 {
			continue
		}
		body, err := os.ReadFile(abs)
		if err != nil {
			continue
		}
		var names []string
		for _, m := range testFuncDecl.FindAllStringSubmatch(string(body), -1) {
			names = append(names, m[1])
		}
		if len(names) == 0 {
			continue
		}
		pkgs := packagesForPaths([]string{rel})
		if len(pkgs) == 0 {
			continue
		}
		logging.Session("Test gate: %s is gated by build tag(s) %v; running its %d test(s) under them", rel, tags, len(names))
		gated := verifyTests(ctx, workspace, pkgs,
			"-count=1", "-tags", strings.Join(tags, ","), "-run", "^("+strings.Join(names, "|")+")$")
		if gated.Verdict() != VerifyPassed {
			return gated
		}
	}
	return v
}
