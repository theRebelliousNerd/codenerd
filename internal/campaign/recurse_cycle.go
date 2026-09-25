package campaign

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/gates"
	"codenerd/internal/projectdoc"
	"codenerd/internal/tools"
)

// The recurse cycle: measure, pick, fix, ratchet, one node at a time, bottom
// to top, pass after pass (Docs/journeys/10-forever-loop.md).
//
//	pass  measure the workspace-wide gates (once, then each pass's close)
//	visit for each node, leaves first:
//	        measure the node's own gates
//	        the kernel picks a finding (recurse_next)          -- or none
//	        one attempt: the executor edits the workspace
//	        re-measure the node and the ratchet gates
//	        the kernel judges it (recurse_ratchet): keep = commit, else revert
//	        until the kernel picks nothing more for this visit
//	close re-measure every workspace-wide gate; start the next pass
//
// Go runs gates, git and the executor, and asserts what it saw. What to
// attempt and whether to keep it are rules in policy/recurse.mg.

// RecurseAttempt is one attempt handed to the model.
type RecurseAttempt struct {
	Pass  int
	Cycle int
	Node  SubsystemNode
	// Finding is what to fix. Its gate, re-run, is the acceptance witness.
	Finding gates.Finding
	// Evidence is the output of the gate run that reported the finding.
	Evidence string
	// Check is that gate's command; the attempt is done when it passes.
	Check []string
	// Angle is an improvement attempt's angle ("stabilize", "harden",
	// "simplify", "extend"); "" for a fix. An improvement has no Finding.
	Angle string
	// Metrics are the numbers the improvement must move, measured before the
	// attempt (gates.MetricTests, MetricLines, MetricCoverage).
	Metrics map[string]int
	// NorthStar is the node's (else the workspace's) north star from nerd.md,
	// which the extend angle draws on. Empty when none is declared.
	NorthStar string
}

// RecurseExecutor runs one attempt to completion: a model turn or a one-task
// campaign that edits the workspace. What it reports is not evidence -- the
// loop re-measures -- so its error is recorded, never trusted either way.
type RecurseExecutor func(ctx context.Context, a RecurseAttempt) error

// RecurseCycleConfig configures a run.
type RecurseCycleConfig struct {
	Workspace string
	// Kernel carries policy/recurse.mg.
	Kernel core.Kernel
	// Execute runs one attempt.
	Execute RecurseExecutor
	// Passes bounds the run; zero runs until ctx is cancelled.
	Passes int
	// Subsystems narrows the sweep to these nodes and what they depend on.
	Subsystems []string
	// Branch is where kept attempts are committed; default DefaultRecurseBranch.
	Branch string
	// Progress receives one line per state change. Nil discards.
	Progress io.Writer
}

// RecurseCycleResult is what a run did.
type RecurseCycleResult struct {
	Passes     int
	Cycles     int
	Kept       int
	Reverted   int
	Refused    int
	Unverified int
	// Improved counts kept improvement attempts (included in Kept).
	Improved int
	// Stalled and Refusals are the findings the loop stopped attempting.
	Stalled  []string
	Refusals []string
	// UnavailableGates are gates that could not run here; what they cover is
	// unverified, never passed.
	UnavailableGates []string
}

// Summary is the run in a few lines, the same on every surface.
func (r *RecurseCycleResult) Summary() string {
	if r == nil {
		return ""
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Recurse: %d passes, %d attempts: %d kept (%d improvements), %d reverted, %d refused, %d unverified.",
		r.Passes, r.Cycles, r.Kept, r.Improved, r.Reverted, r.Refused, r.Unverified)
	if len(r.Stalled) > 0 {
		fmt.Fprintf(&b, "\n  stalled (failed the same way twice; retried once their node changes): %d", len(r.Stalled))
	}
	if len(r.Refusals) > 0 {
		fmt.Fprintf(&b, "\n  refused (reached for a forbidden path; the owner's to lift): %d", len(r.Refusals))
	}
	for _, u := range r.UnavailableGates {
		fmt.Fprintf(&b, "\n  gate that could not run: %s", u)
	}
	return b.String()
}

// gateRun is a gate's last measurement for one node (or the workspace).
type gateRun struct {
	result   gates.Result
	findings []gates.Finding
}

func (g gateRun) verdict() string {
	switch {
	case g.result.Unverified():
		return verdictUnverified
	case g.result.Passed:
		return verdictPass
	default:
		return verdictFail
	}
}

