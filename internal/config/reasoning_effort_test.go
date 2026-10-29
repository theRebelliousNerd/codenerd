package config

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// Every slot resolves to a concrete effort: its own when config.json sets one,
// else its default. The classifier's default is the lowest its provider
// accepts.
func TestReasoningEffort_EverySlotResolves(t *testing.T) {
	tests := []struct {
		name string
		cfg  *UserConfig
		got  func(*UserConfig) string
		want string
	}{
		{name: "main default", cfg: &UserConfig{}, got: (*UserConfig).GetReasoningEffort, want: DefaultReasoningEffort},
		{name: "main set", cfg: &UserConfig{ReasoningEffort: "xhigh"}, got: (*UserConfig).GetReasoningEffort, want: "xhigh"},
		{name: "nil config", got: (*UserConfig).GetReasoningEffort, want: DefaultReasoningEffort},
		{name: "worker default", cfg: &UserConfig{Worker: &WorkerLLMConfig{Provider: "meta", Model: "m"}},
			got: func(c *UserConfig) string { return c.GetWorkerLLMConfig().ReasoningEffort }, want: DefaultWorkerReasoningEffort},
		{name: "worker set", cfg: &UserConfig{Worker: &WorkerLLMConfig{Provider: "meta", Model: "m", ReasoningEffort: "xhigh"}},
			got: func(c *UserConfig) string { return c.GetWorkerLLMConfig().ReasoningEffort }, want: "xhigh"},
		{name: "planner default", cfg: &UserConfig{Planner: &PlannerLLMConfig{Provider: "anthropic", Model: "claude-opus-5-5"}},
			got: func(c *UserConfig) string { return c.GetPlannerLLMConfig().ReasoningEffort }, want: DefaultPlannerReasoningEffort},
		{name: "classification default on meta", cfg: &UserConfig{},
			got: func(c *UserConfig) string { return c.GetClassificationReasoningEffort("meta") }, want: "minimal"},
		{name: "classification default on anthropic", cfg: &UserConfig{},
			got: func(c *UserConfig) string { return c.GetClassificationReasoningEffort("anthropic") }, want: "low"},
		{name: "classification set wins on every provider", cfg: &UserConfig{ClassificationReasoningEffort: "medium"},
			got: func(c *UserConfig) string { return c.GetClassificationReasoningEffort("meta") }, want: "medium"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.got(tt.cfg); got != tt.want {
				t.Errorf("effort = %q, want %q", got, tt.want)
			}
		})
	}
}

// A profile route takes the effort of the slot whose call it took, that slot's
// provider options when it stays on its provider, and its token ceilings only
// when it also runs its model.
func TestSecondaryLLMConfig_RouteTo(t *testing.T) {
	opts := &AnthropicProviderConfig{Thinking: "adaptive"}
	planner := &SecondaryLLMConfig{Provider: "anthropic", Model: "claude-opus-5-5", ReasoningEffort: "high",
		MaxOutputTokens: 64000, MaxInputTokens: 150000, Anthropic: opts, Endpoint: "https://gateway.example/v1"}
	tests := []struct {
		name    string
		serving *SecondaryLLMConfig
		route   [2]string
		want    SecondaryLLMConfig
	}{
		{name: "same provider and model keeps the slot whole", serving: planner, route: [2]string{"anthropic", "claude-opus-5-5"},
			want: SecondaryLLMConfig{Provider: "anthropic", Model: "claude-opus-5-5", ReasoningEffort: "high", MaxOutputTokens: 64000, MaxInputTokens: 150000, Anthropic: opts, Endpoint: "https://gateway.example/v1"}},
		{name: "same provider, another model keeps options and gateway, not ceilings", serving: planner, route: [2]string{"anthropic", "claude-haiku-5-5"},
			want: SecondaryLLMConfig{Provider: "anthropic", Model: "claude-haiku-5-5", ReasoningEffort: "high", Anthropic: opts, Endpoint: "https://gateway.example/v1"}},
		{name: "another provider keeps only the effort", serving: planner, route: [2]string{"meta", "muse-spark-1.3-contributor"},
			want: SecondaryLLMConfig{Provider: "meta", Model: "muse-spark-1.3-contributor", ReasoningEffort: "high"}},
		{name: "no serving slot takes the worker default", route: [2]string{"meta", "m"},
			want: SecondaryLLMConfig{Provider: "meta", Model: "m", ReasoningEffort: DefaultWorkerReasoningEffort}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.serving.RouteTo(tt.route[0], tt.route[1]); *got != tt.want {
				t.Errorf("RouteTo = %+v, want %+v", *got, tt.want)
			}
		})
	}
}

