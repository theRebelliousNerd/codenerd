package northstar

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// alignmentPrompts builds one check's system prompt and user message, failing
// the test when the prompt cannot be compiled.
func alignmentPrompts(t *testing.T, g *Guardian, vision *Vision, subject ...string) (systemPrompt, userPrompt string) {
	t.Helper()
	s := ""
	if len(subject) > 0 {
		s = subject[0]
	}
	workspace := ""
	if g.store != nil {
		workspace = filepath.Dir(filepath.Dir(g.store.Path()))
	}
	scope := ManualAlignmentScope(workspace, s)
	coverage, module := g.resolveAlignmentScope(scope, vision)
	systemPrompt, userPrompt, err := g.buildAlignmentPrompts(context.Background(), vision, s, "", alignmentScopeSection(coverage), module)
	if err != nil {
		t.Fatalf("buildAlignmentPrompts: %v", err)
	}
	return systemPrompt, userPrompt
}

// guardianAtomContent reads the guardian atoms the binary ships.
func guardianAtomContent(t *testing.T) map[string]string {
	t.Helper()
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
	return corpus
}

// The guardian's system prompt is compiled from its atoms and nothing else:
// every scaffolding atom, the module-refinement rule exactly when a module
// governs the subject, and no shard-turn protocol -- the reply is parsed as
// SCORE/RESULT lines, not a Piggyback envelope.
func TestAlignmentSystemPrompt_IsCompiledFromTheGuardianAtoms(t *testing.T) {
	atoms := guardianAtomContent(t)
	for _, tc := range []struct {
		name          string
		refinesModule bool
	}{{"project", false}, {"module", true}} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := compileAlignmentSystemPrompt(context.Background(), tc.refinesModule)
			if err != nil {
				t.Fatalf("compileAlignmentSystemPrompt(%v): %v", tc.refinesModule, err)
			}
			for _, id := range []string{atomGuardianRole, atomGuardianTask, atomGuardianOutputContract, atomGuardianUserInstruction} {
				if !strings.Contains(got, atoms[id]) {
					t.Errorf("the compiled prompt lacks atom %q", id)
				}
			}
			if has := strings.Contains(got, atoms[atomGuardianModuleRefinement]); has != tc.refinesModule {
				t.Errorf("module refinement present = %v, want %v", has, tc.refinesModule)
			}
			for _, leaked := range []string{"OUTPUT PROTOCOL: PIGGYBACK ENVELOPE", "CONTROL-PACKET FIELD ORDER", "You are codeNERD"} {
				if strings.Contains(got, leaked) {
					t.Errorf("the alignment prompt carries %q", leaked)
				}
			}
		})
	}
}

// Every atom in the guardian file reaches some alignment compile.
func TestAlignmentAtoms_EveryCorpusAtomIsCompiled(t *testing.T) {
	module, err := compileAlignmentSystemPrompt(context.Background(), true)
	if err != nil {
		t.Fatalf("compileAlignmentSystemPrompt: %v", err)
	}
	for id, content := range guardianAtomContent(t) {
		if !strings.Contains(module, content) {
			t.Errorf("corpus atom %q is in guardian_alignment.yaml but no alignment compile carries it", id)
		}
	}
}