type recurseRun struct {
	cfg     RecurseCycleConfig
	root    string
	git     recurseGit
	policy  recursePolicy
	journal *recurseJournal
	doc     *projectdoc.Document
	out     io.Writer
	result  RecurseCycleResult

	cycle int
	// state is every gate's last measurement, keyed by gateKey. It is what
	// the tree looks like now: a kept attempt replaces the entries it
	// re-measured, a reverted one leaves them.
	state map[string]gateRun
	// seenLastPass and seenThisPass are the finding IDs each pass reported;
	// a finding absent last pass is a regression this pass.
	seenLastPass map[string]bool
	seenThisPass map[string]bool
}

func gateKey(gateID, node string) string { return gateID + "@" + node }

// RunRecurseCycles runs the loop until cfg.Passes passes finish or ctx is
// cancelled. A cancelled run leaves the tree as the last kept attempt left it:
// an attempt in flight is reverted before it returns, or on the next start.
func RunRecurseCycles(ctx context.Context, cfg RecurseCycleConfig) (*RecurseCycleResult, error) {
	if cfg.Kernel == nil || cfg.Execute == nil {
		return nil, errors.New("recurse: a kernel and an executor are required")
	}
	if cfg.Passes < 0 {
		return nil, errors.New("recurse: passes must be >= 0")
	}
	root, err := tools.CanonicalWorkspaceRoot(cfg.Workspace)
	if err != nil {
		return nil, fmt.Errorf("recurse: %w", err)
	}
	branch := cfg.Branch
	if branch == "" {
		branch = DefaultRecurseBranch
	}
	out := cfg.Progress
	if out == nil {
		out = io.Discard
	}
	doc, err := projectdoc.Load(root)
	if err != nil {
		return nil, fmt.Errorf("recurse: %w", err)
	}
	r := &recurseRun{
		cfg: cfg, root: root, git: recurseGit{root: root}, policy: recursePolicy{k: cfg.Kernel},
		doc: doc, out: out, state: map[string]gateRun{}, seenThisPass: map[string]bool{},
	}

	// One loop per workspace: a second would commit and revert the first's
	// attempts, and would settle its in-flight attempt out from under it.
	release, err := acquireRecurseLock(root)
	if err != nil {
		return nil, err
	}
	defer release()
	// A stop left over from a run that already ended is not for this one.
	if err := os.Remove(recurseStopPath(root)); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	// An attempt a killed run left in flight is settled before the checkout
	// is judged clean: its writes are the loop's, not the owner's.
	history, err := readRecurseJournal(root)
	if err != nil {
		return nil, err
	}
	if err := r.settleInFlight(ctx, history); err != nil {
		return nil, err
	}
	if err := r.git.prepare(ctx, branch); err != nil {
		return nil, err
	}
	journal, err := openRecurseJournal(root)
	if err != nil {
		return nil, err
	}
	defer journal.close()
	r.journal = journal
	// settleInFlight may have appended; read again for the resume point.
	if history, err = readRecurseJournal(root); err != nil {
		return nil, err
	}
	pass, done, err := r.resume(history)
	if err != nil {
		return nil, err
	}

	runErr := r.passes(ctx, pass, done)
	r.result.Stalled, _ = r.policy.stalled()
	r.result.Refusals, _ = r.policy.refused()
	return &r.result, runErr
}

// boundary is checked between nodes and between attempts: the loop ends on
// a cancelled context or on a stop request, never in the middle of judging an
// attempt.
func (r *recurseRun) boundary(ctx context.Context, pass int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !recurseStopRequested(r.root) {
		return nil
	}
	if err := os.Remove(recurseStopPath(r.root)); err != nil && !os.IsNotExist(err) {
		return err
	}
	if err := r.journal.append(recurseRecord{Step: stepStopped, Pass: pass}); err != nil {
		return err
	}
	r.logf("recurse: stopped on request")
	return ErrRecurseStopped
}

func (r *recurseRun) logf(format string, args ...any) {
	fmt.Fprintf(r.out, format+"\n", args...)
}

// passes runs from pass first; done lists the nodes pass first already
// finished (a resumed run).
func (r *recurseRun) passes(ctx context.Context, first int, done map[string]bool) error {
	for pass := first; ; pass++ {
		if r.cfg.Passes > 0 && r.result.Passes >= r.cfg.Passes {
			return nil
		}
		if err := r.boundary(ctx, pass); err != nil {
			return err
		}
		if err := r.pass(ctx, pass, done); err != nil {
			return err
		}
		done = nil
		r.result.Passes++
	}
}

