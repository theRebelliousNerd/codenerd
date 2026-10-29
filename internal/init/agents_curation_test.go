package init

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// scriptedInteractiveConfig returns an InteractiveConfig that answers the
// prompts from a canned script and writes its output to a temp file, so agent
// curation can be driven without a terminal.
func scriptedInteractiveConfig(t *testing.T, script string) *InteractiveConfig {
	t.Helper()
	out, err := os.CreateTemp(t.TempDir(), "curation-*.out")
	if err != nil {
		t.Fatalf("create output file: %v", err)
	}
	t.Cleanup(func() { _ = out.Close() })
	return &InteractiveConfig{
		Reader: bufio.NewReader(strings.NewReader(script)),
		Writer: out,
	}
}

func agentNames(agents []RecommendedAgent) []string {
	names := make([]string, 0, len(agents))
	for _, agent := range agents {
		names = append(names, agent.Name)
	}
	return names
}

func TestMergeTypeUAgents_WhenDefinitionsProvided_ShouldAppendThemToRecommended(t *testing.T) {
	ini := &Initializer{config: InitConfig{
		Workspace: t.TempDir(),
		TypeUAgents: []TypeUAgentDefinition{
			{Name: "K8sExpert", Role: "Kubernetes specialist", Topics: []string{"helm", "kubectl"}},
		},
	}}

	merged := ini.mergeTypeUAgents([]RecommendedAgent{{Name: "GoExpert", Type: "persistent"}})

	if len(merged) != 2 {
		t.Fatalf("merged agents = %v, want GoExpert plus the Type U agent", agentNames(merged))
	}
	user := merged[1]
	if user.Name != "K8sExpert" {
		t.Fatalf("merged[1] = %q, want K8sExpert", user.Name)
	}
	if user.Type != "user" {
		t.Errorf("Type U agent Type = %q, want \"user\"", user.Type)
	}
	if len(user.Topics) != 2 {
		t.Errorf("Type U agent topics = %v, want both research topics carried through", user.Topics)
	}
	if len(user.Permissions) == 0 {
		t.Error("Type U agent has no permissions; it would spawn unable to read anything")
	}
}

func TestMergeTypeUAgents_WhenNameCollidesWithDetectedAgent_ShouldReplaceIt(t *testing.T) {
	ini := &Initializer{config: InitConfig{
		Workspace: t.TempDir(),
		TypeUAgents: []TypeUAgentDefinition{
			{Name: "GoExpert", Role: "our house Go rules", Topics: []string{"house style"}},
		},
	}}

	// Both agents lowercase to the same .nerd/shards/{name}_knowledge.db path,
	// so keeping both would have them fight over one database.
	merged := ini.mergeTypeUAgents([]RecommendedAgent{
		{Name: "GoExpert", Type: "persistent", Description: "built-in"},
		{Name: "TestArchitect", Type: "persistent"},
	})

	if len(merged) != 2 {
		t.Fatalf("merged agents = %v, want the collision replaced rather than duplicated", agentNames(merged))
	}
	if merged[0].Type != "user" || merged[0].Description != "our house Go rules" {
		t.Errorf("collision kept the built-in agent: %+v", merged[0])
	}
}

func TestMergeTypeUAgents_WhenNoDefinitions_ShouldLeaveRecommendedUnchanged(t *testing.T) {
	ini := &Initializer{config: InitConfig{Workspace: t.TempDir()}}
	in := []RecommendedAgent{{Name: "GoExpert"}}
	if got := ini.mergeTypeUAgents(in); len(got) != 1 || got[0].Name != "GoExpert" {
		t.Fatalf("mergeTypeUAgents changed the set with no definitions: %v", agentNames(got))
	}
}

func TestCurateAgents_WhenUserAcceptsRecommended_ShouldDropOptionalAgents(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ini := &Initializer{config: InitConfig{
		Workspace:     workspace,
		Interactive:   true,
		InteractiveIO: scriptedInteractiveConfig(t, "y\n"),
	}}

	offered := []RecommendedAgent{
		{Name: "SecurityAuditor", Priority: 90},
		{Name: "GoExpert", Priority: 100},
		{Name: "RedisExpert", Priority: 40},
	}
	profile := ProjectProfile{Language: "go"}

	result := &InitResult{}
	curated := ini.curateAgents(context.Background(), offered, profile, result)

	got := map[string]bool{}
	for _, agent := range curated {
		got[agent.Name] = true
	}
	if !got["SecurityAuditor"] || !got["GoExpert"] {
		t.Errorf("curated = %v, want the recommended core and language agents kept", agentNames(curated))
	}
	if got["RedisExpert"] {
		t.Errorf("curated = %v, want the optional agent dropped", agentNames(curated))
	}

	// The choice must be recorded so a later --force run can honor it.
	prefs, err := LoadAgentPreferences(workspace)
	if err != nil {
		t.Fatalf("LoadAgentPreferences: %v", err)
	}
	if prefs == nil {
		t.Fatal("agent selection was not persisted to .nerd/preferences.json")
	}
	if len(prefs.AcceptedAgents) == 0 {
		t.Errorf("accepted agents not recorded: %+v", prefs)
	}
	foundRejected := false
	for _, name := range prefs.RejectedAgents {
		if name == "RedisExpert" {
			foundRejected = true
		}
	}
	if !foundRejected {
		t.Errorf("rejected agents = %v, want RedisExpert recorded", prefs.RejectedAgents)
	}
}

