package prompt_evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/atomjudge"
	"codenerd/internal/prompt"
	"codenerd/internal/typesafe"
)

// genericJev reads an atom whose text holds "GENERIC" as known by default
// and covers nothing.
type genericJev struct{}

func (genericJev) Ask(_ context.Context, state any, questions map[string]typesafe.Question) (*typesafe.Response, error) {
	text := state.(map[string]any)["guidance"].(map[string]any)["text"].(string)
	answers := map[string]json.RawMessage{}
	for name := range questions {
		if strings.HasPrefix(name, "covers_") {
			answers[name] = json.RawMessage(`{"type":"noul","noul":0.05}`)
			continue
		}
		score := 0.3
		if name == atomjudge.QuestionKnown && strings.Contains(text, "GENERIC") {
			score = 3.9
		}
		answers[name] = json.RawMessage(fmt.Sprintf(`{"type":"score","score":%g,"confidence":0.9,"legend":{"0":"a"},"probabilities":{"0":1}}`, score))
	}
	raw, _ := json.Marshal(map[string]any{"model": "jev-test", "answers": answers, "usage": map[string]int{"input_tokens": 1, "output_tokens": 0}})
	var r typesafe.Response
	return &r, json.Unmarshal(raw, &r)
}

func generated(id, content string) *GeneratedAtom {
	return &GeneratedAtom{Atom: &prompt.PromptAtom{ID: id, Content: content, ShardTypes: []string{"/coder"}}, ShardType: "/coder"}
}

// A generated atom the admission check refuses is not kept and is counted;
// an oversize one is refused without asking Jev; the shipped corpus is loaded
// once, lazily.
func TestAdmitGenerated_RefusesKnownByDefaultAndOversize(t *testing.T) {
	pe := &PromptEvolver{evolvedAtoms: map[string]*GeneratedAtom{}}
	loads := 0
	pe.SetAtomAdmission(atomjudge.Admission{Asker: genericJev{}, MaxChars: 200, Yes: 0.7, Compare: 3},
		func() ([]*prompt.PromptAtom, error) {
			loads++
			return []*prompt.PromptAtom{{ID: "shipped/x", Content: "Wrap errors with %w."}}, nil
		})
	kept, refused := pe.admitGenerated(context.Background(), []*GeneratedAtom{
		generated("evolved/a", "GENERIC: write clear names."),
		generated("evolved/b", "Pass the tool's revision token back on every edit of the same element."),
		generated("evolved/c", strings.Repeat("x", 201)),
	})
	if refused != 2 || len(kept) != 1 || kept[0].Atom.ID != "evolved/b" {
		t.Fatalf("kept %d (first %v), refused %d", len(kept), kept, refused)
	}
	pe.admitGenerated(context.Background(), []*GeneratedAtom{generated("evolved/d", "Pin the model.")})
	if loads != 1 {
		t.Fatalf("shipped corpus loaded %d times, want once", loads)
	}
}

// Without an installed check every atom is kept: the log says it is not
// configured. Boot always installs one.
func TestAdmitGenerated_NoCheckKeepsEverything(t *testing.T) {
	pe := &PromptEvolver{evolvedAtoms: map[string]*GeneratedAtom{}}
	kept, refused := pe.admitGenerated(context.Background(), []*GeneratedAtom{generated("evolved/a", "GENERIC")})
	if len(kept) != 1 || refused != 0 {
		t.Fatalf("kept %d refused %d", len(kept), refused)
	}
}
