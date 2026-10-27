package config

import (
	"fmt"
	"strconv"
	"strings"
)

// AnthropicAPIVersion is shared by inference and broker token counting.
// https://platform.claude.com/docs/en/api/versioning
const AnthropicAPIVersion = "2026-10-21"

// AnthropicProviderConfig configures the direct Messages API. Empty thinking
// settings preserve the model's defaults, including always-on adaptive thinking
// on current models.
//
// Anthropic's server-side web tools are opt-in here (web_search, web_fetch):
// the endpoint runs them inside the request and returns their results and
// citations with the reply, which the client records as grounding sources.
// They ride the client's non-streaming requests (tool loops, completions,
// structured output); a streamed reply is not grounded. Every other tool --
// anything that touches the workspace -- is still codeNERD's own, executed
// through the governed executor, which never executes a server_tool_use.
type AnthropicProviderConfig struct {
	WorkspaceID          string `json:"workspace_id,omitempty"`
	Thinking             string `json:"thinking,omitempty"`               // adaptive, enabled (legacy), disabled, between_tools
	ThinkingBudgetTokens int    `json:"thinking_budget_tokens,omitempty"` // only with enabled
	ThinkingDisplay      string `json:"thinking_display,omitempty"`       // summarized or omitted
	StrictTools          bool   `json:"strict_tools,omitempty"`

	// WebSearch declares the server web_search tool on the slot's requests.
	// Off by default: a search is billed per use ($10 per 1,000).
	WebSearch bool `json:"web_search,omitempty"`
	// WebFetch declares the server web_fetch tool, which fetches a URL the
	// conversation names. Off by default.
	WebFetch bool `json:"web_fetch,omitempty"`
	// WebSearchMaxUses caps searches per request, 1-20; unset is
	// DefaultAnthropicWebToolMaxUses while web_search is on.
	WebSearchMaxUses int `json:"web_search_max_uses,omitempty"`
	// WebFetchMaxUses caps fetches per request, 1-20; unset is
	// DefaultAnthropicWebToolMaxUses while web_fetch is on.
	WebFetchMaxUses int `json:"web_fetch_max_uses,omitempty"`
}

// DefaultAnthropicWebToolMaxUses is the per-request cap on a server web tool
// whose max_uses the config leaves unset.
const DefaultAnthropicWebToolMaxUses = 5

// MaxAnthropicWebToolUses is the largest per-request cap the config accepts.
const MaxAnthropicWebToolUses = 20

// WebSearchUses is the max_uses sent with the web_search tool: 0 while the
// tool is off, else the configured cap or the default.
func (c AnthropicProviderConfig) WebSearchUses() int {
	return webToolUses(c.WebSearch, c.WebSearchMaxUses)
}

// WebFetchUses is the max_uses sent with the web_fetch tool: 0 while the tool
// is off, else the configured cap or the default.
func (c AnthropicProviderConfig) WebFetchUses() int {
	return webToolUses(c.WebFetch, c.WebFetchMaxUses)
}

func webToolUses(on bool, configured int) int {
	switch {
	case !on:
		return 0
	case configured > 0:
		return configured
	default:
		return DefaultAnthropicWebToolMaxUses
	}
}

// withWebToolDefaults is c with each unset max_uses written as its default,
// for `nerd config full`.
func (c AnthropicProviderConfig) withWebToolDefaults() AnthropicProviderConfig {
	if c.WebSearchMaxUses == 0 {
		c.WebSearchMaxUses = DefaultAnthropicWebToolMaxUses
	}
	if c.WebFetchMaxUses == 0 {
		c.WebFetchMaxUses = DefaultAnthropicWebToolMaxUses
	}
	return c
}

// Validate checks model-independent settings. The provider remains authoritative
// for model-specific capabilities; no configured model is silently substituted.
func (c AnthropicProviderConfig) Validate() error {
	switch c.Thinking {
	case "", "adaptive", "enabled", "disabled", "between_tools":
	default:
		return fmt.Errorf("anthropic.thinking must be adaptive, enabled, disabled, or between_tools")
	}
	if c.Thinking == "enabled" {
		if c.ThinkingBudgetTokens < 1024 {
			return fmt.Errorf("anthropic.thinking_budget_tokens must be at least 1024 with enabled thinking")
		}
	} else if c.ThinkingBudgetTokens != 0 {
		return fmt.Errorf("anthropic.thinking_budget_tokens requires thinking=enabled")
	}
	if c.ThinkingDisplay != "" && c.ThinkingDisplay != "summarized" && c.ThinkingDisplay != "omitted" {
		return fmt.Errorf("anthropic.thinking_display must be summarized or omitted")
	}
	if c.ThinkingDisplay != "" && c.Thinking != "adaptive" && c.Thinking != "enabled" {
		return fmt.Errorf("anthropic.thinking_display requires thinking=adaptive or enabled")
	}
	for _, uses := range []struct {
		field string
		value int
	}{{"web_search_max_uses", c.WebSearchMaxUses}, {"web_fetch_max_uses", c.WebFetchMaxUses}} {
		if uses.value < 0 || uses.value > MaxAnthropicWebToolUses {
			return fmt.Errorf("anthropic.%s must be 1-%d (omit it for %d), got %d", uses.field, MaxAnthropicWebToolUses, DefaultAnthropicWebToolMaxUses, uses.value)
		}
	}
	return nil
}

