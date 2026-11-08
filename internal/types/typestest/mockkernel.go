// Package typestest provides shared test doubles for the interfaces declared in
// internal/types.
//
// It is a separate package, not a _test.go file inside internal/types, because
// the consumers are other packages' tests: internal/campaign, internal/session,
// internal/system, internal/shards, internal/core and cmd/nerd/chat each carry a
// hand-written mockKernel today, and each one drifts from types.Kernel on its
// own schedule. A test-only file inside types would not be importable at all;
// this is (OPEN-QUESTIONS Q5, settled).
//
// It cannot create an import cycle: it imports internal/types and nothing else
// from this repo, and internal/types must never import it.
package typestest

import (
	"errors"
	"fmt"
	"sync"

	"codeberg.org/TauCeti/mangle-go/analysis"

	"codenerd/internal/types"
)

// MockKernel is an in-memory types.Kernel that also implements
// types.KernelTransactor, so code under test that calls types.NewKernelTx does
// not panic — the failure mode that makes hand-rolled mocks useless the moment
// the code they exercise starts batching updates.
//
// It stores facts, not derivations: there is no evaluation, so Query returns
// exactly what was asserted under a predicate. That is enough to assert "the
// code wrote the fact it claims to write" and deliberately not enough to test
// rule behaviour — use a real kernel for that.
type MockKernel struct {
	mu sync.Mutex

	facts map[string][]types.Fact

	// Recorded call log, in order, for tests that assert on sequencing
	// (e.g. that a retract preceded the re-assert).
	Calls []string

	// Failure injection. When set, the matching method returns this error and
	// makes no state change.
	AssertErr     error
	QueryErr      error
	LoadFactsErr  error
	RetractErr    error
	CommitErr     error
	ProgramInfoFn func() *analysis.ProgramInfo

	// Commits counts committed transactions. A test that expects one atomic
	// update should see 1, not N — that is the whole point of KernelTransactor.
	Commits int
}

// NewMockKernel returns an empty MockKernel ready for use.
func NewMockKernel() *MockKernel {
	return &MockKernel{facts: make(map[string][]types.Fact)}
}

func (m *MockKernel) ensure() {
	if m.facts == nil {
		m.facts = make(map[string][]types.Fact)
	}
}

func (m *MockKernel) record(format string, args ...any) {
	m.Calls = append(m.Calls, fmt.Sprintf(format, args...))
}

// --- types.Kernel ---------------------------------------------------------

func (m *MockKernel) LoadFacts(facts []types.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("LoadFacts(%d)", len(facts))
	if m.LoadFactsErr != nil {
		return m.LoadFactsErr
	}
	m.ensure()
	for _, f := range facts {
		m.facts[f.Predicate] = append(m.facts[f.Predicate], f)
	}
	return nil
}

func (m *MockKernel) Query(predicate string) ([]types.Fact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("Query(%s)", predicate)
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	m.ensure()
	src := m.facts[predicate]
	out := make([]types.Fact, len(src))
	for i, f := range src {
		cp := f
		if f.Args != nil {
			args := make([]any, len(f.Args))
			copy(args, f.Args)
			cp.Args = args
		}
		out[i] = cp
	}
	return out, nil
}

func (m *MockKernel) QueryAll() (map[string][]types.Fact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("QueryAll()")
	if m.QueryErr != nil {
		return nil, m.QueryErr
	}
	m.ensure()
	out := make(map[string][]types.Fact, len(m.facts))
	for k, v := range m.facts {
		cp := make([]types.Fact, len(v))
		for i, f := range v {
			dup := f
			if f.Args != nil {
				args := make([]any, len(f.Args))
				copy(args, f.Args)
				dup.Args = args
			}
			cp[i] = dup
		}
		out[k] = cp
	}
	return out, nil
}

func (m *MockKernel) Assert(fact types.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("Assert(%s)", fact.Predicate)
	if m.AssertErr != nil {
		return m.AssertErr
	}
	// Reject what a real kernel rejects: ToAtom is the same conversion
	// RealKernel performs on assert, so a mock that accepts a fact the kernel
	// would refuse hides the bug until production.
	if _, err := fact.ToAtom(); err != nil {
		return err
	}
	m.ensure()
	m.facts[fact.Predicate] = append(m.facts[fact.Predicate], fact)
	return nil
}

func (m *MockKernel) AssertBatch(facts []types.Fact) error {
	if len(facts) == 0 {
		return nil
	}
	// A batch is a convenience, not a transaction: good facts land, rejections
	// are collected and returned, matching RealKernel.AssertBatch. Stopping at
	// the first rejection would hide later facts that production would store.
	var rejected []error
	for _, f := range facts {
		if err := m.Assert(f); err != nil {
			rejected = append(rejected, fmt.Errorf("%s: %w", f.String(), err))
			continue
		}
	}
	if len(rejected) > 0 {
		return fmt.Errorf("AssertBatch rejected %d of %d facts: %w", len(rejected), len(facts), errors.Join(rejected...))
	}
	return nil
}

func (m *MockKernel) Retract(predicate string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("Retract(%s)", predicate)
	if m.RetractErr != nil {
		return m.RetractErr
	}
	m.ensure()
	delete(m.facts, predicate)
	return nil
}

func (m *MockKernel) RetractFact(fact types.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("RetractFact(%s)", fact.Predicate)
	if m.RetractErr != nil {
		return m.RetractErr
	}
	// Fail closed like RealKernel.RetractFact: matching needs a first argument,
	// so an arg-less fact is a caller bug, not a no-op retract-all.
	if len(fact.Args) == 0 {
		return fmt.Errorf("retractFact %s: fact must have at least one argument for matching", fact.Predicate)
	}
	m.ensure()
	kept := filterFacts(m.facts[fact.Predicate], func(f types.Fact) bool {
		return !firstArgEqual(f, fact)
	})
	if len(kept) == 0 {
		delete(m.facts, fact.Predicate)
	} else {
		m.facts[fact.Predicate] = kept
	}
	return nil
}

