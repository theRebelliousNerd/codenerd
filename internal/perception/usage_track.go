package perception

import (
	"context"

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

// trackUsage records one billed LLM turn against the tracker carried by ctx.
//
// Call it once per billed request, at the point where the final token counts
// are known — after a non-streaming response parses, or after a stream reaches
// its usage chunk. Calling it per retry attempt is correct (each attempt that
// reached the model is billed); calling it per streamed delta is not.
//
// Doing nothing when ctx carries no tracker is deliberate: clients are also
// constructed in tests and one-shot tools, and metering must never be a reason
// a completion path fails.
func trackUsage(ctx context.Context, model string, provider Provider, input, output int, operation string) {
	if input <= 0 && output <= 0 {
		return
	}
	usage.TrackFromContext(ctx, model, usageProviderID(provider), input, output, operation)
}

// usageOpFor picks the operation id for a request by whether it offered tools.
func usageOpFor(toolCount int) string {
	if toolCount > 0 {
		return usageOpToolGen
	}
	return usageOpChat
}

// geminiOutputTokens folds thinking tokens into the billed output count.
//
// Google reports candidatesTokenCount without thoughtsTokenCount but bills both
// as output. Recording only candidates under-reports a thinking-heavy turn by
// most of its actual cost, which is the majority of Gemini 3 traffic here.
func geminiOutputTokens(candidates, thoughts int) int {
	return candidates + thoughts
}
