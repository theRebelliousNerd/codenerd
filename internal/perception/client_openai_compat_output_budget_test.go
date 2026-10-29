package perception

import "testing"

// A slot that does not configure max_output_tokens must still get the budget the
// model can actually emit. Measured 2026-09-17 in a live chat session: main,
// worker and therefore every shard sat at 16384 on Meta while this project's own
// planner slot gives the same muse-spark model 131072, and a reasoning model
// whose completion budget runs out returns an EMPTY reply rather than a
// truncated one -- step planning failed twice with finish_reason "stop" and no
// content before falling back to a single pass.
func TestNewOpenAICompatClient_UnconfiguredMetaSlotGetsTheModelsOutputBudget(t *testing.T) {
	cfg := DefaultOpenAICompatConfig(ProviderMeta, "k")
	cfg.MaxOutputTokens = 0 // what every slot without an explicit max_output_tokens passes

	c, err := NewOpenAICompatClient(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatClient: %v", err)
	}
	if got := c.maxOutputTokens; got != 131072 {
		t.Errorf("unconfigured Meta slot got max_output_tokens %d, want 131072", got)
	}
}

// A vendor this project has no capability figure for keeps the conservative
// default: the point is to match the model, not to raise every ceiling.
func TestNewOpenAICompatClient_UnconfiguredNonMetaSlotKeepsTheGenericDefault(t *testing.T) {
	cfg := DefaultOpenAICompatConfig(ProviderDashScope, "k")
	cfg.MaxOutputTokens = 0

	c, err := NewOpenAICompatClient(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatClient: %v", err)
	}
	if got := c.maxOutputTokens; got != 16384 {
		t.Errorf("unconfigured DashScope slot got max_output_tokens %d, want 16384", got)
	}
}

// An explicit ceiling is a caller asking for a short answer and must survive:
// the default fills a gap, it does not overrule config. 8192 is above the Meta
// floor so the clamp cannot be what keeps it.
func TestNewOpenAICompatClient_ExplicitMaxOutputTokensIsNotOverruled(t *testing.T) {
	cfg := DefaultOpenAICompatConfig(ProviderMeta, "k")
	cfg.MaxOutputTokens = 8192

	c, err := NewOpenAICompatClient(cfg)
	if err != nil {
		t.Fatalf("NewOpenAICompatClient: %v", err)
	}
	if got := c.maxOutputTokens; got != 8192 {
		t.Errorf("explicit max_output_tokens became %d, want 8192", got)
	}
}
