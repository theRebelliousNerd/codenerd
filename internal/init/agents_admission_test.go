package init

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/atomjudge"
	"codenerd/internal/typesafe"
)

// knownWhen reads an atom as known by default (the known Score's top level)
// when its text contains the marker, and as atomic and specific otherwise.
type knownWhen struct{ marker string }

func (k knownWhen) Ask(_ context.Context, state any, questions map[string]typesafe.Question) (*typesafe.Response, error) {
	text := state.(map[string]any)["guidance"].(map[string]any)["text"].(string)
	answers := map[string]json.RawMessage{}
	for name := range questions {
		if strings.HasPrefix(name, "covers_") {
			answers[name] = json.RawMessage(`{"type":"noul","noul":0.05}`)
			continue
		}
		score := 0.3
		if name == atomjudge.QuestionKnown && strings.Contains(text, k.marker) {
			score = 3.9
		}
		answers[name] = json.RawMessage(fmt.Sprintf(`{"type":"score","score":%g,"confidence":0.9,"legend":{"0":"a"},"probabilities":{"0":1}}`, score))
	}
	raw, _ := json.Marshal(map[string]any{"model": "jev-test", "answers": answers, "usage": map[string]int{"input_tokens": 1, "output_tokens": 0}})
	var r typesafe.Response
	return &r, json.Unmarshal(raw, &r)
}

// A refused generated atom is not written; a refused base piece hands its id
// to the next admitted piece of its body, so references keep resolving; the
// template identity is never judged.
func TestAdmitAgentAtoms_RefusedAtomsAreDroppedAndTheBaseIDSurvives(t *testing.T) {
	ini := &Initializer{config: InitConfig{Workspace: t.TempDir()}}
	ini.admissionOnce.Do(func() {
		ini.admission = atomjudge.Admission{Asker: knownWhen{"GENERIC"}, MaxChars: 1500, Yes: 0.7, Compare: 3}
	})
	method := "## Write clean code\nGENERIC: use clear names.\n\n## Pin the RS256 key\nThe auth service signs with RS256; never verify with HS256.\n\n## Retries\nGENERIC: retry transient failures."
	domain := "## Token cache\nThe cache key is the token's kid header."
	atoms := agentPromptAtoms("authx", "AuthX", "auth specialist", "    - jwt", "jwt", method, domain, 1500)
	kept := admitAgentAtoms(context.Background(), ini.atomAdmission(), "authx", atoms)

	var ids []string
	for _, a := range kept {
		ids = append(ids, a.ID)
		if strings.Contains(a.Content, "GENERIC") {
			t.Errorf("known-by-default atom %s was kept", a.ID)
		}
	}
	want := []string{"authx/identity", "authx/methodology", "authx/domain"}
	if strings.Join(ids, ",") != strings.Join(want, ",") {
		t.Fatalf("kept %v, want %v", ids, want)
	}
	if !strings.Contains(kept[1].Content, "RS256") {
		t.Fatalf("the base id did not pass to the admitted piece: %q", kept[1].Content)
	}
	yml := renderPromptsYAML("AuthX", kept)
	if err := validatePromptsYAML([]byte(yml), "authx", 1500); err != nil {
		t.Fatalf("admitted set does not validate: %v\n%s", err, yml)
	}
}

// Without Jev every atom within the bound is admitted, unjudged.
func TestAdmitAgentAtoms_NoJevAdmitsWithinTheBound(t *testing.T) {
	ini := &Initializer{config: InitConfig{Workspace: t.TempDir()}}
	atoms := agentPromptAtoms("authx", "AuthX", "auth", "    - jwt", "jwt", "## A\nGENERIC one.", "## B\nGENERIC two.", 1500)
	if kept := admitAgentAtoms(context.Background(), ini.atomAdmission(), "authx", atoms); len(kept) != len(atoms) {
		t.Fatalf("kept %d of %d without Jev", len(kept), len(atoms))
	}
}

// sectionedLLM writes one generic and one specific section for every body.
type sectionedLLM struct{ stubHostileLLM }

func (*sectionedLLM) CompleteWithSystem(context.Context, string, string) (string, error) {
	return "## Clean code\nGENERIC: use clear names.\n\n## Key rotation\nThe auth service rotates its RS256 key every 24h; cache verifiers by kid.", nil
}

// The production generation path (generateAgentPromptsYAMLWithContext) runs
// the admission check: the known-by-default section never reaches
// prompts.yaml, the specific one does, one atom per heading.
func TestGenerateAgentPromptsYAML_AdmissionRunsOnTheGenerationPath(t *testing.T) {
	tmp := t.TempDir()
	ini := withInitKernel(t, &Initializer{config: InitConfig{Workspace: tmp, LLMClient: &sectionedLLM{}}})
	ini.admissionOnce.Do(func() {
		ini.admission = atomjudge.Admission{Asker: knownWhen{"GENERIC"}, MaxChars: 1500, Yes: 0.7, Compare: 3}
	})
	agent := RecommendedAgent{Name: "AuthExpert", Description: "JWT verification", Topics: []string{"jwt"}}
	if err := ini.generateAgentPromptsYAMLWithContext(context.Background(), agent); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(tmp, ".nerd", "agents", "authexpert", "prompts.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "GENERIC") || !strings.Contains(string(data), "RS256") {
		t.Fatalf("admission did not filter the generated atoms:\n%s", data)
	}
	if ids := parseAtomIDs(t, data); strings.Join(ids, ",") != "authexpert/identity,authexpert/methodology,authexpert/domain" {
		t.Fatalf("ids %v", ids)
	}
}
