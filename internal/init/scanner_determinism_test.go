package init

import (
	"os"
	"path/filepath"
	"testing"
)

// A polyglot monorepo must classify the same way every time.
//
// detectLanguageFromFiles returns early when a config sits at the workspace
// root, so that path is decided by slice order and is already stable. The
// monorepo path underneath it is the one that counted matches into a map and
// then ranged that map with a strict `>` — so a repo with one Go module and
// one Node package in sibling directories, an ordinary shape rather than an
// edge case, was classified by Go's randomised iteration order. `nerd init`
// could label the same workspace differently on two runs, and the primary
// language decides which build and test commands the agent reaches for
// afterwards.
//
// The alphabetical tie-break this pins is arbitrary. Its being FIXED is not: a
// stable wrong answer can be found and argued with, a varying one cannot.
func TestMonorepoLanguageIsStableOnATie(t *testing.T) {
	root := t.TempDir()
	seed := map[string]string{
		"services/api/go.mod":       "module example.com/api\n",
		"services/web/package.json": "{}\n",
	}
	for rel, body := range seed {
		full := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatalf("seed %s: %v", rel, err)
		}
	}

	i := &Initializer{config: InitConfig{Workspace: root}}
	first := i.detectLanguageFromFiles()
	if first == "" {
		t.Fatal("no language detected from a monorepo holding two config files; " +
			"this test cannot see the tie it exists to check")
	}

	// With two tied keys, a randomised order survives 200 draws unchanged with
	// probability 2^-199, so a pass here means the order is fixed.
	for n := 0; n < 200; n++ {
		if got := i.detectLanguageFromFiles(); got != first {
			t.Fatalf("iteration %d detected %q, first detected %q: the workspace is "+
				"classified by coin flip", n, got, first)
		}
	}
}
