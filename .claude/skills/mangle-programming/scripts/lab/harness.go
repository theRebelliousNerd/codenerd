// Package harness runs one Mangle program on the pinned engine with every
// Mangle-only feature switched on, so an experiment can use the whole
// language from a Go test. new-lab.sh copies this file into
// _mangle_lab/harness/; edit the skill's copy, not the lab copy.
package harness

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
	"codeberg.org/TauCeti/mangle-go/parse"
	"codeberg.org/TauCeti/mangle-go/provenance"
)

// External answers an external predicate: given the bound input constants,
// it returns the output columns of every matching row.
type External func(inputs []ast.Constant) ([][]ast.BaseTerm, error)

type config struct {
	bounds    analysis.BoundsCheckingMode
	now       time.Time
	limit     int
	externals map[ast.PredicateSym]engine.ExternalPredicateCallback
}

// Option changes how Run analyzes or evaluates.
type Option func(*config)

// LooseBounds analyzes the way codeNERD's kernel does: Decl bounds unchecked.
func LooseBounds() Option { return func(c *config) { c.bounds = analysis.NoBoundsChecking } }

// At pins evaluation time, which temporal operators and `now` read.
func At(t time.Time) Option { return func(c *config) { c.now = t } }

// FactLimit caps created facts; the default is the kernel's 100000.
func FactLimit(n int) Option { return func(c *config) { c.limit = n } }

// WithExternal registers Go code behind an external predicate. The program
// must declare it with descr [external(), mode(...)].
func WithExternal(name string, arity int, fn External) Option {
	return func(c *config) {
		c.externals[ast.PredicateSym{Symbol: name, Arity: arity}] = callback{fn}
	}
}

type callback struct{ fn External }

func (callback) ShouldPushdown() bool { return false }
func (callback) ShouldQuery([]ast.Constant, []ast.BaseTerm, []ast.Term) bool {
	return true
}
func (c callback) ExecuteQuery(in []ast.Constant, _ []ast.BaseTerm, _ []ast.Term, cb func([]ast.BaseTerm)) error {
	rows, err := c.fn(in)
	for _, r := range rows {
		cb(r)
	}
	return err
}

// Result is an evaluated program.
type Result struct {
	t        testing.TB
	Info     *analysis.ProgramInfo
	Store    factstore.FactStore
	Temporal *factstore.TemporalStore
	Recorder *provenance.MemoryRecorder
	Stats    engine.Stats
}

// Run parses, analyzes with strict bounds, and evaluates src. Analysis or
// evaluation errors fail the test; use Try to inspect them instead.
func Run(t testing.TB, src string, opts ...Option) *Result {
	t.Helper()
	r, err := Try(t, src, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

// Try is Run that returns the error instead of failing.
func Try(t testing.TB, src string, opts ...Option) (*Result, error) {
	t.Helper()
	c := config{bounds: analysis.ErrorForBoundsMismatch, now: time.Now(), limit: 100000,
		externals: map[ast.PredicateSym]engine.ExternalPredicateCallback{}}
	for _, o := range opts {
		o(&c)
	}
	unit, err := parse.Unit(strings.NewReader(src))
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}
	info, err := analysis.AnalyzeAndCheckBounds([]parse.SourceUnit{unit}, nil, c.bounds)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}
	ts := factstore.NewTemporalStore()
	store := factstore.NewTemporalFactStoreAdapterAt(ts, c.now)
	rec := provenance.NewMemoryRecorder()
	stats, err := engine.EvalProgramWithStats(info, store,
		engine.WithTemporalStore(ts), engine.WithEvaluationTime(c.now),
		engine.WithExternalPredicates(c.externals), engine.WithCreatedFactLimit(c.limit),
		engine.WithDerivationRecorder(rec), engine.WithDeterministicOrder())
	r := &Result{t: t, Info: info, Store: store, Temporal: ts, Recorder: rec, Stats: stats}
	if err != nil {
		return r, fmt.Errorf("eval: %w", err)
	}
	return r, nil
}

// Facts returns every fact of pred (name/arity, e.g. "path/2"), sorted.
// Temporal facts carry their interval.
func (r *Result) Facts(pred string) []string {
	r.t.Helper()
	var name string
	var arity int
	if _, err := fmt.Sscanf(strings.Replace(pred, "/", " ", 1), "%s %d", &name, &arity); err != nil {
		r.t.Fatalf("Facts(%q): want name/arity: %v", pred, err)
	}
	q := ast.NewQuery(ast.PredicateSym{Symbol: name, Arity: arity})
	var out []string
	if decl, ok := r.Info.Decls[q.Predicate]; ok && decl.IsTemporal() {
		_ = r.Temporal.GetAllFacts(q, func(tf factstore.TemporalFact) error {
			out = append(out, tf.Atom.String()+"@"+tf.Interval.String())
			return nil
		})
	} else {
		_ = r.Store.GetFacts(q, func(a ast.Atom) error { out = append(out, a.String()); return nil })
	}
	sort.Strings(out)
	return out
}

// Why prints the recorded proof of a ground goal such as "path(1,3)".
func (r *Result) Why(goal string) string {
	r.t.Helper()
	a, err := parse.Atom(goal)
	if err != nil {
		r.t.Fatalf("Why(%q): %v", goal, err)
	}
	proofs, err := provenance.BuildFromRecording(r.Recorder, r.Store, a, provenance.Options{})
	if err != nil {
		return "no proof: " + err.Error()
	}
	var b bytes.Buffer
	_ = provenance.Print(&b, proofs)
	return b.String()
}

// ProofFacts returns the proof of goal as Mangle facts (proves/premise/...)
// so a second program can reason over why the first one decided.
func (r *Result) ProofFacts(goal string) []ast.Atom {
	r.t.Helper()
	a, err := parse.Atom(goal)
	if err != nil {
		r.t.Fatalf("ProofFacts(%q): %v", goal, err)
	}
	proofs, err := provenance.BuildFromRecording(r.Recorder, r.Store, a, provenance.Options{})
	if err != nil {
		r.t.Fatalf("ProofFacts(%q): %v", goal, err)
	}
	out := factstore.NewSimpleInMemoryStore()
	if err := provenance.EmitFacts(proofs, out); err != nil {
		r.t.Fatal(err)
	}
	var atoms []ast.Atom
	for _, p := range out.ListPredicates() {
		_ = out.GetFacts(ast.NewQuery(p), func(x ast.Atom) error { atoms = append(atoms, x); return nil })
	}
	return atoms
}
