package gates

import (
	"path"
	"slices"
	"strings"

	"codenerd/internal/lintreport"
)

// A recurse cycle re-measures the per-package lint gates only over the
// packages it touched (MeasureOptions.ScopedPackages): G5 over the whole of a
// large module is minutes, and a cycle touches a package or two. A scoped
// measurement is compared with the full baseline restricted to the same scope
// (RestrictTo), and a kept cycle's result is laid over the baseline (Overlay),
// so the baseline stays a whole-module picture between full measurements.

// InScope reports whether a finding's file lies in one of the package
// patterns ("./lib" is the package lib itself; "./lib/..." lib and below;
// "." the module root package).
func InScope(f lintreport.Finding, patterns []string) bool {
	dir := path.Dir(f.File)
	for _, p := range patterns {
		p = strings.TrimPrefix(strings.TrimSpace(p), "./")
		if base, ok := strings.CutSuffix(p, "/..."); ok {
			if base == "." || base == "" || dir == base || strings.HasPrefix(dir, base+"/") {
				return true
			}
			continue
		}
		if p == "" {
			p = "."
		}
		if dir == p {
			return true
		}
	}
	return false
}

// RestrictTo is s seen through after's scopes: each gate after measured over a
// scope keeps only s's findings in that scope, so the two count the same
// packages. Other gates are s's own.
func (s Snapshot) RestrictTo(after Snapshot) Snapshot {
	out := s
	out.ID = s.ID + "@" + after.ID
	out.Gates = make([]Measurement, 0, len(s.Gates))
	for _, g := range s.Gates {
		a, ok := after.Gate(g.Gate)
		if ok && len(a.Scope) > 0 && (g.Status == MeasurePass || g.Status == MeasureFail) {
			g = restrict(g, a.Scope, true)
		}
		out.Gates = append(out.Gates, g)
	}
	return out
}

// Overlay is s with after laid over it: a gate after measured over a scope
// keeps s's findings outside the scope and takes after's inside it; a gate
// after measured whole is after's. The result is the tree after a kept
// cycle. It is composed, not measured: its ID is s's and after's joined, and
// ComposedOf names every snapshot its rows came from, so it is never taken
// for a fresh whole-module measurement under after's ID.
func (s Snapshot) Overlay(after Snapshot) Snapshot {
	out := after
	out.ID = s.ID + "+" + after.ID
	out.ComposedOf = append(composedOf(s), composedOf(after)...)
	out.Gates = make([]Measurement, 0, len(s.Gates))
	for _, g := range s.Gates {
		a, ok := after.Gate(g.Gate)
		switch {
		case !ok:
		case slices.Equal(a.Scope, []string{NoPackageScope}):
			// Scoped to no package: nothing was observed, so nothing about
			// the gate changes -- not even an unavailable gate's status,
			// which the empty pass used to turn into a whole-module pass
			// (audit scoped-gate-overlay-evidence-conservation).
		case len(a.Scope) == 0 || a.Status == MeasureUnavailable || a.Status == MeasureUnverified:
			g = a
		case g.Status == MeasureUnavailable || g.Status == MeasureUnverified:
			// The baseline never gave this gate a verdict, so outside the
			// scope nothing is known: one measured package cannot complete
			// it. Keep the baseline's status and reason, with what the
			// scoped run did observe.
			kept := g
			kept.Findings = append([]lintreport.Finding{}, a.Findings...)
			kept.Count = len(kept.Findings)
			kept.Unenumerated = unenumerated(a)
			g = kept
		default:
			kept := restrict(g, a.Scope, false)
			kept.Findings = append(kept.Findings, a.Findings...)
			lintreport.Sort(kept.Findings)
			kept.Count = len(kept.Findings)
			kept.Unenumerated = unenumerated(g) || unenumerated(a)
			kept.Status = findingsStatus(kept)
			kept.Scope = nil
			g = kept
		}
		out.Gates = append(out.Gates, g)
	}
	return out
}

// findingsStatus is the status of a recounted pass/fail measurement: a
// failure while it holds a finding, or while part of its failure is one no
// finding accounts for (Unenumerated), which no restriction can locate; a
// pass otherwise. An unenumerated failure used to restrict and compose to a
// pass, because zero retained findings looked like a clean scope.
func findingsStatus(m Measurement) MeasureStatus {
	if m.Count > 0 || m.Unenumerated {
		return MeasureFail
	}
	return MeasurePass
}

// unenumerated reports whether part of m's failure is one no finding
// accounts for: flagged at measurement, or a failure with no finding at all
// (a snapshot written before the flag existed).
func unenumerated(m Measurement) bool {
	return m.Unenumerated || (m.Status == MeasureFail && len(m.Findings) == 0)
}

// composedOf is the snapshots s's rows came from: its ComposedOf, or s itself
// when it was measured.
func composedOf(s Snapshot) []string {
	if len(s.ComposedOf) > 0 {
		return append([]string(nil), s.ComposedOf...)
	}
	return []string{s.ID}
}

// restrict keeps m's findings in (inside true) or outside (inside false) the
// package patterns, recounting and re-deciding a findings-only status.
func restrict(m Measurement, patterns []string, inside bool) Measurement {
	m.Unenumerated = unenumerated(m)
	var kept []lintreport.Finding
	for _, f := range m.Findings {
		if InScope(f, patterns) == inside {
			kept = append(kept, f)
		}
	}
	m.Findings = kept
	if m.Findings == nil {
		m.Findings = []lintreport.Finding{}
	}
	m.Count = len(kept)
	m.Status = findingsStatus(m)
	if inside {
		m.Scope = append([]string(nil), patterns...)
		m.Packages = append([]string(nil), patterns...)
	}
	return m
}
