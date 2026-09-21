package perception

import (
	"context"
	"testing"

	"codenerd/internal/types"
)

// The defect these guard (F-META-2, reported from live token accounting):
// ~11 million tokens ran through "muse-spark-1.2" when every Meta request is
// supposed to use "muse-spark-1.2-contributor". On a single day's log: 482
// completions on the plain model, zero on the contributor tier.
//
// Nothing failed, which is what made it survive — Meta serves both names and
// the calls succeed either way. The wrong one is a different commercial tier,
// so the only symptom is on the bill.
//
// Two sources agreed on the wrong value: the vendor default in
// openAICompatVendorDefaults, and six separate keys in .nerd/config.json
// (worker, four shard_profiles, default_shard). Normalization lives in the
// client because that is the one place every request's model flows through —
// no config key, wizard choice, per-shard profile or CtxKeyModelName override
// can route Meta traffic off the contributor tier.
//
// 2026-09-21: the guard no longer knows a model name. What it falls back to is
// the contributor model the workspace configured; with none configured nothing
// is permitted to run, and it says so by sending no model at all.

func TestNormalizeMetaModel_FallsBackToTheConfiguredContributorModel(t *testing.T) {
	const configured = metaContributorModel
	cases := map[string]string{
		"muse-spark-1.3":             configured,
		"muse-spark-1.2":             configured,
		"stealth/union-alpha":        configured, // a shard profile naming another vendor's model
		"muse-spark-1.3-contributor": "muse-spark-1.3-contributor",
		"muse-spark-1.2-contributor": "muse-spark-1.2-contributor",
		"":                           configured,
		"  muse-spark-1.3  ":         configured,
	}
	for in, want := range cases {
		if got := normalizeMetaModel(in, configured); got != want {
			t.Errorf("normalizeMetaModel(%q, configured) = %q, want %q", in, got, want)
		}
	}
}

// With no contributor model configured there is nothing permitted to run: the
// guard returns no model rather than inventing one, so the request fails at the
// vendor instead of spending on a tier nobody chose.
func TestNormalizeMetaModel_InventsNothingWhenNoneIsConfigured(t *testing.T) {
	for _, configured := range []string{"", "muse-spark-1.3", "stealth/union-alpha"} {
		for _, requested := range []string{"", "muse-spark-1.2", "gpt-anything"} {
			if got := normalizeMetaModel(requested, configured); got != "" {
				t.Errorf("normalizeMetaModel(%q, %q) = %q, want no model", requested, configured, got)
			}
		}
	}
}

// A future contributor-tier model must pass through untouched rather than
// being pinned to today's version number.
func TestNormalizeMetaModel_AcceptsAnyContributorModel(t *testing.T) {
	if got := normalizeMetaModel("muse-spark-2.0-contributor", ""); got != "muse-spark-2.0-contributor" {
		t.Errorf("a newer contributor model was rewritten to %q", got)
	}
}

// A workspace that configures a Meta model off the contributor tier gets no
// client: there is no permitted model to run and none is substituted.
func TestMetaClient_RefusesAConfiguredModelOffTheContributorTier(t *testing.T) {
	cfg := DefaultOpenAICompatConfig(ProviderMeta, "test-key")
	cfg.Model = "muse-spark-1.3"
	if _, err := NewOpenAICompatClient(cfg); err == nil {
		t.Fatal("a Meta client was built on a non-contributor model")
	}
}

// No vendor default exists: a config that names no model yields a client with
// no model, never one the code chose.
func TestCompatClient_HasNoDefaultModel(t *testing.T) {
	for _, vendor := range []Provider{ProviderMeta, ProviderDashScope, ProviderMoonshot} {
		if got := DefaultOpenAICompatConfig(vendor, "test-key").Model; got != "" {
			t.Errorf("%s: DefaultOpenAICompatConfig invented the model %q", vendor, got)
		}
	}
}

// The configured model is what runs.
func TestMetaClient_RunsTheConfiguredModel(t *testing.T) {
	c := newTestCompatClient(t, ProviderMeta, "https://api.meta.ai/v1")

	if got := c.ModelForContext(context.Background()); got != metaContributorModel {
		t.Errorf("model = %q, want the configured %q", got, metaContributorModel)
	}
}

// The per-shard override path is the one that could still smuggle the wrong
// tier through: shard_profiles.<type>.model reaches the client as
// CtxKeyModelName.
func TestMetaClient_OverrideCannotEscapeContributorTier(t *testing.T) {
	c := newTestCompatClient(t, ProviderMeta, "https://api.meta.ai/v1")

	ctx := types.WithModelName(context.Background(), "muse-spark-1.2")
	if got := c.ModelForContext(ctx); got != metaContributorModel {
		t.Errorf("a per-shard override routed Meta traffic to %q", got)
	}

	// And the request actually sent must carry it.
	if req := c.buildRequest(ctx, nil, false); req.Model != metaContributorModel {
		t.Errorf("request model = %q, want %q", req.Model, metaContributorModel)
	}
}

// SetModel normalizes on the way in, so GetModel reports what will be sent
// rather than what was asked for — a client that reports one model and sends
// another is its own defect.
func TestMetaClient_SetModelNormalizes(t *testing.T) {
	c := newTestCompatClient(t, ProviderMeta, "https://api.meta.ai/v1")

	c.SetModel("muse-spark-1.2")
	if got := c.GetModel(); got != metaContributorModel {
		t.Errorf("GetModel() = %q after SetModel(plain), want %q", got, metaContributorModel)
	}
}

// The constraint is Meta-specific. Other vendors must be untouched, or this
// becomes a bug for every provider that happens to share the client.
func TestNormalizeModel_LeavesOtherVendorsAlone(t *testing.T) {
	for _, vendor := range []Provider{ProviderDashScope, ProviderMoonshot} {
		c := newTestCompatClient(t, vendor, "https://example.invalid/v1")
		ctx := types.WithModelName(context.Background(), "some-custom-model")
		if got := c.ModelForContext(ctx); got != "some-custom-model" {
			t.Errorf("%s: model override was rewritten to %q", vendor, got)
		}
	}
}
