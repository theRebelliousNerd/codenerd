package init

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/tools/research"
)

// groundingScriptedLLM is the package's scripted LLM with Gemini grounding
// control, so the grounded path of generateStrategicKnowledge runs without
// network access. The research package's groundingClient is the same shape
// but unexported, so it cannot be reused here.
type groundingScriptedLLM struct {
	*scriptedLLM
	urls []string
}

func (g *groundingScriptedLLM) GetLastGroundingSources() []string { return nil }
func (g *groundingScriptedLLM) IsGoogleSearchEnabled() bool       { return false }
func (g *groundingScriptedLLM) IsURLContextEnabled() bool         { return len(g.urls) > 0 }
func (g *groundingScriptedLLM) SetEnableGoogleSearch(bool)        {}
func (g *groundingScriptedLLM) SetEnableURLContext(bool)          {}
func (g *groundingScriptedLLM) SetURLContextURLs(u []string)      { g.urls = u }

func TestRecordWithheldDocURLs_WhenURLsWithheld_AppendsLimitationNamingThem(t *testing.T) {
	knowledge := &StrategicKnowledge{Limitations: []string{"heuristic detection"}}
	withheld := []string{"https://docs.example.com/a", "https://docs.example.com/b"}

	recordWithheldDocURLs(knowledge, withheld)

	if len(knowledge.Limitations) != 2 {
		t.Fatalf("Limitations = %v, want the parsed entry plus the withhold", knowledge.Limitations)
	}
	if knowledge.Limitations[0] != "heuristic detection" {
		t.Fatalf("Limitations[0] = %q, the parsed entry must survive", knowledge.Limitations[0])
	}
	got := knowledge.Limitations[1]
	if !strings.Contains(got, "2 documentation URLs") {
		t.Fatalf("limitation must count the withhold, got %q", got)
	}
	for _, u := range withheld {
		if !strings.Contains(got, u) {
			t.Fatalf("limitation must name %s, got %q", u, got)
		}
	}
}

func TestRecordWithheldDocURLs_WhenNothingWithheld_LeavesKnowledgeAlone(t *testing.T) {
	knowledge := &StrategicKnowledge{Limitations: []string{"heuristic detection"}}
	recordWithheldDocURLs(knowledge, nil)
	recordWithheldDocURLs(knowledge, []string{})
	if len(knowledge.Limitations) != 1 || knowledge.Limitations[0] != "heuristic detection" {
		t.Fatalf("Limitations = %v, want the parsed entry untouched", knowledge.Limitations)
	}
	empty := &StrategicKnowledge{}
	recordWithheldDocURLs(empty, nil)
	if len(empty.Limitations) != 0 {
		t.Fatalf("Limitations = %v, want no spurious entry", empty.Limitations)
	}
	recordWithheldDocURLs(nil, []string{"https://docs.example.com/a"})
}

// strategicDocURLs caps at 20 upstream (research.GetDocURLsForTechs), so a
// withhold is unreachable through generateStrategicKnowledge today and the
// recording above is pinned directly. This test drives the full grounded
// path with a fitting URL set and pins that nothing spurious is recorded.
func TestGenerateStrategicKnowledge_WhenGroundingWithholdsNothing_AddsNoLimitation(t *testing.T) {
	workspace := strategicTestWorkspace(t)
	llm := &groundingScriptedLLM{scriptedLLM: &scriptedLLM{relevance: "[]", strategic: strategicJSONBody}}
	ini := &Initializer{config: InitConfig{Workspace: workspace, LLMClient: llm}}
	ini.grounding = research.NewGroundingHelper(llm)
	if !ini.grounding.IsGroundingAvailable() {
		t.Fatal("the grounding fake must take the grounded path")
	}

	knowledge, err := ini.generateStrategicKnowledge(context.Background(), ProjectProfile{Name: "demo", Language: "go"}, nil)
	if err != nil {
		t.Fatalf("generateStrategicKnowledge: %v", err)
	}
	if len(llm.urls) == 0 {
		t.Fatal("the grounded path never armed URL context on the fake")
	}
	if knowledge.ProjectVision != "Bootstrap a workspace the logic kernel can reason over" {
		t.Fatalf("ProjectVision = %q, want the parsed analysis", knowledge.ProjectVision)
	}
	for _, limitation := range knowledge.Limitations {
		if strings.Contains(limitation, "URL context omitted") {
			t.Fatalf("a fitting URL set must not record a withhold: %v", knowledge.Limitations)
		}
	}
}
