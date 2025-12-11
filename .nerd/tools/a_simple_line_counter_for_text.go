package tools

import (
	"bufio"
	"context"
	"fmt"
	"strings"
)

const ToolDescriptionASimpleLineCounterForText = "A simple line counter for text files. Counts the number of lines in the provided text string."

// aSimpleLineCounterForText counts the number of lines in the provided text string.
// It returns the count as a string.
func aSimpleLineCounterForText(ctx context.Context, input string) (string, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Handle empty input
	if input == "" {
		return "0", error
	}

	// Use a scanner to count lines efficiently
	scanner := bufio.NewScanner(strings.NewReader(input))
	lineCount := 0
	for scanner.Scan() {
		lineCount++
	}

	// Check for any scanning errors
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error scanning text: %w", err)
	}

	return fmt.Sprintf("%d", lineCount), nil
}