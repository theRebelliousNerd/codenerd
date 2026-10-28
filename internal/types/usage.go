package types

// UsageMetadata is the normalized usage report of one provider attempt.
//
// It is the single shape every provider adapter reduces its native usage block
// to, and the same value reaches every consumer: the usage tracker's aggregates,
// the broker's per-call receipt (through usage.Observer), and the terminal
// result of a streamed completion. Two consumers fed from two different
// projections of one report is how a streamed turn's cache breakdown used to
// reach usage.json and vanish from the receipt.
//
// Category semantics are disjoint-by-inclusion and fixed here, not per
// provider:
//
//   - InputTokens is the whole prompt, INCLUDING cache reads and writes.
//     A provider that bills them apart (Anthropic's input_tokens is only the
//     uncached remainder) folds them back in its adapter.
//   - CachedContentTokens (cache read) and CacheWriteTokens are sub-counts of
//     InputTokens; 0 <= read + write <= input.
//   - ThinkingTokens (reasoning) is a sub-count of OutputTokens. A provider
//     that reports reasoning beside its output count (Gemini's
//     thoughtsTokenCount) folds it into OutputTokens in its adapter.
//
// So uncached input is InputTokens - CachedContentTokens - CacheWriteTokens and
// the billed total is InputTokens + OutputTokens; no sub-count is ever added
// again.
type UsageMetadata struct {
	InputTokens         int `json:"input_tokens"`
	OutputTokens        int `json:"output_tokens"`
	TotalTokens         int `json:"total_tokens"`
	ThinkingTokens      int `json:"thinking_tokens,omitempty"`       // Subset of OutputTokens used for reasoning
	CachedContentTokens int `json:"cached_content_tokens,omitempty"` // Subset of InputTokens read from a prompt cache
	// CacheWriteTokens is the part of InputTokens written to a prompt cache on
	// this request (Anthropic's cache_creation_input_tokens, billed at a
	// premium). Zero on providers that cache implicitly.
	CacheWriteTokens int `json:"cache_write_tokens,omitempty"`

	// Known names the categories the provider actually reported. A zero count
	// whose bit is clear is "not reported", not "observed zero": a provider
	// that never reports cache traffic must not be read as a cache miss, and
	// a policy that sees a missing field must not conclude the call was cheap.
	Known UsageFields `json:"known,omitempty"`
	// Partial marks counts the adapter could not finish observing — a stream
	// canceled or cut before the provider's final usage block. The counts are
	// lower bounds, never a complete bill.
	Partial bool `json:"partial,omitempty"`
}

// UsageFields is a set of usage categories a provider reported.
type UsageFields uint8

const (
	// UsageTokens: input and output totals.
	UsageTokens UsageFields = 1 << iota
	// UsageCacheRead: CachedContentTokens.
	UsageCacheRead
	// UsageCacheWrite: CacheWriteTokens.
	UsageCacheWrite
	// UsageReasoning: ThinkingTokens.
	UsageReasoning
)

// Has reports whether every category in f was reported.
func (u UsageMetadata) Has(f UsageFields) bool { return u.Known&f == f }

// Reported reports whether the provider supplied token totals at all.
func (u UsageMetadata) Reported() bool { return u.Has(UsageTokens) }

// Plus is the usage of two billed attempts that served one logical call. Every
// count adds; a category is known only when both reports knew it, because a sum
// with one unknown term is a lower bound, not a count; and partiality spreads.
func (u UsageMetadata) Plus(v UsageMetadata) UsageMetadata {
	return UsageMetadata{
		InputTokens:         u.InputTokens + v.InputTokens,
		OutputTokens:        u.OutputTokens + v.OutputTokens,
		TotalTokens:         u.TotalTokens + v.TotalTokens,
		ThinkingTokens:      u.ThinkingTokens + v.ThinkingTokens,
		CachedContentTokens: u.CachedContentTokens + v.CachedContentTokens,
		CacheWriteTokens:    u.CacheWriteTokens + v.CacheWriteTokens,
		Known:               u.Known & v.Known,
		Partial:             u.Partial || v.Partial,
	}
}

// SameCounts reports whether two reports carry the same counts for every
// category both of them know. It is the consistency check between the two
// routes one bill can arrive by (the usage observer and a response-carried
// block): agreement is one bill, disagreement is evidence, never a second
// charge.
func (u UsageMetadata) SameCounts(v UsageMetadata) bool {
	both := u.Known & v.Known
	if both&UsageTokens != 0 && (u.InputTokens != v.InputTokens || u.OutputTokens != v.OutputTokens) {
		return false
	}
	if both&UsageCacheRead != 0 && u.CachedContentTokens != v.CachedContentTokens {
		return false
	}
	if both&UsageCacheWrite != 0 && u.CacheWriteTokens != v.CacheWriteTokens {
		return false
	}
	if both&UsageReasoning != 0 && u.ThinkingTokens != v.ThinkingTokens {
		return false
	}
	return true
}
