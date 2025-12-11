package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

const ToolDescriptionCreateAToolThatValidatesJ = "Validates JSON syntax and returns validation result"

// createAToolThatValidatesJ validates JSON syntax and returns validation result
func createAToolThatValidatesJ(ctx context.Context, input string) (string, error) {
	// Check for context cancellation
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	default:
	}

	// Handle empty input
	if input == "" {
		return "Error: empty input provided", nil
	}

	// Validate JSON syntax
	var js json.RawMessage
	err := json.Unmarshal([]byte(input), &js)
	if err != nil {
		return fmt.Sprintf("Invalid JSON: %v", err), nil
	}

	return "Valid JSON", nil
}