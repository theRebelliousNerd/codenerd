package perception

import (
	"os"
	"path/filepath"
	"testing"
)

// A learned_taxonomy.mg overlay whose rules fail stratification is rejected and
// the taxonomy engine is kept. The rejection must leave the engine exactly as it
// was: classification with the bad overlay equals classification without it.
func TestTaxonomyWorkspace_RejectedOverlayLeavesClassificationUnchanged(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".nerd", "mangle"), 0o755); err != nil {
		t.Fatal(err)
	}
	overlay := "Decl overlay_start(S) bound [/name].\n" +
		"Decl overlay_paradox(S) bound [/name].\n" +
		"overlay_paradox(X) :- overlay_start(X), !overlay_paradox(X).\n"
	if err := os.WriteFile(filepath.Join(root, ".nerd", "mangle", "learned_taxonomy.mg"), []byte(overlay), 0o644); err != nil {
		t.Fatal(err)
	}

	control, err := NewTaxonomyEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer control.StopWorker()
	damaged, err := NewTaxonomyEngine()
	if err != nil {
		t.Fatal(err)
	}
	defer damaged.StopWorker()
	damaged.SetWorkspace(root)
	if damaged.learnedPath != "" {
		t.Fatalf("a rejected overlay must not be recorded as loaded: %q", damaged.learnedPath)
	}

	candidates, err := control.GetVerbs()
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []string{"fix this bug", "run tests", "explain how the kernel boots", "please review my changes"} {
		wantVerb, wantConf, wantErr := control.ClassifyInput(input, candidates)
		gotVerb, gotConf, gotErr := damaged.ClassifyInput(input, candidates)
		if (wantErr == nil) != (gotErr == nil) {
			t.Errorf("%q: control err %v, rejected-overlay err %v", input, wantErr, gotErr)
			continue
		}
		if gotVerb != wantVerb || gotConf != wantConf {
			t.Errorf("%q: rejected overlay classified %s (%.2f), control %s (%.2f)", input, gotVerb, gotConf, wantVerb, wantConf)
		}
	}
}
