package northstar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestAlignmentAtoms_ShouldMatchTheCorpusYAML is what makes "the guardian
// prompt is atomized" a fact rather than a claim.
//
// internal/northstar cannot import internal/prompt (see alignment_prompt.go),
// so the guardian keeps a resolved copy of the atom bodies. This test parses
// the corpus YAML that ships in the binary and fails on any divergence, in
// either direction: a corpus edit that never reaches the guardian, or a
// guardian edit that never reaches the corpus.
func TestAlignmentAtoms_ShouldMatchTheCorpusYAML(t *testing.T) {
	path := filepath.Join(findRepoRoot(t), "internal", "prompt", "atoms", "northstar", "guardian_alignment.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read guardian alignment atoms: %v", err)
	}

	var defs []struct {
		ID      string `yaml:"id"`
		Content string `yaml:"content"`
	}
	if err := yaml.Unmarshal(data, &defs); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}

	corpus := make(map[string]string, len(defs))
	for _, d := range defs {
		corpus[d.ID] = strings.TrimSpace(d.Content)
	}

	for _, id := range AlignmentAtomIDs() {
		want, ok := corpus[id]
		if !ok {
			t.Errorf("atom %q is used by the Guardian but absent from %s", id, filepath.Base(path))
			continue
		}
		if got := AlignmentAtom(id); got != want {
			t.Errorf("atom %q has drifted from the corpus.\nguardian:\n%s\n\ncorpus:\n%s", id, got, want)
		}
	}

	used := map[string]bool{}
	for _, id := range AlignmentAtomIDs() {
		used[id] = true
	}
	for id := range corpus {
		if !used[id] {
			t.Errorf("corpus atom %q is in the guardian_alignment file but no longer composed into any prompt", id)
		}
	}
}

func TestAlignmentAtom_WhenResolverInstalled_ShouldPreferHostContent(t *testing.T) {
	t.Cleanup(func() { SetAlignmentAtomResolver(nil) })
	SetAlignmentAtomResolver(func(id string) (string, bool) {
		if id == atomGuardianUserInstruction {
			return "  evolved instruction  ", true
		}
		return "", false
	})

	if got := AlignmentAtom(atomGuardianUserInstruction); got != "evolved instruction" {
		t.Errorf("resolver content ignored: got %q", got)
	}
	if got := AlignmentAtom(atomGuardianRole); !strings.HasPrefix(got, "You are the Northstar Alignment Guardian") {
		t.Errorf("unresolved atom did not fall back to the built-in copy: %q", got)
	}
}
