package perception

import (
	"testing"

	"codenerd/internal/store"
)

// TestGetVerbs_StableTieOrder pins total corpus ordering: verbs sharing a
// priority (50, 65, 70, ...) sort by verb ascending, identically on every
// call. Corpus order feeds candidate order feeds the score aggregation, so
// an unstable sort is nondeterministic classification.
func TestGetVerbs_StableTieOrder(t *testing.T) {
	if SharedTaxonomy == nil {
		t.Skip("SharedTaxonomy not initialized")
	}
	var first []string
	for i := 0; i < 20; i++ {
		verbs, err := SharedTaxonomy.GetVerbs()
		if err != nil {
			t.Fatalf("GetVerbs: %v", err)
		}
		order := make([]string, len(verbs))
		for j, v := range verbs {
			order[j] = v.Verb
		}
		if i == 0 {
			first = order
			continue
		}
		for j := range first {
			if order[j] != first[j] {
				t.Fatalf("run %d differs at %d: %s vs %s", i, j, order[j], first[j])
			}
		}
	}
	// Spot-check tie groups the corpus actually holds.
	pos := map[string]int{}
	for i, v := range first {
		pos[v] = i
	}
	ties := [][2]string{{"/converse", "/greet"}, {"/forget", "/remember"}}
	for _, tie := range ties {
		a, aok := pos[tie[0]]
		b, bok := pos[tie[1]]
		if !aok || !bok {
			t.Errorf("tie verbs %v missing from corpus", tie)
			continue
		}
		if a > b {
			t.Errorf("tie order: %s (pos %d) after %s (pos %d), want verb-ascending",
				tie[0], a, tie[1], b)
		}
	}
}

// TestEnsureDefaults_MergesMissingVerbs pins the stale-DB upgrade: a
// database holding an older corpus (any subset of verbs) gains exactly the
// missing defaults on EnsureDefaults — and keeps verbs the defaults never
// had (user/learned additions are never removed).
func TestEnsureDefaults_MergesMissingVerbs(t *testing.T) {
	localDB, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer localDB.Close()
	ts := NewTaxonomyStore(localDB)

	// Simulate an older corpus: two verbs only, one of them foreign.
	if err := ts.StoreVerbDef("/review", "/query", "/reviewer", 88); err != nil {
		t.Fatalf("StoreVerbDef: %v", err)
	}
	if err := ts.StoreVerbDef("/custom", "/query", "/none", 10); err != nil {
		t.Fatalf("StoreVerbDef custom: %v", err)
	}

	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Fatalf("NewTaxonomyEngine: %v", err)
	}
	defer eng.StopWorker()
	eng.SetStore(ts)
	if err := eng.EnsureDefaults(); err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}

	facts, err := ts.LoadAllTaxonomyFacts()
	if err != nil {
		t.Fatalf("LoadAllTaxonomyFacts: %v", err)
	}
	defs := map[string]bool{}
	for _, f := range facts {
		if f.Predicate == "verb_def" && len(f.Args) > 0 {
			if verb, ok := f.Args[0].(string); ok {
				defs[verb] = true
			}
		}
	}
	for _, entry := range DefaultTaxonomyData {
		if !defs[entry.Verb] {
			t.Errorf("default verb %s missing after merge", entry.Verb)
		}
	}
	if !defs["/custom"] {
		t.Error("foreign verb /custom removed by merge, want preserved")
	}
	if !defs["/deploy"] || !defs["/converse"] {
		t.Error("new verbs /deploy//converse missing after merge (stale-DB regression)")
	}

	// Second run is a no-op: no duplicates.
	before := len(facts)
	if err := eng.EnsureDefaults(); err != nil {
		t.Fatalf("second EnsureDefaults: %v", err)
	}
	facts, err = ts.LoadAllTaxonomyFacts()
	if err != nil {
		t.Fatalf("LoadAllTaxonomyFacts: %v", err)
	}
	if len(facts) != before {
		t.Errorf("second merge changed fact count %d -> %d, want idempotent", before, len(facts))
	}
}

// TestTaxonomyStore_LearnedExemplarIntConfidence pins the Mangle-compat
// normalization: float confidence persists as int64 0-100 and reloads as a
// whole number the /number Decl accepts.
func TestTaxonomyStore_LearnedExemplarIntConfidence(t *testing.T) {
	localDB, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("NewLocalStore: %v", err)
	}
	defer localDB.Close()
	ts := NewTaxonomyStore(localDB)

	if err := ts.StoreLearnedExemplar("Nuke it", "/delete", "database", "", 0.95); err != nil {
		t.Fatalf("StoreLearnedExemplar: %v", err)
	}
	facts, err := ts.LoadAllTaxonomyFacts()
	if err != nil {
		t.Fatalf("LoadAllTaxonomyFacts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("loaded %d facts, want 1", len(facts))
	}
	if len(facts[0].Args) != 5 {
		t.Fatalf("arity=%d, want 5", len(facts[0].Args))
	}
	conf, ok := facts[0].Args[4].(int64)
	if !ok {
		t.Fatalf("confidence type %T, want int64", facts[0].Args[4])
	}
	if conf != 95 {
		t.Errorf("confidence=%d, want 95", conf)
	}
}
