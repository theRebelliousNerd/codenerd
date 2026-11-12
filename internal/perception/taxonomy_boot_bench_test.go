package perception

import "testing"

// NewTaxonomyEngine runs at package init, so it is paid by every nerd process.
// It must analyze the taxonomy program once and run it to fixpoint once for
// the default verb data. Asserting entry by entry under AutoEval ran one
// fixpoint per synonym and pattern (about 1.3 s at every process start).
func TestNewTaxonomyEngine_BuildsAndEvaluatesOnce(t *testing.T) {
	t.Chdir(t.TempDir()) // no relative .nerd/mangle/learned_taxonomy.mg to load
	tax, err := NewTaxonomyEngine()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(tax.worker.Stop)

	work := tax.engine.WorkCounts()
	if work.ProgramBuilds != 1 {
		t.Errorf("ProgramBuilds = %d, want 1 for the static taxonomy schemas", work.ProgramBuilds)
	}
	if work.Evaluations != 1 {
		t.Errorf("Evaluations = %d, want 1 for the default taxonomy data", work.Evaluations)
	}

	verbs, err := tax.GetVerbs()
	if err != nil {
		t.Fatal(err)
	}
	if len(verbs) != len(DefaultTaxonomyData) {
		t.Fatalf("GetVerbs returned %d verbs, want %d", len(verbs), len(DefaultTaxonomyData))
	}
	want := 0
	got := 0
	for _, entry := range DefaultTaxonomyData {
		want += len(entry.Synonyms)
	}
	for _, v := range verbs {
		got += len(v.Synonyms)
	}
	if got != want {
		t.Fatalf("synonyms = %d, want %d: batching must keep every default fact", got, want)
	}
}

// BenchmarkNewTaxonomyEngine measures building the shared verb taxonomy
// engine: the static schemas plus the default verb data.
func BenchmarkNewTaxonomyEngine(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		t, err := NewTaxonomyEngine()
		if err != nil {
			b.Fatal(err)
		}
		t.worker.Stop()
	}
}
