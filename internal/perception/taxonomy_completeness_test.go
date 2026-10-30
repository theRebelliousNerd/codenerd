package perception

import (
	"fmt"
	"testing"

	"codenerd/internal/store"
)

// EnsureDefaults took a stored verb_def as proof the verb's whole seed group
// was stored. A group cut off after its definition (a failed synonym write,
// a stopped process) stayed incomplete on every later boot, and a synonym
// or pattern added to an already-stored verb never reached the store (audit
// taxonomy-initialization-completeness). It now reconciles tuple by tuple.
func TestEnsureDefaults_CompletesAPartialGroupTupleByTuple(t *testing.T) {
	localDB, err := store.NewLocalStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer localDB.Close()
	ts := NewTaxonomyStore(localDB)

	var entry TaxonomyDef
	for _, e := range DefaultTaxonomyData {
		if len(e.Synonyms) > 1 && len(e.Patterns) > 0 {
			entry = e
			break
		}
	}
	if entry.Verb == "" {
		t.Fatal("no default verb with synonyms and patterns")
	}
	// The definition and one synonym committed; the rest never did.
	if err := ts.StoreVerbDef(entry.Verb, entry.Category, entry.ShardType, entry.Priority); err != nil {
		t.Fatal(err)
	}
	if err := ts.StoreVerbSynonym(entry.Verb, entry.Synonyms[0]); err != nil {
		t.Fatal(err)
	}

	eng, err := NewTaxonomyEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer eng.StopWorker()
	eng.SetStore(ts)
	if err := eng.EnsureDefaults(); err != nil {
		t.Fatal(err)
	}

	facts, err := ts.LoadAllTaxonomyFacts()
	if err != nil {
		t.Fatal(err)
	}
	have := map[string]bool{}
	for _, f := range facts {
		if len(f.Args) >= 2 {
			have[fmt.Sprint(f.Predicate, "|", f.Args[0], "|", f.Args[1])] = true
		}
	}
	for _, syn := range entry.Synonyms {
		if !have["verb_synonym|"+entry.Verb+"|"+syn] {
			t.Errorf("%s synonym %q is still missing after EnsureDefaults", entry.Verb, syn)
		}
	}
	for _, pat := range entry.Patterns {
		if !have["verb_pattern|"+entry.Verb+"|"+pat] {
			t.Errorf("%s pattern %q is still missing after EnsureDefaults", entry.Verb, pat)
		}
	}

	// A second pass writes nothing new.
	before := len(facts)
	if err := eng.EnsureDefaults(); err != nil {
		t.Fatal(err)
	}
	after, _ := ts.LoadAllTaxonomyFacts()
	if len(after) != before {
		t.Fatalf("a repeated EnsureDefaults went from %d to %d facts", before, len(after))
	}
}