func TestCurateAgents_WhenUserDeclines_ShouldKeepOnlyCoreAgents(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ini := &Initializer{config: InitConfig{
		Workspace:     workspace,
		Interactive:   true,
		InteractiveIO: scriptedInteractiveConfig(t, "n\n"),
	}}

	curated := ini.curateAgents(context.Background(), []RecommendedAgent{
		{Name: "SecurityAuditor"},
		{Name: "TestArchitect"},
		{Name: "GoExpert"},
	}, ProjectProfile{Language: "go"}, &InitResult{})

	for _, agent := range curated {
		if agent.Name == "GoExpert" {
			t.Fatalf("curated = %v, want only the core agents after \"n\"", agentNames(curated))
		}
	}
	if len(curated) != 2 {
		t.Fatalf("curated = %v, want both core agents", agentNames(curated))
	}
}

func TestCurateAgents_WhenNotInteractive_ShouldKeepEveryRecommendedAgent(t *testing.T) {
	ini := &Initializer{config: InitConfig{Workspace: t.TempDir(), Interactive: false}}
	offered := []RecommendedAgent{{Name: "GoExpert"}, {Name: "RedisExpert"}}

	curated := ini.curateAgents(context.Background(), offered, ProjectProfile{Language: "go"}, &InitResult{})
	if len(curated) != 2 {
		t.Fatalf("curated = %v, want the untouched recommended set", agentNames(curated))
	}
}

// A non-interactive environment must never block init on a prompt nobody can
// answer. Under `go test` stdout is a pipe, so the terminal probe is false here
// and curation is skipped even though Interactive is set.
func TestCurateAgents_WhenNoTerminalAvailable_ShouldSkipPromptAndKeepRecommended(t *testing.T) {
	ini := &Initializer{config: InitConfig{Workspace: t.TempDir(), Interactive: true}}
	offered := []RecommendedAgent{{Name: "GoExpert"}, {Name: "RedisExpert"}}

	result := &InitResult{}
	curated := ini.curateAgents(context.Background(), offered, ProjectProfile{Language: "go"}, result)
	if len(curated) != 2 {
		t.Fatalf("curated = %v, want the untouched recommended set", agentNames(curated))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("skipping the prompt should be silent, got warnings: %v", result.Warnings)
	}
}

func TestStdioIsTerminal_WhenRunningUnderGoTest_ShouldReportFalse(t *testing.T) {
	// This is the property the curation gate depends on: `go test` attaches
	// /dev/null to stdin (a character device, which is why a stdin-only probe
	// is not enough) and a pipe to stdout.
	if stdioIsTerminal() {
		t.Error("stdioIsTerminal() reported a terminal under go test; agent curation would prompt in CI")
	}
}

func TestCurateAgents_WhenPreviousRunAutoAccepted_ShouldNotPrompt(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := SaveAgentPreferences(workspace, &AgentSelectionPreferences{AutoAcceptRecommended: true}); err != nil {
		t.Fatalf("SaveAgentPreferences: %v", err)
	}

	// An empty script would produce EOF if the prompt were reached.
	ini := &Initializer{config: InitConfig{
		Workspace:     workspace,
		Interactive:   true,
		InteractiveIO: scriptedInteractiveConfig(t, ""),
	}}

	result := &InitResult{}
	curated := ini.curateAgents(context.Background(), []RecommendedAgent{{Name: "GoExpert"}, {Name: "RedisExpert"}}, ProjectProfile{Language: "go"}, result)
	if len(curated) != 2 {
		t.Fatalf("curated = %v, want the recommended set auto-accepted", agentNames(curated))
	}
	if len(result.Warnings) != 0 {
		t.Errorf("auto-accept should not surface warnings: %v", result.Warnings)
	}
}

// A broken or closed stdin must cost the user a warning, not their workspace.
func TestCurateAgents_WhenInputEndsImmediately_ShouldDegradeToRecommended(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	ini := &Initializer{config: InitConfig{
		Workspace:     workspace,
		Interactive:   true,
		InteractiveIO: scriptedInteractiveConfig(t, ""),
	}}

	result := &InitResult{}
	curated := ini.curateAgents(context.Background(), []RecommendedAgent{{Name: "GoExpert"}}, ProjectProfile{Language: "go"}, result)
	if len(curated) != 1 {
		t.Fatalf("curated = %v, want the recommended set preserved on read failure", agentNames(curated))
	}
	if len(result.Warnings) == 0 {
		t.Error("a failed prompt must be reported as a warning")
	}
}
