package core

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/types"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/builtin"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

// =============================================================================
// INCLUSION CONSTRAINTS AT A BATCH BOUNDARY
// =============================================================================
//
// A Decl's `inclusion [...]` block says which facts must exist for a row to
// mean anything: `task_dependency(T, D) ... inclusion [campaign_task_id(D)]`
// says a dependency names a task the plan holds. Nothing evaluates these
// during the fixpoint, and checking them per assert would be wrong anyway:
// arrival order is not a semantic violation, and the referenced row may
// simply not have been written yet.
//
// So they are checked over a complete batch -- one epoch of a closed
// reference domain, such as a campaign snapshot -- before it is published:
//
//   - The batch is staged in a scratch store; the referenced relations are
//     derived there from the batch alone (their complete closure, never a
//     partially evaluated relation).
//   - A referenced relation whose derivation reaches anything the batch does
//     not hold (another predicate, an external() callback) belongs to an open
//     domain. Its membership is UNRESOLVED, not false: a missing row in an
//     open domain is not proof that no such row exists.
//   - A violation quarantines the whole batch: the caller does not publish it,
//     the previous epoch stays in force, and the error names the producer,
//     the predicate, the row and the constraint that failed.
//
// The production edge is the campaign plan (schemas_campaign.mg): a task's
// phase, a task or phase dependency, a context edge must name a task or phase
// of the same plan. Every load and sync of a campaign passes its full
// snapshot, so every epoch revalidates every edge.

// maxInclusionDiagnostics bounds the violations an InclusionError carries;
// the count of the rest is still reported.
const maxInclusionDiagnostics = 32

// InclusionViolation is one row whose inclusion constraint does not hold.
type InclusionViolation struct {
	Predicate string
	Fact      string
	Reason    string
}

// InclusionUnresolved names a constrained predicate whose referenced domain
// the batch does not close, so its rows could not be decided.
type InclusionUnresolved struct {
	Predicate string
	Rows      int
	Reason    string
}

// InclusionReport is the verdict over one batch.
type InclusionReport struct {
	Producer   string
	Checked    int
	Violations []InclusionViolation
	Omitted    int // violations beyond maxInclusionDiagnostics
	Unresolved []InclusionUnresolved
}

// InclusionError quarantines a batch. It carries the report so a caller can
// show the producer exactly which rows to fix.
type InclusionError struct {
	Report InclusionReport
}

func (e *InclusionError) Error() string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "batch from %s quarantined: %d of %d constrained rows violate their inclusion constraints",
		e.Report.Producer, len(e.Report.Violations)+e.Report.Omitted, e.Report.Checked)
	for _, v := range e.Report.Violations {
		fmt.Fprintf(&sb, "\n  %s: %s", v.Fact, v.Reason)
	}
	if e.Report.Omitted > 0 {
		fmt.Fprintf(&sb, "\n  ... and %d more", e.Report.Omitted)
	}
	return sb.String()
}

// ErrInclusionViolation is matched by every *InclusionError via errors.Is.
var ErrInclusionViolation = errors.New("inclusion constraint violated")

func (e *InclusionError) Is(target error) bool { return target == ErrInclusionViolation }

var _ types.BatchInclusionChecker = (*RealKernel)(nil)

// CheckBatchInclusion checks facts, one complete batch from producer, against
// the inclusion constraints of their Decls. It returns an *InclusionError when
// any decidable row violates its constraint, and nil otherwise; rows of open
// domains are logged as unresolved, never rejected.
func (k *RealKernel) CheckBatchInclusion(producer string, facts []Fact) error {
	report, err := k.InclusionReport(producer, facts)
	if err != nil {
		return err
	}
	for _, u := range report.Unresolved {
		logging.KernelDebug("inclusion: %s: %d %s rows unresolved: %s", producer, u.Rows, u.Predicate, u.Reason)
	}
	if len(report.Violations) > 0 {
		return &InclusionError{Report: report}
	}
	return nil
}

