package session_test

import (
	"slices"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/shards"
	"codenerd/internal/types"
)

// TestRemovedTestsGate_MatchesOnShardedKernel is the production shape of the
// /test_retention gate: each shard evaluates the corpus over its own facts,
// so a measurement homed away from turn_gate would derive the verdict on a
// single kernel and not on the one that ships. External test package: shards
// transitively imports session, so an internal test would cycle.
func TestRemovedTestsGate_MatchesOnShardedKernel(t *testing.T) {
	cortex := bootRetentionCortex(t)
	single, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	both := retentionPair{t: t, cortex: cortex, single: single}

	const (
		red        = "/turn_retention_red"
		green      = "/turn_retention_green"
		unmeasured = "/turn_retention_unmeasured"
	)
	// The closure is a green build and a green suite; the retention
	// measurement is the only thing that differs between the turns.
	both.assert(append(retentionClosure(red),
		core.Fact{Predicate: "turn_removed_test_ran", Args: []any{types.MangleAtom(red)}},
		core.Fact{Predicate: "turn_removed_test", Args: []any{types.MangleAtom(red), types.MangleString("x_test.go:TestGone")}},
	)...)
	both.assert(append(retentionClosure(green),
		core.Fact{Predicate: "turn_removed_test_ran", Args: []any{types.MangleAtom(green)}},
	)...)
	// A write the harness never measured for removals: owed nothing.
	both.assert(
		core.Fact{Predicate: "turn_written", Args: []any{types.MangleAtom(unmeasured), types.MangleString("pkg/foo.go"), types.MangleString(".go")}},
	)

	both.parity(
		"turn_gate", "turn_owes_gate", "turn_red_gate", "turn_unmet_gate",
		"turn_has_removed_test", "turn_verified", "turn_unverified",
		"turn_missing_evidence", "turn_done",
	)

	both.gate(red, "/test_retention", "/failing")
	both.gate(green, "/test_retention", "/passing")
	both.noGate(unmeasured, "/test_retention")

	both.has("turn_owes_gate", red, "/test_retention")
	both.has("turn_owes_gate", green, "/test_retention")
	both.lacks("turn_owes_gate", unmeasured, "/test_retention")
	both.has("turn_red_gate", red, "/test_retention")
	both.lacks("turn_red_gate", green, "/test_retention")
	both.has("turn_missing_evidence", red, "/tests_removed")
	both.lacks("turn_missing_evidence", green, "/tests_removed")
	both.lacks("turn_verified", red, "")
	both.has("turn_verified", green, "")
	both.lacks("turn_done", red, "")
	both.has("turn_done", green, "")
}

// retentionClosure is a green write turn's verdict inputs: evidence, a Go
// write, a passing build and a passing suite. No turn_verb, so /pinned is
// not owed; no catalog, so /check is not owed.
func retentionClosure(turn string) []core.Fact {
	atom := types.MangleAtom(turn)
	return []core.Fact{
		{Predicate: "turn_evidence", Args: []any{atom, types.MangleAtom("/fix"), 1, 1, 1, types.MangleAtom("/false"), types.MangleAtom("/false")}},
		{Predicate: "turn_written", Args: []any{atom, types.MangleString("pkg/foo.go"), types.MangleString(".go")}},
		{Predicate: "turn_gate", Args: []any{atom, types.MangleAtom("/build"), types.MangleAtom("/passing")}},
		{Predicate: "turn_test_measured", Args: []any{atom, types.MangleAtom("/passing")}},
	}
}

func bootRetentionCortex(t *testing.T) *core.CortexKernel {
	t.Helper()
	ws := t.TempDir()
	cortex := core.NewCortexKernel("cortex")
	if err := cortex.SetSharedPredicates(shards.SharedPredicates()); err != nil {
		t.Fatal(err)
	}
	for _, manifest := range shards.DefaultShardPredicateManifests() {
		shard, err := core.NewKernelShard(core.KernelShardConfig{
			Domain:          manifest.Domain,
			WorkspaceRoot:   ws,
			OwnedPredicates: append([]string(nil), manifest.OwnedPredicates...),
		})
		if err != nil {
			t.Fatalf("shard %s: %v", manifest.Domain, err)
		}
		if err := cortex.RegisterShard(shard); err != nil {
			t.Fatalf("register %s: %v", manifest.Domain, err)
		}
	}
	schemas, policy, err := core.DefaultCorpusText()
	if err != nil {
		t.Fatal(err)
	}
	owners := map[string]string{}
	for _, manifest := range shards.DefaultShardPredicateManifests() {
		for _, pred := range manifest.OwnedPredicates {
			owners[pred] = manifest.Domain
		}
	}
	shared := map[string]struct{}{}
	for _, pred := range shards.SharedPredicates() {
		shared[pred] = struct{}{}
	}
	dm, err := core.BuildDerivationMap(schemas+"\n"+policy, nil, owners, shared, "cortex")
	if err != nil {
		t.Fatal(err)
	}
	cortex.SetDerivationMap(dm)
	if err := cortex.Evaluate(); err != nil {
		t.Fatal(err)
	}
	return cortex
}

