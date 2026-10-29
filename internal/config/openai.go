package config

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
)

// OpenAIProviderConfig is the OpenAI API block ("openai" in config.json).
//
// The OpenAI client speaks the Responses API (POST /v1/responses). Every field
// here is a request option; none names a model. Empty fields are omitted on
// the wire, which leaves the API's own default in force.
type OpenAIProviderConfig struct {
	// EnableWebSearch attaches OpenAI's hosted web_search tool to completions
	// and tool turns. Nil means the default (off): a search is billed per call
	// on top of tokens, and codeNERD has its own research tools. An explicit
	// grounded_web_search call searches regardless.
	EnableWebSearch *bool `json:"enable_web_search,omitempty"`

	// SearchContextSize is how much retrieved page text the search attaches:
	// low, medium, or high. Empty is medium.
	SearchContextSize string `json:"search_context_size,omitempty"`

	// ServiceTier is the processing tier: auto, default, flex, fast, priority
	// (the older name for fast), or ultrafast (GPT-6 Astra, GPT-6.1 Sol).
	// Empty omits it: the project's default tier.
	ServiceTier string `json:"service_tier,omitempty"`

	// Verbosity is text.verbosity: low, medium, or high. Empty omits it.
	Verbosity string `json:"verbosity,omitempty"`

	// ReasoningMode is reasoning.mode: standard or pro (GPT-5.6 and GPT-6
	// families). Pro spends more compute per answer. Empty omits it.
	ReasoningMode string `json:"reasoning_mode,omitempty"`

	// ReasoningContext is reasoning.context: auto, current_turn, or all_turns.
	// The client replays every earlier reasoning item, so all_turns lets the
	// model use them. Empty omits it: the model's default.
	ReasoningContext string `json:"reasoning_context,omitempty"`

	// ReasoningSummary is reasoning.summary: auto, concise, or detailed. When
	// set, streamed summaries reach the thoughts channel. Empty asks for none.
	ReasoningSummary string `json:"reasoning_summary,omitempty"`

	// PromptCacheTTL is prompt_cache_options.ttl on models that take it
	// (GPT-5.6 and later): 30m or 12h. A 12h write bills 2.5x the input rate,
	// a 30m write 1.25x. Empty omits it (the API default, 30m). Earlier models
	// get prompt_cache_retention "24h", where caching costs nothing extra.
	PromptCacheTTL string `json:"prompt_cache_ttl,omitempty"`
}

// openAIVocab is each enumerated field's accepted values, for Check.
var openAIVocab = []struct {
	field  string
	get    func(*OpenAIProviderConfig) string
	values []string
}{
	{"search_context_size", func(o *OpenAIProviderConfig) string { return o.SearchContextSize }, []string{"low", "medium", "high"}},
	{"service_tier", func(o *OpenAIProviderConfig) string { return o.ServiceTier }, []string{"auto", "default", "flex", "fast", "priority", "ultrafast"}},
	{"verbosity", func(o *OpenAIProviderConfig) string { return o.Verbosity }, []string{"low", "medium", "high"}},
	{"reasoning_mode", func(o *OpenAIProviderConfig) string { return o.ReasoningMode }, []string{"standard", "pro"}},
	{"reasoning_context", func(o *OpenAIProviderConfig) string { return o.ReasoningContext }, []string{"auto", "current_turn", "all_turns"}},
	{"reasoning_summary", func(o *OpenAIProviderConfig) string { return o.ReasoningSummary }, []string{"auto", "concise", "detailed"}},
	{"prompt_cache_ttl", func(o *OpenAIProviderConfig) string { return o.PromptCacheTTL }, []string{"30m", "12h"}},
}

// WebSearchEnabled reports the resolved switch. Nil means off.
func (o *OpenAIProviderConfig) WebSearchEnabled() bool {
	return o != nil && o.EnableWebSearch != nil && *o.EnableWebSearch
}

// Check rejects any enumerated field outside the API's vocabulary. Empty is
// the default, never an error.
func (o *OpenAIProviderConfig) Check(path string) []Problem {
	if o == nil {
		return nil
	}
	var out []Problem
	for _, v := range openAIVocab {
		got := strings.TrimSpace(v.get(o))
		if got == "" || containsString(v.values, got) {
			continue
		}
		out = append(out, Problem{
			Severity: SeverityError,
			Path:     path + "." + v.field,
			Message:  fmt.Sprintf("%q is not a %s", got, strings.ReplaceAll(v.field, "_", " ")),
			Fix:      strings.Join(v.values, ", "),
		})
	}
	return out
}

// GetOpenAIConfig returns a copy of the OpenAI block; a nil block is all
// defaults.
func (c *UserConfig) GetOpenAIConfig() *OpenAIProviderConfig {
	if c == nil || c.OpenAI == nil {
		return &OpenAIProviderConfig{}
	}
	cfg := *c.OpenAI
	if cfg.EnableWebSearch != nil {
		v := *cfg.EnableWebSearch
		cfg.EnableWebSearch = &v
	}
	return &cfg
}

