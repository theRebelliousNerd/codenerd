package perception

import (
	"context"

	"codenerd/internal/types"
	"codenerd/internal/usage"
)

// Operation ids recorded on the usage tracker's ByOperation dimension. They are
// a closed vocabulary on purpose: an operation string invented per call site
// fragments the breakdown into rows nobody can compare.
const (
	usageOpChat    = "chat"            // plain completion, streaming or not
	usageOpToolGen = "tool_gen"        // completion that carried tool definitions
	usageOpSearch  = "grounded_search" // provider-side web search turn
)

// canonicalProviderIDs is the set of provider strings the usage tracker may see
// on its ByProvider dimension.
//
// These are exactly the ids config uses to name an engine
// (UserConfig.GetActiveProvider / SetAPIKeyForProvider). Metering that invents
// its own spelling — "z.ai", "google", "OpenAI" — splits one provider's spend
// across several rows and makes the breakdown unreconcilable with the config
// that chose the provider in the first place. Enforced by
// TestCanonicalProviderIDs_WhenComparedToConfig_ShouldBeAccepted.
var canonicalProviderIDs = map[Provider]bool{
	ProviderZAI:        true,
	ProviderAnthropic:  true,
	ProviderOpenAI:     true,
	ProviderGemini:     true,
	ProviderXAI:        true,
	ProviderOpenRouter: true,
	ProviderOllama:     true,
	ProviderDashScope:  true,
	ProviderMeta:       true,
	ProviderMoonshot:   true,
}

// usageProviderID renders p as the provider id recorded by the tracker. An
// unknown provider is still recorded (losing the tokens would be worse than
// recording them under an odd key) but is prefixed so it is obvious in the
// breakdown that a producer is using a non-canonical id.
func usageProviderID(p Provider) string {
	if canonicalProviderIDs[p] {
		return string(p)
	}
	if p == "" {
		return "unknown"
	}
	return "unregistered:" + string(p)
}

// reportUsage delivers one billed attempt's normalized usage to the tracker
// and to the in-flight observer carried by ctx (usage.RecordFromContext), the
// same report to both.
//
// Call it once per billed attempt, at the point where the final counts are
// known — after a non-streaming response parses, or when a stream ends (and
// BEFORE its terminal result is published, so a broker settling on that
// terminal has already seen the report). Calling it per retry attempt is
// correct (each attempt that reached the model is billed); calling it per
// streamed delta is not.
//
// A report with neither counts nor a partial flag is dropped: nothing was
// observed. A partial report with zero counts is still delivered so the
// receipt can say the accounting is incomplete rather than free.
//
// Doing nothing when ctx carries no tracker is deliberate: clients are also
// constructed in tests and one-shot tools, and metering must never be a reason
// a completion path fails.
func reportUsage(ctx context.Context, model string, provider Provider, operation string, u types.UsageMetadata) {
	if u.InputTokens <= 0 && u.OutputTokens <= 0 && !u.Partial {
		return
	}
	usage.RecordFromContext(ctx, usage.Report{
		Model:     model,
		Provider:  usageProviderID(provider),
		Operation: operation,
		Usage:     u,
	})
}

// usageOpFor picks the operation id for a request by whether it offered tools.
func usageOpFor(toolCount int) string {
	if toolCount > 0 {
		return usageOpToolGen
	}
	return usageOpChat
}

// chatUsage is the Chat Completions usage block every OpenAI-shaped vendor
// returns (OpenAI, OpenRouter, xAI, Z.AI, DashScope, Moonshot, Meta's chat
// surface). The detail blocks are pointers so a category the vendor did not
// report stays unknown instead of reading as an observed zero.
type chatUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
	// PromptTokensDetails.CachedTokens is the cache-read subset of
	// PromptTokens.
	PromptTokensDetails *struct {
		CachedTokens *int `json:"cached_tokens"`
	} `json:"prompt_tokens_details,omitempty"`
	// CompletionTokensDetails.ReasoningTokens is the reasoning subset of
	// CompletionTokens.
	CompletionTokensDetails *struct {
		ReasoningTokens *int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details,omitempty"`
}

// metadata normalizes the block. Both sub-counts are already inside their
// totals in this API, so nothing is folded.
func (u chatUsage) metadata() types.UsageMetadata {
	m := types.UsageMetadata{
		InputTokens:  u.PromptTokens,
		OutputTokens: u.CompletionTokens,
		TotalTokens:  u.PromptTokens + u.CompletionTokens,
	}
	if u.PromptTokens > 0 || u.CompletionTokens > 0 {
		m.Known |= types.UsageTokens
	}
	if u.PromptTokensDetails != nil && u.PromptTokensDetails.CachedTokens != nil {
		m.CachedContentTokens = *u.PromptTokensDetails.CachedTokens
		m.Known |= types.UsageCacheRead
	}
	if u.CompletionTokensDetails != nil && u.CompletionTokensDetails.ReasoningTokens != nil {
		m.ThinkingTokens = *u.CompletionTokensDetails.ReasoningTokens
		m.Known |= types.UsageReasoning
	}
	return m
}

// geminiUsage is Gemini's usageMetadata block.
//
// Google reports candidatesTokenCount WITHOUT thoughtsTokenCount but bills both
// as output, so the normalized output folds them together and keeps thoughts
// as the reasoning sub-count. Recording only candidates under-reported a
// thinking-heavy turn by most of its cost, which is the majority of Gemini 3
// traffic here. cachedContentTokenCount is already inside promptTokenCount.
type geminiUsage struct {
	PromptTokenCount        int  `json:"promptTokenCount"`
	CandidatesTokenCount    int  `json:"candidatesTokenCount"`
	TotalTokenCount         int  `json:"totalTokenCount"`
	ThoughtsTokenCount      *int `json:"thoughtsTokenCount,omitempty"`
	CachedContentTokenCount *int `json:"cachedContentTokenCount,omitempty"`
}

// thoughts is the reasoning count, 0 when unreported.
func (u geminiUsage) thoughts() int {
	if u.ThoughtsTokenCount != nil {
		return *u.ThoughtsTokenCount
	}
	return 0
}

// cached is the cache-read count, 0 when unreported.
func (u geminiUsage) cached() int {
	if u.CachedContentTokenCount != nil {
		return *u.CachedContentTokenCount
	}
	return 0
}

// outputTokens is the billed output: candidates plus thoughts.
func (u geminiUsage) outputTokens() int { return u.CandidatesTokenCount + u.thoughts() }

// metadata normalizes the block.
func (u geminiUsage) metadata() types.UsageMetadata {
	out := u.outputTokens()
	m := types.UsageMetadata{
		InputTokens:  u.PromptTokenCount,
		OutputTokens: out,
		TotalTokens:  u.PromptTokenCount + out,
	}
	if u.PromptTokenCount > 0 || out > 0 {
		m.Known |= types.UsageTokens
	}
	if u.ThoughtsTokenCount != nil {
		m.ThinkingTokens = *u.ThoughtsTokenCount
		m.Known |= types.UsageReasoning
	}
	if u.CachedContentTokenCount != nil {
		m.CachedContentTokens = *u.CachedContentTokenCount
		m.Known |= types.UsageCacheRead
	}
	return m
}