// ValidateAnthropicEffort validates the shared reasoning_effort knob when an
// Anthropic slot consumes it. Availability of xhigh/max depends on the model.
func ValidateAnthropicEffort(effort string) error {
	switch effort {
	case "", "low", "medium", "high", "xhigh", "max":
		return nil
	default:
		return fmt.Errorf("Anthropic reasoning_effort must be low, medium, high, xhigh, or max")
	}
}

// AnthropicSupportsAdaptive reports whether a model runs adaptive thinking:
// every Claude 5+ family model, Opus and Sonnet from 4.6, and the Mythos
// preview. Date-suffixed 4.x ids (claude-opus-4-20250514) do not.
//
// This and the other Anthropic model-capability functions live here, beside
// the config they qualify, so the load-time check and the client decide a
// model's capabilities by one table. They read the configured model name and
// answer what it can do; none of them picks or substitutes a model.
func AnthropicSupportsAdaptive(model string) bool {
	for _, family := range []string{"opus", "sonnet", "haiku", "fable", "mythos"} {
		prefix := "claude-" + family + "-"
		if !strings.HasPrefix(model, prefix) {
			continue
		}
		version := strings.Split(strings.TrimPrefix(model, prefix), "-")
		major, _ := strconv.Atoi(version[0])
		if major >= 5 {
			return true
		}
		if major == 4 && len(version) > 1 && (family == "opus" || family == "sonnet") {
			minor, _ := strconv.Atoi(version[1])
			return minor >= 6 && minor < 100
		}
	}
	return model == "claude-mythos-preview"
}

// AnthropicSupportsStructuredOutput reports whether a model takes native JSON
// outputs (output_config.format). The legacy models that predate them do not;
// custom model names and proxy aliases are allowed, and the endpoint validates
// their actual capabilities.
func AnthropicSupportsStructuredOutput(model string) bool {
	return !strings.HasPrefix(model, "claude-3") && model != "claude-opus-4" && model != "claude-sonnet-4" &&
		!strings.HasPrefix(model, "claude-opus-4-2025") && !strings.HasPrefix(model, "claude-sonnet-4-2025") && !strings.HasPrefix(model, "claude-opus-4-1")
}

// AnthropicWebToolVersions is the pair of server web-tool versions a model
// takes. Opus 5.5, 5, 4.8, 4.7 and 4.6 and Sonnet 5.5, 5 and 4.6 take the
// dynamic-filtering versions (web_search_20260209, web_fetch_20260209), which
// run code execution internally to filter results, so it is never declared
// beside them; every other model, Haiku 5.5 included, takes the basic ones.
func AnthropicWebToolVersions(model string) (search, fetch string) {
	if anthropicDynamicWebTools(model) {
		return "web_search_20260209", "web_fetch_20260209"
	}
	return "web_search_20250305", "web_fetch_20250910"
}

// anthropicDynamicWebTools reports whether model is one of the versions that
// take the dynamic-filtering web tools. A date suffix (claude-opus-4-6-20260115)
// is not a minor version.
func anthropicDynamicWebTools(model string) bool {
	versions := map[string][]string{
		"opus":   {"5.5", "5.0", "4.8", "4.7", "4.6"},
		"sonnet": {"5.5", "5.0", "4.6"},
	}
	for family, accepted := range versions {
		prefix := "claude-" + family + "-"
		if !strings.HasPrefix(model, prefix) {
			continue
		}
		parts := strings.Split(strings.TrimPrefix(model, prefix), "-")
		major, err := strconv.Atoi(parts[0])
		if err != nil {
			return false
		}
		minor := 0
		if len(parts) > 1 {
			if m, err := strconv.Atoi(parts[1]); err == nil && m < 100 {
				minor = m
			}
		}
		version := strconv.Itoa(major) + "." + strconv.Itoa(minor)
		for _, v := range accepted {
			if v == version {
				return true
			}
		}
		return false
	}
	return false
}

// AnthropicSupportsEffort reports whether a model accepts
// output_config.effort: the adaptive models and Opus 4.5. A model that does
// not is never sent one, so a reasoning_effort written for it would do
// nothing; the config check refuses that.
func AnthropicSupportsEffort(model string) bool {
	return AnthropicSupportsAdaptive(model) || strings.HasPrefix(model, "claude-opus-4-5")
}