// A profile's own effort rides the request; one with none attaches nothing,
// so the serving slot's stands.
func TestShardProfileContext_CarriesTheProfileEffort(t *testing.T) {
	if got, ok := types.ReasoningEffortFromContext(ShardProfileContext(context.Background(), ShardProfile{ReasoningEffort: "high"})); !ok || got != "high" {
		t.Errorf("effort on the context = (%q, %v), want (high, true)", got, ok)
	}
	if got, ok := types.ReasoningEffortFromContext(ShardProfileContext(context.Background(), ShardProfile{})); ok {
		t.Errorf("a profile with no effort attached %q", got)
	}
}

// The deployment: Meta main, worker and classifier at xhigh/xhigh/(default
// minimal), an Anthropic Opus planner, and a reviewer profile routed to Opus
// with no effort of its own. It loads; a value a serving provider or model
// cannot take refuses the file at its field.
func TestCheck_ReasoningEfforts(t *testing.T) {
	deployment := func() *UserConfig {
		return &UserConfig{
			Provider: "meta", Model: "muse-spark-1.3-contributor", MetaAPIKey: "mk", AnthropicAPIKey: "ak",
			ReasoningEffort: "xhigh", ClassificationModel: "muse-spark-1.3-contributor",
			Worker:  &WorkerLLMConfig{Provider: "meta", Model: "muse-spark-1.3-contributor", ReasoningEffort: "xhigh"},
			Planner: &PlannerLLMConfig{Provider: "anthropic", Model: "claude-opus-5-5", ReasoningEffort: "high"},
			ShardProfiles: map[string]ShardProfile{
				"reviewer": {Provider: "anthropic", Model: "claude-opus-5-5"},
				"coder":    {},
			},
		}
	}
	tests := []struct {
		name   string
		mutate func(*UserConfig)
		path   string // "" means the file has no error
	}{
		{name: "the deployment loads", mutate: func(*UserConfig) {}},
		{name: "minimal on the Meta main slot", mutate: func(c *UserConfig) {
			c.ReasoningEffort = "minimal"
			delete(c.ShardProfiles, "reviewer")
		}},
		{name: "minimal main inherited by the Anthropic route", mutate: func(c *UserConfig) { c.ReasoningEffort = "minimal" },
			path: "shard_profiles.reviewer.reasoning_effort"},
		{name: "max on the Meta main slot", mutate: func(c *UserConfig) { c.ReasoningEffort = "max" }, path: "reasoning_effort"},
		{name: "a word no provider takes", mutate: func(c *UserConfig) { c.Worker.ReasoningEffort = "turbo" }, path: "worker.reasoning_effort"},
		{name: "minimal on the Anthropic planner", mutate: func(c *UserConfig) { c.Planner.ReasoningEffort = "minimal" }, path: "planner.reasoning_effort"},
		{name: "an effort for a model that takes none", mutate: func(c *UserConfig) {
			c.Planner.Model = "claude-haiku-4-5"
		}, path: "planner.reasoning_effort"},
		{name: "minimal classification on Meta tiers", mutate: func(c *UserConfig) { c.ClassificationReasoningEffort = "minimal" }},
		{name: "max classification on Meta tiers", mutate: func(c *UserConfig) { c.ClassificationReasoningEffort = "max" }, path: "classification_reasoning_effort"},
		{name: "a route inheriting an effort its provider rejects", mutate: func(c *UserConfig) {
			c.Worker.ReasoningEffort = "minimal"
		}, path: "shard_profiles.reviewer.reasoning_effort"},
		{name: "the route's own effort fixes it", mutate: func(c *UserConfig) {
			c.Worker.ReasoningEffort = "minimal"
			p := c.ShardProfiles["reviewer"]
			p.ReasoningEffort = "high"
			c.ShardProfiles["reviewer"] = p
		}},
		{name: "a profile effort one serving slot rejects", mutate: func(c *UserConfig) {
			c.ShardProfiles["coder"] = ShardProfile{ReasoningEffort: "max"}
		}, path: "shard_profiles.coder.reasoning_effort"},
		{name: "a profile effort every serving slot takes", mutate: func(c *UserConfig) {
			c.ShardProfiles["coder"] = ShardProfile{ReasoningEffort: "high"}
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := deployment()
			tt.mutate(cfg)
			errs := Errors(cfg.Check(nil))
			if tt.path == "" {
				if len(errs) != 0 {
					t.Fatalf("refused: %v", errs)
				}
				return
			}
			if p := problemAt(errs, SeverityError, tt.path); p == nil {
				t.Fatalf("no error at %s; got %v", tt.path, errs)
			} else if p.Fix == "" {
				t.Errorf("the error at %s names no fix: %+v", tt.path, *p)
			}
		})
	}
}

