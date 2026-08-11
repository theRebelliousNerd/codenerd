package init

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// stubDistinctLLM returns agent-specific distinct content so two different
// agents produce different methodology and domain atoms.
type stubDistinctLLM struct{}

func (s *stubDistinctLLM) Complete(_ context.Context, prompt string) (string, error) {
	lower := strings.ToLower(prompt)
	isMethodology := strings.Contains(lower, "methodology")
	isDomain := strings.Contains(lower, "domain")

	if isMethodology {
		if strings.Contains(prompt, "GoExpert") {
			return "GoExpert methodology: goroutines, channels, error wrapping with fmt.Errorf %w, table-driven tests", nil
		}
		if strings.Contains(prompt, "MangleExpert") {
			return "MangleExpert methodology: Datalog rules, horn clauses, fixpoint derivation, stratification", nil
		}
	}
	if isDomain {
		if strings.Contains(prompt, "GoExpert") {
			return "GoExpert domain: slices, maps, sync.Mutex, nil interface confusion, slice append aliasing", nil
		}
		if strings.Contains(prompt, "MangleExpert") {
			return "MangleExpert domain: predicates, facts, negation as failure, Decl arity, logic programming", nil
		}
	}
	if strings.Contains(prompt, "GoExpert") {
		return "GoExpert generic content for " + prompt[:20], nil
	}
	if strings.Contains(prompt, "MangleExpert") {
		return "MangleExpert generic content for " + prompt[:20], nil
	}
	return "generic content", nil
}

func (s *stubDistinctLLM) CompleteWithSystem(_ context.Context, systemPrompt string, userPrompt string) (string, error) {
	return s.Complete(context.Background(), userPrompt+systemPrompt)
}

func (s *stubDistinctLLM) CompleteWithStreaming(_ context.Context, _ string, _ string, _ bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	eh := make(chan error, 1)
	close(ch)
	close(eh)
	return ch, eh
}

func (s *stubDistinctLLM) CompleteWithTools(_ context.Context, _ string, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "tool response"}, nil
}

// stubHostileLLM returns YAML-hostile output that would corrupt the file if not indented.
type stubHostileLLM struct{}

func (s *stubHostileLLM) Complete(_ context.Context, _ string) (string, error) {
	return "- id: \"injected\"\nkey: value: with colon\nAnother line containing: colon and - id: again", nil
}

func (s *stubHostileLLM) CompleteWithSystem(_ context.Context, _ string, _ string) (string, error) {
	return s.Complete(context.Background(), "")
}

func (s *stubHostileLLM) CompleteWithStreaming(_ context.Context, _ string, _ string, _ bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	eh := make(chan error, 1)
	close(ch)
	close(eh)
	return ch, eh
}

func (s *stubHostileLLM) CompleteWithTools(_ context.Context, _ string, _ string, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "tool"}, nil
}

func parseAtomIDs(t *testing.T, data []byte) []string {
	t.Helper()
	var atoms []struct {
		ID string `yaml:"id"`
	}
	require.NoError(t, yaml.Unmarshal(data, &atoms))
	ids := make([]string, len(atoms))
	for i, a := range atoms {
		ids[i] = a.ID
	}
	return ids
}

