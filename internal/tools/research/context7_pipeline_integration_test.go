package research_test

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/knowledge"
	"codenerd/internal/prompt"
	"codenerd/internal/store"
	"codenerd/internal/system"
	"codenerd/internal/tools"
	"codenerd/internal/tools/research"
)

func TestContext7MasterDocumentationReachesProductionExpertPrompt(t *testing.T) {
	// Keep the full factory and compiler, but make this fixture independent of
	// ambient credentials, providers and local embedding models.
	for _, name := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "XAI_API_KEY", "ZAI_API_KEY", "OPENROUTER_API_KEY"} {
		t.Setenv(name, "")
	}
	cfg := config.DefaultUserConfig()
	cfg.Provider = ""
	cfg.APIKey, cfg.AnthropicAPIKey, cfg.OpenAIAPIKey, cfg.GeminiAPIKey = "", "", "", ""
	cfg.XAIAPIKey, cfg.ZAIAPIKey, cfg.OpenRouterAPIKey = "", "", ""
	cfg.Embedding = &config.EmbeddingConfig{Provider: "none"}
	cortex, err := system.BootCortexWithConfig(t.Context(), system.BootConfig{
		Workspace: t.TempDir(), UserConfigOverride: cfg,
		DisableSystemShards: []string{"constitution_gate", "perception_firewall", "executive_policy", "session_planner", "tactile_router", "campaign_runner", "mangle_repair", "legislator"},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cortex.Close() })
	compiler := cortex.JITCompiler
	if compiler == nil {
		t.Fatal("factory omitted production JIT compiler")
	}
	const marker = "zephyrquark calibration master-ref-acquisition-witness"
	const manifestLink = "https://github.com/fixtureowner/fixturerepo/blob/master/docs/intro.md"
	sourceURL := research.PinContext7MasterForIntegration(t, "# Calibration\n"+marker+". Keep the module documentation on its discovered ref.")
	registry := tools.NewRegistry()
	if err := registry.Register(research.Context7Tool()); err != nil {
		t.Fatal(err)
	}
	// This fixture grants only the named source operation. It exercises the
	// normal registry boundary without changing any production permission rule.
	registry.SetAllowlist(&tools.Allowlist{Enforced: true, Names: []string{"context7_fetch"}})
	registry.SetWriteGuard(func(_ context.Context, name string, _ map[string]any) error {
		if name != "context7_fetch" {
			return fmt.Errorf("unexpected fixture tool %q", name)
		}
		return nil
	})
	kbPath := filepath.Join(t.TempDir(), "refexpert.db")
	kb, err := store.NewLocalStore(kbPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = kb.Close() })
	fetchCalls := 0
	acquirer := knowledge.NewAcquirer(knowledge.Config{Context7: func(ctx context.Context, topic string) (string, error) {
		fetchCalls++
		result, err := registry.Execute(ctx, "context7_fetch", map[string]any{"topic": topic})
		if err != nil {
			return "", err
		}
		return result.Result, result.Error
	}})
	added, outcomes, err := knowledge.Fill(t.Context(), kb, acquirer, []knowledge.Subject{{Agent: "refexpert", Topic: "fixtureowner/fixturerepo"}})
	if err != nil || added == 0 || fetchCalls != 1 {
		t.Fatalf("Fill added=%d calls=%d outcomes=%+v err=%v", added, fetchCalls, outcomes, err)
	}
	acquired := false
	for _, outcome := range outcomes {
		acquired = acquired || outcome.Source == "context7" && outcome.Status == knowledge.StatusAcquired
	}
	if !acquired {
		t.Fatalf("Context7 acquired outcome absent: %+v", outcomes)
	}
	atoms, err := kb.GetAllKnowledgeAtoms()
	if err != nil {
		t.Fatal(err)
	}
	var acquiredContent []string
	for _, atom := range atoms {
		acquiredContent = append(acquiredContent, atom.Content)
	}
	stored := strings.Join(acquiredContent, "\n")
	if !strings.Contains(stored, marker) || !strings.Contains(stored, sourceURL) || !strings.Contains(stored, manifestLink) {
		t.Fatalf("acquisition dropped master body/provenance: %+v", atoms)
	}
	cc := prompt.NewCompilationContext().WithShard("refexpert", "refexpert", "").
		WithSemanticQuery("zephyrquark calibration fixtureowner fixturerepo", 5).WithTokenBudget(20000, 1000)
	before, err := compiler.Compile(t.Context(), cc)
	if err != nil || strings.Contains(before.Prompt, marker) {
		t.Fatalf("before registration err=%v, source fabricated=%v", err, before != nil && strings.Contains(before.Prompt, marker))
	}
	// Give the compiler its own handle; the acquisition store retains ownership
	// of the handle used by Fill.
	db, err := sql.Open("sqlite3", kbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := compiler.RegisterShardDB("refexpert", db); err != nil {
		_ = db.Close()
		t.Fatal(err)
	}
	after, err := compiler.Compile(t.Context(), cc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(after.Prompt, marker) || !strings.Contains(after.Prompt, sourceURL) || !strings.Contains(after.Prompt, manifestLink) {
		t.Fatal("production compiler dropped acquired master body or provenance")
	}
	sibling, err := compiler.Compile(t.Context(), prompt.NewCompilationContext().WithShard("sibling", "sibling", "").
		WithSemanticQuery("zephyrquark calibration fixtureowner fixturerepo", 5).WithTokenBudget(20000, 1000))
	if err != nil || strings.Contains(sibling.Prompt, marker) {
		t.Fatalf("sibling err=%v, source leaked=%v", err, sibling != nil && strings.Contains(sibling.Prompt, marker))
	}
}