// `nerd config full` writes every effort down as the value in force: the
// slots' defaults, and the classifier's resolved for the provider that
// classifies.
func TestFullJSON_WritesTheEffortsInForce(t *testing.T) {
	tests := []struct {
		name               string
		raw                string
		wantMain           string
		wantWorker         string
		wantPlanner        string
		wantClassification string
	}{
		{name: "an empty file", raw: `{}`,
			wantMain: DefaultReasoningEffort, wantWorker: DefaultWorkerReasoningEffort, wantPlanner: DefaultPlannerReasoningEffort, wantClassification: "low"},
		{name: "a Meta worker classifies at minimal",
			raw:      `{"provider":"anthropic","model":"claude-sonnet-5-5","reasoning_effort":"high","worker":{"provider":"meta","model":"muse-spark-1.3-contributor"}}`,
			wantMain: "high", wantWorker: DefaultWorkerReasoningEffort, wantPlanner: DefaultPlannerReasoningEffort, wantClassification: "minimal"},
		{name: "set values are kept",
			raw:      `{"provider":"meta","model":"muse-spark-1.3-contributor","classification_reasoning_effort":"low","worker":{"provider":"meta","model":"muse-spark-1.3-contributor","reasoning_effort":"xhigh"}}`,
			wantMain: DefaultReasoningEffort, wantWorker: "xhigh", wantPlanner: DefaultPlannerReasoningEffort, wantClassification: "low"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			full, err := FullJSON([]byte(tt.raw))
			if err != nil {
				t.Fatalf("FullJSON: %v", err)
			}
			var doc struct {
				ReasoningEffort               string `json:"reasoning_effort"`
				ClassificationReasoningEffort string `json:"classification_reasoning_effort"`
				Worker                        struct {
					ReasoningEffort string `json:"reasoning_effort"`
				} `json:"worker"`
				Planner struct {
					ReasoningEffort string `json:"reasoning_effort"`
				} `json:"planner"`
			}
			if err := json.Unmarshal(full, &doc); err != nil {
				t.Fatalf("FullJSON is not JSON: %v", err)
			}
			got := []string{doc.ReasoningEffort, doc.Worker.ReasoningEffort, doc.Planner.ReasoningEffort, doc.ClassificationReasoningEffort}
			want := []string{tt.wantMain, tt.wantWorker, tt.wantPlanner, tt.wantClassification}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("main, worker, planner, classification = %v, want %v", got, want)
			}
		})
	}
}

