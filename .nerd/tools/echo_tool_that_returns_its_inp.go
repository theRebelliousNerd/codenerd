package tools

import (
	"context"
)

// ToolDescriptionEchoToolThatReturnsItsInp describes the echo tool functionality.
const ToolDescriptionEchoToolThatReturnsItsInp = "echo tool that returns its input"

// echoToolThatReturnsItsInp returns the input string unchanged.
// It takes a context for cancellation and a string input, and returns the same string.
func echoToolThatReturnsItsInp(ctx context.Context, input string) (string, error) {
	// Check if context is cancelled before proceeding
	if err := ctx.Err(); err != nil {
		return "", err
	}

	// Return the input string unchanged
	return input, nil
}