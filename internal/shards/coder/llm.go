package coder

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// =============================================================================
// LLM INTERACTION
// =============================================================================

// llmCompleteWithRetry calls LLM with exponential backoff retry logic.
func (c *CoderShard) llmCompleteWithRetry(ctx context.Context, systemPrompt, userPrompt string, maxRetries int) (string, error) {
	if c.llmClient == nil {
		return "", fmt.Errorf("no LLM client configured")
	}

	var lastErr error
	baseDelay := 500 * time.Millisecond

	for attempt := 0; attempt < maxRetries; attempt++ {
		// Add attempt number to context if retrying
		if attempt > 0 {
			fmt.Printf("[CoderShard:%s] LLM retry attempt %d/%d\n", c.id, attempt+1, maxRetries)

			// Exponential backoff: 500ms, 1s, 2s
			delay := baseDelay * time.Duration(1<<uint(attempt))
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("context cancelled during retry backoff: %w", ctx.Err())
			case <-time.After(delay):
			}
		}

		fmt.Println("DEBUG: llm.go Calling client.CompleteWithSystem")
		response, err := c.llmClient.CompleteWithSystem(ctx, systemPrompt, userPrompt)
		if err == nil {
			return response, nil
		}

		lastErr = err

		// Check if error is retryable
		if !isRetryableError(err) {
			return "", fmt.Errorf("non-retryable error: %w", err)
		}
	}

	return "", fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// isRetryableError determines if an error should be retried.
func isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	// Network errors - retryable
	retryablePatterns := []string{
		"timeout",
		"connection",
		"network",
		"temporary",
		"rate limit",
		"503",
		"502",
		"429",
		"context deadline exceeded",
	}

	for _, pattern := range retryablePatterns {
		if strings.Contains(errStr, pattern) {
			return true
		}
	}

	// Auth errors - not retryable
	nonRetryablePatterns := []string{
		"unauthorized",
		"forbidden",
		"invalid api key",
		"401",
		"403",
	}

	for _, pattern := range nonRetryablePatterns {
		if strings.Contains(errStr, pattern) {
			return false
		}
	}

	// Default: retry
	return true
}
