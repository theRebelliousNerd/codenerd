package xaioauth

import (
	"context"

	"codenerd/internal/types"
)

// CompleteWithStreaming implements types.LLMClient.
// SuperGrok OAuth streaming is currently non-SSE: it completes once and yields
// the full text as one chunk. CompleteWithSystem has already validated the
// response and reported usage, so the stream's terminal claims no more than
// that call established.
func (c *Client) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, _ bool) (<-chan string, <-chan types.StreamResult) {
	return types.StreamWhole(ctx, func(ctx context.Context) (string, error) {
		return c.CompleteWithSystem(ctx, systemPrompt, userPrompt)
	})
}