func (m *MockKernel) UpdateSystemFacts() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("UpdateSystemFacts()")
	return nil
}

// GetProgramInfo returns nil unless ProgramInfoFn is set. Callers that need
// declarations should use a real kernel; a fabricated ProgramInfo is worse than
// none because it would let a test pass against declarations that do not exist.
func (m *MockKernel) GetProgramInfo() *analysis.ProgramInfo {
	m.mu.Lock()
	fn := m.ProgramInfoFn
	m.mu.Unlock()
	if fn != nil {
		return fn()
	}
	return nil
}

func (m *MockKernel) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("Reset()")
	m.facts = make(map[string][]types.Fact)
}

func (m *MockKernel) AppendPolicy(policy string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("AppendPolicy(%d bytes)", len(policy))
}

func (m *MockKernel) RetractExactFactsBatch(facts []types.Fact) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("RetractExactFactsBatch(%d)", len(facts))
	if m.RetractErr != nil {
		return m.RetractErr
	}
	m.ensure()
	for _, target := range facts {
		kept := filterFacts(m.facts[target.Predicate], func(f types.Fact) bool {
			return !allArgsEqual(f, target)
		})
		if len(kept) == 0 {
			delete(m.facts, target.Predicate)
		} else {
			m.facts[target.Predicate] = kept
		}
	}
	return nil
}

func (m *MockKernel) RemoveFactsByPredicateSet(predicates map[string]struct{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("RemoveFactsByPredicateSet(%d)", len(predicates))
	if m.RetractErr != nil {
		return m.RetractErr
	}
	m.ensure()
	for p := range predicates {
		delete(m.facts, p)
	}
	return nil
}

// --- types.KernelTransactor ----------------------------------------------

// Transaction returns a buffering transaction. Nothing it records is visible to
// Query until Commit, which is the property production code relies on.
func (m *MockKernel) Transaction() types.KernelTransaction {
	return &mockTx{k: m}
}

// FactCount returns how many facts are stored under a predicate.
func (m *MockKernel) FactCount(predicate string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.facts[predicate])
}

type mockTxOp struct {
	kind      string // "assert", "retract", "retract_fact", "retract_exact", "retract_set"
	fact      types.Fact
	predicate string
	set       map[string]struct{}
}

type mockTx struct {
	k   *MockKernel
	ops []mockTxOp
}

func (t *mockTx) Retract(predicate string) {
	t.ops = append(t.ops, mockTxOp{kind: "retract", predicate: predicate})
}

func (t *mockTx) RetractFact(fact types.Fact) {
	t.ops = append(t.ops, mockTxOp{kind: "retract_fact", fact: fact})
}

func (t *mockTx) RetractExactFact(fact types.Fact) {
	t.ops = append(t.ops, mockTxOp{kind: "retract_exact", fact: fact})
}

func (t *mockTx) RetractPredicateSet(predicates map[string]struct{}) {
	cp := make(map[string]struct{}, len(predicates))
	for p := range predicates {
		cp[p] = struct{}{}
	}
	t.ops = append(t.ops, mockTxOp{kind: "retract_set", set: cp})
}

func (t *mockTx) Assert(fact types.Fact) {
	t.ops = append(t.ops, mockTxOp{kind: "assert", fact: fact})
}

func (t *mockTx) Commit() error {
	t.k.mu.Lock()
	commitErr := t.k.CommitErr
	t.k.mu.Unlock()
	if commitErr != nil {
		return commitErr
	}
	for _, op := range t.ops {
		var err error
		switch op.kind {
		case "assert":
			err = t.k.Assert(op.fact)
		case "retract":
			err = t.k.Retract(op.predicate)
		case "retract_fact":
			err = t.k.RetractFact(op.fact)
		case "retract_exact":
			err = t.k.RetractExactFactsBatch([]types.Fact{op.fact})
		case "retract_set":
			err = t.k.RemoveFactsByPredicateSet(op.set)
		}
		if err != nil {
			return err
		}
	}
	t.ops = nil
	t.k.mu.Lock()
	t.k.Commits++
	t.k.mu.Unlock()
	return nil
}

func filterFacts(facts []types.Fact, keep func(types.Fact) bool) []types.Fact {
	out := make([]types.Fact, 0, len(facts))
	for _, f := range facts {
		if keep(f) {
			out = append(out, f)
		}
	}
	return out
}

func firstArgEqual(a, b types.Fact) bool {
	if len(a.Args) == 0 || len(b.Args) == 0 {
		return len(a.Args) == len(b.Args)
	}
	return types.ExtractString(a.Args[0]) == types.ExtractString(b.Args[0])
}

func allArgsEqual(a, b types.Fact) bool {
	if len(a.Args) != len(b.Args) {
		return false
	}
	for i := range a.Args {
		if types.ExtractString(a.Args[i]) != types.ExtractString(b.Args[i]) {
			return false
		}
	}
	return true
}

// Compile-time proof that the mock keeps up with both interfaces. If
// types.Kernel gains a method, this file fails to build — which is the point:
// a mock that silently stops satisfying the interface it doubles is how test
// suites end up exercising an obsolete contract.
var (
	_ types.Kernel            = (*MockKernel)(nil)
	_ types.KernelTransactor  = (*MockKernel)(nil)
	_ types.KernelTransaction = (*mockTx)(nil)
)