// The server web tools are opt-in with a 1-20 per-request cap that defaults to
// 5 while the tool is on, and their version follows the model.
func TestAnthropicProviderConfig_WebTools(t *testing.T) {
	bounds := []struct {
		name    string
		cfg     AnthropicProviderConfig
		wantErr bool
	}{
		{name: "unset", cfg: AnthropicProviderConfig{WebSearch: true}},
		{name: "the floor", cfg: AnthropicProviderConfig{WebSearch: true, WebSearchMaxUses: 1}},
		{name: "the ceiling", cfg: AnthropicProviderConfig{WebFetch: true, WebFetchMaxUses: MaxAnthropicWebToolUses}},
		{name: "negative", cfg: AnthropicProviderConfig{WebSearchMaxUses: -1}, wantErr: true},
		{name: "over the ceiling", cfg: AnthropicProviderConfig{WebFetchMaxUses: MaxAnthropicWebToolUses + 1}, wantErr: true},
	}
	for _, tt := range bounds {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.cfg.Validate(); (err != nil) != tt.wantErr {
				t.Errorf("Validate() = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	uses := []struct {
		name               string
		cfg                AnthropicProviderConfig
		wantSearch, wantFe int
	}{
		{name: "off", cfg: AnthropicProviderConfig{WebSearchMaxUses: 9}},
		{name: "on with defaults", cfg: AnthropicProviderConfig{WebSearch: true, WebFetch: true}, wantSearch: DefaultAnthropicWebToolMaxUses, wantFe: DefaultAnthropicWebToolMaxUses},
		{name: "on with caps", cfg: AnthropicProviderConfig{WebSearch: true, WebSearchMaxUses: 3, WebFetch: true, WebFetchMaxUses: 2}, wantSearch: 3, wantFe: 2},
	}
	for _, tt := range uses {
		t.Run(tt.name, func(t *testing.T) {
			if s, f := tt.cfg.WebSearchUses(), tt.cfg.WebFetchUses(); s != tt.wantSearch || f != tt.wantFe {
				t.Errorf("uses = (%d, %d), want (%d, %d)", s, f, tt.wantSearch, tt.wantFe)
			}
		})
	}

	versions := []struct {
		model         string
		search, fetch string
	}{
		{"claude-opus-5-5", "web_search_20260209", "web_fetch_20260209"},
		{"claude-opus-5", "web_search_20260209", "web_fetch_20260209"},
		{"claude-opus-4-8", "web_search_20260209", "web_fetch_20260209"},
		{"claude-opus-4-6-20260115", "web_search_20260209", "web_fetch_20260209"},
		{"claude-sonnet-5-5", "web_search_20260209", "web_fetch_20260209"},
		{"claude-sonnet-4-6", "web_search_20260209", "web_fetch_20260209"},
		{"claude-sonnet-4-5", "web_search_20250305", "web_fetch_20250910"},
		{"claude-haiku-5-5", "web_search_20250305", "web_fetch_20250910"},
		{"claude-opus-4-20250514", "web_search_20250305", "web_fetch_20250910"},
	}
	for _, tt := range versions {
		t.Run(tt.model, func(t *testing.T) {
			if s, f := AnthropicWebToolVersions(tt.model); s != tt.search || f != tt.fetch {
				t.Errorf("AnthropicWebToolVersions(%s) = (%s, %s), want (%s, %s)", tt.model, s, f, tt.search, tt.fetch)
			}
		})
	}
}

// `nerd config full` writes the web-tool caps in force; the tools stay off.
func TestFullJSON_WritesTheWebToolDefaults(t *testing.T) {
	full, err := FullJSON([]byte(`{"provider":"anthropic","model":"claude-opus-5-5","anthropic":{"web_search":true,"web_fetch_max_uses":2}}`))
	if err != nil {
		t.Fatalf("FullJSON: %v", err)
	}
	var doc struct {
		Anthropic AnthropicProviderConfig `json:"anthropic"`
	}
	if err := json.Unmarshal(full, &doc); err != nil {
		t.Fatal(err)
	}
	want := AnthropicProviderConfig{WebSearch: true, WebSearchMaxUses: DefaultAnthropicWebToolMaxUses, WebFetchMaxUses: 2}
	if doc.Anthropic != want {
		t.Errorf("anthropic = %+v, want %+v", doc.Anthropic, want)
	}
}
