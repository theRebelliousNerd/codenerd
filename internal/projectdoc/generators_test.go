package projectdoc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const generatorDoc = `---
schema: nerd/v1
generators:
  - id: corpus
    run: go run ./cmd/build_corpus
    outputs: [out/corpus.db]
    when: after editing a source
---
`

func TestParse_GeneratorsAreDeclaredWithTheirOutputs(t *testing.T) {
	doc, err := Parse([]byte(generatorDoc))
	if err != nil {
		t.Fatal(err)
	}
	g, ok := doc.Spec.Generator("corpus")
	if !ok || g.Run != "go run ./cmd/build_corpus" || len(g.Outputs) != 1 || g.Outputs[0] != "out/corpus.db" {
		t.Fatalf("generator = %+v, %v", g, ok)
	}
	if _, ok := doc.Spec.Generator("absent"); ok {
		t.Fatal("an undeclared generator was found")
	}
}

// A generator's writes must be named, workspace-relative, and inside the
// workspace: they are what the regenerate tool records and can undo.
func TestParse_GeneratorsAreValidated(t *testing.T) {
	for name, edit := range map[string][2]string{
		"no id":           {"  - id: corpus\n", "  - id: \"\"\n"},
		"no run":          {"    run: go run ./cmd/build_corpus\n", "    run: \"\"\n"},
		"no outputs":      {"    outputs: [out/corpus.db]\n", ""},
		"absolute output": {"[out/corpus.db]", "[/etc/passwd]"},
		"escaping output": {"[out/corpus.db]", "[../elsewhere/x]"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Parse([]byte(strings.Replace(generatorDoc, edit[0], edit[1], 1))); err == nil {
				t.Fatal("an invalid generator was accepted")
			}
		})
	}
	dup := strings.Replace(generatorDoc, "    when: after editing a source\n", "    when: after editing a source\n  - id: corpus\n    run: x\n    outputs: [y]\n", 1)
	if _, err := Parse([]byte(dup)); err == nil {
		t.Fatal("a duplicate generator id was accepted")
	}
}

// This repository's own nerd.md declares the generators its freshness checks
// name, and every declared output exists.
func TestRepositoryNerdMD_DeclaresItsGenerators(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Load(root)
	if err != nil || doc == nil {
		t.Fatalf("load nerd.md: %v", err)
	}
	for _, id := range []string{"atom-importance", "prompt-corpus", "predicate-corpus"} {
		g, ok := doc.Spec.Generator(id)
		if !ok {
			t.Fatalf("nerd.md declares no generator %q", id)
		}
		for _, out := range g.Outputs {
			if _, err := os.Stat(filepath.Join(root, out)); err != nil {
				t.Errorf("generator %q output %s: %v", id, out, err)
			}
		}
	}
}
