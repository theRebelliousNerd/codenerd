package session

import (
	"context"
	"fmt"
	"strings"

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

	var sb strings.Builder
	for _, turn := range turns {
		role := "Assistant"
		if turn.Role == "user" {
			role = "User"
		}
		sb.WriteString(fmt.Sprintf("%s: %s\n", role, turn.Content))
	}

	prompt := fmt.Sprintf(`Summarize the following conversation history into a concise context string.
Retain key decisions, facts, user preferences, and the current state of the task.
Discard small talk and redundant clarifications.

Conversation:
%s

Summary:`, sb.String())

	// Use a system prompt to enforce the role
	systemPrompt := "You are a context compressor. Your job is to summarize conversation history to retain memory for an AI agent."

	summary, err := sc.client.CompleteWithSystem(ctx, systemPrompt, prompt)
	if err != nil {
		return "", fmt.Errorf("semantic compression failed: %w", err)
	}

	return strings.TrimSpace(summary), nil
}
