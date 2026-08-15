package init

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/types"
)

// scriptedLLM answers by inspecting the prompt, so a single client can serve
// both the document-relevance pass and the strategic analysis pass without any
// network access.
type scriptedLLM struct {
	relevance string
	strategic string

	mu      sync.Mutex
	prompts []string
}

func (s *scriptedLLM) Complete(_ context.Context, prompt string) (string, error) {
	s.mu.Lock()
	s.prompts = append(s.prompts, prompt)
	s.mu.Unlock()
	// The relevance pass is the only one that asks for per-document verdicts.
	if strings.Contains(prompt, "index") && strings.Contains(prompt, "relevant") {
		return s.relevance, nil
	}
	return s.strategic, nil
}

func (s *scriptedLLM) CompleteWithSystem(ctx context.Context, _, userPrompt string) (string, error) {
	return s.Complete(ctx, userPrompt)
}

func (s *scriptedLLM) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{}, nil
}

func (s *scriptedLLM) CompleteWithStreaming(ctx context.Context, _, userPrompt string, _ bool) (<-chan string, <-chan error) {
	content := make(chan string, 1)
	errs := make(chan error, 1)
	go func() {
		defer close(content)
		defer close(errs)
		resp, err := s.Complete(ctx, userPrompt)
		if err != nil {
			errs <- err
			return
		}
		content <- resp
	}()
	return content, errs
}

func strategicTestWorkspace(t *testing.T) string {
	t.Helper()
	workspace := t.TempDir()
	readme := "# Demo\n\nA small service used to exercise strategic knowledge parsing.\n"
	if err := os.WriteFile(filepath.Join(workspace, "README.md"), []byte(readme), 0o644); err != nil {
		t.Fatalf("write README: %v", err)
	}
	return workspace
}

const strategicJSONBody = `{
  "project_vision": "Bootstrap a workspace the logic kernel can reason over",
  "core_philosophy": "Logic determines Reality; the Model merely describes it",
  "design_principles": ["Mangle decides", "Go measures"],
  "architecture_style": "neuro-symbolic",
  "key_components": [
    {"name": "Initializer", "purpose": "cold start", "location": "internal/init", "interfaces": "Initialize", "depends_on": ["core"]}
  ],
  "data_flow_pattern": "scan -> profile -> facts -> knowledge bases",
  "core_patterns": [
    {"name": "Phase runner", "description": "ordered phases with ETA", "used_in": "Initialize", "why": "observability"}
  ],
  "communication_flow": "phases append to InitResult",
  "core_capabilities": ["profile detection"],
  "extension_points": ["Type U agents"],
  "safety_constraints": ["writes stay under .nerd/"],
  "limitations": ["heuristic detection"],
  "learning_mechanisms": ["autopoiesis"],
  "future_directions": ["monorepo profiles"]
}`

func TestGenerateStrategicKnowledge_WhenResponseIsFencedJSON_ShouldParseEveryField(t *testing.T) {
	workspace := strategicTestWorkspace(t)
	llm := &scriptedLLM{
		relevance: "[{\"index\":0,\"relevant\":true,\"reason\":\"vision doc\"}]",
		strategic: "Here is the analysis.\n```json\n" + strategicJSONBody + "\n```\n",
	}
	ini := &Initializer{config: InitConfig{Workspace: workspace, LLMClient: llm}}

	knowledge, err := ini.generateStrategicKnowledge(context.Background(), ProjectProfile{Name: "demo", Language: "go"}, nil)
	if err != nil {
		t.Fatalf("generateStrategicKnowledge: %v", err)
	}
	if knowledge.ProjectVision != "Bootstrap a workspace the logic kernel can reason over" {
		t.Errorf("ProjectVision = %q", knowledge.ProjectVision)
	}
	if knowledge.ArchitectureStyle != "neuro-symbolic" {
		t.Errorf("ArchitectureStyle = %q", knowledge.ArchitectureStyle)
	}
	if len(knowledge.KeyComponents) != 1 || knowledge.KeyComponents[0].Name != "Initializer" {
		t.Errorf("KeyComponents = %+v", knowledge.KeyComponents)
	}
	if len(knowledge.CorePatterns) != 1 || knowledge.CorePatterns[0].Why != "observability" {
		t.Errorf("CorePatterns = %+v", knowledge.CorePatterns)
	}
	if len(knowledge.SafetyConstraints) != 1 {
		t.Errorf("SafetyConstraints = %v", knowledge.SafetyConstraints)
	}
}

func TestGenerateStrategicKnowledge_WhenResponseIsBareJSON_ShouldParse(t *testing.T) {
	workspace := strategicTestWorkspace(t)
	llm := &scriptedLLM{
		relevance: "[]",
		strategic: strategicJSONBody,
	}
	ini := &Initializer{config: InitConfig{Workspace: workspace, LLMClient: llm}}

	knowledge, err := ini.generateStrategicKnowledge(context.Background(), ProjectProfile{Name: "demo", Language: "go"}, nil)
	if err != nil {
		t.Fatalf("generateStrategicKnowledge: %v", err)
	}
	if knowledge.CorePhilosophy != "Logic determines Reality; the Model merely describes it" {
		t.Errorf("CorePhilosophy = %q", knowledge.CorePhilosophy)
	}
}

