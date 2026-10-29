package mangle

import (
	"sort"
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

// layerProgram holds every shape a layer must not mix up: independent
// components (which may share a layer), recursion, negation, an aggregate and
// its consumer, a lattice whose merge reads a ranking the lattice rule does
// not name in its body (a top-down read analysis.Stratify cannot see), and a
// deferred premise.
const layerProgram = `
	Decl edge(X, Y).
	Decl reach(X, Y).
	Decl raised(X).
	Decl cleared(X).
	Decl open(X).
	Decl open_count(N).
	Decl busy().
	Decl island_a(X).
	Decl island_b(X).
	Decl island_c(X).
	Decl score(K, O, S).
	Decl promoted(O).
	Decl rank(O, R).
	Decl best(K, B)
		descr [fundep([K], [B]), merge([B], "better")].
	Decl better(A, B, Best) descr [mode('+', '+', '-'), deferred()].
	Decl pick(K, O).
	Decl outranks(A, B) descr [mode('+', '+'), deferred()].
	Decl flagged(O).
	Decl ranked_pair(A, B).

	reach(X, Y) :- edge(X, Y).
	reach(X, Z) :- edge(X, Y), reach(Y, Z).
	open(X) :- raised(X), !cleared(X).
	open_count(N) :- open(_) |> do fn:group_by(), let N = fn:count().
	busy() :- open_count(N), N >= 2.
	island_a(X) :- raised(X).
	island_b(X) :- island_a(X).
	island_c(X) :- edge(X, _).

	rank(O, 0) :- score(_, O, _), promoted(O).
	rank(O, 1) :- score(_, O, _), !promoted(O).
	best(K, B) :- score(K, O, S), B = fn:pair(O, S).
	better(A, B, A) :- :match_pair(A, OA, SA), :match_pair(B, OB, SB), SA > SB.
	better(A, B, B) :- :match_pair(A, OA, SA), :match_pair(B, OB, SB), SB > SA.
	better(A, B, A) :- :match_pair(A, OA, SA), :match_pair(B, OB, SB), SA = SB, rank(OA, RA), rank(OB, RB), RA < RB.
	better(A, B, B) :- :match_pair(A, OA, SA), :match_pair(B, OB, SB), SA = SB, rank(OA, RA), rank(OB, RB), RB < RA.
	pick(K, O) :- best(K, B), :match_pair(B, O, _).
	outranks(A, B) :- rank(A, RA), rank(B, RB), RA < RB.
	ranked_pair(A, B) :- flagged(A), flagged(B), outranks(A, B).

	edge(/a, /b). edge(/b, /c). edge(/c, /a).
	raised(/a). raised(/b). raised(/c). cleared(/b).
	score(/k1, /x, 2). score(/k1, /y, 2). score(/k2, /x, 1). score(/k2, /z, 3).
	promoted(/y).
	flagged(/x). flagged(/y).
`

func analyzeForLayers(t *testing.T, src string) *analysis.ProgramInfo {
	t.Helper()
	unit, err := ParseUnit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	info, err := analysis.AnalyzeOneUnit(unit, nil)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}
	return info
}