func (r *recurseRun) pass(ctx context.Context, pass int, done map[string]bool) error {
	// The graph and the gates are re-derived each pass: the loop changes the
	// code, and the code is what they are derived from.
	nodes, err := RecurseSweepOrder(ctx, r.root, r.cfg.Subsystems)
	if err != nil {
		return err
	}
	set, err := gates.Detect(r.root)
	if err != nil {
		return err
	}
	r.noteUnavailable(set)
	ratchetKinds, err := r.policy.ratchetKinds()
	if err != nil {
		return err
	}

	if len(done) == 0 || len(r.state) == 0 {
		if err := r.measureWorkspace(ctx, set, nodes); err != nil {
			return err
		}
	}
	if err := r.journal.append(recurseRecord{Step: stepPassStart, Pass: pass, Open: r.openCount()}); err != nil {
		return err
	}
	r.logf("recurse pass %d: %d nodes, %d open findings", pass, len(nodes), r.openCount())

	for _, node := range nodes {
		if done[node.ID] {
			continue
		}
		if err := r.boundary(ctx, pass); err != nil {
			return err
		}
		if err := r.visit(ctx, pass, node, nodes, set, ratchetKinds); err != nil {
			return err
		}
	}

	if err := r.measureWorkspace(ctx, set, nodes); err != nil {
		return err
	}
	r.seenLastPass, r.seenThisPass = r.seenThisPass, map[string]bool{}
	return r.journal.append(recurseRecord{Step: stepPassEnd, Pass: pass, Open: r.openCount()})
}

func (r *recurseRun) noteUnavailable(set gates.Set) {
	r.result.UnavailableGates = r.result.UnavailableGates[:0]
	for _, u := range set.Unavailable {
		r.result.UnavailableGates = append(r.result.UnavailableGates, u.Gate.ID+": "+u.Reason)
	}
}

