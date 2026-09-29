package shards

import (
	"slices"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// TestTurnGate_DerivedRulesMatchOnShardedKernel is the production shape of
// the three gates that moved out of Go. Each shard evaluates the corpus
// over its own facts, so a measurement homed away from turn_gate would
// derive the verdict on a single kernel and not on the one that ships.
func TestTurnGate_DerivedRulesMatchOnShardedKernel(t *testing.T) {
	cortex := bootGateCortex(t)
	single, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	both := gatePair{t: t, cortex: cortex, single: single}

	const (
		oldTest    = "/turn_gate_old_test"
		newTest    = "/turn_gate_new_test"
		unnamed    = "/turn_gate_unnamed"
		vetOld     = "/turn_gate_vet_old"
		vetNew     = "/turn_gate_vet_new"
		vetDup     = "/turn_gate_vet_dup"
		checkGreen = "/turn_gate_check_green"
		checkRed   = "/turn_gate_check_red"
		checkEarly = "/turn_gate_check_early"
	)

	both.assert(append(goWrite(oldTest),
		gf("turn_test_measured", ma(oldTest), ma("/failing")),
		gf("turn_failing_test", ma(oldTest), ms("TestAlwaysFails"), ms("always fails")),
		gf("turn_test_failed_before", ma(oldTest), ms("TestAlwaysFails")),
	)...)
	both.assert(append(goWrite(newTest),
		gf("turn_test_measured", ma(newTest), ma("/failing")),
		gf("turn_failing_test", ma(newTest), ms("TestAlwaysFails"), ms("always fails")),
		gf("turn_failing_test", ma(newTest), ms("TestOK"), ms("bad")),
		gf("turn_test_failed_before", ma(newTest), ms("TestAlwaysFails")),
	)...)
	both.assert(append(goWrite(unnamed),
		gf("turn_test_measured", ma(unnamed), ma("/failing")),
	)...)
	both.assert(
		gf("turn_vet_ran", ma(vetOld)),
		gf("turn_vet_finding", ma(vetOld), ms("p/state.go"), ms("unreachable code"), int64(1)),
		gf("turn_vet_before", ma(vetOld), ms("p/state.go"), ms("unreachable code"), int64(1)),
	)
	both.assert(
		gf("turn_vet_ran", ma(vetNew)),
		gf("turn_vet_finding", ma(vetNew), ms("p/use.go"), ms("passes lock by value: p.State contains sync.Mutex"), int64(1)),
	)
	both.assert(
		gf("turn_vet_ran", ma(vetDup)),
		gf("turn_vet_finding", ma(vetDup), ms("p/state.go"), ms("unreachable code"), int64(2)),
		gf("turn_vet_before", ma(vetDup), ms("p/state.go"), ms("unreachable code"), int64(1)),
	)
	both.assert(append(otherWrite(checkGreen),
		gf("turn_write_seq", ma(checkGreen), int64(1)),
		gf("turn_check_run", ma(checkGreen), int64(2), int64(0)),
		gf("turn_test_run", ma(checkGreen), int64(3), int64(0)),
	)...)
	both.assert(append(otherWrite(checkRed),
		gf("turn_write_seq", ma(checkRed), int64(1)),
		gf("turn_check_run", ma(checkRed), int64(2), int64(1)),
		gf("turn_test_run", ma(checkRed), int64(3), int64(0)),
	)...)
	both.assert(append(otherWrite(checkEarly),
		gf("turn_write_seq", ma(checkEarly), int64(2)),
		gf("turn_check_run", ma(checkEarly), int64(1), int64(0)),
	)...)

	both.parity(
		"turn_gate", "turn_red_gate", "turn_unmet_gate", "turn_missing_evidence",
		"turn_own_test_failure", "turn_vet_new", "turn_vet_red",
		"turn_last_check", "turn_last_test_run",
	)

	both.gate(oldTest, "/test", "/passing")
	both.gate(newTest, "/test", "/failing")
	both.gate(unnamed, "/test", "/failing")
	both.gate(vetOld, "/vet", "/passing")
	both.gate(vetNew, "/vet", "/failing")
	both.gate(vetDup, "/vet", "/failing")
	both.gate(checkGreen, "/check", "/passing")
	both.gate(checkGreen, "/test_run", "/passing")
	both.gate(checkRed, "/check", "/failing")
	both.gate(checkRed, "/test_run", "/passing")
	both.noGate(checkEarly, "/check")
	both.noGate(checkEarly, "/test_run")

	both.has("turn_red_gate", newTest, "/test")
	both.has("turn_red_gate", unnamed, "/test")
	both.lacks("turn_red_gate", oldTest, "/test")
	both.has("turn_vet_red", vetNew, "")
	both.has("turn_vet_red", vetDup, "")
	both.lacks("turn_vet_red", vetOld, "")
	both.has("turn_red_gate", checkRed, "/check")
	both.lacks("turn_red_gate", checkGreen, "/check")
	both.has("turn_unmet_gate", checkEarly, "/check")
	both.lacks("turn_red_gate", checkEarly, "/check")
	both.has("turn_missing_evidence", checkRed, "/check_not_green")
	both.lacks("turn_missing_evidence", checkGreen, "/check_not_green")
	both.has("turn_missing_evidence", checkEarly, "/check_not_green")
}

func bootGateCortex(t *testing.T) *core.CortexKernel {
	t.Helper()
	ws := t.TempDir()
	cortex := core.NewCortexKernel("cortex")
	if err := cortex.SetSharedPredicates(SharedPredicates()); err != nil {
		t.Fatal(err)
	}
	for _, manifest := range DefaultShardPredicateManifests() {
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
	for _, manifest := range DefaultShardPredicateManifests() {
		for _, pred := range manifest.OwnedPredicates {
			owners[pred] = manifest.Domain
		}
	}
	shared := map[string]struct{}{}
	for _, pred := range SharedPredicates() {
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

type gateQuerier interface {
	Query(string) ([]core.Fact, error)
	Assert(core.Fact) error
}

type gatePair struct {
	t      *testing.T
	cortex *core.CortexKernel
	single *core.RealKernel
}

func (p gatePair) assert(facts ...core.Fact) {
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

func (p gatePair) parity(preds ...string) {
	p.t.Helper()
	for _, pred := range preds {
		got := p.rows(p.cortex, pred)
		want := p.rows(p.single, pred)
		if slices.Equal(got, want) {
			continue
		}
		p.t.Errorf("parity %s: only cortex %q; only single %q", pred, onlyGateRows(got, want), onlyGateRows(want, got))
	}
}

func (p gatePair) gate(turn, gate, verdict string) {
	p.t.Helper()
	want := gate + "\t" + verdict
	for _, side := range []struct {
		name string
		q    gateQuerier
	}{{"cortex", p.cortex}, {"single", p.single}} {
		got := p.tails(side.q, "turn_gate", turn)
		if !slices.Contains(got, want) {
			p.t.Errorf("%s turn_gate(%s) = %q, want %s", side.name, turn, got, want)
		}
	}
}

func (p gatePair) noGate(turn, gate string) {
	p.t.Helper()
	prefix := gate + "\t"
	for _, side := range []struct {
		name string
		q    gateQuerier
	}{{"cortex", p.cortex}, {"single", p.single}} {
		for _, row := range p.tails(side.q, "turn_gate", turn) {
			if strings.HasPrefix(row, prefix) {
				p.t.Errorf("%s turn_gate(%s) = %q, want no %s verdict", side.name, turn, row, gate)
			}
		}
	}
}

func (p gatePair) has(pred, turn, row string) {
	p.t.Helper()
	for _, side := range []struct {
		name string
		q    gateQuerier
	}{{"cortex", p.cortex}, {"single", p.single}} {
		if !slices.Contains(p.tails(side.q, pred, turn), row) {
			p.t.Errorf("%s %s(%s) = %q, want %s", side.name, pred, turn, p.tails(side.q, pred, turn), row)
		}
	}
}

func (p gatePair) lacks(pred, turn, row string) {
	p.t.Helper()
	for _, side := range []struct {
		name string
		q    gateQuerier
	}{{"cortex", p.cortex}, {"single", p.single}} {
		if slices.Contains(p.tails(side.q, pred, turn), row) {
			p.t.Errorf("%s %s(%s) = %q, want it not to include %s", side.name, pred, turn, p.tails(side.q, pred, turn), row)
		}
	}
}

func (p gatePair) rows(q gateQuerier, pred string) []string {
	p.t.Helper()
	facts, err := q.Query(pred)
	if err != nil {
		p.t.Fatalf("query %s: %v", pred, err)
	}
	out := make([]string, 0, len(facts))
	for _, fact := range facts {
		out = append(out, renderGateFact(fact))
	}
	slices.Sort(out)
	return out
}

func (p gatePair) tails(q gateQuerier, pred, turn string) []string {
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

func renderGateFact(fact core.Fact) string {
	parts := make([]string, len(fact.Args))
	for i, arg := range fact.Args {
		parts[i] = types.ExtractString(arg)
	}
	return fact.Predicate + "(" + strings.Join(parts, ", ") + ")"
}

func onlyGateRows(have, other []string) []string {
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

func ma(s string) types.MangleAtom { return types.MangleAtom(s) }

func ms(s string) types.MangleString { return types.MangleString(s) }

func gf(pred string, args ...any) core.Fact {
	return core.Fact{Predicate: pred, Args: args}
}

func goWrite(turn string) []core.Fact {
	return []core.Fact{
		gf("turn_verb", ma(turn), ma("/fix")),
		gf("turn_written", ma(turn), ms("pkg/foo.go"), ms(".go")),
		gf("turn_evidence", ma(turn), ma("/fix"), 1, 1, 1, ma("/false"), ma("/false")),
	}
}

func otherWrite(turn string) []core.Fact {
	return []core.Fact{
		gf("turn_verb", ma(turn), ma("/fix")),
		gf("turn_declared_check", ma(turn)),
		gf("turn_written", ma(turn), ms("policy/x.mg"), ms(".mg")),
		gf("turn_evidence", ma(turn), ma("/fix"), 1, 1, 1, ma("/false"), ma("/false")),
	}
}
