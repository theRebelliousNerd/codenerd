package defaults

import "testing"

// goProduces decides whether Go naming a predicate is evidence that anything
// fills it. The interesting case is an external-predicate registration, which
// reaches the engine only when the corpus Decl carries external():
// kernel_eval.go builds the callback map and then keeps only the entries whose
// decl.IsExternal() is true, discarding the rest silently.
//
// The case that forced this: a dogfood run registered four handlers
// (target_is_large and friends) against plain Decls. The handlers could never
// run, the rules they feed still derived nothing, and the run's own verdict was
// /unverified -- but TestStarvedPredicateBudget reported all four as "no longer
// starved", because their names appear in Go strings at the registration and
// inside the handler bodies. A gate that certifies inert code is worse than no
// gate, so the three branches below are pinned.
func TestGoProduces(t *testing.T) {
	set := func(names ...string) map[string]struct{} {
		m := make(map[string]struct{}, len(names))
		for _, n := range names {
			m[n] = struct{}{}
		}
		return m
	}

	cases := []struct {
		name       string
		pred       string
		registered map[string]struct{}
		asserts    map[string]struct{}
		externals  map[string]struct{}
		want       bool
		why        string
	}{
		{
			name:       "unregistered predicate keeps the over-approximation",
			pred:       "modified_function",
			registered: set(),
			asserts:    set(),
			externals:  set(),
			want:       true,
			why:        "nothing registers it, so any mention still counts as possibly produced",
		},
		{
			name:       "registration behind an external Decl reaches the engine",
			pred:       "query_learned",
			registered: set("query_learned"),
			asserts:    set(),
			externals:  set("query_learned"),
			want:       true,
			why:        "decl.IsExternal() is true, so kernel_eval keeps the callback",
		},
		{
			name:       "registration against a plain Decl produces nothing",
			pred:       "target_is_large",
			registered: set("target_is_large"),
			asserts:    set(),
			externals:  set(),
			want:       false,
			why:        "the callback is filtered out; the handler never runs",
		},
		{
			name:       "an inert registration is rescued by a real assert",
			pred:       "target_is_large",
			registered: set("target_is_large"),
			asserts:    set("target_is_large"),
			externals:  set(),
			want:       true,
			why:        "the external route is dead but Go still writes the fact directly",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := goProduces(tc.pred, tc.registered, tc.asserts, tc.externals)
			if got != tc.want {
				t.Errorf("goProduces(%q) = %v, want %v — %s", tc.pred, got, tc.want, tc.why)
			}
		})
	}
}

// The gate's judgement must match the engine's, so the corpus itself has to
// agree that an external() Decl is what BuildExternalPredicates registrations
// are matched against. If nothing in the corpus is declared external any more,
// the exemption branch above is dead and this gate has quietly become a rule
// that every registration is inert.
func TestExternalDeclsExistInCorpus(t *testing.T) {
	root := repoRootFrom(t, mustGetwd(t))
	_, _, _, externalDecl := mangleCorpus(t, root+"/internal/core/defaults")
	_, registered, _ := goStringLiterals(t, root)

	if len(registered) == 0 {
		t.Fatal("no mkPred registrations found — the registration regex broke and every external would read as inert")
	}
	if len(externalDecl) == 0 {
		t.Fatal("no external() Decl found in the corpus — either the descriptor changed spelling or the external mechanism is gone")
	}

	var matched int
	for pred := range registered {
		if _, ok := externalDecl[pred]; ok {
			matched++
		}
	}
	if matched == 0 {
		t.Errorf("none of the %d registered predicates has an external() Decl; "+
			"every external handler in this build is unreachable", len(registered))
	}
}
