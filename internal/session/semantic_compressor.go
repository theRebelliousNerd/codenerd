package session

import (
	"context"
	"fmt"
	"strings"

	"codenerd/internal/broker"
	"codenerd/internal/logging"
	"codenerd/internal/perception"
	"codenerd/internal/types"
)

// SemanticCompressor implements the Compressor interface using an LLM.
type SemanticCompressor struct {
	client types.LLMClient
}

// NewSemanticCompressor creates a new SemanticCompressor.
func NewSemanticCompressor(client types.LLMClient) *SemanticCompressor {
	return &SemanticCompressor{
		client: client,
	}
}

// Compress summarizes a list of conversation turns into a single string.
func (sc *SemanticCompressor) Compress(ctx context.Context, turns []perception.ConversationTurn) (string, error) {
	if len(turns) == 0 {
		return "", nil
	}

	logging.SessionDebug("Compressing %d turns via SemanticCompressor", len(turns))

	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context cancelled before compression: %w", err)
	}

	var sb strings.Builder
	estimatedSize := min(len(turns)*100,
		// Cap initial allocation
		10*1024*1024)
	sb.Grow(estimatedSize)

	// This cap used to be `const maxTokens = 64000 // approx chars limit` — a
	// number that named tokens, measured characters, was known to no other part
	// of the system, and was chosen by nobody still available to ask. It now
	// derives from the one ledger, converted through the one learned ratio.
	maxChars := summarizationCharAllowance()
	totalChars := 0

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
		case "":
			role = "Assistant" // Default for empty
		}

		// Clean content (Gap 8 - basic unprintable cleanup)
		content := strings.Map(func(r rune) rune {
			if r < 32 && r != '\n' && r != '\t' && r != '\r' {
				return -1
			}
			return r
		}, turn.Content)

		line := fmt.Sprintf("<turn role=\"%s\">\n%s\n</turn>\n", role, content)

		if totalChars+len(line) > maxChars {
			sb.WriteString("\n[... CONVERSATION TRUNCATED DUE TO LENGTH ...]\n")
			break
		}
		sb.WriteString(line)
		totalChars += len(line)
	}

	if sb.Len() == 0 {
		return "", nil
	}

	prompt := fmt.Sprintf(`Summarize the following conversation history into a concise context string.
Retain key decisions, facts, user preferences, and the current state of the task.
Discard small talk and redundant clarifications.

<conversation>
%s
</conversation>

Please provide only the summary text without any surrounding formatting.`, sb.String())

	// Use a system prompt to enforce the role
	systemPrompt := "You are a context compressor. Your job is to summarize conversation history to retain memory for an AI agent."

	summary, err := sc.client.CompleteWithSystem(ctx, systemPrompt, prompt)
	if err != nil {
		return "", fmt.Errorf("semantic compression failed: %w", err)
	}

	return strings.TrimSpace(summary), nil
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
