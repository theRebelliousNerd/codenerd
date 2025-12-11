package tools

import (
	"context"
	"fmt"
)

const ToolDescriptionASimpleHelloWorldGreeter = "A simple hello world greeter that takes a name as input and returns a greeting message."

// aSimpleHelloWorldGreeter takes a name as input and returns a greeting message.
// It's a simple tool that demonstrates basic input/output handling.
func aSimpleHelloWorldGreeter(ctx context.Context, input string) (string, error) {
	// Check if context is cancelled
	if err := ctx.Err(); err != nil {
		return "", fmt.Errorf("context cancelled: %w", err)
	}

	// Validate input
	if input == "" {
		return "", fmt.Errorf("input name cannot be empty")
	}

	// Generate greeting
	greeting := fmt.Sprintf("Hello, %s! Welcome to the world.", input)

	return greeting, nil
}