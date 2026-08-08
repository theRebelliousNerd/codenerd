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

func TestNormalizeMetaModel_ForcesContributorTier(t *testing.T) {
	cases := map[string]string{
		"muse-spark-1.2":             metaContributorModel,
		"muse-spark-1.1":             metaContributorModel,
		"muse-spark-1.2-contributor": "muse-spark-1.2-contributor",
		"":                           metaContributorModel,
		"  muse-spark-1.2  ":         metaContributorModel,
	}
	for in, want := range cases {
		if got := normalizeMetaModel(in); got != want {
			t.Errorf("normalizeMetaModel(%q) = %q, want %q", in, got, want)
		}
	}
}

// A future contributor-tier model must pass through untouched rather than
// being pinned to today's version number.
func TestNormalizeMetaModel_AcceptsAnyContributorModel(t *testing.T) {
	if got := normalizeMetaModel("muse-spark-2.0-contributor"); got != "muse-spark-2.0-contributor" {
		t.Errorf("a newer contributor model was rewritten to %q", got)
	}
}

// The client default must already be the contributor tier.
func TestMetaClient_DefaultsToContributor(t *testing.T) {
	c := newTestCompatClient(t, ProviderMeta, "https://api.meta.ai/v1")

	if got := c.ModelForContext(context.Background()); got != metaContributorModel {
		t.Errorf("default model = %q, want %q", got, metaContributorModel)
	}
}

// The per-shard override path is the one that could still smuggle the wrong
// tier through: shard_profiles.<type>.model reaches the client as
// CtxKeyModelName.
func TestMetaClient_OverrideCannotEscapeContributorTier(t *testing.T) {
	c := newTestCompatClient(t, ProviderMeta, "https://api.meta.ai/v1")

	ctx := context.WithValue(context.Background(), types.CtxKeyModelName, "muse-spark-1.2")
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
		ctx := context.WithValue(context.Background(), types.CtxKeyModelName, "some-custom-model")
		if got := c.ModelForContext(ctx); got != "some-custom-model" {
			t.Errorf("%s: model override was rewritten to %q", vendor, got)
		}
	}
}