func evalDump(t *testing.T, info *analysis.ProgramInfo, strata []analysis.Nodeset, predToStratum map[ast.PredicateSym]int) string {
	t.Helper()
	store := factstore.NewSimpleInMemoryStore()
	if _, err := engine.EvalStratifiedProgramWithStats(info, strata, predToStratum, store); err != nil {
		t.Fatalf("eval: %v", err)
	}
	var lines []string
	_ = factstore.GetAllFacts(store, func(a ast.Atom) error {
		lines = append(lines, a.String())
		return nil
	})
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// checkLayering asserts the layering contract against the strata it came
// from: every component lands whole in one layer, every body premise of
// another component lies in an earlier layer, and every pair of components
// joined by a top-down read keeps its original order.
func checkLayering(t *testing.T, info *analysis.ProgramInfo, strata []analysis.Nodeset, predToStratum map[ast.PredicateSym]int, layers []analysis.Nodeset, predToLayer map[ast.PredicateSym]int) {
	t.Helper()
	if len(predToLayer) != len(predToStratum) {
		t.Fatalf("layers hold %d predicates, the strata %d", len(predToLayer), len(predToStratum))
	}
	for i, stratum := range strata {
		layer := -1
		for sym := range stratum {
			l, ok := predToLayer[sym]
			if !ok {
				t.Fatalf("%v (stratum %d) is in no layer", sym, i)
			}
			if _, in := layers[l][sym]; !in {
				t.Fatalf("predToLayer puts %v in layer %d, which does not hold it", sym, l)
			}
			if layer >= 0 && l != layer {
				t.Fatalf("stratum %d is split across layers %d and %d", i, layer, l)
			}
			layer = l
		}
	}
	rulesByHead := make(map[ast.PredicateSym][]ast.Clause)
	for _, r := range info.Rules {
		rulesByHead[r.Head.Predicate] = append(rulesByHead[r.Head.Predicate], r)
	}
	topDown := topDownDependencies(info, rulesByHead)
	for _, rule := range info.Rules {
		head := rule.Head.Predicate
		for _, premise := range rule.Premises {
			sym, ok := premiseSymbol(premise)
			if !ok || predToStratum[sym] == predToStratum[head] {
				continue
			}
			if _, idb := predToLayer[sym]; idb && predToLayer[sym] >= predToLayer[head] {
				t.Fatalf("%v reads %v in its body but is in layer %d, not after %d", head, sym, predToLayer[head], predToLayer[sym])
			}
		}
		for td := range topDownClosure(head, topDown) {
			for _, tdRule := range rulesByHead[td] {
				for _, premise := range tdRule.Premises {
					sym, ok := premiseSymbol(premise)
					if !ok {
						continue
					}
					s, idb := predToStratum[sym]
					if !idb || s == predToStratum[head] {
						continue
					}
					wasBefore := s < predToStratum[head]
					if predToLayer[sym] == predToLayer[head] || (predToLayer[sym] < predToLayer[head]) != wasBefore {
						t.Fatalf("%v reads %v top-down through %v: original order before=%v, layers %d and %d",
							head, sym, td, wasBefore, predToLayer[sym], predToLayer[head])
					}
				}
			}
		}
	}
}

// Layering merges independent components, keeps every dependency and every
// top-down read in order, and derives exactly what the original strata do.
// analysis.Stratify orders independent components by map iteration, so it is
// run repeatedly: each original order must be preserved, whichever it is.
func TestLayerStrataPreservesEvaluation(t *testing.T) {
	info := analyzeForLayers(t, layerProgram)
	for run := 0; run < 20; run++ {
		strata, predToStratum, err := analysis.Stratify(analysis.Program{
			EdbPredicates: info.EdbPredicates, IdbPredicates: info.IdbPredicates, Rules: info.Rules,
		})
		if err != nil {
			t.Fatal(err)
		}
		layers, predToLayer := LayerStrata(info, strata, predToStratum)
		checkLayering(t, info, strata, predToStratum, layers, predToLayer)
		if len(layers) >= len(strata) {
			t.Fatalf("run %d: %d strata became %d layers; independent components were not merged", run, len(strata), len(layers))
		}
		if got, want := evalDump(t, info, layers, predToLayer), evalDump(t, info, strata, predToStratum); got != want {
			t.Fatalf("run %d: layered evaluation diverged\n--- layers\n%s\n--- strata\n%s", run, got, want)
		}
	}
}

// The independent islands share a layer; a negation, an aggregate and its
// consumer each force a later one.
func TestLayerStrataShape(t *testing.T) {
	info := analyzeForLayers(t, layerProgram)
	layers, predToLayer, err := Stratify(info)
	if err != nil {
		t.Fatal(err)
	}
	layerOf := func(name string) int {
		for sym, l := range predToLayer {
			if sym.Symbol == name {
				return l
			}
		}
		t.Fatalf("%s is in no layer", name)
		return -1
	}
	if layerOf("island_a") != layerOf("island_c") || layerOf("island_a") != layerOf("reach") {
		t.Fatalf("independent components in different layers: island_a %d, island_c %d, reach %d",
			layerOf("island_a"), layerOf("island_c"), layerOf("reach"))
	}
	if !(layerOf("open") < layerOf("open_count") && layerOf("open_count") < layerOf("busy")) {
		t.Fatalf("aggregate chain out of order: open %d, open_count %d, busy %d", layerOf("open"), layerOf("open_count"), layerOf("busy"))
	}
	if layerOf("best") >= layerOf("pick") {
		t.Fatalf("a lattice's consumer shares or precedes its layer: best %d, pick %d", layerOf("best"), layerOf("pick"))
	}
	if len(layers) == 0 {
		t.Fatal("no layers")
	}
}
