package chat

import (
	"codenerd/internal/config"
	"codenerd/internal/perception"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func requireLiveLLMClient(t *testing.T) perception.LLMClient {
	t.Helper()

	if os.Getenv("CODENERD_LIVE_LLM") != "1" {
		t.Skip("skipping live LLM test: set CODENERD_LIVE_LLM=1 to enable")
	}

	configPath := config.DefaultUserConfigPath()
	cfg, err := config.LoadUserConfig(configPath)
	if err != nil {
		t.Skipf("skipping live LLM test: load config %s: %v", configPath, err)
	}

	apiKey := cfg.ZAIAPIKey
	if apiKey == "" && cfg.Provider == "zai" && cfg.APIKey != "" {
		apiKey = cfg.APIKey
	}
	if apiKey == "" && cfg.Provider == "" && cfg.APIKey != "" {
		apiKey = cfg.APIKey
	}
	if apiKey == "" {
		t.Skipf("skipping live LLM test: zai_api_key not configured in %s", configPath)
	}

	client := perception.NewZAIClient(apiKey)
	if cfg.Provider == "zai" && cfg.Model != "" {
		client.SetModel(cfg.Model)
	}
	return client
}

func TestAnalyzeNorthstarDocs_LiveLLM(t *testing.T) {
	client := requireLiveLLMClient(t)

	dir := t.TempDir()
	docPath := filepath.Join(dir, "doc.md")
	content := strings.Join([]string{
		"Problem: builds take 20 minutes and block developer flow.",
		"Target users: QA engineers and backend developers.",
		"Capabilities: fast incremental builds, offline mode, Windows support.",
		"Constraints: must run on Windows, minimal setup.",
		"Risks: dependency security scanning is required.",
	}, "\n")
	if err := os.WriteFile(docPath, []byte(content), 0644); err != nil {
		t.Fatalf("write doc: %v", err)
	}

	m := Model{client: client}
	msg := m.analyzeNorthstarDocs([]string{docPath})()
	result, ok := msg.(northstarDocsAnalyzedMsg)
	if !ok {
		t.Fatalf("expected northstarDocsAnalyzedMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("LLM analysis failed: %v", result.err)
	}
	if len(result.facts) < 2 {
		t.Fatalf("expected at least 2 insights, got %d", len(result.facts))
	}
	for i, fact := range result.facts {
		if strings.TrimSpace(fact) == "" {
			t.Fatalf("insight %d is empty", i)
		}
	}
}

func TestGenerateRequirementsWithLLM_LiveLLM(t *testing.T) {
	client := requireLiveLLMClient(t)

	m := Model{
		client: client,
		northstarWizard: &NorthstarWizardState{
			Mission: "Deliver fast, offline-first developer tooling.",
			Problem: "Local builds are slow and disrupt flow.",
			Vision:  "Instant builds for Windows-first teams.",
			Capabilities: []Capability{
				{Description: "Incremental builds under 30 seconds", Timeline: "now", Priority: "critical"},
				{Description: "Offline mode for travel", Timeline: "6mo", Priority: "high"},
			},
			Risks: []Risk{
				{Description: "Dependency vulnerabilities", Likelihood: "medium", Impact: "high", Mitigation: "automatic scanning"},
			},
			Personas: []UserPersona{
				{Name: "QA engineer", Needs: []string{"fast test cycles", "reliable builds"}},
			},
		},
	}

	msg := m.generateRequirementsWithLLM()()
	result, ok := msg.(requirementsGeneratedMsg)
	if !ok {
		t.Fatalf("expected requirementsGeneratedMsg, got %T", msg)
	}
	if result.err != nil {
		t.Fatalf("LLM requirements failed: %v", result.err)
	}
	if len(result.requirements) == 0 {
		t.Fatalf("expected at least 1 requirement, got %d", len(result.requirements))
	}
	if len(result.requirements) > 15 {
		t.Fatalf("expected at most 15 requirements, got %d", len(result.requirements))
	}

	seen := make(map[string]struct{})
	for i, req := range result.requirements {
		if req.ID == "" || !strings.HasPrefix(req.ID, "REQ-") {
			t.Fatalf("requirement %d has invalid ID %q", i, req.ID)
		}
		if _, ok := seen[req.ID]; ok {
			t.Fatalf("duplicate requirement ID %q", req.ID)
		}
		seen[req.ID] = struct{}{}

		if strings.TrimSpace(req.Description) == "" {
			t.Fatalf("requirement %d has empty description", i)
		}

		switch req.Type {
		case "functional", "non-functional", "constraint":
		default:
			t.Fatalf("requirement %d has unexpected type %q", i, req.Type)
		}

		switch req.Priority {
		case "must-have", "should-have", "nice-to-have":
		default:
			t.Fatalf("requirement %d has unexpected priority %q", i, req.Priority)
		}
	}
}