// measureWorkspace runs every workspace-scoped gate.
func (r *recurseRun) measureWorkspace(ctx context.Context, set gates.Set, nodes []SubsystemNode) error {
	for _, g := range set.Gates {
		if g.Scope != gates.ScopeAll {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		r.state[gateKey(g.ID, "")] = r.measure(ctx, g, "", nodes)
	}
	return nil
}

// measure runs one gate and attributes its findings to nodes. A node-scoped
// run's findings belong to the node it ran for; a workspace run's to the node
// whose directory holds the finding's target. A node that is a collapsed
// import cycle runs the gate once per directory, and fails if any run fails.
func (r *recurseRun) measure(ctx context.Context, g gates.Gate, nodeID string, nodes []SubsystemNode) gateRun {
	paths := []string{""}
	if nodeID != "" {
		paths = []string{nodeID}
		if n, ok := nodeByID(nodes, nodeID); ok && len(n.Paths) > 0 {
			paths = n.Paths
		}
	}
	var run gateRun
	for i, path := range paths {
		res := gates.Run(ctx, r.root, g, path)
		fs := gates.Findings(r.root, res)
		for j := range fs {
			if nodeID != "" {
				fs[j].Node = nodeID
			} else {
				fs[j].Node = attributeNode(fs[j].Target, nodes)
			}
			r.seenThisPass[fs[j].ID] = true
		}
		run.findings = append(run.findings, fs...)
		if i == 0 {
			run.result = res
			continue
		}
		// Combine: the first failure's command is the check, every run's
		// output is the evidence, and one unverified run leaves the whole
		// node unverified.
		run.result.Output += "\n" + res.Output
		if run.result.Passed && !res.Passed {
			run.result.Argv, run.result.ExitCode = res.Argv, res.ExitCode
		}
		run.result.Passed = run.result.Passed && res.Passed
		if run.result.Err == nil {
			run.result.Err = res.Err
		}
	}
	return run
}

func nodeByID(nodes []SubsystemNode, id string) (SubsystemNode, bool) {
	for _, n := range nodes {
		if n.ID == id {
			return n, true
		}
	}
	return SubsystemNode{}, false
}

// attributeNode names the node whose directory holds target (a file, or
// file::test). A target in no node's directory -- "." for a failure the output
// did not locate -- goes to the wiring node, which closes every pass.
func attributeNode(target string, nodes []SubsystemNode) string {
	t, _, _ := strings.Cut(target, "::")
	best, bestLen := RecurseWiringNodeID, -1
	for _, n := range nodes {
		for _, p := range n.Paths {
			var match bool
			if p == "." {
				match = t != "." && !strings.Contains(t, "/")
			} else {
				match = t == p || strings.HasPrefix(t, p+"/")
			}
			if match && len(p) > bestLen {
				best, bestLen = n.ID, len(p)
			}
		}
	}
	return best
}

func (r *recurseRun) openCount() int {
	seen := map[string]bool{}
	for _, run := range r.state {
		for _, f := range run.findings {
			seen[f.ID] = true
		}
	}
	return len(seen)
}

// nodeGates are the node-scoped gates that run on node.
func nodeGates(set gates.Set, node SubsystemNode) []gates.Gate {
	if node.CrossCutting {
		return nil
	}
	var out []gates.Gate
	for _, g := range set.Gates {
		if g.Scope == gates.ScopeNode && g.AppliesTo(node.Languages) {
			out = append(out, g)
		}
	}
	return out
}

// open is what the visit can attempt: the node's own gates' findings, and the
// workspace gates' findings attributed to it -- except a workspace gate that a
// per-node gate of its kind and language already measures (`go test ./...`
// beside `go test {pkg}`). Its failures are the per-node gate's, read with the
// package that failed; attempting them again from the whole-tree run would be
// the same failure twice, and its test failures name no package to put them
// in. Such a gate still runs at every pass boundary and in the ratchet.
func (r *recurseRun) open(node SubsystemNode, own []gates.Gate, set gates.Set) []gates.Finding {
	var out []gates.Finding
	for _, g := range own {
		out = append(out, r.state[gateKey(g.ID, node.ID)].findings...)
	}
	for _, g := range set.Gates {
		if g.Scope != gates.ScopeAll || measuredPerNode(set, g) {
			continue
		}
		for _, f := range r.state[gateKey(g.ID, "")].findings {
			if f.Node == node.ID {
				out = append(out, f)
			}
		}
	}
	gates.SortFindings(out)
	return out
}

// measuredPerNode reports whether a node-scoped gate of g's kind and language
// exists in set.
func measuredPerNode(set gates.Set, g gates.Gate) bool {
	for _, other := range set.Gates {
		if other.Scope == gates.ScopeNode && other.Kind == g.Kind && other.Language == g.Language {
			return true
		}
	}
	return false
}

func (r *recurseRun) regressions(open []gates.Finding) map[string]bool {
	if r.seenLastPass == nil {
		return nil
	}
	out := map[string]bool{}
	for _, f := range open {
		if !r.seenLastPass[f.ID] {
			out[f.ID] = true
		}
	}
	return out
}

func (r *recurseRun) visit(ctx context.Context, pass int, node SubsystemNode, nodes []SubsystemNode, set gates.Set, ratchetKinds map[gates.Kind]bool) error {
	own := nodeGates(set, node)
	for _, g := range own {
		if err := ctx.Err(); err != nil {
			return err
		}
		r.state[gateKey(g.ID, node.ID)] = r.measure(ctx, g, node.ID, nodes)
	}
	open := r.open(node, own, set)
	scope := visitScope{pass: pass, node: node, nodes: nodes, set: set, own: own, ratchetKinds: ratchetKinds}
	if err := r.journal.append(recurseRecord{Step: stepVisit, Pass: pass, Node: node.ID, Open: len(open)}); err != nil {
		return err
	}
	if err := r.policy.beginVisit(); err != nil {
		return err
	}
	if err := r.policy.visit(node.ID, open, r.regressions(open)); err != nil {
		return err
	}
	for {
		if err := r.boundary(ctx, pass); err != nil {
			return err
		}
		id, err := r.policy.next()
		if err != nil {
			return err
		}
		if id == "" {
			break
		}
		if err := r.attempt(ctx, scope, id, open); err != nil {
			return err
		}
		open = r.open(node, own, set)
		if err := r.policy.visit(node.ID, open, r.regressions(open)); err != nil {
			return err
		}
	}
	// Fixing what is red is the floor; every visit, red or green, ends with
	// one measured improvement attempt.
	if err := r.boundary(ctx, pass); err != nil {
		return err
	}
	if err := r.improve(ctx, scope); err != nil {
		return err
	}
	open = r.open(node, own, set)
	return r.journal.append(recurseRecord{Step: stepVisitDone, Pass: pass, Node: node.ID, Open: len(open)})
}

// inFlight is written before an attempt runs and removed after its ratchet,
// so a run killed in between can put the tree back on its next start.
type inFlight struct {
	Cycle           int      `json:"cycle"`
	Pass            int      `json:"pass"`
	Node            string   `json:"node"`
	Finding         string   `json:"finding"`
	Angle           string   `json:"angle,omitempty"`
	UntrackedBefore []string `json:"untracked_before"`
}

func inFlightPath(root string) string {
	return filepath.Join(root, ".nerd", "recurse", "inflight.json")
}

// visitScope is what one visit's attempts share.
type visitScope struct {
	pass         int
	node         SubsystemNode
	nodes        []SubsystemNode
	set          gates.Set
	own          []gates.Gate
	ratchetKinds map[gates.Kind]bool
}

// attemptSpec is one attempt: a fix of a finding, or an improvement.
type attemptSpec struct {
	finding   *gates.Finding
	evidence  string
	check     []string
	targetKey string
	angle     string
	describe  string
	subject   string
}

// attempt tries to fix the finding the kernel picked.
func (r *recurseRun) attempt(ctx context.Context, v visitScope, id string, open []gates.Finding) error {
	var target gates.Finding
	for _, f := range open {
		if f.ID == id {
			target = f
		}
	}
	if target.ID == "" {
		return fmt.Errorf("recurse: the kernel picked %s, which is not open on %s", id, v.node.ID)
	}
	if err := r.policy.attempted(id); err != nil {
		return err
	}
	source := r.sourceOf(target, v.node)
	return r.run(ctx, v, attemptSpec{
		finding:   &target,
		evidence:  source.result.Output,
		check:     source.result.Argv,
		targetKey: gateKey(target.Gate, sourceNode(target, v.node, v.set)),
		describe:  fmt.Sprintf("%s (%s)", target.Message, target.Gate),
		subject:   fmt.Sprintf("recurse: %s: %s", v.node.ID, oneLine(target.Message)),
	})
}

// improve runs the visit's improvement step from the angle the kernel aims it
// at. A node where the pass's angle has nothing to measure is skipped, and the
// journal says so.
func (r *recurseRun) improve(ctx context.Context, v visitScope) error {
	if v.node.CrossCutting {
		return nil
	}
	measurable := r.metrics(v.node, v.own, r.state)
	angle, err := r.policy.improveAngle(v.node.ID, v.pass, measurable)
	if err != nil {
		return err
	}
	if angle == "" {
		r.logf("recurse: %s: no improvement this pass (its angle has nothing to measure here)", v.node.ID)
		return r.journal.append(recurseRecord{Step: stepImproveSkipped, Pass: v.pass, Node: v.node.ID, Detail: "the pass's angle has no measurable metric on this node"})
	}
	return r.run(ctx, v, attemptSpec{
		angle:    angle,
		describe: "improve: " + angle,
		subject:  fmt.Sprintf("recurse: %s: %s", v.node.ID, angle),
	})
}

// run is one attempt from start to verdict: record it in flight, hand it to
// the executor, re-measure, let the kernel judge, keep or revert.
func (r *recurseRun) run(ctx context.Context, v visitScope, spec attemptSpec) error {
	node := v.node
	r.cycle++
	cycle := r.cycle
	r.result.Cycles++
	findingID := ""
	if spec.finding != nil {
		findingID = spec.finding.ID
	}
	before := r.metrics(node, v.own, r.state)

	untracked, err := r.git.untracked(ctx)
	if err != nil {
		return err
	}
	if err := writeInFlight(r.root, inFlight{Cycle: cycle, Pass: v.pass, Node: node.ID, Finding: findingID, Angle: spec.angle, UntrackedBefore: sortedSet(untracked)}); err != nil {
		return err
	}
	if err := r.journal.append(recurseRecord{Step: stepAttempt, Pass: v.pass, Cycle: cycle, Node: node.ID, Finding: findingID, Angle: spec.angle, Detail: spec.describe}); err != nil {
		return err
	}
	r.logf("recurse cycle %d: %s: %s", cycle, node.ID, spec.describe)

	a := RecurseAttempt{
		Pass: v.pass, Cycle: cycle, Node: node, Evidence: spec.evidence, Check: spec.check,
		Angle: spec.angle, Metrics: before, NorthStar: r.northStar(node),
	}
	if spec.finding != nil {
		a.Finding = *spec.finding
	}
	execErr := r.cfg.Execute(ctx, a)
	if ctx.Err() != nil {
		// Stopped mid-attempt: put the tree back now rather than leave it for
		// the next start. The in-flight record stays until that succeeds.
		if changed, err := r.changedByAttempt(context.WithoutCancel(ctx), untracked); err == nil {
			if err := r.git.revert(context.WithoutCancel(ctx), changed); err == nil {
				_ = os.Remove(inFlightPath(r.root))
			}
		}
		return ctx.Err()
	}

	changed, err := r.changedByAttempt(ctx, untracked)
	if err != nil {
		return err
	}
	after := map[string]gateRun{}
	afterMetrics := before
	if len(changed) > 0 {
		for _, g := range v.own {
			after[gateKey(g.ID, node.ID)] = r.measure(ctx, g, node.ID, v.nodes)
		}
		for _, g := range v.set.Gates {
			isTarget := spec.finding != nil && g.ID == spec.finding.Gate
			if g.Scope == gates.ScopeAll && (v.ratchetKinds[g.Kind] || isTarget) {
				after[gateKey(g.ID, "")] = r.measure(ctx, g, "", v.nodes)
			}
		}
		merged := make(map[string]gateRun, len(r.state)+len(after))
		for k, run := range r.state {
			merged[k] = run
		}
		for k, run := range after {
			merged[k] = run
		}
		afterMetrics = r.metrics(node, v.own, merged)
	}

	in := ratchetInput{
		Cycle: cycle, TargetID: findingID, Improve: spec.angle,
		Before: before, After: afterMetrics,
		Changed: len(changed) > 0, Forbidden: r.forbidden(changed),
	}
	keys := make([]string, 0, len(after))
	for k := range after {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	targetUnverified := false
	for _, k := range keys {
		b, a := r.state[k], after[k]
		if a.result.Gate.Kind == gates.Test && !a.result.Unverified() {
			in.Tested = true
		}
		in.Gates = append(in.Gates, ratchetGate{
			Gate: k, Before: b.verdict(), After: a.verdict(),
			BeforeCount: len(b.findings), AfterCount: len(a.findings),
		})
		if spec.finding != nil && k == spec.targetKey {
			targetUnverified = a.result.Unverified()
			in.TargetOK = !targetUnverified && !hasFinding(a.findings, findingID)
		}
	}
	verdict, err := r.policy.ratchet(in)
	if err != nil {
		return err
	}

	rec := recurseRecord{Step: stepRatchet, Pass: v.pass, Cycle: cycle, Node: node.ID, Finding: findingID, Angle: spec.angle, Metrics: metricsDelta(before, afterMetrics)}
	if execErr != nil {
		rec.Detail = "executor: " + execErr.Error()
	}
	switch verdict {
	case ratchetKeep:
		trailer := findingID
		if trailer == "" {
			trailer = "improve:" + spec.angle
		}
		hash, err := r.git.commit(ctx, changed, spec.subject, cycle, trailer)
		if err != nil {
			return err
		}
		for k, run := range after {
			r.state[k] = run
		}
		rec.Outcome, rec.Commit = outcomeKept, hash
		r.result.Kept++
		if spec.angle != "" {
			r.result.Improved++
		}
	case ratchetRefuse, ratchetRevert:
		if err := r.git.revert(ctx, changed); err != nil {
			return err
		}
		rec.Outcome = outcomeReverted
		switch {
		case verdict == ratchetRefuse:
			rec.Outcome = outcomeRefused
			rec.Detail = strings.TrimSpace("wrote forbidden paths: " + strings.Join(in.Forbidden, ", ") + " " + rec.Detail)
			r.result.Refused++
		case targetUnverified:
			rec.Outcome = outcomeUnverified
			r.result.Unverified++
		default:
			r.result.Reverted++
		}
		if spec.finding != nil {
			rec.Signature = r.failureSignature(cycle, *spec.finding, after[spec.targetKey], len(changed) > 0)
		}
	default:
		return fmt.Errorf("recurse: unknown ratchet verdict %q", verdict)
	}
	if err := r.policy.endCycle(); err != nil {
		return err
	}
	switch {
	case spec.finding != nil:
		err = r.policy.attempt(findingID, node.ID, cycle, rec.Outcome, rec.Signature)
	case rec.Outcome == outcomeKept:
		err = r.policy.nodeKept(node.ID, cycle)
	}
	if err != nil {
		return err
	}
	if err := r.journal.append(rec); err != nil {
		return err
	}
	r.logf("recurse cycle %d: %s %s", cycle, strings.TrimPrefix(rec.Outcome, "/"), rec.Metrics)
	return os.Remove(inFlightPath(r.root))
}

// metrics measures the numbers an attempt is judged by, reading coverage
// from the node's test runs in runs.
func (r *recurseRun) metrics(node SubsystemNode, own []gates.Gate, runs map[string]gateRun) map[string]int {
	m := map[string]int{}
	if n, err := gates.CountTests(r.root); err == nil {
		m[gates.MetricTests] = n
	}
	if !node.CrossCutting && len(node.Paths) > 0 {
		if n, err := gates.SourceLines(r.root, node.Paths, slices.Contains(node.Languages, "rust")); err == nil {
			m[gates.MetricLines] = n
		}
	}
	for _, g := range own {
		if g.Kind != gates.Test {
			continue
		}
		if cov, ok := gates.Coverage(runs[gateKey(g.ID, node.ID)].result.Output); ok {
			m[gates.MetricCoverage] = cov
			break
		}
	}
	return m
}

// metricsDelta renders before and after for the journal: "tests 10->12".
func metricsDelta(before, after map[string]int) string {
	var parts []string
	for _, name := range sortedMetricNames(before) {
		if a, ok := after[name]; ok {
			parts = append(parts, fmt.Sprintf("%s %d->%d", name, before[name], a))
		}
	}
	return strings.Join(parts, ", ")
}

// northStar is the north star nerd.md declares for node -- a nerd.md in the
// node's own directory first, else the workspace's.
func (r *recurseRun) northStar(node SubsystemNode) string {
	docs, err := projectdoc.LoadAll(r.root)
	if err != nil {
		return ""
	}
	var own, root *projectdoc.Document
	for _, d := range docs {
		dir := filepath.ToSlash(filepath.Dir(d.Path))
		if dir == "." {
			root = d
		} else if slices.Contains(node.Paths, dir) {
			own = d
		}
	}
	for _, d := range []*projectdoc.Document{own, root} {
		if d == nil || d.Spec.Northstar == nil {
			continue
		}
		ns := d.Spec.Northstar
		var b strings.Builder
		b.WriteString(strings.TrimSpace(ns.Purpose))
		for _, req := range ns.Requirements {
			fmt.Fprintf(&b, "\n- %s: %s", req.ID, strings.TrimSpace(req.Statement))
		}
		if text := strings.TrimSpace(b.String()); text != "" {
			return text
		}
	}
	return ""
}

// changedByAttempt is what the attempt changed, less the loop's own ledger
// under .nerd/recurse, which a workspace that does not ignore .nerd would
// otherwise hand to every attempt as a forbidden write.
func (r *recurseRun) changedByAttempt(ctx context.Context, untrackedBefore map[string]bool) ([]string, error) {
	changed, err := r.git.changed(ctx, untrackedBefore)
	if err != nil {
		return nil, err
	}
	out := changed[:0]
	for _, p := range changed {
		if !strings.HasPrefix(p, ".nerd/recurse/") {
			out = append(out, p)
		}
	}
	return out, nil
}

// sourceOf is the gate run that reported f.
func (r *recurseRun) sourceOf(f gates.Finding, node SubsystemNode) gateRun {
	if run, ok := r.state[gateKey(f.Gate, node.ID)]; ok && hasFinding(run.findings, f.ID) {
		return run
	}
	return r.state[gateKey(f.Gate, "")]
}

// sourceNode is the state key's node part for f's gate.
func sourceNode(f gates.Finding, node SubsystemNode, set gates.Set) string {
	for _, g := range set.Gates {
		if g.ID == f.Gate && g.Scope == gates.ScopeNode {
			return node.ID
		}
	}
	return ""
}

func hasFinding(fs []gates.Finding, id string) bool {
	for _, f := range fs {
		if f.ID == id {
			return true
		}
	}
	return false
}

// failureSignature says how an attempt failed, so the kernel can tell a
// repeated failure from a new one: the target's own message if it is still
// open, else the gates that got worse, else that nothing changed.
func (r *recurseRun) failureSignature(cycle int, target gates.Finding, after gateRun, changed bool) string {
	if !changed {
		return "no change: " + target.Signature
	}
	for _, f := range after.findings {
		if f.ID == target.ID {
			return "open: " + f.Signature
		}
	}
	worse, err := r.policy.worseGates(cycle)
	if err == nil && len(worse) > 0 {
		return "worse: " + strings.Join(worse, ", ")
	}
	return "unresolved: " + target.Signature
}

// forbidden returns the changed paths nerd.md forbids, plus anything under
// .git or .nerd, which no attempt may write whatever nerd.md says.
func (r *recurseRun) forbidden(changed []string) []string {
	var out []string
	for _, p := range changed {
		if p == ".git" || strings.HasPrefix(p, ".git/") || p == ".nerd" || strings.HasPrefix(p, ".nerd/") {
			out = append(out, p)
			continue
		}
		if r.doc != nil {
			if _, hit := r.doc.ForbidsPath(p); hit {
				out = append(out, p)
			}
		}
	}
	return out
}

// oneLine is the first line of s, for a commit subject or a title.
func oneLine(s string) string {
	s, _, _ = strings.Cut(s, "\n")
	return strings.TrimSpace(s)
}

func sortedSet(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func writeInFlight(root string, f inFlight) error {
	p := inFlightPath(root)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.Marshal(f)
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// settleInFlight finishes an attempt a killed run left behind. If its commit
// landed, the journal is told it was kept; otherwise its writes are put back.
// Either way the next run starts from a tree the ratchet has judged.
func (r *recurseRun) settleInFlight(ctx context.Context, history []recurseRecord) error {
	data, err := os.ReadFile(inFlightPath(r.root))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	var f inFlight
	if err := json.Unmarshal(data, &f); err != nil {
		return fmt.Errorf("recurse: %s is unreadable; inspect the tree and remove it: %w", inFlightPath(r.root), err)
	}
	for _, h := range history {
		if h.Step == stepRatchet && h.Cycle == f.Cycle {
			// Its verdict was journaled; only the cleanup was lost.
			return os.Remove(inFlightPath(r.root))
		}
	}
	journal, err := openRecurseJournal(r.root)
	if err != nil {
		return err
	}
	defer journal.close()
	kept, err := r.git.keptCycles(ctx, 50)
	if err != nil {
		return err
	}
	rec := recurseRecord{Step: stepRatchet, Pass: f.Pass, Cycle: f.Cycle, Node: f.Node, Finding: f.Finding, Angle: f.Angle}
	if hash, ok := kept[f.Cycle]; ok {
		rec.Outcome, rec.Commit, rec.Detail = outcomeKept, hash, "settled on restart: the commit landed"
	} else {
		before := map[string]bool{}
		for _, p := range f.UntrackedBefore {
			before[p] = true
		}
		changed, err := r.changedByAttempt(ctx, before)
		if err != nil {
			return err
		}
		if err := r.git.revert(ctx, changed); err != nil {
			return err
		}
		rec.Outcome, rec.Detail = outcomeReverted, "settled on restart: interrupted before its verdict"
		rec.Signature = "interrupted"
	}
	if err := journal.append(rec); err != nil {
		return err
	}
	return os.Remove(inFlightPath(r.root))
}

// resume reads where the last run stopped: the pass it was in, the nodes that
// pass finished, the cycle count, and every attempt's outcome (the stall rule
// needs them). A run whose last pass ended starts the next one.
func (r *recurseRun) resume(history []recurseRecord) (int, map[string]bool, error) {
	pass, done, ended := 0, map[string]bool{}, false
	for _, h := range history {
		r.cycle = max(r.cycle, h.Cycle)
		switch h.Step {
		case stepPassStart:
			if h.Pass != pass || ended {
				done = map[string]bool{}
			}
			pass, ended = h.Pass, false
		case stepVisitDone:
			if h.Pass == pass {
				done[h.Node] = true
			}
		case stepPassEnd:
			if h.Pass == pass {
				ended = true
			}
		case stepRatchet:
			var err error
			switch {
			case h.Finding != "":
				err = r.policy.attempt(h.Finding, h.Node, h.Cycle, h.Outcome, h.Signature)
			case h.Outcome == outcomeKept:
				err = r.policy.nodeKept(h.Node, h.Cycle)
			}
			if err != nil {
				return 0, nil, err
			}
		}
	}
	if len(history) == 0 {
		return 0, nil, nil
	}
	if ended {
		return pass + 1, nil, nil
	}
	return pass, done, nil
}