// InclusionReport is CheckBatchInclusion's full verdict.
func (k *RealKernel) InclusionReport(producer string, facts []Fact) (InclusionReport, error) {
	report := InclusionReport{Producer: producer}
	if k == nil || len(facts) == 0 {
		return report, nil
	}

	// Admit the batch under the program the kernel will evaluate it with.
	k.mu.Lock()
	if k.policyDirty || k.programInfo == nil {
		if err := k.rebuildProgram(); err != nil {
			k.mu.Unlock()
			return report, fmt.Errorf("inclusion check: %w", err)
		}
	}
	info, strata := k.programInfo, k.strata
	limit := k.effectiveDerivedFactLimitLocked()
	atoms := make([]ast.Atom, 0, len(facts))
	for _, f := range facts {
		atom, err := k.factToAtomLocked(sanitizeFactForNumericPredicates(f))
		if err != nil {
			continue // admission refuses it on load and says why; not this check's verdict
		}
		atoms = append(atoms, atom)
	}
	k.mu.Unlock()

	batchPreds := make(map[ast.PredicateSym]struct{})
	for _, a := range atoms {
		batchPreds[a.Predicate] = struct{}{}
	}

	// Which rows are constrained, and by which relations.
	type constrained struct {
		atom ast.Atom
		refs []ast.PredicateSym
	}
	var rows []constrained
	refSet := make(map[ast.PredicateSym]struct{})
	for _, a := range atoms {
		refs := inclusionReferences(info.Decls[a.Predicate])
		if len(refs) == 0 {
			continue
		}
		rows = append(rows, constrained{a, refs})
		for _, r := range refs {
			refSet[r] = struct{}{}
		}
	}
	if len(rows) == 0 {
		return report, nil
	}

	closure := newInclusionClosure(info, batchPreds)
	closed := make(map[ast.PredicateSym]string) // ref -> "" when closed, else why not
	for ref := range refSet {
		closed[ref] = closure.openReason(ref)
	}

	// Stage the batch and derive the closed referenced relations from it.
	store := factstore.NewSimpleInMemoryStore()
	for _, a := range atoms {
		store.Add(a)
	}
	if err := closure.derive(info, strata, store, limit); err != nil {
		return report, fmt.Errorf("inclusion check: derive reference domains: %w", err)
	}

	checker := engine.NewInclusionCheckerFromDesugared(info.Decls)
	typeChecker := builtin.NewTypeCheckerFromDesugared(info.Decls)
	unresolved := make(map[string]*InclusionUnresolved)
	markUnresolved := func(pred, reason string) {
		u, ok := unresolved[pred]
		if !ok {
			u = &InclusionUnresolved{Predicate: pred, Reason: reason}
			unresolved[pred] = u
		}
		u.Rows++
	}
	for _, row := range rows {
		reason := ""
		for _, ref := range row.refs {
			if why := closed[ref]; why != "" {
				reason = fmt.Sprintf("references %s/%d, %s", ref.Symbol, ref.Arity, why)
				break
			}
		}
		if reason == "" && typeChecker.CheckTypeBounds(row.atom) != nil {
			// Admitted only under the bare-string /name tolerance
			// (kernel_fact_decl.go); the upstream checker selects
			// alternatives by strict type, so it cannot decide this row.
			reason = "admitted under the bare-string /name tolerance"
		}
		if reason != "" {
			markUnresolved(row.atom.Predicate.Symbol, reason)
			continue
		}
		report.Checked++
		if err := checker.CheckFact(row.atom, store); err != nil {
			if len(report.Violations) >= maxInclusionDiagnostics {
				report.Omitted++
				continue
			}
			report.Violations = append(report.Violations, InclusionViolation{
				Predicate: row.atom.Predicate.Symbol,
				Fact:      row.atom.String(),
				Reason:    err.Error(),
			})
		}
	}
	for _, u := range unresolved {
		report.Unresolved = append(report.Unresolved, *u)
	}
	sort.Slice(report.Unresolved, func(i, j int) bool { return report.Unresolved[i].Predicate < report.Unresolved[j].Predicate })
	return report, nil
}

