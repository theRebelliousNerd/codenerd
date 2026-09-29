package system

import (
	"slices"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// TestTurnVerdict_OnProductionCortex is the production shape of the turn
// verdict. NewDomainCortex boots the domain shards from
// shards.DefaultShardPredicateManifests, the same table factory.go installs.
// witness_step6_test.go derives turn_verified on a hand-built manifest that
// leaves the derived predicates unowned; this test uses the manifests that
// ship.
//
// The facts are the ones a write turn's closure asserts, in the shapes those
// helpers emit (internal/session):
//   - turn_verb: assertTurnVerb, (Turn, Verb)
//   - turn_declared_check: prepareTurnCatalog, (Turn) when the task carries a check
//   - turn_evidence: assertTurnEvidence, (Turn, Verb, tools, writes, test runs,
//     claimed-output, dream). TestCheck.Ran counts as one test run. A completed
//     write records 1, 1, 1 and /false, /false.
//   - turn_written: assertTurnWrites, (Turn, path, lower-case extension)
//   - turn_gate plus build_state and test_state: recordBuildState, one verdict
//     per gate that ran
//   - turn_changed_element: assertTurnElements, (Turn, "fn:<pkg>.<Name>")
//   - turn_element_measured: the coverage walk in recordBuildState, the same ref
//
// Each derived predicate is read through CortexKernel.Query and compared with
// a single RealKernel that was fed the same facts.
func TestTurnVerdict_OnProductionCortex(t *testing.T) {
	ck, err := NewDomainCortex(t.TempDir())
	if err != nil {
		t.Fatalf("NewDomainCortex: %v", err)
	}
	single, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	both := verdictPair{t: t, cortex: ck, single: single}

	// (c) before the write: the catalog offers run_check, and nothing is owed
	// until a file exists.
	both.assert(verdictFacts(
		verb(turnCheck, "/fix"),
		declaredCheck(turnCheck),
	)...)
	both.parity("turn_catalog", "turn_owes_gate")
	both.has("turn_catalog", turnCheck, "/run_check")
	both.exact("turn_owes_gate", turnCheck)

	// (c) the write. (d) is the same Go write: /fix plus .go owes /pinned.
	both.assert(written(turnCheck, "pkg/checked.go", ".go"))
	both.parity("turn_catalog", "turn_owes_gate", "turn_write_class", "has_turn_written")
	both.has("turn_owes_gate", turnCheck, "/check")
	both.has("turn_owes_gate", turnCheck, "/pinned")

	// The closure. (a) every owed gate green and the changed element measured
	// by that passing run. (b) the same turn with the /test gate red. (c) the
	// check turn, witnessed, with no turn_gate for /check.
	both.assert(append(append(
		greenClosure(turnGreen, "pkg/foo.go", refGreen),
		redTestClosure(turnRed, "pkg/red.go", refRed)...),
		checkClosure(turnCheck, refCheck)...)...)

	both.parity(
		"turn_wrote", "has_turn_written", "turn_write_class",
		"turn_catalog", "turn_owes_gate", "turn_unmet_gate", "turn_red_gate",
		"has_unmet_gate", "has_red_gate",
		"turn_verified", "turn_unverified", "turn_missing_evidence",
		"turn_executed", "turn_done",
		"witness_owed", "witness_executed", "witness_met",
		"turn_unwitnessed", "turn_has_unwitnessed",
		"hollow_success", "has_hollow_success", "turn_build_red", "turn_vet_red",
	)

	// (a)
	both.exact("turn_verified", turnGreen, present)
	both.exact("turn_unverified", turnGreen)
	both.exact("turn_missing_evidence", turnGreen)
	both.exact("witness_met", turnGreen, refGreen)
	both.exact("turn_unwitnessed", turnGreen)
	both.exact("turn_owes_gate", turnGreen, "/build", "/pinned", "/test")

	// (d) on the green /fix Go write, stated on its own.
	both.has("turn_owes_gate", turnGreen, "/pinned")

	// (b) The changed element is owed, so an empty turn_unwitnessed is the
	// red gate suppressing it (witness.mg), not a turn that changed nothing.
	both.exact("witness_owed", turnRed, refRed)
	both.exact("turn_unwitnessed", turnRed)
	both.exact("turn_has_unwitnessed", turnRed)
	both.exact("turn_verified", turnRed)
	both.exact("turn_unverified", turnRed, present)
	both.exact("turn_missing_evidence", turnRed, "/tests_not_green")
	both.exact("turn_red_gate", turnRed, "/test")
	if rows := both.tails(both.cortex, "turn_unwitnessed", ""); len(rows) != 0 {
		t.Errorf("turn_unwitnessed = %q, want no rows under a red /test gate", rows)
	}

	// (c) no green run_check after the write.
	both.has("turn_catalog", turnCheck, "/run_check")
	both.exact("turn_owes_gate", turnCheck, "/build", "/check", "/pinned", "/test")
	both.exact("turn_verified", turnCheck)
	both.exact("turn_unverified", turnCheck, present)
	both.exact("turn_missing_evidence", turnCheck, "/check_not_green")
	both.exact("turn_unwitnessed", turnCheck)
}

const (
	turnGreen = "/turn_verdict_green"
	turnRed   = "/turn_verdict_red"
	turnCheck = "/turn_verdict_check"
	refGreen  = "fn:foo.Target"
	refRed    = "fn:red.Helper"
	refCheck  = "fn:checked.Target"
	// present marks a unary derived row. tails renders a one-argument fact as "".
	present = ""
)

type verdictPair struct {
	t      *testing.T
	cortex *core.CortexKernel
	single *core.RealKernel
}

func (p verdictPair) assert(facts ...core.Fact) {
	p.t.Helper()
	for _, f := range facts {
		if err := p.cortex.Assert(f); err != nil {
			p.t.Fatalf("cortex assert %s%v: %v", f.Predicate, f.Args, err)
		}
		if err := p.single.Assert(f); err != nil {
			p.t.Fatalf("single assert %s%v: %v", f.Predicate, f.Args, err)
		}
	}
}

// parity reports a predicate whose derived rows differ between the production
// cortex and the single kernel. A difference here is a shard-home split: the
// single kernel is the oracle for what the rules derive.
func (p verdictPair) parity(preds ...string) {
	p.t.Helper()
	for _, pred := range preds {
		got := p.rows(p.cortex, pred)
		want := p.rows(p.single, pred)
		if slices.Equal(got, want) {
			continue
		}
		p.t.Errorf("parity %s: cortex %d rows, single %d; only cortex %q; only single %q",
			pred, len(got), len(want), onlyRows(got, want), onlyRows(want, got))
	}
}

func (p verdictPair) exact(pred, turn string, want ...string) {
	p.t.Helper()
	gotC := p.tails(p.cortex, pred, turn)
	gotS := p.tails(p.single, pred, turn)
	slices.Sort(want)
	if !slices.Equal(gotC, gotS) {
		p.t.Errorf("%s(%s): production cortex = %q, single kernel = %q", pred, turn, gotC, gotS)
	}
	if !slices.Equal(gotS, want) {
		p.t.Errorf("%s(%s): single kernel = %q, want %q", pred, turn, gotS, want)
	}
}

func (p verdictPair) has(pred, turn, row string) {
	p.t.Helper()
	for _, side := range []struct {
		name string
		q    verdictQuerier
	}{
		{"production cortex", p.cortex},
		{"single kernel", p.single},
	} {
		got := p.tails(side.q, pred, turn)
		if !slices.Contains(got, row) {
			p.t.Errorf("%s %s(%s) = %q, want it to include %s", side.name, pred, turn, got, row)
		}
	}
}

type verdictQuerier interface {
	Query(predicate string) ([]core.Fact, error)
}

func (p verdictPair) rows(q verdictQuerier, pred string) []string {
	p.t.Helper()
	facts, err := q.Query(pred)
	if err != nil {
		p.t.Fatalf("query %s: %v", pred, err)
	}
	out := make([]string, 0, len(facts))
	for _, f := range facts {
		out = append(out, renderFact(f))
	}
	slices.Sort(out)
	return out
}

// tails is the fact's arguments after the turn, one string per row. turn == ""
// keeps every row, which is how the red-gate case asks for any turn_unwitnessed.
func (p verdictPair) tails(q verdictQuerier, pred, turn string) []string {
	p.t.Helper()
	facts, err := q.Query(pred)
	if err != nil {
		p.t.Fatalf("query %s: %v", pred, err)
	}
	var out []string
	for _, f := range facts {
		if len(f.Args) == 0 {
			continue
		}
		key := types.ExtractString(f.Args[0])
		if turn != "" && key != turn {
			continue
		}
		if turn == "" {
			out = append(out, renderFact(f))
			continue
		}
		if len(f.Args) == 1 {
			out = append(out, "")
			continue
		}
		parts := make([]string, len(f.Args)-1)
		for i, a := range f.Args[1:] {
			parts[i] = types.ExtractString(a)
		}
		out = append(out, strings.Join(parts, "\t"))
	}
	slices.Sort(out)
	return out
}

func renderFact(f core.Fact) string {
	parts := make([]string, len(f.Args))
	for i, a := range f.Args {
		parts[i] = types.ExtractString(a)
	}
	return f.Predicate + "(" + strings.Join(parts, ", ") + ")"
}

func onlyRows(have, other []string) []string {
	seen := make(map[string]struct{}, len(other))
	for _, s := range other {
		seen[s] = struct{}{}
	}
	var out []string
	for _, s := range have {
		if _, ok := seen[s]; !ok {
			out = append(out, s)
		}
	}
	return out
}

func verdictFacts(facts ...core.Fact) []core.Fact { return facts }

func atom(s string) types.MangleAtom { return types.MangleAtom(s) }

func mstr(s string) types.MangleString { return types.MangleString(s) }

func verb(turn, name string) core.Fact {
	return core.Fact{Predicate: "turn_verb", Args: []any{atom(turn), atom(name)}}
}

func declaredCheck(turn string) core.Fact {
	return core.Fact{Predicate: "turn_declared_check", Args: []any{atom(turn)}}
}

func written(turn, path, ext string) core.Fact {
	return core.Fact{Predicate: "turn_written", Args: []any{atom(turn), mstr(path), mstr(ext)}}
}

func evidence(turn, name string) core.Fact {
	// 1 tool, 1 write, 1 test run: mutationResult plus TestCheck.Ran.
	return core.Fact{Predicate: "turn_evidence", Args: []any{
		atom(turn), atom(name), 1, 1, 1, atom("/false"), atom("/false"),
	}}
}

func gate(turn, name, verdict string) core.Fact {
	return core.Fact{Predicate: "turn_gate", Args: []any{atom(turn), atom(name), atom(verdict)}}
}

func sessionState(pred, verdict string) core.Fact {
	return core.Fact{Predicate: pred, Args: []any{atom(verdict)}}
}

func changed(turn, ref string) core.Fact {
	return core.Fact{Predicate: "turn_changed_element", Args: []any{atom(turn), mstr(ref)}}
}

func measured(turn, ref string) core.Fact {
	return core.Fact{Predicate: "turn_element_measured", Args: []any{atom(turn), mstr(ref)}}
}

// greenClosure is a /fix Go write whose build, tests and pinning gate passed
// and whose changed element the passing run measured.
func greenClosure(turn, path, ref string) []core.Fact {
	return []core.Fact{
		verb(turn, "/fix"),
		evidence(turn, "/fix"),
		written(turn, path, ".go"),
		sessionState("build_state", "/passing"),
		sessionState("test_state", "/passing"),
		gate(turn, "/build", "/passing"),
		gate(turn, "/test", "/passing"),
		gate(turn, "/pinned", "/passing"),
		changed(turn, ref),
		measured(turn, ref),
	}
}

// redTestClosure is greenClosure with the /test gate red. The element was
// still measured: execution under a failing run is not a witness, and the
// red gate is what withholds turn_unwitnessed.
func redTestClosure(turn, path, ref string) []core.Fact {
	facts := greenClosure(turn, path, ref)
	for i, f := range facts {
		if f.Predicate == "turn_gate" && len(f.Args) == 3 && types.ExtractString(f.Args[1]) == "/test" {
			facts[i] = gate(turn, "/test", "/failing")
		}
		if f.Predicate == "test_state" {
			facts[i] = sessionState("test_state", "/failing")
		}
	}
	return facts
}

// checkClosure finishes the check turn. turn_verb, turn_declared_check and
// turn_written were asserted when the catalog and the write were measured.
// No /check gate: the acceptance command never ran after the write.
func checkClosure(turn, ref string) []core.Fact {
	return []core.Fact{
		evidence(turn, "/fix"),
		sessionState("build_state", "/passing"),
		sessionState("test_state", "/passing"),
		gate(turn, "/build", "/passing"),
		gate(turn, "/test", "/passing"),
		gate(turn, "/pinned", "/passing"),
		changed(turn, ref),
		measured(turn, ref),
	}
}
