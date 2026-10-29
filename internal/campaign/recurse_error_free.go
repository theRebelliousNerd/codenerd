package campaign

import (
	"context"
	"fmt"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"codenerd/internal/gates"
)

// Rung R7 in the recurse loop: with RecurseCycleConfig.MeasureGates set, the
// error-free gates' findings are work the loop picks from, and every pick is
// proved by its own witness.
//
// The latest snapshot (gateBaseline) is grouped into units of work: every
// finding of one check, from one gate, in one package (gates.FindingGroups).
// Each group becomes a finding of the node its package belongs to, so the
// visit's candidates, stalls, refusals and history apply to it unchanged, and
// its shape is asserted as recurse_ef_group for policy/recurse.mg to rank:
// single-file groups with the workspace's own lint findings, multi-file
// classes after everything else, unused-code checks never. The attempt's
// brief is the symptom -- each finding's location, check and message -- and
// what should hold; its acceptance witness is the group's gate over the
// package with only that check (gates.WitnessArgv), which the ratchet runs
// again itself: the cycle is kept only when the witness passes and no gate
// got worse.

// refreshGates measures the error-free gates in full when the loop has none
// yet, or when FullGateMeasureEvery cycles have been judged on scoped
// measurements, and rebuilds the groups when the baseline changed. It runs
// between attempts, never during one.
func (r *recurseRun) refreshGates(ctx context.Context, nodes []SubsystemNode) error {
	if r.cfg.MeasureGates == nil {
		return nil
	}
	every := r.cfg.FullGateMeasureEvery
	if r.gateBaseline == nil || (every > 0 && r.cyclesSinceFull >= every) {
		if err := ctx.Err(); err != nil {
			return err
		}
		full := r.cfg.MeasureGates(ctx, nil)
		r.gateBaseline = &full
		r.cyclesSinceFull = 0
		r.efDirty = true
		r.logf("recurse: error-free gates measured in full (%s)", full.ID)
	}
	if !r.efDirty {
		return nil
	}
	r.efDirty = false
	r.efGroups = map[string]gates.FindingGroup{}
	r.efOpen = nil
	for _, g := range gates.FindingGroups(*r.gateBaseline) {
		r.efGroups[g.ID] = g
		target := strings.TrimPrefix(g.Package, "./")
		r.efOpen = append(r.efOpen, gates.Finding{
			ID:        g.ID,
			Gate:      "ef:" + g.Gate,
			Kind:      gates.Lint,
			Node:      attributeNode(target, nodes),
			Target:    target,
			Message:   groupMessage(g),
			Signature: g.Gate + " " + g.Check + " " + g.Package,
		})
	}
	return nil
}

// groupMessage names a group the way the finding list names a finding.
func groupMessage(g gates.FindingGroup) string {
	first := g.Findings[0]
	if len(g.Findings) == 1 {
		return fmt.Sprintf("%s %s: %s:%d: %s", g.Label, g.Check, first.File, first.Line, first.Message)
	}
	return fmt.Sprintf("%s %s: %d findings in %d file(s) of %s, first %s:%d: %s", g.Label, g.Check, len(g.Findings), g.Files, g.Package, first.File, first.Line, first.Message)
}

// errorFreeSpec is the attempt for a group. The brief is the symptom and
// what should hold, never a cause: the findings as the gate reported them,
// and the witness that must exit 0.
func errorFreeSpec(target gates.Finding, g gates.FindingGroup, node SubsystemNode) attemptSpec {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s) reports %d finding(s) of %s in %s:\n", g.Label, g.Gate, len(g.Findings), g.Check, g.Package)
	for _, f := range g.Findings {
		fmt.Fprintf(&b, "  %s:%d:%d: %s\n", f.File, f.Line, f.Column, f.Message)
	}
	fmt.Fprintf(&b, "\nWhat should hold: `%s` exits 0 -- no %s finding remains in %s -- and no other gate reports more findings than before.\n",
		strings.Join(g.Witness, " "), g.Check, g.Package)
	group := g
	return attemptSpec{
		finding:  &target,
		evidence: b.String(),
		check:    append([]string(nil), g.Witness...),
		group:    &group,
		describe: fmt.Sprintf("%s (%s)", target.Message, target.Gate),
		subject:  fmt.Sprintf("recurse: %s: %s %s in %s", node.ID, g.Label, g.Check, g.Package),
	}
}

// goPackagesOf is the Go packages the changed paths are in, as package
// patterns: "./dir", or "." for the module root. Non-Go files name none.
func goPackagesOf(changed []string) []string {
	seen := map[string]bool{}
	out := []string{} // non-nil: a cycle that touched no Go package scopes the lint gates to none
	for _, p := range changed {
		p = filepath.ToSlash(p)
		if !strings.HasSuffix(p, ".go") {
			continue
		}
		pkg := "."
		if dir := path.Dir(p); dir != "." {
			pkg = "./" + dir
		}
		if !seen[pkg] {
			seen[pkg] = true
			out = append(out, pkg)
		}
	}
	sort.Strings(out)
	return out
}
