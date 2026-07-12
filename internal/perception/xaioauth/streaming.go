package xaioauth

import (
	"context"
)

// CompleteWithStreaming implements types.LLMClient.
// SuperGrok OAuth streaming is currently non-SSE: it completes once and yields the full text.
// (True SSE can be added later without changing the interface.)
func (c *Client) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, _ bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 1)
	errorChan := make(chan error, 1)

	go func() {
		defer close(contentChan)
		defer close(errorChan)

		res, err := c.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err != nil {
			errorChan <- err
			return
		}
		contentChan <- res
	}()

	return contentChan, errorChan
}