type retentionPair struct {
	t      *testing.T
	cortex *core.CortexKernel
	single *core.RealKernel
}

type retentionQuerier interface {
	Query(string) ([]core.Fact, error)
	Assert(core.Fact) error
}

func (p retentionPair) assert(facts ...core.Fact) {
	p.t.Helper()
	for _, fact := range facts {
		if err := p.cortex.Assert(fact); err != nil {
			p.t.Fatalf("cortex assert %s%v: %v", fact.Predicate, fact.Args, err)
		}
		if err := p.single.Assert(fact); err != nil {
			p.t.Fatalf("single assert %s%v: %v", fact.Predicate, fact.Args, err)
		}
	}
}

func (p retentionPair) parity(preds ...string) {
	p.t.Helper()
	for _, pred := range preds {
		got := p.rows(p.cortex, pred)
		want := p.rows(p.single, pred)
		if slices.Equal(got, want) {
			continue
		}
		p.t.Errorf("parity %s: only cortex %q; only single %q", pred, onlyRows(got, want), onlyRows(want, got))
	}
}

func (p retentionPair) gate(turn, gate, verdict string) {
	p.t.Helper()
	want := gate + "\t" + verdict
	for _, side := range []struct {
		name string
		q    retentionQuerier
	}{{"cortex", p.cortex}, {"single", p.single}} {
		if !slices.Contains(p.tails(side.q, "turn_gate", turn), want) {
			p.t.Errorf("%s turn_gate(%s) = %q, want %s", side.name, turn, p.tails(side.q, "turn_gate", turn), want)
		}
	}
}

func (p retentionPair) noGate(turn, gate string) {
	p.t.Helper()
	prefix := gate + "\t"
	for _, side := range []struct {
		name string
		q    retentionQuerier
	}{{"cortex", p.cortex}, {"single", p.single}} {
		for _, row := range p.tails(side.q, "turn_gate", turn) {
			if strings.HasPrefix(row, prefix) {
				p.t.Errorf("%s turn_gate(%s) = %q, want no %s verdict", side.name, turn, row, gate)
			}
		}
	}
}

func (p retentionPair) has(pred, turn, row string) {
	p.t.Helper()
	for _, side := range []struct {
		name string
		q    retentionQuerier
	}{{"cortex", p.cortex}, {"single", p.single}} {
		if !slices.Contains(p.tails(side.q, pred, turn), row) {
			p.t.Errorf("%s %s(%s) = %q, want %s", side.name, pred, turn, p.tails(side.q, pred, turn), row)
		}
	}
}

func (p retentionPair) lacks(pred, turn, row string) {
	p.t.Helper()
	for _, side := range []struct {
		name string
		q    retentionQuerier
	}{{"cortex", p.cortex}, {"single", p.single}} {
		if slices.Contains(p.tails(side.q, pred, turn), row) {
			p.t.Errorf("%s %s(%s) = %q, want it not to include %s", side.name, pred, turn, p.tails(side.q, pred, turn), row)
		}
	}
}

func (p retentionPair) rows(q retentionQuerier, pred string) []string {
	p.t.Helper()
	facts, err := q.Query(pred)
	if err != nil {
		p.t.Fatalf("query %s: %v", pred, err)
	}
	out := make([]string, 0, len(facts))
	for _, fact := range facts {
		parts := make([]string, len(fact.Args))
		for i, arg := range fact.Args {
			parts[i] = types.ExtractString(arg)
		}
		out = append(out, fact.Predicate+"("+strings.Join(parts, ", ")+")")
	}
	slices.Sort(out)
	return out
}

func (p retentionPair) tails(q retentionQuerier, pred, turn string) []string {
	p.t.Helper()
	facts, err := q.Query(pred)
	if err != nil {
		p.t.Fatalf("query %s: %v", pred, err)
	}
	var out []string
	for _, fact := range facts {
		if len(fact.Args) == 0 || types.ExtractString(fact.Args[0]) != turn {
			continue
		}
		if len(fact.Args) == 1 {
			out = append(out, "")
			continue
		}
		parts := make([]string, len(fact.Args)-1)
		for i, arg := range fact.Args[1:] {
			parts[i] = types.ExtractString(arg)
		}
		out = append(out, strings.Join(parts, "\t"))
	}
	slices.Sort(out)
	return out
}

func onlyRows(have, other []string) []string {
	seen := make(map[string]struct{}, len(other))
	for _, row := range other {
		seen[row] = struct{}{}
	}
	var out []string
	for _, row := range have {
		if _, ok := seen[row]; !ok {
			out = append(out, row)
		}
	}
	return out
}
