package session

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"codenerd/internal/broker"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// SemanticCompressor implements the Compressor interface using an LLM.
type SemanticCompressor struct {
	client types.LLMClient
	// allowance is the per-call input bound in characters. Production reads
	// it from the window ledger (summarizationCharAllowance); tests pin it.
	allowance func() int
}

// NewSemanticCompressor creates a new SemanticCompressor.
func NewSemanticCompressor(client types.LLMClient) *SemanticCompressor {
	return &SemanticCompressor{client: client, allowance: summarizationCharAllowance}
}

const compressorSystemPrompt = "You are a context compressor. Your job is to summarize conversation history to retain memory for an AI agent."

// Compress summarizes a list of conversation turns into a single string.
//
// Every turn reaches a summary. History larger than one call's allowance is
// split into consecutive parts on turn boundaries (a single turn larger than
// the allowance is split inside, rune-safe), each part is summarized, and the
// part summaries are summarized again until one call holds them. This used to
// keep the oldest turns that fit and drop the rest behind a "CONVERSATION
// TRUNCATED" marker: the newest turns, the ones the task state lives in,
// were the ones a long session lost.
func (sc *SemanticCompressor) Compress(ctx context.Context, turns []perception.ConversationTurn) (string, error) {
	if len(turns) == 0 {
		return "", nil
	}
	logging.SessionDebug("Compressing %d turns via SemanticCompressor", len(turns))
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context cancelled before compression: %w", err)
	}
	allowance := summarizationCharAllowance
	if sc.allowance != nil {
		allowance = sc.allowance
	}
	limit := allowance()

	var lines []string
	for _, turn := range turns {
		if strings.TrimSpace(turn.Content) == "" {
			continue // Skip empty turns (Gap 1)
		}
		role := "Assistant"
		switch strings.ToLower(turn.Role) {
		case "user":
			role = "User"
		case "tool":
			role = "Tool"
		case "system":
			role = "System"
		}
		// Clean content (Gap 8 - basic unprintable cleanup)
		content := strings.Map(func(r rune) rune {
			if r < 32 && r != '\n' && r != '\t' && r != '\r' {
				return -1
			}
			return r
		}, turn.Content)
		lines = append(lines, fmt.Sprintf("<turn role=\"%s\">\n%s\n</turn>\n", role, content))
	}
	if len(lines) == 0 {
		return "", nil
	}

	prev := 0
	for {
		parts := packParts(lines, limit)
		if len(parts) == 1 {
			return sc.summarize(ctx, parts[0], "")
		}
		// Each round must shrink the history. Summaries that are no shorter
		// than what they summarize would loop forever.
		if prev != 0 && len(parts) >= prev {
			return "", fmt.Errorf("semantic compression failed: %d part summaries do not fit one call of %d characters", len(parts), limit)
		}
		prev = len(parts)
		summaries := make([]string, 0, len(parts))
		for i, part := range parts {
			if err := ctx.Err(); err != nil {
				return "", fmt.Errorf("context cancelled during compression: %w", err)
			}
			summary, err := sc.summarize(ctx, part, fmt.Sprintf("This is part %d of %d of the history, in order.", i+1, len(parts)))
			if err != nil {
				return "", err
			}
			summaries = append(summaries, fmt.Sprintf("<summary part=\"%d\">\n%s\n</summary>\n", i+1, summary))
		}
		lines = summaries
	}
}

// summarize runs one compression call over text that fits the allowance.
func (sc *SemanticCompressor) summarize(ctx context.Context, text, scope string) (string, error) {
	if scope != "" {
		scope += "\n"
	}
	prompt := fmt.Sprintf(`Summarize the following conversation history into a concise context string.
Retain key decisions, facts, user preferences, and the current state of the task.
Discard small talk and redundant clarifications.
%s
<conversation>
%s
</conversation>

Please provide only the summary text without any surrounding formatting.`, scope, text)
	summary, err := sc.client.CompleteWithSystem(ctx, compressorSystemPrompt, prompt)
	if err != nil {
		return "", fmt.Errorf("semantic compression failed: %w", err)
	}
	return strings.TrimSpace(summary), nil
}

// packParts groups lines, in order, into parts of at most limit characters.
// A line longer than limit is split into limit-sized pieces on rune
// boundaries, so nothing is dropped and no part exceeds the bound.
func packParts(lines []string, limit int) []string {
	if limit <= 0 {
		limit = fallbackSummarizationChars
	}
	var parts []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			parts = append(parts, cur.String())
			cur.Reset()
		}
	}
	for _, line := range lines {
		for len(line) > limit {
			flush()
			cut := limit
			for cut > 0 && !utf8.RuneStart(line[cut]) {
				cut--
			}
			if cut == 0 {
				cut = limit
			}
			parts = append(parts, line[:cut])
			line = line[cut:]
		}
		if cur.Len()+len(line) > limit {
			flush()
		}
		cur.WriteString(line)
	}
	flush()
	return parts
}

// summarizationCharAllowance is how much prior-turn text may be fed to a
// compression call, in characters.
//
// Compression is itself an inference call and is charged to the same window as
// everything else. Half the available window is given to the input so the
// instructions and the summary it produces both have room; the result is
// converted from tokens to characters through the broker's learned ratio rather
// than a constant, so it tracks the actual tokenizer of the actual model.
func summarizationCharAllowance() int {
	available := broker.Default().Ledger().Available()
	if available <= 0 {
		// No configured window. Fall back to a bounded default rather than an
		// unbounded read: an unbounded compression input is how a summarization
		// call becomes more expensive than the history it was meant to shrink.
		return fallbackSummarizationChars
	}

	ratio := broker.Default().TextCounter("").Ratio()
	if ratio <= 0 {
		ratio = broker.DefaultSeedRatio
	}

	chars := int(float64(available/2) * ratio)
	if chars < minSummarizationChars {
		return minSummarizationChars
	}
	if chars > maxSummarizationChars {
		return maxSummarizationChars
	}
	return chars
}

const (
	// fallbackSummarizationChars applies before a window is configured.
	fallbackSummarizationChars = 64000
	// minSummarizationChars keeps compression useful on a very small window: a
	// summary built from two turns is not worth the call that produced it.
	minSummarizationChars = 8000
	// maxSummarizationChars bounds a single compression call on a very large
	// window, where feeding the whole thing in costs more than the compression
	// saves.
	maxSummarizationChars = 400000
)
