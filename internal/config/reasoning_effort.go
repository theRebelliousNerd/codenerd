package config

import (
	"fmt"
	"strings"
)

// Every model request carries a reasoning effort: the one config.json sets for
// its slot, else that slot's default below. The accessors (GetReasoningEffort,
// GetClassificationReasoningEffort, GetWorkerLLMConfig, GetPlannerLLMConfig,
// GetMainLLMConfig, SecondaryLLMConfig.RouteTo) resolve every slot to a
// concrete effort, so no client picks one of its own, and `nerd config full`
// writes the resolved values down (FullJSON). A client sends the effort
// wherever its provider and model accept one (Meta; Anthropic models with
// AnthropicSupportsEffort) and omits the field elsewhere.
const (
	// DefaultReasoningEffort is the main slot's effort.
	DefaultReasoningEffort = "medium"
	// DefaultWorkerReasoningEffort is the bulk worker slot's effort.
	DefaultWorkerReasoningEffort = "medium"
	// DefaultPlannerReasoningEffort is the planner slot's effort: the slot
	// exists for the turns whose quality is the product.
	DefaultPlannerReasoningEffort = "high"
)

// DefaultClassificationReasoningEffort is the intent classifier's effort on
// provider when classification_reasoning_effort is unset: the lowest the
// provider accepts, because classification is a short label on the critical
// path of every turn. Meta takes minimal (probed live 2026-09-03: the same
// label at a third of the wall time); Anthropic's floor is low.
func DefaultClassificationReasoningEffort(provider string) string {
	if strings.EqualFold(strings.TrimSpace(provider), "meta") {
		return "minimal"
	}
	return "low"
}

// ValidateMetaEffort validates a reasoning_effort a Meta slot consumes. Muse
// Spark rejects "none" and "max".
func ValidateMetaEffort(effort string) error {
	switch effort {
	case "", "minimal", "low", "medium", "high", "xhigh":
		return nil
	default:
		return fmt.Errorf("Meta reasoning_effort must be minimal, low, medium, high, or xhigh")
	}
}

// ValidateReasoningEffort judges an effort against the provider and model it
// will be sent to: the provider's vocabulary, and, when model is named, for
// Anthropic whether the model accepts an effort at all
// (AnthropicSupportsEffort) -- an effort written for a model that takes none
// would never be sent, a setting that does nothing. Pass an empty model to
// judge the vocabulary alone, as for an effort a slot inherits rather than
// writes. Providers whose clients send no effort accept anything: the field
// is omitted on their wire.
func ValidateReasoningEffort(provider, model, effort string) error {
	effort = strings.TrimSpace(effort)
	if effort == "" {
		return nil
	}
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "anthropic":
		if err := ValidateAnthropicEffort(effort); err != nil {
			return err
		}
		if model = strings.TrimSpace(model); model != "" && !AnthropicSupportsEffort(model) {
			return fmt.Errorf("%q takes no reasoning effort, so %q would never be sent", model, effort)
		}
		return nil
	case "meta":
		return ValidateMetaEffort(effort)
	case "openai":
		return ValidateOpenAIEffort(model, effort)
	default:
		return nil
	}
}

// GetReasoningEffort is the main slot's effort: reasoning_effort, else
// DefaultReasoningEffort.
func (c *UserConfig) GetReasoningEffort() string {
	if c != nil {
		if effort := strings.TrimSpace(c.ReasoningEffort); effort != "" {
			return effort
		}
	}
	return DefaultReasoningEffort
}

// GetClassificationReasoningEffort is the intent classifier's effort on a
// classifying tier served by provider (the worker's provider, or the main
// provider's fast tier): classification_reasoning_effort, else the lowest
// effort that provider accepts (DefaultClassificationReasoningEffort).
func (c *UserConfig) GetClassificationReasoningEffort(provider string) string {
	if c != nil {
		if effort := strings.TrimSpace(c.ClassificationReasoningEffort); effort != "" {
			return effort
		}
	}
	return DefaultClassificationReasoningEffort(provider)
}

// classificationTiers are the provider/model pairs that may classify a turn:
// the worker's, when a worker is configured (the primary), and the main
// provider's fast tier (classification_model, else the main model).
func (c *UserConfig) classificationTiers() []struct{ provider, model string } {
	var tiers []struct{ provider, model string }
	if worker := c.GetWorkerLLMConfig(); worker != nil {
		tiers = append(tiers, struct{ provider, model string }{worker.Provider, worker.Model})
	}
	model := strings.TrimSpace(c.ClassificationModel)
	if model == "" {
		model = c.Model
	}
	return append(tiers, struct{ provider, model string }{c.Provider, model})
}

