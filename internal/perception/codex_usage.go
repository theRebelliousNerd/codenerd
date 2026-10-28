package perception

import (
	"context"
	"encoding/json"
	"strings"

	"codenerd/internal/types"
)

// codexTurnUsage is the usage block of a `codex exec --json` turn.completed
// event. As in the OpenAI APIs, the cached and reasoning counts are subsets of
// input and output; the pointers keep a category the CLI did not emit unknown
// instead of an observed zero.
type codexTurnUsage struct {
	InputTokens           int  `json:"input_tokens"`
	CachedInputTokens     *int `json:"cached_input_tokens"`
	CacheWriteInputTokens *int `json:"cache_write_input_tokens"`
	OutputTokens          int  `json:"output_tokens"`
	ReasoningOutputTokens *int `json:"reasoning_output_tokens"`
}

func (u codexTurnUsage) metadata() types.UsageMetadata {
	m := types.UsageMetadata{
		InputTokens:  u.InputTokens,
		OutputTokens: u.OutputTokens,
		TotalTokens:  u.InputTokens + u.OutputTokens,
	}
	if u.InputTokens > 0 || u.OutputTokens > 0 {
		m.Known |= types.UsageTokens
	}
	if u.CachedInputTokens != nil {
		m.CachedContentTokens = *u.CachedInputTokens
		m.Known |= types.UsageCacheRead
	}
	if u.CacheWriteInputTokens != nil {
		m.CacheWriteTokens = *u.CacheWriteInputTokens
		m.Known |= types.UsageCacheWrite
	}
	if u.ReasoningOutputTokens != nil {
		m.ThinkingTokens = *u.ReasoningOutputTokens
		m.Known |= types.UsageReasoning
	}
	return m
}

// codexExecUsage sums the usage of every completed turn in a `codex exec
// --json` stdout stream. ok is false when the stream carried none (a run that
// died before its first turn finished), which the caller must leave unreported
// rather than record as free.
func codexExecUsage(stdout string) (types.UsageMetadata, bool) {
	var total types.UsageMetadata
	found := false
	for line := range strings.SplitSeq(stdout, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "{") {
			continue
		}
		var event struct {
			Type  string          `json:"type"`
			Usage *codexTurnUsage `json:"usage"`
		}
		if err := json.Unmarshal([]byte(line), &event); err != nil || event.Type != "turn.completed" || event.Usage == nil {
			continue
		}
		turn := event.Usage.metadata()
		if !found {
			total, found = turn, true
			continue
		}
		total = total.Plus(turn)
	}
	return total, found
}

// reportCodexUsage delivers a codex run's own token accounting. The CLI bills
// the user's subscription whether or not the run produced an answer, so this
// is called for failed runs too.
func (c *CodexCLIClient) reportCodexUsage(ctx context.Context, model, stdout string) {
	u, ok := codexExecUsage(stdout)
	if !ok {
		return
	}
	if model == "" {
		model = "codex-cli"
	}
	reportUsage(ctx, model, ProviderOpenAI, usageOpChat, u)
}