// inclusionReferences lists the relations a desugared Decl's inclusion
// constraints consult, in every alternative.
func inclusionReferences(decl *ast.Decl) []ast.PredicateSym {
	if decl == nil || decl.Constraints == nil {
		return nil
	}
	seen := make(map[ast.PredicateSym]struct{})
	var out []ast.PredicateSym
	add := func(a ast.Atom) {
		if _, ok := seen[a.Predicate]; !ok {
			seen[a.Predicate] = struct{}{}
			out = append(out, a.Predicate)
		}
	}
	for _, a := range decl.Constraints.Consequences {
		add(a)
	}
	for _, alt := range decl.Constraints.Alternatives {
		for _, a := range alt {
			add(a)
		}
	}
	return out
}

// inclusionClosure answers whether a referenced relation is decided by the
// batch alone, and derives the ones that are.
type inclusionClosure struct {
	batch    map[ast.PredicateSym]struct{}
	rules    map[ast.PredicateSym][]ast.Clause
	external map[ast.PredicateSym]struct{}
	// merge maps a lattice to its merge predicate. The Decl calls the
	// comparator, no rule body names it, yet it decides which rows the
	// lattice keeps: it is part of the lattice's closure.
	merge   map[ast.PredicateSym]ast.PredicateSym
	verdict map[ast.PredicateSym]string
	derived map[ast.PredicateSym]struct{} // heads needed by a closed reference
}

func newInclusionClosure(info *analysis.ProgramInfo, batch map[ast.PredicateSym]struct{}) *inclusionClosure {
	c := &inclusionClosure{
		batch:    batch,
		rules:    make(map[ast.PredicateSym][]ast.Clause),
		external: make(map[ast.PredicateSym]struct{}),
		merge:    make(map[ast.PredicateSym]ast.PredicateSym),
		verdict:  make(map[ast.PredicateSym]string),
		derived:  make(map[ast.PredicateSym]struct{}),
	}
	for _, r := range info.Rules {
		c.rules[r.Head.Predicate] = append(c.rules[r.Head.Predicate], r)
	}
	for sym, d := range info.Decls {
		if d != nil && d.IsExternal() {
			c.external[sym] = struct{}{}
		}
		if d != nil && len(d.FunDeps()) == 1 {
			if _, m := d.MergePredicate(); m.Symbol != "" {
				c.merge[sym] = m
			}
		}
	}
	return c
}

// openReason returns "" when pred's rows are fully determined by the batch,
// and otherwise why they are not. Only a top-level verdict is cached: inside
// a recursive cycle a back edge is provisionally closed, and caching that
// would let a sibling of an open predicate pass as closed.
func (c *inclusionClosure) openReason(pred ast.PredicateSym) string {
	if v, ok := c.verdict[pred]; ok {
		return v
	}
	reached := make(map[ast.PredicateSym]struct{})
	reason := c.walk(pred, reached)
	c.verdict[pred] = reason
	if reason == "" {
		// Everything reached is closed too, so its rules may run.
		for sym := range reached {
			if len(c.rules[sym]) > 0 {
				c.derived[sym] = struct{}{}
			}
		}
	}
	return reason
}

func (c *inclusionClosure) walk(pred ast.PredicateSym, reached map[ast.PredicateSym]struct{}) string {
	if _, seen := reached[pred]; seen {
		return "" // back edge: decided by the rest of the cycle
	}
	reached[pred] = struct{}{}
	switch {
	case pred.IsBuiltin():
		return ""
	case hasKey(c.external, pred):
		return "an external() predicate"
	}
	// Without its comparator's rules a lattice keeps every row as
	// incomparable, so a losing value would pass as a member (audit
	// mangle-inclusion-projection-closure). What the comparator reads must
	// be closed too, or the domain is open.
	if m, ok := c.merge[pred]; ok {
		if why := c.walk(m, reached); why != "" {
			return fmt.Sprintf("merged by %s/%d, %s", m.Symbol, m.Arity, why)
		}
	}
	switch {
	case len(c.rules[pred]) == 0:
		if !hasKey(c.batch, pred) {
			return fmt.Sprintf("whose domain %s/%d is not in this batch", pred.Symbol, pred.Arity)
		}
		return ""
	}
	for _, r := range c.rules[pred] {
		for _, p := range r.Premises {
			for _, dep := range premiseSyms(p) {
				if why := c.walk(dep, reached); why != "" {
					return fmt.Sprintf("derived from %s/%d, %s", dep.Symbol, dep.Arity, why)
				}
			}
		}
	}
	return ""
}