// RouteTo is the slot a call this slot would have served runs on when a shard
// profile routes it to its own provider and model: that provider and model,
// with this slot's reasoning effort. A profile's own reasoning_effort rides the
// request (ShardProfileContext) and wins over it. The planner's route of a
// /review keeps the planner's effort; the worker's route of a coding step
// keeps the worker's.
//
// This slot's provider options (anthropic) and gateway (endpoint) carry over
// when the route stays on its provider, and its token ceilings
// (max_input_tokens, max_output_tokens) when it also runs its model: a cap set
// for one model says nothing about another, whose calls keep the session's
// budget. A route to another provider takes that provider's top-level options
// and the main base_url, as any slot does.
func (s *SecondaryLLMConfig) RouteTo(provider, model string) *SecondaryLLMConfig {
	route := &SecondaryLLMConfig{Provider: provider, Model: model, ReasoningEffort: DefaultWorkerReasoningEffort}
	if s == nil {
		return route
	}
	route.ReasoningEffort = s.ReasoningEffort
	if !strings.EqualFold(strings.TrimSpace(s.Provider), strings.TrimSpace(provider)) {
		return route
	}
	route.Anthropic = s.Anthropic
	route.Endpoint = s.Endpoint
	if strings.TrimSpace(s.Model) == strings.TrimSpace(model) {
		route.MaxInputTokens = s.MaxInputTokens
		route.MaxOutputTokens = s.MaxOutputTokens
	}
	return route
}

// GetMainLLMConfig is the main slot in a secondary slot's shape, with its
// effort resolved: what a profile route made from a main-slot call inherits.
func (c *UserConfig) GetMainLLMConfig() *SecondaryLLMConfig {
	if c == nil {
		return &SecondaryLLMConfig{ReasoningEffort: DefaultReasoningEffort}
	}
	return &SecondaryLLMConfig{
		Provider:        strings.ToLower(strings.TrimSpace(c.Provider)),
		Model:           c.Model,
		MaxOutputTokens: c.MaxOutputTokens,
		ReasoningEffort: c.GetReasoningEffort(),
		Anthropic:       c.Anthropic,
	}
}

// servingSlots are the slots a shard's call may be served by, by name: the
// main slot, the worker, and the planner (a reasoning verb's turn), each
// resolved. A profile's effort reaches whichever serves the call.
func (c *UserConfig) servingSlots() []struct {
	name string
	slot *SecondaryLLMConfig
} {
	out := []struct {
		name string
		slot *SecondaryLLMConfig
	}{{"main", c.GetMainLLMConfig()}}
	if worker := c.GetWorkerLLMConfig(); worker != nil {
		out = append(out, struct {
			name string
			slot *SecondaryLLMConfig
		}{"worker", worker})
	}
	if planner := c.GetPlannerLLMConfig(); planner != nil {
		out = append(out, struct {
			name string
			slot *SecondaryLLMConfig
		}{"planner", planner})
	}
	return out
}

// withResolvedDefaults is c with the defaults that are resolved rather than
// static written down as the values in force, for `nerd config full`: every
// reasoning effort -- the main slot's, the classifier's (on the primary
// classifying tier's provider), and the worker's and planner's, whose blocks
// are made when absent so their defaults show -- and every Anthropic block's
// web-tool caps.
func (c *UserConfig) withResolvedDefaults() {
	resolved := c.Anthropic.orZero().withWebToolDefaults()
	c.Anthropic = &resolved
	c.ReasoningEffort = c.GetReasoningEffort()
	tiers := c.classificationTiers()
	c.ClassificationReasoningEffort = c.GetClassificationReasoningEffort(tiers[0].provider)
	if c.Worker == nil {
		c.Worker = &WorkerLLMConfig{}
	}
	if strings.TrimSpace(c.Worker.ReasoningEffort) == "" {
		c.Worker.ReasoningEffort = DefaultWorkerReasoningEffort
	}
	if c.Planner == nil {
		c.Planner = &PlannerLLMConfig{}
	}
	if strings.TrimSpace(c.Planner.ReasoningEffort) == "" {
		c.Planner.ReasoningEffort = DefaultPlannerReasoningEffort
	}
	// A slot's own anthropic block replaces the top-level one, so it is only
	// resolved where the file wrote one: making one here would change what
	// the slot is sent if the document were merged back.
	for _, slot := range []*SecondaryLLMConfig{c.Worker, c.Planner} {
		if slot.Anthropic != nil {
			resolved := slot.Anthropic.withWebToolDefaults()
			slot.Anthropic = &resolved
		}
	}
}

// orZero is the block, or its zero value when absent.
func (c *AnthropicProviderConfig) orZero() AnthropicProviderConfig {
	if c == nil {
		return AnthropicProviderConfig{}
	}
	return *c
}
