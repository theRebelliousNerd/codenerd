package init

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
)

type generationWitnessClient struct {
	stubDistinctLLM
	deadline time.Time
	user     string
	system   string
	calls    int
}

func (c *generationWitnessClient) CompleteWithSystem(ctx context.Context, system, user string) (string, error) {
	c.deadline, _ = ctx.Deadline()
	c.system, c.user = system, user
	c.calls++
	return "Specialist guidance grounded in the requested domain", ctx.Err()
}

func TestAgentGenerationHonorsProviderAndParentDeadlines(t *testing.T) {
	previous := config.GetLLMTimeouts()
	t.Cleanup(func() { config.SetLLMTimeouts(previous) })
	configured := previous
	configured.PerCallTimeout = time.Minute
	config.SetLLMTimeouts(configured)
	c := &generationWitnessClient{}
	i := &Initializer{config: InitConfig{LLMClient: c}}
	t.Cleanup(func() { _ = i.Close() })
	before := time.Now()
	if _, err := i.generateMethodologyContent(context.Background(), RecommendedAgent{Name: "TokenExpert"}); err != nil {
		t.Fatal(err)
	}
	if c.deadline.Before(before.Add(50 * time.Second)) {
		t.Fatalf("provider deadline was shortened: %v", c.deadline.Sub(before))
	}
	if !strings.Contains(c.user, "TokenExpert") || strings.Contains(c.user, "When creating agent knowledge bases") {
		t.Fatal("task must remain separate from compiled system guidance")
	}
	if c.system == "" {
		t.Fatal("missing JIT or declared fallback context")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	parentDeadline, _ := ctx.Deadline()
	if _, err := i.generateDomainContent(ctx, RecommendedAgent{Name: "TokenExpert"}); err != nil {
		t.Fatal(err)
	}
	if c.deadline.After(parentDeadline) {
		t.Fatal("generation extended caller deadline")
	}
	cancel()
	if _, err := i.generateDomainContent(ctx, RecommendedAgent{Name: "TokenExpert"}); err == nil {
		t.Fatal("canceled generation succeeded")
	}
	if c.calls != 2 {
		t.Fatalf("canceled operation reached provider: %d calls", c.calls)
	}
}

func TestAgentInitializationPreservesCuratedPromptsWithoutKnowledgeDB(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".nerd", "agents", "tokenexpert", "prompts.yaml")
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	original := buildPromptsYAML("tokenexpert", "TokenExpert", "efficiency", "    - accounting", "accounting", "curated methodology witness", "curated domain witness")
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	c := &generationWitnessClient{}
	i := &Initializer{config: InitConfig{Workspace: root, LLMClient: c}}
	if err := i.generateAgentPromptsYAMLWithContext(context.Background(), RecommendedAgent{Name: "TokenExpert"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != original || c.calls != 0 {
		t.Fatal("initialization replaced curated atoms or paid to regenerate them")
	}
}