func premiseSyms(t ast.Term) []ast.PredicateSym {
	switch v := t.(type) {
	case ast.Atom:
		return []ast.PredicateSym{v.Predicate}
	case ast.NegAtom:
		return []ast.PredicateSym{v.Atom.Predicate}
	case ast.TemporalAtom:
		return []ast.PredicateSym{v.Atom.Predicate}
	case ast.TemporalLiteral:
		return premiseSyms(v.Literal)
	}
	return nil
}

// derive evaluates the closed referenced relations over store, in the
// program's stratum order. Program facts of those relations are part of the
// immutable program and are included, each with its time entry: the engine
// reads InitialFactTimes[i] for every initial fact it admits, so the two
// slices are filtered in lockstep.
func (c *inclusionClosure) derive(info *analysis.ProgramInfo, strata []analysis.Nodeset, store factstore.FactStore, limit int) error {
	if len(c.derived) == 0 {
		return nil
	}
	if len(info.InitialFactTimes) != len(info.InitialFacts) {
		return fmt.Errorf("inclusion: program has %d initial facts but %d time entries", len(info.InitialFacts), len(info.InitialFactTimes))
	}
	var rules []ast.Clause
	for _, r := range info.Rules {
		if hasKey(c.derived, r.Head.Predicate) {
			rules = append(rules, r)
		}
	}
	var initial []ast.Atom
	var initialTimes []*ast.Interval
	for i, f := range info.InitialFacts {
		if hasKey(c.derived, f.Predicate) || hasKey(c.batch, f.Predicate) {
			initial = append(initial, f)
			initialTimes = append(initialTimes, info.InitialFactTimes[i])
		}
	}
	var sub []analysis.Nodeset
	predToStratum := make(map[ast.PredicateSym]int)
	// A stratum is a layer of independent components (mangle.Stratify); the
	// closure keeps its own members of each.
	for _, stratum := range strata {
		members := make(analysis.Nodeset)
		for sym := range stratum {
			if hasKey(c.derived, sym) {
				members[sym] = struct{}{}
				predToStratum[sym] = len(sub)
			}
		}
		if len(members) > 0 {
			sub = append(sub, members)
		}
	}
	prog := &analysis.ProgramInfo{
		EdbPredicates:    info.EdbPredicates,
		IdbPredicates:    info.IdbPredicates,
		InitialFacts:     initial,
		InitialFactTimes: initialTimes,
		Rules:            rules,
		Decls:            info.Decls,
	}
	_, err := engine.EvalStratifiedProgramWithStats(prog, sub, predToStratum, store, engine.WithCreatedFactLimit(limit))
	return err
}

func hasKey[K comparable, V any](m map[K]V, k K) bool {
	_, ok := m[k]
	return ok
}

// CheckBatchInclusion routes a batch the way LoadFacts would and checks each
// shard's part against that shard's program. A reference that crosses into
// another shard is a predicate the shard's part does not hold, so it is
// unresolved there, never false.
func (c *CortexKernel) CheckBatchInclusion(producer string, facts []Fact) error {
	batches, err := c.groupByTarget(facts)
	if err != nil {
		return err
	}
	var errs []error
	for domain, batch := range batches {
		shard, _ := c.GetShard(domain)
		if shard == nil || shard.Kernel() == nil {
			continue
		}
		if err := shard.Kernel().CheckBatchInclusion(producer, batch); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

var _ types.BatchInclusionChecker = (*CortexKernel)(nil)