func containsString(list []string, s string) bool { return slices.Contains(list, s) }

// =============================================================================
// OPENAI MODEL CAPABILITIES
//
// Detection on a configured model name, never a model choice. The config check
// and the client share these so a setting the API would reject is refused at
// load instead of on the first request.
// =============================================================================

// OpenAIEffortVocabulary is every reasoning.effort value the API defines.
// Which ones a given model accepts is narrower (OpenAIModelEfforts).
var OpenAIEffortVocabulary = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

// OpenAIModelVersion parses "gpt-<major>[.<minor>]..." ("gpt-6.1-sol" is 6.1,
// "gpt-6-astra" is 6.0, "gpt-5.5" is 5.5). ok is false for any other name.
func OpenAIModelVersion(model string) (major, minor int, ok bool) {
	rest, found := strings.CutPrefix(strings.ToLower(strings.TrimSpace(model)), "gpt-")
	if !found {
		return 0, 0, false
	}
	end := strings.IndexFunc(rest, func(r rune) bool { return (r < '0' || r > '9') && r != '.' })
	if end < 0 {
		end = len(rest)
	}
	ver := strings.TrimSuffix(rest[:end], ".")
	if ver == "" {
		return 0, 0, false
	}
	majorStr, minorStr, hasMinor := strings.Cut(ver, ".")
	major, err := strconv.Atoi(majorStr)
	if err != nil {
		return 0, 0, false
	}
	if hasMinor {
		if minor, err = strconv.Atoi(minorStr); err != nil {
			return 0, 0, false
		}
	}
	return major, minor, true
}

// openAIAtLeast reports a GPT model of at least major.minor.
func openAIAtLeast(model string, major, minor int) bool {
	ma, mi, ok := OpenAIModelVersion(model)
	return ok && (ma > major || (ma == major && mi >= minor))
}

// openAIFamily reports model == prefix or a dated/suffixed variant of it.
func openAIFamily(model, prefix string) bool {
	m := strings.ToLower(strings.TrimSpace(model))
	return m == prefix || strings.HasPrefix(m, prefix+"-")
}

// OpenAIModelEfforts is the reasoning efforts a model accepts, per OpenAI's
// model documentation, or nil when codeNERD has no record for the model (the
// vocabulary alone is then checked). GPT-6 Astra and GPT-6.1 Sol reject
// "none" and "minimal"; Terra, Luna and GPT-6 Sol accept "none".
func OpenAIModelEfforts(model string) []string {
	switch {
	case openAIFamily(model, "gpt-6-astra"), openAIFamily(model, "gpt-6.1-sol"):
		return []string{"low", "medium", "high", "xhigh", "max"}
	case openAIFamily(model, "gpt-6.1-terra"), openAIFamily(model, "gpt-6-luna"), openAIFamily(model, "gpt-6-sol"):
		return []string{"none", "low", "medium", "high", "xhigh", "max"}
	}
	return nil
}

// ValidateOpenAIEffort checks an effort against the API vocabulary and, when
// model is named and known, against what that model accepts.
func ValidateOpenAIEffort(model, effort string) error {
	effort = strings.TrimSpace(effort)
	if effort == "" {
		return nil
	}
	if !containsString(OpenAIEffortVocabulary, effort) {
		return fmt.Errorf("OpenAI reasoning_effort must be one of %s", strings.Join(OpenAIEffortVocabulary, ", "))
	}
	if accepted := OpenAIModelEfforts(model); accepted != nil && !containsString(accepted, effort) {
		return fmt.Errorf("%s accepts reasoning_effort %s, not %q", model, strings.Join(accepted, ", "), effort)
	}
	return nil
}

// OpenAIIsReasoningModel reports a model that reasons: GPT-5 and later and the
// o-series. These reject temperature and top_p unless effort is "none".
func OpenAIIsReasoningModel(model string) bool {
	if openAIAtLeast(model, 5, 0) {
		return true
	}
	m := strings.ToLower(strings.TrimSpace(model))
	return len(m) > 1 && m[0] == 'o' && m[1] >= '1' && m[1] <= '9'
}

// OpenAIUsesCacheOptions reports a model that takes prompt_cache_options
// (GPT-5.6 and later) instead of prompt_cache_retention.
func OpenAIUsesCacheOptions(model string) bool {
	return openAIAtLeast(model, 5, 6)
}

// OpenAIMaxOutputTokens is the largest max_output_tokens a model accepts, or 0
// when codeNERD has no record (the client then uses its own default).
func OpenAIMaxOutputTokens(model string) int {
	m := strings.ToLower(strings.TrimSpace(model))
	switch {
	case openAIAtLeast(m, 5, 0):
		return 128000
	case strings.HasPrefix(m, "gpt-4.1"):
		return 32768
	case strings.HasPrefix(m, "gpt-4o"):
		return 16384
	case OpenAIIsReasoningModel(m):
		return 100000
	}
	return 0
}