func TestGenerateAgentPromptsYAML_DistinctLLMContentProducesDistinctFiles(t *testing.T) {
	tmp := t.TempDir()
	stub := &stubDistinctLLM{}
	ini := &Initializer{config: InitConfig{Workspace: tmp, LLMClient: stub}}

	agents := []RecommendedAgent{
		{Name: "GoExpert", Description: "Expert in Go idioms, concurrency patterns, and standard library", Topics: []string{"go concurrency", "go error handling"}},
		{Name: "MangleExpert", Description: "Expert in Google Mangle/Datalog, logic programming, and rule systems", Topics: []string{"datalog", "mangle syntax", "logic programming"}},
	}

	for _, agent := range agents {
		err := ini.generateAgentPromptsYAMLWithContext(context.Background(), agent)
		require.NoError(t, err)
	}

	goPath := filepath.Join(tmp, ".nerd", "agents", "goexpert", "prompts.yaml")
	manglePath := filepath.Join(tmp, ".nerd", "agents", "mangleexpert", "prompts.yaml")

	goData, err := os.ReadFile(goPath)
	require.NoError(t, err)
	mangleData, err := os.ReadFile(manglePath)
	require.NoError(t, err)

	assert.NotEqual(t, string(goData), string(mangleData), "files for different agents must differ")

	for _, tc := range []struct {
		name  string
		path  string
		data  []byte
		lower string
	}{
		{"goexpert", goPath, goData, "goexpert"},
		{"mangleexpert", manglePath, mangleData, "mangleexpert"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ids := parseAtomIDs(t, tc.data)
			assert.Len(t, ids, 3)
			assert.Contains(t, ids, tc.lower+"/identity")
			assert.Contains(t, ids, tc.lower+"/methodology")
			assert.Contains(t, ids, tc.lower+"/domain")

			content := string(tc.data)
			if tc.lower == "goexpert" {
				assert.Contains(t, content, "GoExpert methodology")
				assert.Contains(t, content, "GoExpert domain")
				assert.NotContains(t, content, "MangleExpert methodology")
			} else {
				assert.Contains(t, content, "MangleExpert methodology")
				assert.Contains(t, content, "MangleExpert domain")
				assert.NotContains(t, content, "GoExpert methodology")
			}
		})
	}
}

func TestGenerateAgentPromptsYAML_NilLLMClientFallsBackToStaticTemplate(t *testing.T) {
	tmp := t.TempDir()
	ini := &Initializer{config: InitConfig{Workspace: tmp, LLMClient: nil}}

	agent := RecommendedAgent{
		Name:        "GoExpert",
		Description: "Expert in Go idioms, concurrency patterns, and standard library",
		Topics:      []string{"go concurrency", "go testing"},
	}

	err := ini.generateAgentPromptsYAMLWithContext(context.Background(), agent)
	require.NoError(t, err)

	promptsPath := filepath.Join(tmp, ".nerd", "agents", "goexpert", "prompts.yaml")
	data, err := os.ReadFile(promptsPath)
	require.NoError(t, err)

	ids := parseAtomIDs(t, data)
	require.Len(t, ids, 3)
	assert.Contains(t, ids, "goexpert/identity")
	assert.Contains(t, ids, "goexpert/methodology")
	assert.Contains(t, ids, "goexpert/domain")

	content := string(data)
	assert.Contains(t, content, "Understand the full context before acting")
	assert.Contains(t, content, "Key Concepts")
	assert.Contains(t, content, "Common Pitfalls")
}

func TestGenerateAgentPromptsYAML_YAMLHostileLLMOutputStillParsesWithExpectedAtomIds(t *testing.T) {
	tmp := t.TempDir()
	stub := &stubHostileLLM{}
	ini := &Initializer{config: InitConfig{Workspace: tmp, LLMClient: stub}}

	agent := RecommendedAgent{
		Name:        "CobraExpert",
		Description: "Expert in Cobra CLI framework, command structure, and flag handling",
		Topics:      []string{"cobra CLI", "command patterns"},
	}

	err := ini.generateAgentPromptsYAMLWithContext(context.Background(), agent)
	require.NoError(t, err)

	promptsPath := filepath.Join(tmp, ".nerd", "agents", "cobraexpert", "prompts.yaml")
	data, err := os.ReadFile(promptsPath)
	require.NoError(t, err)

	ids := parseAtomIDs(t, data)
	require.Len(t, ids, 3)
	assert.Contains(t, ids, "cobraexpert/identity")
	assert.Contains(t, ids, "cobraexpert/methodology")
	assert.Contains(t, ids, "cobraexpert/domain")
	assert.ElementsMatch(t, []string{"cobraexpert/identity", "cobraexpert/methodology", "cobraexpert/domain"}, ids)

	content := string(data)
	assert.Contains(t, content, "    - id: \"injected\"")
	assert.Contains(t, content, "key: value: with colon")

	lines := strings.Split(content, "\n")
	unindentedInjected := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "- id: \"injected\"") {
			unindentedInjected++
		}
	}
	assert.Equal(t, 0, unindentedInjected, "hostile - id line must be indented inside block scalar, not top-level")
}