// A JSON string value containing a closing brace used to terminate the object
// early, so a perfectly good analysis was thrown away for the profile-only
// fallback.
func TestGenerateStrategicKnowledge_WhenStringValueContainsBrace_ShouldStillParse(t *testing.T) {
	workspace := strategicTestWorkspace(t)
	body := `{"project_vision": "emit facts like project_language(/go). and rules { head :- body }", "architecture_style": "neuro-symbolic"}`
	llm := &scriptedLLM{relevance: "[]", strategic: body}
	ini := &Initializer{config: InitConfig{Workspace: workspace, LLMClient: llm}}

	knowledge, err := ini.generateStrategicKnowledge(context.Background(), ProjectProfile{Name: "demo", Language: "go", Architecture: "layered"}, nil)
	if err != nil {
		t.Fatalf("generateStrategicKnowledge: %v", err)
	}
	if !strings.Contains(knowledge.ProjectVision, "head :- body") {
		t.Errorf("ProjectVision = %q, want the full string with the embedded brace", knowledge.ProjectVision)
	}
	if knowledge.ArchitectureStyle != "neuro-symbolic" {
		t.Errorf("ArchitectureStyle = %q, want the parsed value rather than the profile fallback", knowledge.ArchitectureStyle)
	}
}

func TestGenerateStrategicKnowledge_WhenResponseIsUnparseable_ShouldFallBackToProfile(t *testing.T) {
	workspace := strategicTestWorkspace(t)
	llm := &scriptedLLM{relevance: "[]", strategic: "I could not analyse this repository."}
	ini := &Initializer{config: InitConfig{Workspace: workspace, LLMClient: llm}}

	profile := ProjectProfile{
		Name:         "demo",
		Description:  "a demo project",
		Language:     "go",
		Framework:    "gin",
		Architecture: "layered",
		Patterns:     []string{"repository"},
	}
	knowledge, err := ini.generateStrategicKnowledge(context.Background(), profile, nil)
	if err != nil {
		t.Fatalf("generateStrategicKnowledge must degrade, not fail: %v", err)
	}
	if knowledge.ProjectVision != profile.Description {
		t.Errorf("fallback ProjectVision = %q, want the profile description", knowledge.ProjectVision)
	}
	if knowledge.ArchitectureStyle != "layered" {
		t.Errorf("fallback ArchitectureStyle = %q", knowledge.ArchitectureStyle)
	}
	if len(knowledge.DesignPrinciples) != 1 {
		t.Errorf("fallback DesignPrinciples = %v, want the profile patterns", knowledge.DesignPrinciples)
	}
}

func TestGenerateStrategicKnowledge_WhenNoLLMClient_ShouldReturnError(t *testing.T) {
	ini := &Initializer{config: InitConfig{Workspace: strategicTestWorkspace(t)}}
	if _, err := ini.generateStrategicKnowledge(context.Background(), ProjectProfile{}, nil); err == nil {
		t.Fatal("expected an error when no LLM client is configured")
	}
}

// The relevance pass asks for a JSON array. extractJSON used to return only the
// first object of an unfenced array, which made the unmarshal fail and silently
// dropped every run back to the priority heuristic.
func TestFilterDocumentsByRelevance_WhenResponseIsBareJSONArray_ShouldApplyVerdicts(t *testing.T) {
	llm := &scriptedLLM{
		relevance: `[{"index":0,"relevant":true,"reason":"states the vision"},{"index":1,"relevant":false,"reason":"changelog"}]`,
	}
	ini := &Initializer{config: InitConfig{Workspace: t.TempDir(), LLMClient: llm}}

	docs := []DocumentInfo{
		{Path: "VISION.md", Title: "Vision", Content: "why we exist", Priority: 9},
		{Path: "CHANGELOG.md", Title: "Changelog", Content: "v1.0.1", Priority: 9},
	}
	filtered := ini.filterDocumentsByRelevance(context.Background(), docs)

	if len(filtered) != 1 {
		t.Fatalf("filtered = %d docs, want only the relevant one (LLM verdicts were ignored)", len(filtered))
	}
	if filtered[0].Path != "VISION.md" {
		t.Errorf("filtered[0] = %q, want VISION.md", filtered[0].Path)
	}
	if filtered[0].Reasoning != "states the vision" {
		t.Errorf("Reasoning = %q, want the LLM's reason rather than a fallback label", filtered[0].Reasoning)
	}
}

func TestExtractJSON_WhenVariousShapes_ShouldReturnTheWholeValue(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "fenced object",
			input: "prose\n```json\n{\"a\":1}\n```\ntrailing",
			want:  `{"a":1}`,
		},
		{
			name:  "bare array",
			input: `[{"index":0},{"index":1}]`,
			want:  `[{"index":0},{"index":1}]`,
		},
		{
			name:  "array after prose",
			input: "Here you go: [{\"index\":0}]",
			want:  `[{"index":0}]`,
		},
		{
			name:  "brace inside string value",
			input: `{"rule":"head :- body }","ok":true}`,
			want:  `{"rule":"head :- body }","ok":true}`,
		},
		{
			name:  "escaped quote inside string value",
			input: `{"quote":"he said \"} \" loudly","ok":true}`,
			want:  `{"quote":"he said \"} \" loudly","ok":true}`,
		},
		{
			name:  "nested object",
			input: `noise {"outer":{"inner":[1,2]}} noise`,
			want:  `{"outer":{"inner":[1,2]}}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractJSON(tt.input); got != tt.want {
				t.Errorf("extractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}
