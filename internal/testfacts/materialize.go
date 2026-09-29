package testfacts

import (
	"maps"
	"slices"
	"strings"
)

// result materializes the accumulated events into a deterministic Result.
func (a *accumulator) result() *Result {
	res := &Result{Status: StatusUnknown}
	buildLines := make(map[string][]string, len(a.buildOut))
	for ip, b := range a.buildOut {
		buildLines[ip] = splitLines(b.String())
	}
	// Build failures stand alone: a failed dependency may never get
	// package events of its own, so diagnostics are grouped by mapped
	// package over sorted ImportPaths, not by package entry.
	for _, ip := range slices.Sorted(maps.Keys(a.buildOut)) {
		pkgName := stripVariant(ip)
		for _, bf := range parseBuildLines(pkgName, buildLines[ip]) {
			res.BuildFailures = append(res.BuildFailures, bf)
		}
	}
	for name, pa := range a.pkgs {
		p := &Package{Name: name, Status: pa.status, Elapsed: pa.elapsed}
		p.Output = append(p.Output, buildLinesFor(name, buildLines)...)
		p.Output = append(p.Output, splitLines(pa.out.String())...)
		p.Status = a.finalPkgStatus(pa, p.Output)
		for tname, ta := range pa.tests {
			p.Tests = append(p.Tests, &Test{
				Name:    tname,
				Status:  ta.status,
				Elapsed: ta.elapsed,
				Output:  splitLines(ta.out.String()),
			})
		}
		slices.SortFunc(p.Tests, func(x, y *Test) int { return strings.Compare(x.Name, y.Name) })
		res.Packages = append(res.Packages, p)
	}
	// A build-fail for an ImportPath with no package events (a failed
	// dependency of a tested package) still gets a package entry: its
	// lines need a home, and the run did involve that package.
	for _, ip := range slices.Sorted(maps.Keys(a.buildFailed)) {
		if !a.buildFailed[ip] {
			continue
		}
		name := stripVariant(ip)
		if _, ok := a.pkgs[name]; ok {
			continue
		}
		dup := false
		for _, p := range res.Packages {
			if p.Name == name {
				dup = true
				break
			}
		}
		if dup {
			continue
		}
		res.Packages = append(res.Packages, &Package{
			Name:   name,
			Status: StatusBuildFailed,
			Output: buildLinesFor(name, buildLines),
		})
	}
	slices.SortFunc(res.Packages, func(x, y *Package) int { return strings.Compare(x.Name, y.Name) })
	res.Failures = extractFailures(res.Packages)
	res.Repeats = countRepeats(res.Packages, res.Raw)
	res.Raw = append([]string(nil), a.raw...)
	res.Status = overallStatus(res.Packages)
	return res
}

// buildLinesFor gathers build-output lines mapped to one package over
// sorted ImportPaths, so "pkg" and "pkg [pkg.test]" merge deterministically.
func buildLinesFor(name string, buildLines map[string][]string) []string {
	var out []string
	for _, ip := range slices.Sorted(maps.Keys(buildLines)) {
		if stripVariant(ip) == name {
			out = append(out, buildLines[ip]...)
		}
	}
	return out
}

// finalPkgStatus resolves skips and build failures after all events are
// in: a skip with the marker is no-test-files, a fail naming FailedBuild
// (or shadowed by a build-fail for its ImportPath) is build-failed.
func (a *accumulator) finalPkgStatus(pa *pkgAcc, output []string) Status {
	if pa.failedBld {
		return StatusBuildFailed
	}
	for ip, failed := range a.buildFailed {
		if failed && stripVariant(ip) == pa.name {
			return StatusBuildFailed
		}
	}
	if pa.status == StatusSkip {
		for _, line := range output {
			if strings.Contains(line, noTestFilesMarker) {
				return StatusNoTestFiles
			}
		}
	}
	return pa.status
}

// severity orders verdicts for the overall status; higher wins.
func severity(s Status) int {
	switch s {
	case StatusBuildFailed:
		return 5
	case StatusFail:
		return 4
	case StatusUnknown:
		return 3
	case StatusPass:
		return 2
	case StatusSkip:
		return 1
	case StatusNoTestFiles:
		return 0
	}
	return 3
}

// overallStatus folds package verdicts; a stream with no packages at all
// (all non-JSON input) degrades to unknown rather than failing.
func overallStatus(pkgs []*Package) Status {
	if len(pkgs) == 0 {
		return StatusUnknown
	}
	best := StatusNoTestFiles
	bestSev := -1
	for _, p := range pkgs {
		if s := severity(p.Status); s > bestSev {
			best, bestSev = p.Status, s
		}
	}
	return best
}
